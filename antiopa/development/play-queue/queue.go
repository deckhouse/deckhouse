package main

import "sync"

/*
Очередь для последовательного выполнения модулей и хуков
Вызов хуков может приводить к добавлению в очередь заданий на запуск других хуков или модулей.
Задания добавляются в очередь с конца, выполняются сначала.
Каждое задание ретраится «до победного конца», ожидая успешного выполнения.
Если в конфиге задания есть флаг allowFailure: true, то задание считается завершённым ииз очереди
берётся следующее.

Методы:
Задание:
Создать задание
Очередь:
Добавить задание  Add
Получить задание  Peek
Увеличить failureCount у задания IncrementFailureCount
Удалить задание   Pop


Тип Интерфейса: Задание в очереди
Методы:
IncrementFailureCount() таски имплементят этот метод

Тип: очередь
Свойства:
- mutex для блокировок
- массив с заданиями
*/

type Task interface {
	IncrementFailureCount()
}

type TasksQueue struct {
	m     sync.Mutex
	tasks []interface{}
}

func NewTasksQueue() *TasksQueue {
	return &TasksQueue{
		m:     sync.Mutex{},
		tasks: make([]interface{}, 0),
	}
}

func (tq *TasksQueue) Peek() (task interface{}, err error) {
	if tq.IsEmpty() {
		return nil, err
	}
	return tq.tasks[0], err
}

func (tq *TasksQueue) Push(task interface{}) {
	tq.m.Lock()
	defer tq.m.Unlock()
	tq.tasks = append(tq.tasks, task)
}

func (tq *TasksQueue) Remove() {
	tq.m.Lock()
	defer tq.m.Unlock()
	if tq.isEmpty() {
		return
	}
	tq.tasks = tq.tasks[1:]
}

func (tq *TasksQueue) IncrementFailureCount() {
	tq.m.Lock()
	defer tq.m.Unlock()
	top := tq.tasks[0]
	if v, ok := top.(Task); ok {
		v.IncrementFailureCount()
	}
}

func (tq *TasksQueue) IsEmpty() bool {
	tq.m.Lock()
	defer tq.m.Unlock()
	return tq.isEmpty()
}

func (tq *TasksQueue) isEmpty() bool {
	return len(tq.tasks) == 0
}

// Тип метода — операция над первым элементом очереди
type TaskOperation func(topTask interface{}) string

// Вызов операции над первым элементом очереди с блокировкой
func (tq *TasksQueue) WithLock(operation TaskOperation) string {
	tq.m.Lock()
	defer tq.m.Unlock()
	return operation(tq.tasks[0])
}
