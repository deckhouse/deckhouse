package main

import (
	_ "fmt"
	_ "io"
	_ "os"
	_ "time"

	_ "github.com/deckhouse/deckhouse/antiopa/task"
)

/* FIXME: convert to unit-test
func FillQueue(tq *task.TasksQueue, ch chan int) {
	for i := 0; i < 40; i++ {
		val := i
		cfg := task.UnitConfig{
			BeforeHelm: &val,
		}
		t := task.NewTask(cfg)

		tq.Add(t)
		time.Sleep(time.Second)
	}

	fmt.Printf("Queue filled, len=%d\n", tq.Length())
	ch <- 0
}
*/

func main() {
/* FIXME: convert to unit-test
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
*/
}
