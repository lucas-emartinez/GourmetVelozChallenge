package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"YunoChallenge/internal"

	"github.com/stretchr/testify/assert"
)

// mockOrderService es un mock para OrderService que usaremos en las pruebas.
type mockOrderService struct {
	orders map[string]internal.Order
}

func (m *mockOrderService) AddOrder(ctx context.Context, order internal.Order) error {
	if order.Source == "invalid" {
		return internal.ErrInvalidOrderSource
	}
	m.orders[order.ID] = order
	return nil
}

func (m *mockOrderService) OrderByID(ctx context.Context, id string) (*internal.Order, error) {
	order, exists := m.orders[id]
	if !exists {
		return nil, internal.ErrOrderNotFound
	}
	return &order, nil
}

func (m *mockOrderService) UpdateOrder(ctx context.Context, id string, order internal.Order) error {
	if _, exists := m.orders[id]; !exists {
		return internal.ErrOrderNotFound
	}
	if order.Status == "invalid" {
		return internal.ErrOrderInvalidStatusTransition
	}
	m.orders[id] = order
	return nil
}

func (m *mockOrderService) CancelOrder(ctx context.Context, id string) error {
	if _, exists := m.orders[id]; !exists {
		return internal.ErrOrderNotFound
	}
	delete(m.orders, id)
	return nil
}

func (m *mockOrderService) ActiveOrders() []internal.Order {
	var active []internal.Order
	for _, order := range m.orders {
		if order.Status != internal.StatusDelivered && order.Status != internal.StatusCancelled {
			active = append(active, order)
		}
	}
	return active
}

func (m *mockOrderService) Stats(ctx context.Context) (string, float64, int) {
	var totalPrep time.Duration
	var completed int
	for _, order := range m.orders {
		if order.Status == internal.StatusReady {
			totalPrep += order.PreparationTime
			completed++
		}
	}
	avgPrep := "0s"
	if completed > 0 {
		avgPrep = (totalPrep / time.Duration(completed)).String()
	}
	return avgPrep, float64(len(m.orders)), len(m.orders)
}

// TestCreateOrder_ValidOrder prueba la creación de un pedido válido.
func TestCreateOrder_ValidOrder(t *testing.T) {
	// Arrange
	service := &mockOrderService{orders: make(map[string]internal.Order)}
	handler := CreateOrder(service)

	req, _ := http.NewRequest("POST", "/orders", bytes.NewBuffer([]byte(`{"dishes": ["Pizza"], "source": "delivery", "vip": true}`)))
	rr := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusCreated, rr.Result().StatusCode)
	assert.Empty(t, rr.Body.String())
}

// TestCreateOrder_InvalidJSON prueba la creación con JSON inválido.
func TestCreateOrder_InvalidJSON(t *testing.T) {
	// Arrange
	service := &mockOrderService{orders: make(map[string]internal.Order)}
	handler := CreateOrder(service)

	req, _ := http.NewRequest("POST", "/orders", bytes.NewBuffer([]byte(`{invalid json}`)))
	rr := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rr.Result().StatusCode)
	assert.Contains(t, rr.Body.String(), "invalid character")
}

// TestCreateOrder_InvalidSource prueba la creación con una fuente inválida.
func TestCreateOrder_InvalidSource(t *testing.T) {
	// Arrange
	service := &mockOrderService{orders: make(map[string]internal.Order)}
	handler := CreateOrder(service)

	req, _ := http.NewRequest("POST", "/orders", bytes.NewBuffer([]byte(`{"dishes": ["Pizza"], "source": "invalid", "vip": true}`)))
	rr := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rr.Result().StatusCode)
	assert.Contains(t, rr.Body.String(), "invalid order source")
}

