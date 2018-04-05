package utils

import (
	"bytes"
	"io"
	"sync"
)

type QueueWatcher interface {
	QueueChangeCallback()
}

type Queue struct {
	m              sync.Mutex
	items          []interface{}
	changesEnabled bool           // turn on and off callbacks execution
	changesCount   int            // count changes when callbacks turned off
	queueWatchers  []QueueWatcher // callbacks to be executed on items change
}

// Create new queue
func NewQueue() *Queue {
	return &Queue{
		m:              sync.Mutex{},
		items:          make([]interface{}, 0),
		changesCount:   0,
		changesEnabled: false,
		queueWatchers:  make([]QueueWatcher, 0),
	}
}

// Add last element
func (q *Queue) Add(task interface{}) {
	q.m.Lock()
	defer q.m.Unlock()
	q.items = append(q.items, task)
	q.queueChanged()
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
	q.queueChanged()
	return task
}

// Add first element
func (q *Queue) Push(task interface{}) {
	q.m.Lock()
	defer q.m.Unlock()
	q.items = append([]interface{}{task}, q.items)
	q.queueChanged()
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

// Watcher functions

// Add queue watcher
func (q *Queue) AddWatcher(queueWatcher QueueWatcher) {
	q.queueWatchers = append(q.queueWatchers, queueWatcher)
}

// Вызвать при каждом изменении состава очереди
func (q *Queue) queueChanged() {
	if len(q.queueWatchers) == 0 {
		return
	}

	if q.changesEnabled {
		for _, watcher := range q.queueWatchers {
			watcher.QueueChangeCallback()
		}
	} else {
		q.changesCount++
	}
}

// Включить вызов QueueChangeCallback при каждом изменении
// В паре с ChangesDisabled могут быть использованы, чтобы
// производить массовые изменения. Если runCallbackOnPreviousChanges true,
// то будет вызвана QueueChangeCallback
func (q *Queue) ChangesEnable(runCallbackOnPreviousChanges bool) {
	q.changesEnabled = true
	if runCallbackOnPreviousChanges && q.changesCount > 0 {
		q.changesCount = 0
		q.queueChanged()
	}
}

func (q *Queue) ChangesDisable() {
	q.changesEnabled = false
	q.changesCount = 0
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
