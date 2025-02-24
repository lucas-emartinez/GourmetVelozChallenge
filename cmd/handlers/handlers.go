package handlers

import (
	"YunoChallenge/internal"
	"context"
	"encoding/json"
	"errors"
	"net/http"
)

type (
	// request is a struct that represents an order request
	request struct {
		ID          string   `json:"id"`
		ArrivalTime string   `json:"arrival_time"`
		Dishes      []string `json:"dishes"`
		Status      *string  `json:"status"`
		Source      string   `json:"source"`
		VIP         *bool    `json:"vip"`
	}

	// response is a struct that represents an order response
	response struct {
		ID              string   `json:"id"`
		ArrivalTime     string   `json:"arrival_time"`
		Dishes          []string `json:"dishes"`
		Status          string   `json:"status"`
		Source          string   `json:"source"`
		VIP             bool     `json:"vip"`
		PreparationTime string   `json:"preparation_time"`
	}

	// OrderService is an interface that defines the order service methods
	OrderService interface {
		AddOrder(ctx context.Context, order internal.Order) error
		GetOrderByID(ctx context.Context, id string) (*internal.Order, error)
		UpdateOrder(ctx context.Context, id string, order internal.Order) error
		CancelOrder(ctx context.Context, id string) error
		GetActiveOrders() []internal.Order
	}
)

// CreateOrder is a handler function that creates an order
func CreateOrder(service OrderService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var orderReq request
		err := json.NewDecoder(r.Body).Decode(&orderReq)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		order := internal.Order{
			Dishes: orderReq.Dishes,
			Source: orderReq.Source,
			VIP:    orderReq.VIP,
		}

		err = service.AddOrder(r.Context(), order)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
	}
}

// GetOrder is a handler function that retrieves an order by its ID
func GetOrder(service OrderService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "Missing order ID", http.StatusBadRequest)
			return
		}

		order, err := service.GetOrderByID(r.Context(), id)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			if errors.Is(err, internal.ErrOrderNotFound) {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		response := response{
			ID:              id,
			ArrivalTime:     order.ArrivalTime.Format("2006-01-02 15:04:05"),
			Dishes:          order.Dishes,
			Status:          string(order.Status),
			Source:          order.Source,
			VIP:             *order.VIP,
			PreparationTime: order.PreparationTime.String(),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// UpdateOrder is a handler function that updates an order by its ID
func UpdateOrder(service OrderService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "Missing order ID", http.StatusBadRequest)
			return
		}

		var orderReq request
		err := json.NewDecoder(r.Body).Decode(&orderReq)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		order := internal.Order{
			ID:     id,
			Dishes: orderReq.Dishes,
			Status: internal.OrderStatus(*orderReq.Status),
			Source: orderReq.Source,
			VIP:    orderReq.VIP,
		}

		err = service.UpdateOrder(r.Context(), id, order)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// GetActiveOrders is a handler function that retrieves all active orders
func GetActiveOrders(service OrderService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		activeOrders := service.GetActiveOrders()

		var activeOrdersResponse []response
		for _, order := range activeOrders {
			activeOrdersResponse = append(activeOrdersResponse, response{
				ID:              order.ID,
				ArrivalTime:     order.ArrivalTime.Format("2006-01-02 15:04:05"),
				Dishes:          order.Dishes,
				Status:          string(order.Status),
				Source:          order.Source,
				VIP:             *order.VIP,
				PreparationTime: order.PreparationTime.String(),
			})
		}

		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(activeOrdersResponse)
		if err != nil {
			return
		}
	}
}

// CancelOrder is a handler function that cancels an order
func CancelOrder(service OrderService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			http.Error(w, "Missing order ID", http.StatusBadRequest)
			return
		}

		err := service.CancelOrder(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
