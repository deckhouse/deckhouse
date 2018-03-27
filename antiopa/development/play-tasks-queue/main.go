package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/deckhouse/deckhouse/antiopa/task"
)

func FillQueue(tq *task.TasksQueue, ch chan int) {
	for i := 0; i < 40; i++ {
		val := i
		cfg := task.UnitConfig{
			BeforeHelm: &val,
		}
		t := task.NewTask(cfg)

		tq.Push(t)
		time.Sleep(time.Second)
	}

	fmt.Printf("Queue filled, len=%d\n", tq.Length())
	ch <- 0
}

func main() {
	tq := task.NewTasksQueue("/tmp/antiopa-tasks")

	w := make(chan int, 0)

	go FillQueue(tq, w)
	<-w

	f, err := os.Create("/tmp/dat2")
	if err != nil {
		panic(err)
	}
	io.Copy(f, tq.DumpReader())
	f.Close()
}
