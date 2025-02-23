package orders

import (
	"YunoChallenge/internal"
	"YunoChallenge/internal/repository"
	"context"
	"sync"
)

type (
	OrderRepository interface {
		CreateOrder(ctx context.Context, order internal.Order) error
		GetOrderByID(ctx context.Context, id string) (*internal.Order, error)
		Update(ctx context.Context, order internal.Order) error
		Delete(ctx context.Context, id string) error
	}

	OrderService struct {
		repo      *repository.OrderRepository
		orderChan chan internal.Order
		mu        sync.Mutex
	}
)

func NewOrderService(repo *repository.OrderRepository) *OrderService {
	return &OrderService{
		repo:      repo,
		orderChan: make(chan internal.Order),
	}
}

func (s *OrderService) AddOrder(order internal.Order) error {
	return s.repo.CreateOrder(order)
}

// Implementar otras funciones como GetOrder, UpdateOrderStatus, etc.
