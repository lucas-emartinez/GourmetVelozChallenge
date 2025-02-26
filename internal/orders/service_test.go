package orders

import (
	"YunoChallenge/internal"
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// mockOrderRepository es un mock del OrderRepository para pruebas.
type mockOrderRepository struct {
	orders       map[string]internal.Order
	activeOrders []internal.Order
}

func (m *mockOrderRepository) CreateOrder(ctx context.Context, order *internal.Order) error {
	m.orders[order.ID] = *order
	return nil
}

func (m *mockOrderRepository) GetOrderByID(ctx context.Context, id string) (*internal.Order, error) {
	order, exists := m.orders[id]
	if !exists {
		return nil, nil
	}
	return &order, nil
}

func (m *mockOrderRepository) GetActiveOrders(ctx context.Context) ([]internal.Order, error) {
	return m.activeOrders, nil
}

func (m *mockOrderRepository) Update(ctx context.Context, order internal.Order) error {
	m.orders[order.ID] = order
	return nil
}

func (m *mockOrderRepository) Stats(ctx context.Context) (string, float64, int) {
	var totalPreparationTime time.Duration
	var completedOrders int
	var ordersPerHour float64
	totalActiveOrders := 0
	lastHour := time.Now().Add(-time.Hour)

	for _, order := range m.orders {
		if order.Status != internal.StatusDelivered && order.Status != internal.StatusCancelled {
			totalActiveOrders++
		}
		if order.Status == internal.StatusReady {
			totalPreparationTime += order.PreparationTime
			completedOrders++
		}
		if order.ArrivalTime.After(lastHour) {
			ordersPerHour++
		}
	}

	avgPreparationTime := time.Duration(0)
	if completedOrders > 0 {
		avgPreparationTime = totalPreparationTime / time.Duration(completedOrders)
	}
	return avgPreparationTime.String(), ordersPerHour, totalActiveOrders
}

func TestLoadInitialOrders(t *testing.T) {
	repo := &mockOrderRepository{
		orders: make(map[string]internal.Order),
		activeOrders: []internal.Order{
			{ID: "1", VIP: false, Status: internal.StatusPending},
			{ID: "2", VIP: true, Status: internal.StatusInPreparation},
		},
	}
	service := NewService(repo)
	time.Sleep(100 * time.Millisecond) // Esperar a que loadInitialOrders termine

	service.mu.RLock()
	defer service.mu.RUnlock()
	assert.Equal(t, 1, len(service.queue.regularStatusQueue[internal.StatusPending]))
	assert.Equal(t, 1, len(service.queue.vipStatusQueue[internal.StatusInPreparation]))
	assert.Equal(t, "1", service.queue.regularStatusQueue[internal.StatusPending][0].ID)
	assert.Equal(t, "2", service.queue.vipStatusQueue[internal.StatusInPreparation][0].ID)
}

func TestNewService(t *testing.T) {
	repo := &mockOrderRepository{
		orders: make(map[string]internal.Order),
		activeOrders: []internal.Order{
			{ID: "1", VIP: false, Status: internal.StatusPending},
			{ID: "2", VIP: true, Status: internal.StatusInPreparation},
		},
	}
	service := NewService(repo)
	time.Sleep(100 * time.Millisecond) // Esperar inicialización

	assert.NotNil(t, service.repo)
	assert.NotNil(t, service.eventChan)
	service.mu.RLock()
	defer service.mu.RUnlock()
	assert.Equal(t, 1, len(service.queue.regularStatusQueue[internal.StatusPending]))
	assert.Equal(t, 1, len(service.queue.vipStatusQueue[internal.StatusInPreparation]))
	assert.Equal(t, "1", service.queue.regularStatusQueue[internal.StatusPending][0].ID)
	assert.Equal(t, "2", service.queue.vipStatusQueue[internal.StatusInPreparation][0].ID)
}

func TestAddOrder_ValidOrder(t *testing.T) {
	repo := &mockOrderRepository{orders: make(map[string]internal.Order)}
	service := NewService(repo)
	ctx := context.Background()
	order := internal.Order{
		ID:     "1",
		Dishes: []string{"Pizza"},
		Source: internal.SourceDelivery,
		VIP:    false,
	}

	done := make(chan struct{})
	go func() {
		service.processEvents()
		close(done)
	}()

	err := service.AddOrder(ctx, order)
	assert.NoError(t, err)

	time.Sleep(100 * time.Millisecond) // Esperar a que processEvents termine
	service.Shutdown()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for processEvents")
	}

	service.mu.RLock()
	defer service.mu.RUnlock()
	assert.Equal(t, 1, len(service.queue.regularStatusQueue[internal.StatusPending]))
	assert.Equal(t, "1", service.queue.regularStatusQueue[internal.StatusPending][0].ID)
	assert.Equal(t, internal.StatusPending, service.queue.regularStatusQueue[internal.StatusPending][0].Status)
}

