package orders

import "YunoChallenge/internal"

// EventType define los tipos de eventos que pueden ser procesados por el servicio
type EventType string

const (
	EventAdd      EventType = "add"
	EventUpdate   EventType = "update"
	EventCancel   EventType = "cancel"
	EventNotify   EventType = "notify"
	EventShutdown EventType = "shutdown"
)

// Event es una estructura que representa un evento
type Event struct {
	Type  EventType
	Order internal.Order
	ID    string
}
