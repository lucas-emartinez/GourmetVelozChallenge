package internal

import (
	"errors"
	"time"
)

// ErrOrderNotFound is an error that indicates that an order was not found
var ErrOrderNotFound error = errors.New("order not found")

// ErrOrderInvalidStatusTransition is an error that indicates that the status transition is invalid
var ErrOrderInvalidStatusTransition error = errors.New("invalid status transition")

// ErrInvalidOrderSource is an error that indicates that the order source is invalid
var ErrInvalidOrderSource error = errors.New("invalid order source")

type (
	// Order is a struct that represents an order
	// Priority is a flag that indicates if the order is a VIP order
	Order struct {
		ID              string
		ArrivalTime     time.Time
		Dishes          []string
		Status          OrderStatus
		Source          string
		VIP             *bool
		PreparationTime time.Duration
	}

	// orderStatus is a string that represents the status of an order
	OrderStatus string
	// OrderSource is a string that represents the source of an order
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
