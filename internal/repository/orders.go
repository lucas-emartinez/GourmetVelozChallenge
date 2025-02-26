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

// OrderRepository es una estructura que maneja la persistencia de las órdenes
type OrderRepository struct {
	db *database.Database
}

// NewOrderRepository crea un nuevo OrderRepository
func NewOrderRepository(db *database.Database) *OrderRepository {
	return &OrderRepository{db: db}
}

// CreateOrder crea una nueva orden
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
		durationToMySQLTime(order.PreparationTime),
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

// GetActiveOrders devuelve todas las órdenes activas
// las activas son las que tienen estado pending, preparing, ready
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

// GetOrderByID devuelve una orden por su ID
func (r *OrderRepository) GetOrderByID(ctx context.Context, id string) (*internal.Order, error) {
	query := `SELECT id, arrival_time, dishes, status, source, vip, preparation_time FROM orders WHERE id = ?`

	row := r.db.QueryRowContext(ctx, query, id)
	var order internal.Order
	var dishes string
	var preparationTime sql.NullString

	err := row.Scan(
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
	return &order, nil
}

// Update actualiza los valores de una orden
func (r *OrderRepository) Update(ctx context.Context, order internal.Order) error {
	query := `UPDATE orders SET arrival_time = ?, dishes = ?, status = ?, source = ?, vip = ?, preparation_time = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, order.ArrivalTime, strings.Join(order.Dishes, ","),
		order.Status, order.Source, order.VIP, durationToMySQLTime(order.PreparationTime), order.ID)
	return err
}

// Stats devuelve las estadísticas de las órdenes en la última hora
func (r *OrderRepository) Stats(ctx context.Context) (string, float64, int) {
	query := `
		SELECT 
			COALESCE(AVG(CASE WHEN status = 'ready' THEN TIME_TO_SEC(preparation_time) END), 0) AS avg_preparation_seconds,
			COUNT(CASE WHEN arrival_time >= NOW() - INTERVAL 1 HOUR THEN 1 END) AS orders_per_hour,
			COUNT(CASE WHEN status NOT IN ('delivered', 'cancelled') THEN 1 END) AS total_active_orders
		FROM orders`

	var avgPrepSeconds float64
	var ordersPerHour float64
	var totalActiveOrders int

	err := r.db.QueryRowContext(ctx, query).Scan(&avgPrepSeconds, &ordersPerHour, &totalActiveOrders)
	if err != nil {
		fmt.Println("Error getting stats:", err)
		return "", 0, 0
	}

	return durationToMySQLTime(time.Duration(int(avgPrepSeconds)) * time.Second), ordersPerHour, totalActiveOrders
}

// Convierne una cadena de tiempo MySQL en una duración de GO
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

// Convierte una duración de GO en una cadena de tiempo MySQL
func durationToMySQLTime(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}