func TestAddOrder_InvalidSource(t *testing.T) {
	repo := &mockOrderRepository{orders: make(map[string]internal.Order)}
	service := NewService(repo)
	ctx := context.Background()
	order := internal.Order{
		ID:     "1",
		Source: "invalid",
		VIP:    false,
	}

	err := service.AddOrder(ctx, order)
	assert.Equal(t, internal.ErrInvalidOrderSource, err)
	service.mu.RLock()
	defer service.mu.RUnlock()
	assert.Equal(t, 0, len(service.queue.regularStatusQueue[internal.StatusPending]))
}

func TestOrderByID_OrderFoundInMemory(t *testing.T) {
	repo := &mockOrderRepository{orders: make(map[string]internal.Order)}
	service := NewService(repo)
	ctx := context.Background()
	order := internal.Order{
		ID:     "1",
		Source: internal.SourceDelivery,
		VIP:    false,
	}
	service.mu.Lock()
	service.orderMap["1"] = order
	service.queue.regularStatusQueue[internal.StatusPending] = append(service.queue.regularStatusQueue[internal.StatusPending], &order)
	service.mu.Unlock()

	result, err := service.OrderByID(ctx, "1")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "1", result.ID)
}

func TestOrderByID_OrderFoundInDB(t *testing.T) {
	repo := &mockOrderRepository{
		orders: map[string]internal.Order{
			"1": {ID: "1", Source: internal.SourceDelivery, VIP: true},
		},
	}
	service := NewService(repo)
	ctx := context.Background()

	result, err := service.OrderByID(ctx, "1")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "1", result.ID)
}

func TestOrderByID_OrderNotFound(t *testing.T) {
	repo := &mockOrderRepository{orders: make(map[string]internal.Order)}
	service := NewService(repo)
	ctx := context.Background()

	result, err := service.OrderByID(ctx, "999")
	assert.Error(t, err)
	assert.Equal(t, internal.ErrOrderNotFound, err)
	assert.Nil(t, result)
}

func TestUpdateOrder_ValidUpdate(t *testing.T) {
	repo := &mockOrderRepository{orders: make(map[string]internal.Order)}
	service := NewService(repo)
	ctx := context.Background()
	order := internal.Order{
		ID:     "1",
		Source: internal.SourceDelivery,
		Status: internal.StatusPending,
		Dishes: []string{"Pizza"},
		VIP:    true,
	}
	service.mu.Lock()
	service.orderMap["1"] = order
	service.queue.vipStatusQueue[internal.StatusPending] = append(service.queue.vipStatusQueue[internal.StatusPending], &order)
	service.mu.Unlock()

	update := internal.Order{
		Status: internal.StatusInPreparation,
		Dishes: []string{"Pizza", "Lata CocaCola 500ml"},
		Source: internal.SourcePhone,
	}

	done := make(chan struct{})
	go func() {
		service.processEvents()
		close(done)
	}()

	err := service.UpdateOrder(ctx, "1", update)
	assert.NoError(t, err)

	service.Shutdown()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for processEvents")
	}

	service.mu.RLock()
	defer service.mu.RUnlock()
	assert.Equal(t, 0, len(service.queue.vipStatusQueue[internal.StatusPending]), "Pending debería estar vacío")
	assert.Equal(t, 1, len(service.queue.vipStatusQueue[internal.StatusInPreparation]), "InPreparation debería tener 1 pedido")
	assert.Equal(t, internal.StatusInPreparation, service.queue.vipStatusQueue[internal.StatusInPreparation][0].Status)
	assert.Equal(t, []string{"Pizza", "Lata CocaCola 500ml"}, service.queue.vipStatusQueue[internal.StatusInPreparation][0].Dishes)
}