// TestGetOrder_OrderFound prueba la recuperación de un pedido existente.
func TestGetOrder_OrderFound(t *testing.T) {
	// Arrange
	_, _ = time.LoadLocation("UTC")
	service := &mockOrderService{
		orders: map[string]internal.Order{
			"1": {
				ID:              "1",
				ArrivalTime:     time.Now(),
				Dishes:          []string{"Pizza"},
				Status:          internal.StatusPending,
				Source:          "delivery",
				VIP:             false,
				PreparationTime: 5 * time.Minute,
			},
		},
	}
	handler := GetOrder(service)

	req, _ := http.NewRequest("GET", "/orders/1", nil)
	rr := httptest.NewRecorder()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.SetPathValue("id", "1")
		handler.ServeHTTP(w, r)
	})

	// Act
	testHandler.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusOK, rr.Result().StatusCode)
	assert.Contains(t, rr.Body.String(), `"id":"1"`)
	assert.JSONEq(t, `{"id":"1","arrival_time":"`+service.orders["1"].ArrivalTime.Format("2006-01-02 15:04:05")+
		`","dishes":["Pizza"],"status":"pending","source":"delivery","vip":false,"preparation_time":"5m0s"}`, rr.Body.String())
}

// TestGetOrder_OrderNotFound prueba la recuperación de un pedido no existente.
func TestGetOrder_OrderNotFound(t *testing.T) {
	// Arrange
	service := &mockOrderService{orders: make(map[string]internal.Order)}
	handler := GetOrder(service)

	req, _ := http.NewRequest("GET", "/orders/2", nil)
	rr := httptest.NewRecorder()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.SetPathValue("id", "2")
		handler.ServeHTTP(w, r)
	})

	// Act
	testHandler.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusNotFound, rr.Result().StatusCode)
	assert.Contains(t, rr.Body.String(), `"error":"order not found"`)
}

// TestGetOrder_MissingID prueba la recuperación sin ID.
func TestGetOrder_MissingID(t *testing.T) {
	// Arrange
	service := &mockOrderService{orders: make(map[string]internal.Order)}
	handler := GetOrder(service)

	req, _ := http.NewRequest("GET", "/orders/", nil)
	ctx := context.WithValue(req.Context(), struct{ key string }{"pathValue"}, map[string]string{"id": ""})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rr.Result().StatusCode)
	assert.Contains(t, rr.Body.String(), "Missing order ID")
}

// TestUpdateOrder_ValidUpdate actualización válida de un pedido.
func TestUpdateOrder_ValidUpdate(t *testing.T) {
	// Arrange
	service := &mockOrderService{
		orders: map[string]internal.Order{
			"1": {ID: "1", Status: internal.StatusPending},
		},
	}
	handler := UpdateOrder(service)

	req, _ := http.NewRequest("PUT", "/orders/1", bytes.NewBuffer([]byte(`{"status": "preparing"}`)))
	rr := httptest.NewRecorder()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.SetPathValue("id", "1")
		handler.ServeHTTP(w, r)
	})

	// Act
	testHandler.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusOK, rr.Result().StatusCode)
}

// TestUpdateOrder_OrderNotFound prueba la actualización de un pedido no existente.
func TestUpdateOrder_OrderNotFound(t *testing.T) {
	// Arrange
	service := &mockOrderService{orders: make(map[string]internal.Order)}
	handler := UpdateOrder(service)

	req, _ := http.NewRequest("PUT", "/orders/2", bytes.NewBuffer([]byte(`{"status": "preparing"}`)))
	rr := httptest.NewRecorder()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.SetPathValue("id", "2")
		handler.ServeHTTP(w, r)
	})

	// Act
	testHandler.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusNotFound, rr.Result().StatusCode, "expected status 404")
	assert.Contains(t, rr.Body.String(), `"error":"order not found"`, "expected not found error")
}

// TestUpdateOrder_InvalidStatus prueba la actualización con un estado inválido.
func TestUpdateOrder_InvalidStatus(t *testing.T) {
	// Arrange
	service := &mockOrderService{
		orders: map[string]internal.Order{
			"1": {ID: "1", Status: internal.StatusPending},
		},
	}
	handler := UpdateOrder(service)

	req, _ := http.NewRequest("PUT", "/orders/1", bytes.NewBuffer([]byte(`{"status": "invalid"}`)))
	rr := httptest.NewRecorder()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.SetPathValue("id", "1")
		handler.ServeHTTP(w, r)
	})

	// Act
	testHandler.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rr.Result().StatusCode)
	assert.Contains(t, rr.Body.String(), `"error":"invalid status transition"`)
}

