package task

import (
	"bytes"
	"fmt"
	"io"

	"github.com/deckhouse/deckhouse/antiopa/utils"
	"os"
)

/*
Очередь для последовательного выполнения модулей и хуков
Вызов хуков может приводить к добавлению в очередь заданий на запуск других хуков или модулей.
Задания добавляются в очередь с конца, выполняются сначала.
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

Тип: очередь
Свойства:
- mutex для блокировок
- хэш с заданиями
Методы:
NewQueue — Создать новую пустую очередь
Push — Добавить задание в конец очереди
Peek — Получить задание из начала очереди
Remove — Удалить задание из начала очереди
IsEmpty — Пустая ли очередь
WithLock — произвести операцию над первым элементом с блокировкой
IterateWithLock — произвести операцию над всеми элементами с блокировкой
Этот тип в utils

Тип: TasksQueue
Добавлены специфичные методы как пример:
IncrementFailureCount — увеличить счётчик неудачных запусков
DumpQueue — получить поток списка строк для дампа информации про таски во временный файл
Push — переопределение, чтобы дампать актуальный список тасков в файл
*/

type TextDumper interface {
	DumpAsText() string
}

type FailureCountIncrementable interface {
	IncrementFailureCount()
}

type TasksQueue struct {
	utils.Queue
	DumpFileName string
}

func NewTasksQueue(dumpFileName string) *TasksQueue {
	return &TasksQueue{
		Queue:        utils.Queue{},
		DumpFileName: dumpFileName,
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
	buf.WriteString(fmt.Sprintf("Queue length %d\n", tq.Length()))
	buf.WriteString("\n")

	iterateBuf := tq.Queue.IterateWithLock(func(task interface{}, index int) string {
		if v, ok := task.(TextDumper); ok {
			return v.DumpAsText()
		}
		return fmt.Sprintf("task %d: %+v", index, task)
	})
	return io.MultiReader(&buf, iterateBuf)
}

// Переопределение Push, чтобы дампать новое состояние
// TODO реализовать через канал?
func (tq *TasksQueue) Push(task interface{}) {
	fmt.Println("Push detected")

	tq.Queue.Push(task)

	f, err := os.Create(tq.DumpFileName)
	if err != nil {
		fmt.Printf("Cannot open %s: %s\n", tq.DumpFileName, err)
	}
	_, err = io.Copy(f, tq.DumpReader())
	if err != nil {
		fmt.Printf("Cannot dump tasks to %s: %s\n", tq.DumpFileName, err)
	}
	f.Close()
	if err != nil {
		fmt.Printf("Cannot close %s: %s\n", tq.DumpFileName, err)
	}
}
