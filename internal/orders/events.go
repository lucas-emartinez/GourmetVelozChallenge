package orders

import "YunoChallenge/internal"

// EventType defines the supported event types.
type EventType string

const (
	EventAdd      EventType = "add"
	EventUpdate   EventType = "update"
	EventCancel   EventType = "cancel"
	EventNotify   EventType = "notify"
	EventShutdown EventType = "shutdown"
)

// Event represents an event that can be processed by the service.
type Event struct {
	Type  EventType
	Order internal.Order
	ID    string
}
