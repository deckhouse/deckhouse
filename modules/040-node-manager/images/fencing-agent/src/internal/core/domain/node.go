package domain

import (
	"time"
)

type Node struct {
	Name      string
	Addresses map[string]string
	LastSeen  time.Time
}

type Event struct {
	Node      Node
	EventType EventType
	Timestamp int64 // Unix timestamp
}

type EventType int

const (
	EventTypeJoin  EventType = 1
	EventTypeLeave           = 2
)

type ClusterState struct {
	Nodes []Node
}

func NewNode(name string) Node {
	return Node{
		Name:      name,
		Addresses: make(map[string]string),
	}
}
