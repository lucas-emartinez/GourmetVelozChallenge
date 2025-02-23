package internal

import "time"

type Order struct {
	ID          string    `json:"id"`
	ArrivalTime time.Time `json:"arrival_time"`
	Dishes      []string  `json:"dishes"`
	Status      string    `json:"status"`
	Source      string    `json:"source"`
	Priority    bool      `json:"priority"`
}