func TestUpdateOrder_InvalidTransition(t *testing.T) {
	repo := &mockOrderRepository{orders: make(map[string]internal.Order)}
	service := NewService(repo)
	ctx := context.Background()
	order := internal.Order{
		ID:     "1",
		Source: internal.SourceDelivery,
		Status: internal.StatusPending,
		VIP:    false,
	}
	service.mu.Lock()
	service.orderMap["1"] = order
	service.queue.regularStatusQueue[internal.StatusPending] = append(service.queue.regularStatusQueue[internal.StatusPending], &order)
	service.mu.Unlock()

	update := internal.Order{
		Status: internal.StatusReady,
	}

	err := service.UpdateOrder(ctx, "1", update)
	assert.Equal(t, internal.ErrOrderInvalidStatusTransition, err)
}

func TestUpdateOrder_InvalidTransition_SameStatus(t *testing.T) {
	repo := &mockOrderRepository{orders: make(map[string]internal.Order)}
	service := NewService(repo)
	ctx := context.Background()
	order := internal.Order{
		ID:     "1",
		Source: internal.SourceDelivery,
		Status: internal.StatusPending,
		VIP:    false,
	}
	service.mu.Lock()
	service.orderMap["1"] = order
	service.queue.regularStatusQueue[internal.StatusPending] = append(service.queue.regularStatusQueue[internal.StatusPending], &order)
	service.mu.Unlock()

	update := internal.Order{
		Status: internal.StatusPending,
	}

	err := service.UpdateOrder(ctx, "1", update)
	assert.Equal(t, internal.ErrOrderInvalidStatusTransition, err)
}

func TestCancelOrder(t *testing.T) {
	repo := &mockOrderRepository{orders: make(map[string]internal.Order)}
	service := NewService(repo)
	ctx := context.Background()
	order := internal.Order{
		ID:     "1",
		Source: internal.SourceDelivery,
		Status: internal.StatusPending,
		VIP:    false,
	}
	service.mu.Lock()
	service.orderMap["1"] = order
	service.queue.regularStatusQueue[internal.StatusPending] = append(service.queue.regularStatusQueue[internal.StatusPending], &order)
	service.mu.Unlock()

	done := make(chan struct{})
	go func() {
		service.processEvents()
		close(done)
	}()

	err := service.DeleteOrder(ctx, "1")
	assert.NoError(t, err)

	service.Shutdown()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for processEvents")
	}

	service.mu.RLock()
	defer service.mu.RUnlock()
	assert.Equal(t, 0, len(service.queue.regularStatusQueue[internal.StatusPending]))
	assert.Equal(t, 0, len(service.orderMap))
}

func TestActiveOrders(t *testing.T) {
	repo := &mockOrderRepository{orders: make(map[string]internal.Order)}
	service := NewService(repo)
	now := time.Now().UTC()
	service.mu.Lock()
	service.orderMap["1"] = internal.Order{
		ID:          "1",
		ArrivalTime: now.Add(-10 * time.Minute),
		Status:      internal.StatusPending,
		VIP:         true,
	}
	service.orderMap["2"] = internal.Order{
		ID:          "2",
		ArrivalTime: now.Add(-5 * time.Minute),
		Status:      internal.StatusPending,
		VIP:         false,
	}

	order1 := service.orderMap["1"]
	order2 := service.orderMap["2"]
	service.queue.vipStatusQueue[internal.StatusPending] = append(service.queue.vipStatusQueue[internal.StatusPending], &order1)
	service.queue.regularStatusQueue[internal.StatusPending] = append(service.queue.regularStatusQueue[internal.StatusPending], &order2)
	service.mu.Unlock()

	orders := service.ActiveOrders()
	assert.Equal(t, 2, len(orders))
	assert.Equal(t, "1", orders[0].ID, "VIP debe estar primero por ventaja de tiempo")
	assert.Equal(t, "2", orders[1].ID)
}

