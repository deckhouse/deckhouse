package task

import (
	"bytes"
	"fmt"
	"io"

	"github.com/deckhouse/deckhouse/antiopa/utils"
)

/*
Очередь для последовательного выполнения модулей и хуков
Вызов хуков может приводить к добавлению в очередь заданий на запуск других хуков или модулей.
Задания добавляются в очередь с конца, выполняются с начала.
Каждое задание ретраится «до победного конца», ожидая успешного выполнения.
Если в конфиге задания есть флаг allowFailure: true, то задание считается завершённым и из очереди
берётся следующее.


Тип: Задание в очереди
Методы:
Свойства:
- Имя хукa/модуля
- FailureCount - количество неудачных запусков задания
- Конфиг
  - AllowFailure - игнорировать неудачный запуск задания

Тип: очередь (Этот тип в utils)
Свойства:
- mutex для блокировок
- хэш с заданиями
Методы:
NewQueue — Создать новую пустую очередь
Add — Добавить задание в конец очереди
Peek — Получить задание из начала очереди
Pop — Удалить задание из начала очереди
Push - Добавить задание в начало очереди
IsEmpty — Пустая ли очередь
WithLock — произвести операцию над первым элементом с блокировкой
IterateWithLock — произвести операцию над всеми элементами с блокировкой

Тип: TasksQueue
Добавлены специфичные методы как пример:
IncrementFailureCount — увеличить счётчик неудачных запусков у первого задания
DumpReader — io.Reader для дампа очереди в файл
*/

type TextDumper interface {
	DumpAsText() string
}

type FailureCountIncrementable interface {
	IncrementFailureCount()
}

type TasksQueue struct {
	*utils.Queue
	queueRef *utils.Queue
}

func (tq *TasksQueue) Add(task Task) {
	tq.queueRef.Add(task)
}

func (tq *TasksQueue) Push(task Task) {
	tq.queueRef.Push(task)
}

func (tq *TasksQueue) Peek() (task Task, err error) {
	res, err := tq.queueRef.Peek()
	if err != nil {
		return nil, err
	}

	var ok bool
	if task, ok = res.(Task); !ok {
		return nil, fmt.Errorf("bad data found in queue: %v", res)
	}

	return task, nil
}

func NewTasksQueue() *TasksQueue {
	queue := utils.NewQueue()
	return &TasksQueue{
		Queue:    queue,
		queueRef: queue,
	}
}

func (tq *TasksQueue) IncrementFailureCount() {
	tq.Queue.WithLock(func(topTask interface{}) string {
		if v, ok := topTask.(FailureCountIncrementable); ok {
			v.IncrementFailureCount()
		}
		return ""
	})
}

// прочитать дамп структуры для сохранения во временный файл
func (tq *TasksQueue) DumpReader() io.Reader {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("Queue length %d\n", tq.Queue.Length()))
	buf.WriteString("\n")

	iterateBuf := tq.Queue.IterateWithLock(func(task interface{}, index int) string {
		if v, ok := task.(TextDumper); ok {
			return v.DumpAsText()
		}
		return fmt.Sprintf("task %d: %+v", index, task)
	})
	return io.MultiReader(&buf, iterateBuf)
}
