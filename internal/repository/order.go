package repository

import (
	"YunoChallenge/internal"
	"YunoChallenge/internal/platform/database"
	"strings"
)

type OrderRepository struct {
	db *database.Database
}

func NewOrderRepository(db *database.Database) *OrderRepository {
	return &OrderRepository{db: db}
}

func (r *OrderRepository) CreateOrder(order internal.Order) error {
	query := `INSERT INTO orders (id, arrival_time, dishes, status, source) VALUES (?, ?, ?, ?, ?)`
	_, err := r.db.Exec(query, order.ID, order.ArrivalTime, strings.Join(order.Dishes, ","), order.Status, order.Source)
	return err
}

func (r *OrderRepository) GetOrderByID(id string) (*internal.Order, error) {
	query := `SELECT id, arrival_time, dishes, status, source FROM orders WHERE id = ?`
	row := r.db.QueryRow(query, id)

	var order internal.Order
	var dishes string

	err := row.Scan(&order.ID, &order.ArrivalTime, &dishes, &order.Status, &order.Source)
	if err != nil {
		return nil, err
	}

	order.Dishes = strings.Split(dishes, ",")
	return &order, nil
}

// Implementar otras operaciones como UpdateOrder, DeleteOrder, etc.
