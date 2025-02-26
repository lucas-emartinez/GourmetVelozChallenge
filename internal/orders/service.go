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
	// OrderRepository es una interfaz que define los métodos de un repositorio
	OrderRepository interface {
		// CreateOrder crea un nuevo pedido
		CreateOrder(ctx context.Context, order *internal.Order) error
		// GetOrderByID consulta un pedido por ID
		GetOrderByID(ctx context.Context, id string) (*internal.Order, error)
		// GetActiveOrders devuelve los pedidos activos ordenados
		GetActiveOrders(ctx context.Context) ([]internal.Order, error)
		// Update actualiza un pedido
		Update(ctx context.Context, order internal.Order) error
		// Stats calcula estadísticas
		Stats(ctx context.Context) (string, float64, int)
	}

	// queueSystem representa el sistema de colas por estado
	queueSystem struct {
		regularStatusQueue map[internal.OrderStatus][]*internal.Order
		vipStatusQueue     map[internal.OrderStatus][]*internal.Order
	}

	OrderService struct {
		repo      OrderRepository
		queue     queueSystem
		orderMap  map[string]internal.Order // Mapa para acceso aleatorio
		mu        sync.RWMutex              // Mutex para concurrencia
		eventChan chan Event                // Canal de eventos
	}
)

// New crea un nuevo OrderService con el sistema de colas por estado
func NewService(orderRepository OrderRepository) *OrderService {
	s := &OrderService{
		repo: orderRepository,
		queue: queueSystem{
			regularStatusQueue: make(map[internal.OrderStatus][]*internal.Order),
			vipStatusQueue:     make(map[internal.OrderStatus][]*internal.Order),
		},
		orderMap:  make(map[string]internal.Order),
		eventChan: make(chan Event, 1000),
	}

	// Inicializar colas para cada estado posible
	for _, status := range []internal.OrderStatus{
		internal.StatusPending, internal.StatusInPreparation, internal.StatusReady,
		internal.StatusDelivered, internal.StatusCancelled,
	} {
		s.queue.regularStatusQueue[status] = make([]*internal.Order, 0)
		s.queue.vipStatusQueue[status] = make([]*internal.Order, 0)
	}

	go s.loadInitialOrders(context.Background())
	go s.processEvents()

	return s
}

