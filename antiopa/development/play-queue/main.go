package main

import (
	"fmt"
	"time"
)

type TaskModule struct {
	failureCount int
	Name         string
}

func (t *TaskModule) IncrementFailureCount() {
	t.failureCount++
}

func (t *TaskModule) ToString() string {
	return fmt.Sprintf("TaskModule [%s] failures=%d", t.Name, t.failureCount)
}

func FillQueue(q *TasksQueue, ch chan int) {
	for i := 0; i < 40; i++ {
		t := TaskModule{Name: fmt.Sprintf("task_%04d", i)}
		q.Push(&t)
	}
	fmt.Println("Queue filled")

	ch <- 1
}

// Кривая обработка очереди, специально, чтобы в одной go-рутине
// делался Peek, а во второй Remove того же элемента
func HandleQueue(q *TasksQueue, ch chan int, name string) {
	for !q.IsEmpty() {
		for i := 0; i < 10; i++ {
			if q.IsEmpty() {
				break
			}
			top, _ := q.Peek()
			if top != nil {
				q.IncrementFailureCount()
				taskText := q.WithLock(func(task interface{}) string {
					if v, ok := task.(*TaskModule); ok {
						return v.ToString()
					}
					return ""
				})
				if _, ok := top.(*TaskModule); ok {
					fmt.Printf("%s %s\n", name, taskText)
				} else {
					fmt.Printf("%s top is not TaskModule\n", name)
				}
			} else {
				fmt.Printf("%s top is nil\n", name)
			}
		}
		q.Remove()
		time.Sleep(100 * time.Millisecond)
	}

	ch <- 1
}

func main() {
	fmt.Println("Start")
	q := NewTasksQueue()
	t := TaskModule{Name: "First one"}
	fmt.Println("Push")
	q.Push(&t)

	w := make(chan int, 0)

	go FillQueue(q, w)
	<-w
	fmt.Println("Handle queue")
	go HandleQueue(q, w, "---")
	go HandleQueue(q, w, "+++")
	go HandleQueue(q, w, "OOO")
	go HandleQueue(q, w, "!!!")

	<-w
	<-w
	<-w
	<-w

	fmt.Printf("IsEmpty? %v\n", q.IsEmpty())

	fmt.Println("End")
}
