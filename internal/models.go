package internal

import (
	"errors"
	"time"
)

// ErrOrderNotFound es un error que indica que una orden no fue encontrada
var ErrOrderNotFound error = errors.New("order not found")

// ErrOrderInvalidStatusTransition es un error que indica que la transición de estado de una orden es inválida
var ErrOrderInvalidStatusTransition error = errors.New("invalid status transition")

// ErrInvalidOrderSource es un error que indica que la fuente de una orden es inválida
var ErrInvalidOrderSource error = errors.New("invalid order source")

type (
	// Order es una estructura que representa una orden
	Order struct {
		ID              string
		ArrivalTime     time.Time
		Dishes          []string
		Status          OrderStatus
		Source          OrderSource
		VIP             bool
		PreparationTime time.Duration
	}

	// orderStatus es un tipo que representa el estado de una orden
	OrderStatus string
	// OrderSource es un tipo que representa la fuente de una orden
	OrderSource string
)

const (
	StatusPending       OrderStatus = "pending"
	StatusInPreparation OrderStatus = "preparing"
	StatusReady         OrderStatus = "ready"
	StatusDelivered     OrderStatus = "delivered"
	StatusCancelled     OrderStatus = "cancelled"

	SourceInPerson OrderSource = "presencial"
	SourceDelivery OrderSource = "delivery"
	SourcePhone    OrderSource = "telefono"
)
