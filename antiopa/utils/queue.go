package utils

import (
	"bytes"
	"io"
	"sync"
)

type QueueEvent string

const Changed QueueEvent = "QUEUE_CHANGED"

type Queue struct {
	m       sync.Mutex
	items   []interface{}
	eventCh chan QueueEvent
}

// Create new queue
func NewQueue() *Queue {
	return &Queue{
		m:       sync.Mutex{},
		items:   make([]interface{}, 0),
		eventCh: make(chan QueueEvent, 0),
	}
}

// Add last element
func (q *Queue) Add(task interface{}) {
	q.m.Lock()
	defer q.m.Unlock()
	q.items = append(q.items, task)
	q.eventCh <- Changed
}

// Examine first element (get without delete)
func (q *Queue) Peek() (task interface{}, err error) {
	if q.IsEmpty() {
		return nil, err
	}
	return q.items[0], err
}

// Pop first element (delete)
func (q *Queue) Pop() (task interface{}) {
	q.m.Lock()
	defer q.m.Unlock()
	if q.isEmpty() {
		return
	}
	task = q.items[0]
	q.items = q.items[1:]
	q.eventCh <- Changed
	return task
}

// Add first element
func (q *Queue) Push(task interface{}) {
	q.m.Lock()
	defer q.m.Unlock()
	q.items = append([]interface{}{task}, q.items)
	q.eventCh <- Changed
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

func (q *Queue) EventCh() chan QueueEvent {
	return q.eventCh
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
