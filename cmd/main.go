package main

import (
	"YunoChallenge/cmd/handlers"
	"YunoChallenge/internal/orders"
	"YunoChallenge/internal/platform/config"
	"YunoChallenge/internal/platform/database"
	"YunoChallenge/internal/repository"
	"fmt"
	"net/http"
)

func main() {
	configuration := config.LoadConfig()

	db, err := database.New(configuration.Database)
	if err != nil {
		fmt.Println("Error connecting to the database")
		return
	}
	defer func(db *database.Database) {
		err := db.Close()
		if err != nil {
			fmt.Println("Error closing the database connection")
		}
	}(db)

	orderRepository := repository.NewOrderRepository(db)
	orderService := orders.NewService(orderRepository)

	router := http.NewServeMux()
	loadRoutes(router, orderService)

	server := http.Server{
		Addr:    ":" + configuration.Server.Port,
		Handler: router,
	}

	fmt.Println("Server running on port", configuration.Server.Port)
	err = server.ListenAndServe()
	if err != nil {
		return
	}
}

// loadRouter loads the corresponding router for the application
func loadRoutes(router *http.ServeMux, service *orders.OrderService) {
	router.HandleFunc("POST /orders", handlers.CreateOrder(service))
	router.HandleFunc("GET /orders/{id}", handlers.GetOrder(service))
	router.HandleFunc("PUT /orders/{id}", handlers.UpdateOrder(service))
	router.HandleFunc("GET /orders", handlers.GetActiveOrders(service))
	router.HandleFunc("GET /stats", handlers.GetStats(service)) // Nuevo endpoint
}