// TestUpdateOrder_MissingID actualización sin ID.
func TestUpdateOrder_MissingID(t *testing.T) {
	// Arrange
	service := &mockOrderService{orders: make(map[string]internal.Order)}
	handler := UpdateOrder(service)

	req, _ := http.NewRequest("PUT", "/orders/", bytes.NewBuffer([]byte(`{"status": "preparing"}`)))
	ctx := context.WithValue(req.Context(), struct{ key string }{"pathValue"}, map[string]string{"id": ""})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rr.Result().StatusCode)
	assert.Contains(t, rr.Body.String(), "Missing order ID")
}

// TestUpdateOrder_InvalidJSON actualiza con JSON inválido.
func TestUpdateOrder_InvalidJSON(t *testing.T) {
	// Arrange
	service := &mockOrderService{
		orders: map[string]internal.Order{
			"1": {ID: "1", Status: internal.StatusPending},
		},
	}
	handler := UpdateOrder(service)

	req, _ := http.NewRequest("PUT", "/orders/1", bytes.NewBuffer([]byte(`{invalid json}`)))
	rr := httptest.NewRecorder()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.SetPathValue("id", "1")
		handler.ServeHTTP(w, r)
	})

	// Act
	testHandler.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rr.Result().StatusCode)
	assert.Contains(t, rr.Body.String(), "invalid character")
}

// TestGetActiveOrders_ActiveOrders recupera los pedidos activos
func TestGetActiveOrders_ActiveOrders(t *testing.T) {
	// Arrange
	service := &mockOrderService{
		orders: map[string]internal.Order{
			"1": {
				ID:              "1",
				ArrivalTime:     time.Now(),
				Dishes:          []string{"Pizza"},
				Status:          internal.StatusPending,
				Source:          "delivery",
				VIP:             false,
				PreparationTime: 5 * time.Minute,
			},
		},
	}
	handler := GetActiveOrders(service)

	req, _ := http.NewRequest("GET", "/orders", nil)
	rr := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(rr, req)

	type response struct {
		ID              string   `json:"id"`
		ArrivalTime     string   `json:"arrival_time"`
		Dishes          []string `json:"dishes"`
		Status          string   `json:"status"`
		Source          string   `json:"source"`
		VIP             bool     `json:"vip"`
		PreparationTime string   `json:"preparation_time"`
	}
	type ActiveOrdersResponse struct {
		Pending   []response `json:"pending"`
		Preparing []response `json:"preparing"`
		Ready     []response `json:"ready"`
	}
	var activeOrders ActiveOrdersResponse
	err := json.Unmarshal(rr.Body.Bytes(), &activeOrders)
	assert.NoError(t, err)

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Len(t, activeOrders.Pending, 1)
	assert.Len(t, activeOrders.Preparing, 0)
	assert.Len(t, activeOrders.Ready, 0)
	assert.Equal(t, "1", activeOrders.Pending[0].ID)
	assert.Equal(t, []string{"Pizza"}, activeOrders.Pending[0].Dishes)
	assert.Equal(t, "pending", activeOrders.Pending[0].Status)
	assert.Equal(t, "delivery", activeOrders.Pending[0].Source)
	assert.False(t, activeOrders.Pending[0].VIP)
	assert.Equal(t, "5m0s", activeOrders.Pending[0].PreparationTime)
}

// TestGetStats_Stats prueba la recuperación de estadísticas.
func TestGetStats_Stats(t *testing.T) {
	// Arrange
	service := &mockOrderService{
		orders: map[string]internal.Order{
			"1": {
				ID:              "1",
				ArrivalTime:     time.Now(),
				Dishes:          []string{"Pizza"},
				Status:          internal.StatusReady,
				Source:          "delivery",
				VIP:             false,
				PreparationTime: 5 * time.Minute,
			},
		},
	}
	handler := GetStats(service)

	req, _ := http.NewRequest("GET", "/stats", nil)
	rr := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(rr, req)

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"avg_preparation_time":"5m0s"`)
}
