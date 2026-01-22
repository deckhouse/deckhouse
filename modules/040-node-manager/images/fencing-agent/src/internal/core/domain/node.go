package domain

import (
	"time"
)

type Node struct {
	Name      string
	Addresses map[string]string
	LastSeen  time.Time
}

func NewNode(name string) Node {
	return Node{
		Name:      name,
		Addresses: make(map[string]string),
	}
}
