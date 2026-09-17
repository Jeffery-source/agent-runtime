package event

import "time"

type Event struct {
	Type string    `json:"type"`
	Data any       `json:"data"`
	Time time.Time `json:"time"`
}

type Sink interface {
	Emit(event Event)
}
