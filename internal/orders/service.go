package orders

import (
	"YunoChallenge/internal"
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

type (
	// OrderRepository defines database interactions.
	OrderRepository interface {
		CreateOrder(ctx context.Context, order *internal.Order) error
		GetOrderByID(ctx context.Context, id string) (*internal.Order, error)
		GetActiveOrders(ctx context.Context) ([]internal.Order, error)
		Update(ctx context.Context, order internal.Order) error
	}

	// OrderService manages orders with a single queue and fairness.
	OrderService struct {
		repo          OrderRepository
		vipOrders     map[string]internal.Order
		regularOrders map[string]internal.Order
		mu            sync.RWMutex // Read/Write mutex for the map
		eventChan     chan Event   // Event channel
	}
)

// New creates a new OrderService with an empty queue.
func NewService(orderRepository OrderRepository) *OrderService {
	s := &OrderService{
		repo:          orderRepository,
		vipOrders:     make(map[string]internal.Order),
		regularOrders: make(map[string]internal.Order),
		eventChan:     make(chan Event),
	}

	// initialize the service with the active orders from the database
	go s.loadInitialOrders(context.Background())

	// Start a goroutine to process the events
	go s.processEvents()

	return s
}

// loadInitialOrders carga los pedidos activos desde la base de datos.
func (s *OrderService) loadInitialOrders(ctx context.Context) {
	orders, err := s.repo.GetActiveOrders(ctx)
	if err != nil {
		fmt.Printf("Error loading initial orders: %v\n", err)
		return
	}
	s.mu.Lock()
	for _, order := range orders {
		if *order.VIP {
			s.vipOrders[order.ID] = order
		} else {
			s.regularOrders[order.ID] = order
		}
	}
	s.mu.Unlock()
}

// processEvents listen for events on the channels and processes them according to the event type.
func (s *OrderService) processEvents() {
	for event := range s.eventChan {
		switch event.Type {
		case EventAdd:
			if err := s.repo.CreateOrder(context.Background(), &event.Order); err != nil {
				fmt.Printf("Error guardando pedido: %v\n", err)
				continue
			}
			s.mu.Lock()
			if *event.Order.VIP {
				s.vipOrders[event.Order.ID] = event.Order
			} else {
				s.regularOrders[event.Order.ID] = event.Order
			}
			s.mu.Unlock()

		case EventUpdate:
			if err := s.repo.Update(context.Background(), event.Order); err != nil {
				fmt.Printf("Error actualizando pedido: %v\n", err)
				continue
			}
			s.mu.Lock()
			if *event.Order.VIP {
				s.vipOrders[event.Order.ID] = event.Order
			} else {
				s.regularOrders[event.Order.ID] = event.Order
			}
			s.mu.Unlock()

		case EventCancel:
			s.mu.Lock()
			delete(s.vipOrders, event.ID)
			delete(s.regularOrders, event.ID)
			s.mu.Unlock()

		case EventNotify:
			fmt.Printf("NOTIFICACIÓN: Pedido %s está listo para entrega (Fuente: %s, VIP: %v)\n",
				event.Order.ID, event.Order.Source, *event.Order.VIP)

		case EventShutdown:
			return
		}
	}
}

// AddOrder sends an order to the add channel.
func (s *OrderService) AddOrder(ctx context.Context, order internal.Order) error {
	order.ArrivalTime = time.Now()
	order.Status = internal.StatusPending

	if !isValidSource(internal.OrderSource(order.Source)) {
		return internal.ErrInvalidOrderSource
	}

	select {
	case s.eventChan <- Event{Type: EventAdd, Order: order}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("failed to add order")
	}
}

// GetOrderByID returns an order by its ID.
func (s *OrderService) OrderByID(ctx context.Context, id string) (*internal.Order, error) {
	s.mu.RLock()
	if regularOrder, regularExists := s.regularOrders[id]; regularExists {
		s.mu.RUnlock()
		return &regularOrder, nil
	}

	if vipOrder, vipExists := s.vipOrders[id]; vipExists {
		s.mu.RUnlock()
		return &vipOrder, nil
	}

	s.mu.RUnlock()

	// If the order is not in memory, fetch it from the database
	order, err := s.repo.GetOrderByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get order from database: %w", err)
	}

	if order == nil {
		return nil, internal.ErrOrderNotFound
	}

	return order, nil
}

