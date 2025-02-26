package repository_test

import (
	"YunoChallenge/internal"
	"YunoChallenge/internal/platform/database"
	"YunoChallenge/internal/repository"
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// OrderRepositoryTestSuite DEFINE EL TEST SUITE
type OrderRepositoryTestSuite struct {
	suite.Suite
	db    *database.Database
	sqlDB *sql.DB
	mock  sqlmock.Sqlmock
	repo  *repository.OrderRepository
	ctx   context.Context
}

func (s *OrderRepositoryTestSuite) SetupTest() {
	// Crea una nueva instancia de sqlmock
	var err error
	s.sqlDB, s.mock, err = sqlmock.New()
	assert.NoError(s.T(), err)

	s.db = &database.Database{DB: s.sqlDB}
	s.repo = repository.NewOrderRepository(s.db)
	s.ctx = context.Background()
}

func (s *OrderRepositoryTestSuite) TearDownTest() {
	s.sqlDB.Close()
}

func (s *OrderRepositoryTestSuite) TestCreateOrder() {
	order := &internal.Order{
		ArrivalTime:     time.Now(),
		Dishes:          []string{"pizza", "pasta"},
		Status:          "pending",
		Source:          "app",
		VIP:             true,
		PreparationTime: 15 * time.Minute,
	}

	s.mock.ExpectExec(`INSERT INTO orders`).
		WithArgs(
			order.ArrivalTime,
			"pizza,pasta",
			order.Status,
			order.Source,
			order.VIP,
			"00:15:00",
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := s.repo.CreateOrder(s.ctx, order)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "1", order.ID)
	assert.NoError(s.T(), s.mock.ExpectationsWereMet())
}

func (s *OrderRepositoryTestSuite) TestGetActiveOrders() {
	rows := sqlmock.NewRows([]string{"id", "arrival_time", "dishes", "status", "source", "vip", "preparation_time"}).
		AddRow("1", time.Now(), "pizza,pasta", "pending", "app", true, "00:15:00").
		AddRow("2", time.Now(), "salad", "preparing", "web", false, nil)

	s.mock.ExpectQuery(`SELECT id, arrival_time, dishes, status, source, vip, preparation_time FROM orders`).
		WillReturnRows(rows)

	orders, err := s.repo.GetActiveOrders(s.ctx)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), orders, 2)
	assert.Equal(s.T(), "1", orders[0].ID)
	assert.Equal(s.T(), []string{"pizza", "pasta"}, orders[0].Dishes)
	assert.Equal(s.T(), 15*time.Minute, orders[0].PreparationTime)
	assert.Equal(s.T(), 0*time.Second, orders[1].PreparationTime)
	assert.NoError(s.T(), s.mock.ExpectationsWereMet())
}

func (s *OrderRepositoryTestSuite) TestGetOrderByID() {
	row := sqlmock.NewRows([]string{"id", "arrival_time", "dishes", "status", "source", "vip", "preparation_time"}).
		AddRow("1", time.Now(), "pizza,pasta", "pending", "app", true, "00:15:00")

	s.mock.ExpectQuery(`SELECT id, arrival_time, dishes, status, source, vip, preparation_time FROM orders`).
		WithArgs("1").
		WillReturnRows(row)

	order, err := s.repo.GetOrderByID(s.ctx, "1")
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), order)
	assert.Equal(s.T(), "1", order.ID)
	assert.Equal(s.T(), []string{"pizza", "pasta"}, order.Dishes)
	assert.Equal(s.T(), 15*time.Minute, order.PreparationTime)
	assert.NoError(s.T(), s.mock.ExpectationsWereMet())
}

func (s *OrderRepositoryTestSuite) TestUpdate() {
	order := internal.Order{
		ID:              "1",
		ArrivalTime:     time.Now(),
		Dishes:          []string{"pizza", "pasta"},
		Status:          "preparing",
		Source:          "app",
		VIP:             true,
		PreparationTime: 20 * time.Minute,
	}

	s.mock.ExpectExec(`UPDATE orders SET`).
		WithArgs(
			order.ArrivalTime,
			"pizza,pasta",
			order.Status,
			order.Source,
			order.VIP,
			"00:20:00",
			order.ID,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := s.repo.Update(s.ctx, order)
	assert.NoError(s.T(), err)
	assert.NoError(s.T(), s.mock.ExpectationsWereMet())
}

func (s *OrderRepositoryTestSuite) TestStats() {
	row := sqlmock.NewRows([]string{"avg_preparation_seconds", "orders_per_hour", "total_active_orders"}).
		AddRow(900.0, 5.0, 10)

	s.mock.ExpectQuery(`SELECT COALESCE`).
		WillReturnRows(row)

	avgTime, ordersPerHour, totalActive := s.repo.Stats(s.ctx)
	assert.Equal(s.T(), "00:15:00", avgTime)
	assert.Equal(s.T(), 5.0, ordersPerHour)
	assert.Equal(s.T(), 10, totalActive)
	assert.NoError(s.T(), s.mock.ExpectationsWereMet())
}

func TestOrderRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(OrderRepositoryTestSuite))
}
