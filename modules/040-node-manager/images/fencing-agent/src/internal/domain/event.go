package domain

type EventType int

const (
	EventTypeJoin  EventType = 1
	EventTypeLeave EventType = 2
)

type Event struct {
	Node      Node
	EventType EventType
	Timestamp int64 // Unix timestamp
}