// loadInitialOrders carga los pedidos activos desde la base de datos
func (s *OrderService) loadInitialOrders(ctx context.Context) {
	orders, err := s.repo.GetActiveOrders(ctx)
	if err != nil {
		fmt.Printf("Error al cargar pedidos iniciales: %v\n", err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range orders {
		order := &orders[i] // Convertir a puntero
		s.orderMap[order.ID] = *order
		if order.VIP {
			s.queue.vipStatusQueue[order.Status] = append(s.queue.vipStatusQueue[order.Status], order)
		} else {
			s.queue.regularStatusQueue[order.Status] = append(s.queue.regularStatusQueue[order.Status], order)
		}
	}
}

// processEvents procesa los eventos del canal
func (s *OrderService) processEvents() {
	for event := range s.eventChan {
		s.mu.Lock()
		switch event.Type {
		case EventAdd:
			if err := s.repo.CreateOrder(context.Background(), &event.Order); err != nil {
				fmt.Printf("Error guardando pedido: %v\n", err)
				s.mu.Unlock()
				continue
			}
			order := event.Order
			s.orderMap[order.ID] = order
			if order.VIP {
				s.queue.vipStatusQueue[order.Status] = append(s.queue.vipStatusQueue[order.Status], &order)
			} else {
				s.queue.regularStatusQueue[order.Status] = append(s.queue.regularStatusQueue[order.Status], &order)
			}

		case EventUpdate:
			if err := s.repo.Update(context.Background(), event.Order); err != nil {
				fmt.Printf("Error actualizando pedido: %v\n", err)
				s.mu.Unlock()
				continue
			}
			oldOrder, exists := s.orderMap[event.Order.ID]
			if !exists {
				s.mu.Unlock()
				continue
			}
			// Si el estado cambió, mover el pedido a la cola correspondiente
			if oldOrder.Status != event.Order.Status {
				s.removeFromQueue(oldOrder)
				s.orderMap[event.Order.ID] = event.Order
				if event.Order.VIP {
					s.queue.vipStatusQueue[event.Order.Status] = append(s.queue.vipStatusQueue[event.Order.Status], &event.Order)
				} else {
					s.queue.regularStatusQueue[event.Order.Status] = append(s.queue.regularStatusQueue[event.Order.Status], &event.Order)
				}
			} else {
				s.orderMap[event.Order.ID] = event.Order
			}

		case EventCancel:
			if order, exists := s.orderMap[event.ID]; exists {
				s.removeFromQueue(order)
				delete(s.orderMap, event.ID)
			}

		case EventNotify:
			fmt.Printf("NOTIFICACIÓN: Pedido %s está listo (Fuente: %s, VIP: %v)\n",
				event.Order.ID, event.Order.Source, event.Order.VIP)

		case EventShutdown:
			s.mu.Unlock()
			return
		}
		s.mu.Unlock()
	}
}

// removeFromQueue elimina un pedido de su cola correspondiente
func (s *OrderService) removeFromQueue(order internal.Order) {
	queue := s.queue.regularStatusQueue
	if order.VIP {
		queue = s.queue.vipStatusQueue
	}
	for i, o := range queue[order.Status] {
		if o.ID == order.ID {
			queue[order.Status] = append(queue[order.Status][:i], queue[order.Status][i+1:]...)
			break
		}
	}
}

// AddOrder agrega un pedido a la cola
func (s *OrderService) AddOrder(ctx context.Context, order internal.Order) error {
	order.ArrivalTime = time.Now().UTC()
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
		return fmt.Errorf("cola de eventos llena")
	}
}

// OrderByID consulta un pedido por ID
func (s *OrderService) OrderByID(ctx context.Context, id string) (*internal.Order, error) {
	s.mu.RLock()
	if order, exists := s.orderMap[id]; exists {
		s.mu.RUnlock()
		return &order, nil
	}
	s.mu.RUnlock()

	order, err := s.repo.GetOrderByID(ctx, id)
	if err != nil || order == nil {
		return nil, internal.ErrOrderNotFound
	}
	return order, nil
}

// UpdateOrder actualiza un pedido
func (s *OrderService) UpdateOrder(ctx context.Context, id string, order internal.Order) error {
	s.mu.RLock()
	existingOrder, exists := s.orderMap[id]
	s.mu.RUnlock()
	if !exists {
		return internal.ErrOrderNotFound
	}

	if order.Dishes != nil {
		existingOrder.Dishes = order.Dishes
	}
	if order.Source != "" && isValidSource(internal.OrderSource(order.Source)) {
		existingOrder.Source = order.Source
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
				fmt.Printf("No se pudo notificar pedido %s\n", id)
			}
		}
		if order.Status == internal.StatusCancelled || order.Status == internal.StatusDelivered {
			s.DeleteOrder(ctx, id)
		}
	}

	select {
	case s.eventChan <- Event{Type: EventUpdate, Order: existingOrder}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("no se pudo actualizar pedido %s", id)
	}
}

// DeleteOrder elimina un pedido
func (s *OrderService) DeleteOrder(ctx context.Context, id string) error {
	select {
	case s.eventChan <- Event{Type: EventCancel, ID: id}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("cola de eventos llena")
	}
}

// ActiveOrders devuelve los pedidos activos en orden de prioridad
// VIP tienen prioridad sobre regulares, pero no más allá de un umbral seteado de 15 minutos
func (s *OrderService) ActiveOrders() []internal.Order {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	const threshold = 15 * time.Minute
	var orders []internal.Order

	// Pending con justicia
	for _, o := range s.queue.vipStatusQueue[internal.StatusPending] {
		orders = append(orders, *o)
	}
	for _, o := range s.queue.regularStatusQueue[internal.StatusPending] {
		orders = append(orders, *o)
	}
	sort.Slice(orders, func(i, j int) bool {
		oi, oj := orders[i], orders[j]
		if oi.VIP == oj.VIP {
			return oi.ArrivalTime.Before(oj.ArrivalTime)
		}
		return oi.VIP && now.Sub(oj.ArrivalTime) <= now.Sub(oi.ArrivalTime)+threshold
	})

	// Preparing y Ready
	for _, status := range []internal.OrderStatus{internal.StatusInPreparation, internal.StatusReady} {
		for _, o := range s.queue.vipStatusQueue[status] {
			orders = append(orders, *o)
		}
		for _, o := range s.queue.regularStatusQueue[status] {
			orders = append(orders, *o)
		}
	}

	return orders
}

// Stats calcula estadísticas
func (s *OrderService) Stats(ctx context.Context) (string, float64, int) {
	// Recargar pedidos activos desde la base de datos
	return s.repo.Stats(ctx)
}

// Shutdown cierra el servicio
func (s *OrderService) Shutdown() {
	s.eventChan <- Event{Type: EventShutdown}
	close(s.eventChan)
}

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
	if from == to {
		return false
	}
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