// UpdateOrder updates an order.
func (s *OrderService) UpdateOrder(ctx context.Context, id string, order internal.Order) error {
	s.mu.RLock()
	vipOrder, vipExists := s.vipOrders[id]
	regularOrder, regularExists := s.regularOrders[id]
	s.mu.RUnlock()

	var existingOrder internal.Order
	if vipExists {
		existingOrder = vipOrder
	} else if regularExists {
		existingOrder = regularOrder
	} else {
		return internal.ErrOrderNotFound
	}

	if order.Dishes != nil {
		existingOrder.Dishes = order.Dishes
	}

	if order.Source != "" {
		existingOrder.Source = order.Source
	}

	if order.VIP != nil {
		existingOrder.VIP = order.VIP
	}

	if order.Status != "" && isValidStatus(order.Status) {
		if !isValidTransition(existingOrder.Status, order.Status) {
			return internal.ErrOrderInvalidStatusTransition
		}

		existingOrder.Status = order.Status

		if order.Status == internal.StatusReady {
			existingOrder.PreparationTime = time.Since(existingOrder.ArrivalTime)
			select {
			case s.eventChan <- Event{Type: EventNotify, Order: existingOrder}:
			default:
				fmt.Printf("Failed to notify order %s\n", id)
			}
		}

		if order.Status == internal.StatusCancelled || order.Status == internal.StatusDelivered {
			err := s.CancelOrder(ctx, id)
			if err != nil {
				return err
			}
			return nil
		}
	}

	select {
	case s.eventChan <- Event{Type: EventUpdate, Order: existingOrder}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("failed to update order %s", id)
	}
}

// CancelOrder cancels an order.
func (s *OrderService) CancelOrder(ctx context.Context, id string) error {
	select {
	case s.eventChan <- Event{Type: EventCancel, ID: id}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("cola de eventos llena")
	}
}

// GetActiveOrders returns all the active orders sorted by VIP and arrival time.
func (s *OrderService) ActiveOrders() []internal.Order {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var vipActive []internal.Order
	var regularActive []internal.Order
	for _, order := range s.vipOrders {
		if order.Status != internal.StatusDelivered && order.Status != internal.StatusCancelled {
			vipActive = append(vipActive, order)
		}
	}
	for _, order := range s.regularOrders {
		if order.Status != internal.StatusDelivered && order.Status != internal.StatusCancelled {
			regularActive = append(regularActive, order)
		}
	}

	// Order each queue by arrival time
	sort.Slice(vipActive, func(i, j int) bool {
		return vipActive[i].ArrivalTime.Before(vipActive[j].ArrivalTime)
	})
	sort.Slice(regularActive, func(i, j int) bool {
		return regularActive[i].ArrivalTime.Before(regularActive[j].ArrivalTime)
	})

	// 2:1 ratio (2 VIP / 1 no VIP) - With this ratio, we guarantee that VIP orders are processed faster but without causing
	// starvation for the regular orders.
	var result []internal.Order
	vipIndex, regularIndex := 0, 0
	for vipIndex < len(vipActive) || regularIndex < len(regularActive) {
		// Process 2 VIP Orders
		for i := 0; i < 2 && vipIndex < len(vipActive); i++ {
			result = append(result, vipActive[vipIndex])
			vipIndex++
		}
		// Process 1 regular Order
		if regularIndex < len(regularActive) {
			result = append(result, regularActive[regularIndex])
			regularIndex++
		}
	}

	return result
}

// GetStats returns the statistics of the orders, such as the number of total active orders, orders per hour and average preparation time.
func (s *OrderService) Stats(ctx context.Context) (time.Duration, float64, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var totalPreparationTime time.Duration
	var ordersPerHour float64

	totalActiveOrders := len(s.vipOrders) + len(s.regularOrders)

	// Calculate the total preparation time for current active orders and the number of orders per hour
	for _, order := range s.vipOrders {
		if order.Status == internal.StatusReady {
			totalPreparationTime += order.PreparationTime
		}

		if order.ArrivalTime.After(time.Now().Add(-time.Hour)) {
			ordersPerHour++
		}
	}

	for _, order := range s.regularOrders {
		if order.Status == internal.StatusReady {
			totalPreparationTime += order.PreparationTime
		}

		if order.ArrivalTime.After(time.Now().Add(-time.Hour)) {
			ordersPerHour++
		}
	}

	// Calculate the average preparation time
	avgPreparationTime := time.Duration(0)
	if totalActiveOrders > 0 {
		avgPreparationTime = totalPreparationTime / time.Duration(totalActiveOrders)
	}

	return avgPreparationTime, 0, totalActiveOrders
}

// Shutdown closes the service by closing the quit channel.
func (s *OrderService) Shutdown() {
	s.eventChan <- Event{Type: EventShutdown}
	close(s.eventChan)
}

// isValidStatus checks if a status is valid.
func isValidStatus(status internal.OrderStatus) bool {
	switch status {
	case internal.StatusPending, internal.StatusInPreparation, internal.StatusReady,
		internal.StatusDelivered, internal.StatusCancelled:
		return true
	default:
		return false
	}
}

func isValidSource(source internal.OrderSource) bool {
	switch source {
	case internal.SourceInPerson, internal.SourceDelivery, internal.SourcePhone:
		return true
	default:
		return false
	}
}

func isValidTransition(from, to internal.OrderStatus) bool {
	switch from {
	case internal.StatusPending:
		return to == internal.StatusInPreparation || to == internal.StatusCancelled
	case internal.StatusInPreparation:
		return to == internal.StatusReady || to == internal.StatusCancelled
	case internal.StatusReady:
		return to == internal.StatusDelivered || to == internal.StatusCancelled
	default:
		return false
	}
}
