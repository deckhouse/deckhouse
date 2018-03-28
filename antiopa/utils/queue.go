package utils

import (
	"bytes"
	"io"
	"sync"
)

type Queue struct {
	m     sync.Mutex
	items []interface{}
}

// Create new queue
func NewQueue() *Queue {
	return &Queue{
		m:     sync.Mutex{},
		items: make([]interface{}, 0),
	}
}

// Get first element
func (q *Queue) Peek() (task interface{}, err error) {
	if q.IsEmpty() {
		return nil, err
	}
	return q.items[0], err
}

// Add new element into last position
func (q *Queue) Push(task interface{}) {
	q.m.Lock()
	defer q.m.Unlock()
	q.items = append(q.items, task)
}

// Remove element from top
func (q *Queue) Remove() {
	q.m.Lock()
	defer q.m.Unlock()
	if q.isEmpty() {
		return
	}
	q.items = q.items[1:]
}

func (q *Queue) IsEmpty() bool {
	q.m.Lock()
	defer q.m.Unlock()
	return q.isEmpty()
}

func (q *Queue) isEmpty() bool {
	return len(q.items) == 0
}

func (q *Queue) Length() int {
	return len(q.items)
}

// Тип метода — операция над первым элементом очереди
type TaskOperation func(topTask interface{}) string
type IterateOperation func(task interface{}, index int) string

// Вызов операции над первым элементом очереди с блокировкой
func (q *Queue) WithLock(operation TaskOperation) string {
	q.m.Lock()
	defer q.m.Unlock()
	return operation(q.items[0])
}

// Вызов операции над всеми элементами очереди с её блокировкой
func (q *Queue) IterateWithLock(operation IterateOperation) io.Reader {
	q.m.Lock()
	defer q.m.Unlock()
	var buf bytes.Buffer
	for i := 0; i < len(q.items); i++ {
		buf.WriteString(operation(q.items[i], i))
		buf.WriteString("\n")
	}
	return &buf
}