func TestStats(t *testing.T) {
	now := time.Now().UTC()
	repo := &mockOrderRepository{
		orders: map[string]internal.Order{
			"1": {ID: "1", Status: internal.StatusReady, PreparationTime: 10 * time.Minute, ArrivalTime: now.Add(-70 * time.Minute), VIP: false},
			"2": {ID: "2", Status: internal.StatusPending, ArrivalTime: now.Add(-10 * time.Minute), VIP: false},
			"3": {ID: "3", Status: internal.StatusPending, ArrivalTime: now.Add(-10 * time.Minute), VIP: true},
			"4": {ID: "4", Status: internal.StatusReady, PreparationTime: 10 * time.Minute, ArrivalTime: now.Add(-10 * time.Minute), VIP: true},
		},
	}
	service := NewService(repo)
	ctx := context.Background()

	avgPrep, ordersPerHour, totalActive := service.Stats(ctx)
	assert.Equal(t, "10m0s", avgPrep)
	assert.Equal(t, float64(3), ordersPerHour)
	assert.Equal(t, 4, totalActive)
}

func TestShutdown(t *testing.T) {
	repo := &mockOrderRepository{orders: make(map[string]internal.Order)}
	service := NewService(repo)
	done := make(chan struct{})

	go func() {
		service.processEvents()
		close(done)
	}()

	service.Shutdown()
	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for processEvents to shutdown")
	}

	_, ok := <-service.eventChan
	assert.False(t, ok, "el canal debería estar cerrado")
}

func TestProcessEvents(t *testing.T) {
	repo := &mockOrderRepository{orders: make(map[string]internal.Order)}
	eventChan := make(chan Event, 1000)
	service := &OrderService{
		repo:      repo,
		queue:     queueSystem{vipStatusQueue: make(map[internal.OrderStatus][]*internal.Order), regularStatusQueue: make(map[internal.OrderStatus][]*internal.Order)},
		orderMap:  make(map[string]internal.Order),
		eventChan: eventChan,
	}
	for _, status := range []internal.OrderStatus{
		internal.StatusPending, internal.StatusInPreparation, internal.StatusReady,
		internal.StatusDelivered, internal.StatusCancelled,
	} {
		service.queue.regularStatusQueue[status] = make([]*internal.Order, 0)
		service.queue.vipStatusQueue[status] = make([]*internal.Order, 0)
	}
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		service.processEvents()
	}()

	events := []Event{
		{Type: EventAdd, Order: internal.Order{ID: "1", Source: internal.SourceDelivery, VIP: true}},
		{Type: EventUpdate, Order: internal.Order{ID: "1", Status: internal.StatusInPreparation, VIP: true}},
		{Type: EventNotify, Order: internal.Order{ID: "1", Status: internal.StatusReady, VIP: true}},
		{Type: EventCancel, ID: "1"},
		{Type: EventShutdown},
	}

	for _, event := range events {
		eventChan <- event
	}
	wg.Wait()

	service.mu.RLock()
	defer service.mu.RUnlock()
	assert.Equal(t, 0, len(service.orderMap), "el pedido debería haberse cancelado")
	assert.Equal(t, 1, len(repo.orders), "el pedido debería estar en el repo con el último estado")
	assert.Equal(t, internal.StatusInPreparation, repo.orders["1"].Status)
}

func TestIsValidSource(t *testing.T) {
	validSources := []internal.OrderSource{internal.SourceInPerson, internal.SourceDelivery, internal.SourcePhone}
	invalidSource := internal.OrderSource("invalid")

	for _, src := range validSources {
		assert.True(t, isValidSource(src), fmt.Sprintf("%s debería ser válido", src))
	}
	assert.False(t, isValidSource(invalidSource), "fuente inválida debería fallar")
}

