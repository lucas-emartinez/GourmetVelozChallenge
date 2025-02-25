package repository

import (
	"YunoChallenge/internal"
	"YunoChallenge/internal/platform/database"
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// OrderRepository is a struct that contains a reference to the database
type OrderRepository struct {
	db *database.Database
}

// NewOrderRepository creates a new OrderRepository
func NewOrderRepository(db *database.Database) *OrderRepository {
	return &OrderRepository{db: db}
}

// CreateOrder creates a new order
func (r *OrderRepository) CreateOrder(ctx context.Context, order *internal.Order) error {
	query := `INSERT INTO orders (arrival_time, dishes, status, source, vip, preparation_time) VALUES (?, ?, ?, ?, ?, ?)`

	result, err := r.db.ExecContext(
		ctx,
		query,
		order.ArrivalTime,
		strings.Join(order.Dishes, ","),
		order.Status,
		order.Source,
		order.VIP,
		order.PreparationTime,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert ID: %w", err)
	}

	order.ID = fmt.Sprintf("%d", id)
	return nil
}

func (r *OrderRepository) GetActiveOrders(ctx context.Context) ([]internal.Order, error) {

	query := `SELECT id, arrival_time, dishes, status, source, vip, preparation_time FROM orders
				WHERE status IN ('pending', 'preparing', 'ready')`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []internal.Order
	for rows.Next() {
		var order internal.Order
		var dishes string
		var preparationTime sql.NullString

		err := rows.Scan(
			&order.ID,
			&order.ArrivalTime,
			&dishes,
			&order.Status,
			&order.Source,
			&order.VIP,
			&preparationTime,
		)
		if err != nil {
			return nil, err
		}

		if preparationTime.Valid {
			duration, err := parseMySQLTimeToDuration(preparationTime.String)
			if err != nil {
				return nil, err
			}
			order.PreparationTime = duration
		} else {
			order.PreparationTime = 0
		}

		order.Dishes = strings.Split(dishes, ",")
		orders = append(orders, order)
	}

	return orders, nil
}

// GetOrderByID returns an order by its ID
func (r *OrderRepository) GetOrderByID(ctx context.Context, id string) (*internal.Order, error) {
	query := `SELECT id, arrival_time, dishes, status, source, vip, preparation_time FROM orders WHERE id = ?`

	row := r.db.QueryRowContext(ctx, query, id)
	var order internal.Order
	var dishes string

	err := row.Scan(
		&order.ID,
		&order.ArrivalTime,
		&dishes,
		&order.Status,
		&order.Source,
		&order.VIP,
		&order.PreparationTime,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	order.Dishes = strings.Split(dishes, ",")
	return &order, nil
}

// Update updates the order.
func (r *OrderRepository) Update(ctx context.Context, order internal.Order) error {
	query := `UPDATE orders SET arrival_time = ?, dishes = ?, status = ?, source = ?, vip = ?, preparation_time = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, order.ArrivalTime, strings.Join(order.Dishes, ","),
		order.Status, order.Source, order.VIP, durationToMySQLTime(order.PreparationTime), order.ID)
	return err
}

// Conversion from "HH:MM:SS" to time.Duration
func parseMySQLTimeToDuration(timeStr string) (time.Duration, error) {
	parts := strings.Split(timeStr, ":")
	if len(parts) != 3 {
		return 0, fmt.Errorf("invalid time format: %s", timeStr)
	}

	hours, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, err
	}
	minutes, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, err
	}
	seconds, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, err
	}

	return time.Duration(hours)*time.Hour +
		time.Duration(minutes)*time.Minute +
		time.Duration(seconds)*time.Second.Round(time.Second), nil
}

// Format time.Duration as "HH:MM:SS"
func durationToMySQLTime(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}
