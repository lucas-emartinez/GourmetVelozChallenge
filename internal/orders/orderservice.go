package orders

import (
	"YunoChallenge/internal"
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// OrderRepository defines database interactions.
type (
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
		mu            sync.RWMutex        // Read/Write mutex for the map
		addChan       chan internal.Order // Add order channel
		updateChan    chan internal.Order // Update order channel
		cancelChan    chan string         // Cancel order channel
		quitChan      chan struct{}       // Quit channel
	}
)

// New creates a new OrderService with an empty queue.
func NewService(repo OrderRepository) *OrderService {
	s := &OrderService{
		repo:          repo,
		vipOrders:     make(map[string]internal.Order),
		regularOrders: make(map[string]internal.Order),
		addChan:       make(chan internal.Order, 100), // Buffer para 100 pedidos simultáneos
		updateChan:    make(chan internal.Order, 100),
		cancelChan:    make(chan string, 100),
		quitChan:      make(chan struct{}),
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
	for {
		select {
		case order := <-s.addChan:
			if err := s.repo.CreateOrder(context.Background(), &order); err != nil {
				fmt.Printf("Error saving order: %v\n", err)
				continue
			}
			s.mu.Lock()
			if *order.VIP {
				s.vipOrders[order.ID] = order
			} else {
				s.regularOrders[order.ID] = order
			}
			s.mu.Unlock()

		case order := <-s.updateChan:
			if err := s.repo.Update(context.Background(), order); err != nil {
				fmt.Printf("Error updating order: %v\n", err)
				continue
			}
			s.mu.Lock()
			if *order.VIP {
				s.vipOrders[order.ID] = order
			} else {
				s.regularOrders[order.ID] = order
			}
			s.mu.Unlock()

		case id := <-s.cancelChan:
			s.mu.Lock()
			delete(s.vipOrders, id)
			delete(s.regularOrders, id)
			s.mu.Unlock()

		case <-s.quitChan:
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
	case s.addChan <- order:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("failed to add order")
	}
}

// GetOrderByID returns an order by its ID.
func (s *OrderService) GetOrderByID(ctx context.Context, id string) (*internal.Order, error) {
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
		return fmt.Errorf("order %s not found", id)
	}

	if order.Dishes != nil {
		existingOrder.Dishes = order.Dishes
	}

	if order.Status != "" {
		if !validTransition(existingOrder.Status, order.Status) {
			return fmt.Errorf("invalid status transition from %s to %s", existingOrder.Status, order.Status)
		}

		existingOrder.Status = order.Status

		if order.Status == internal.StatusReady {
			existingOrder.PreparationTime = time.Since(existingOrder.ArrivalTime)
		}

		if order.Status == internal.StatusCancelled || order.Status == internal.StatusDelivered {
			err := s.CancelOrder(ctx, id)
			if err != nil {
				return err
			}
			return nil
		}
	}

	if order.Source != "" {
		existingOrder.Source = order.Source
	}

	if order.VIP != nil {
		existingOrder.VIP = order.VIP
	}

	select {
	case s.updateChan <- existingOrder:
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
	case s.cancelChan <- id:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("failed to cancel order %s", id)
	}
}

// GetActiveOrders returns all the active orders sorted by VIP and arrival time.
func (s *OrderService) GetActiveOrders() []internal.Order {
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

	// Ordenar cada cola por ArrivalTime
	sort.Slice(vipActive, func(i, j int) bool {
		return vipActive[i].ArrivalTime.Before(vipActive[j].ArrivalTime)
	})
	sort.Slice(regularActive, func(i, j int) bool {
		return regularActive[i].ArrivalTime.Before(regularActive[j].ArrivalTime)
	})

	// 2:1 ratio (2 VIP / 1 no VIP)
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

// Shutdown closes the service by closing the quit channel.
func (s *OrderService) Shutdown() {
	close(s.quitChan)
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

func validTransition(from, to internal.OrderStatus) bool {
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