func TestIsValidStatus(t *testing.T) {
	validStatuses := []internal.OrderStatus{
		internal.StatusPending, internal.StatusInPreparation, internal.StatusReady,
		internal.StatusDelivered, internal.StatusCancelled,
	}
	invalidStatus := internal.OrderStatus("invalid")

	for _, status := range validStatuses {
		assert.True(t, isValidStatus(status), fmt.Sprintf("%s debería ser válido", status))
	}
	assert.False(t, isValidStatus(invalidStatus), "estado inválido debería fallar")
}

func TestIsValidTransition(t *testing.T) {
	tests := []struct {
		from  internal.OrderStatus
		to    internal.OrderStatus
		valid bool
	}{
		{internal.StatusPending, internal.StatusInPreparation, true},
		{internal.StatusPending, internal.StatusCancelled, true},
		{internal.StatusPending, internal.StatusReady, false},
		{internal.StatusInPreparation, internal.StatusReady, true},
		{internal.StatusInPreparation, internal.StatusCancelled, true},
		{internal.StatusReady, internal.StatusDelivered, true},
		{internal.StatusReady, internal.StatusCancelled, true},
		{internal.StatusDelivered, internal.StatusPending, false},
	}

	for _, tt := range tests {
		result := isValidTransition(tt.from, tt.to)
		assert.Equal(t, tt.valid, result, fmt.Sprintf("transición de %s a %s debería ser %v", tt.from, tt.to, tt.valid))
	}
}

func TestCase1_AddOrderConcurrent(t *testing.T) {
	repo := &mockOrderRepository{orders: make(map[string]internal.Order)}
	service := NewService(repo)
	ctx := context.Background()
	wg := sync.WaitGroup{}
	numOrders := 100

	done := make(chan struct{})
	go func() {
		service.processEvents()
		close(done)
	}()

	for i := 0; i < numOrders; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			order := internal.Order{
				ID:     fmt.Sprintf("order-%d", i),
				Dishes: []string{"Pizza"},
				Source: internal.SourceDelivery,
				VIP:    true,
			}
			err := service.AddOrder(ctx, order)
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()
	time.Sleep(100 * time.Millisecond)
	service.Shutdown()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for processEvents")
	}

	service.mu.RLock()
	defer service.mu.RUnlock()
	assert.Equal(t, numOrders, len(service.queue.vipStatusQueue[internal.StatusPending]))
	for i := 0; i < numOrders; i++ {
		id := fmt.Sprintf("order-%d", i)
		assert.Contains(t, service.orderMap, id)
	}
}

func TestCase7_ModifyWhileAdding(t *testing.T) {
	repo := &mockOrderRepository{orders: make(map[string]internal.Order)}
	service := NewService(repo)
	ctx := context.Background()
	wg := sync.WaitGroup{}
	order := internal.Order{
		ID:     "order-1",
		Dishes: []string{"Pizza"},
		Source: internal.SourceInPerson,
		Status: internal.StatusPending,
		VIP:    false,
	}
	service.mu.Lock()
	service.orderMap["order-1"] = order
	service.queue.regularStatusQueue[internal.StatusPending] = append(service.queue.regularStatusQueue[internal.StatusPending], &order)
	service.mu.Unlock()

	done := make(chan struct{})
	go func() {
		service.processEvents()
		close(done)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		update := internal.Order{Dishes: []string{"Burger"}}
		err := service.UpdateOrder(ctx, "order-1", update)
		assert.NoError(t, err)
	}()

	for i := 2; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			order := internal.Order{
				ID:     fmt.Sprintf("order-%d", i),
				Dishes: []string{"Salad"},
				Source: internal.SourceDelivery,
				VIP:    false,
			}
			err := service.AddOrder(ctx, order)
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()
	time.Sleep(100 * time.Millisecond)
	service.Shutdown()

	select {
	case <-done:
		// processEvents terminó correctamente
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for processEvents")
	}

	service.mu.RLock()
	defer service.mu.RUnlock()
	assert.Equal(t, []string{"Burger"}, service.orderMap["order-1"].Dishes)
	assert.Equal(t, 4, len(service.orderMap))
	assert.Equal(t, 4, len(service.queue.regularStatusQueue[internal.StatusPending]))
}
