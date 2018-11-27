package task

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTasksQueue_Length(t *testing.T) {
	q := NewTasksQueue()
	FillQueue4(q)

	assert.Equalf(t, 4, q.Length(), "queue length problem")
	q.Pop()
	assert.Equalf(t, 3, q.Length(), "queue length problem after Pop")

	newTask := NewTask(ModuleDelete, "prometheus")
	q.Add(newTask)
	assert.Equalf(t, 4, q.Length(), "queue length problem after Add")

	q.Pop()
	assert.Equalf(t, 3, q.Length(), "queue length problem after second Pop")

	newTaskDelay := NewTaskDelay(time.Second)
	q.Push(newTaskDelay)
	assert.Equalf(t, 4, q.Length(), "queue length problem after Push")

	q.Pop()
	q.Pop()
	assert.Equalf(t, 2, q.Length(), "queue length problem after double Pop")

}

// Тест многопоточного добавления/удаления
func TestTasksQueue_MultiThread(t *testing.T) {
	q := NewTasksQueue()
	FillQueue4(q)
	tasksCount := q.Length()
	queueHandled := make(chan int, 0)
	queueAdd1 := make(chan int, 0)
	queueAdd2 := make(chan int, 0)
	queuePush := make(chan int, 0)
	stopCh := make(chan struct{}, 1)

	fmt.Println("start producers")

	// producer1
	go func(q *TasksQueue, count int) {
		taskPrefix := "ONE"
		i := count
		for {
			t := NewTask(ModuleRun, fmt.Sprintf("%s_%d", taskPrefix, i))
			fmt.Printf("%s add\n", taskPrefix)
			q.Add(t)
			time.Sleep(time.Duration(500 * time.Millisecond))
			i--
			if i == 0 {
				break
			}
		}
		queueAdd1 <- count
	}(q, 4)

	// producer2
	go func(q *TasksQueue, count int) {
		taskPrefix := "TWO-TWO"
		i := count
		for {
			t := NewTask(ModuleRun, fmt.Sprintf("%s_%d", taskPrefix, i))
			fmt.Printf("%s add\n", taskPrefix)
			q.Add(t)
			time.Sleep(time.Duration(300 * time.Millisecond))
			i--
			if i == 0 {
				break
			}
		}
		queueAdd2 <- count
	}(q, 3)

	// pusher
	go func(q *TasksQueue, count int) {
		taskPrefix := "PUSHIT"
		i := count
		for {
			t := NewTask(ModuleRun, fmt.Sprintf("%s_%d", taskPrefix, i))
			fmt.Printf("%s push\n", taskPrefix)
			q.Push(t)
			time.Sleep(time.Duration(150 * time.Millisecond))
			i--
			if i == 0 {
				break
			}
		}
		queuePush <- count
	}(q, 6)

	// consumer
	fmt.Println("start consumer")
	go func(q *TasksQueue) {
		count := 0
		for {
			fmt.Println("consumer select")
			select {
			case <-stopCh:
				fmt.Println("consumer got stop")
				queueHandled <- count
			default:
			}

			fmt.Println("consumer check IsEmpty")
			if q.IsEmpty() {
				fmt.Println("consumer sleep on IsEmpty")
				time.Sleep(time.Second * 5)
				continue
			}

			fmt.Println("consumer start handle loop")
			for {
				t, _ := q.Peek()
				if t != nil {
					fmt.Printf("%03d. %s!\n", count, t.GetName())
					count++
					fmt.Println("consumer Pop")
					q.Pop()
					time.Sleep(100 * time.Millisecond)
				}
				if q.IsEmpty() {
					break
				}
			}
		}
	}(q)

	fmt.Println("started")

	add1 := <-queueAdd1
	fmt.Printf("add1 done: %d\n", add1)
	tasksCount += add1

	add2 := <-queueAdd2
	fmt.Printf("add2 done: %d\n", add2)
	tasksCount += add2

	push := <-queuePush
	fmt.Printf("push done: %d\n", push)
	tasksCount += push

	stopCh <- struct{}{}

	fmt.Println("block on signal from consumer")
	handled := <-queueHandled
	fmt.Printf("consumer done: %d\n", handled)

	// test
	assert.Equalf(t, 0, q.Length(), "queue not handled properly: not empty")
	assert.Equalf(t, tasksCount, handled, "queue not handled properly: added != poped")

}

func TestTasksQueue_FileDumper(t *testing.T) {
	q := NewTasksQueue()

	f, err := ioutil.TempFile("/tmp", "test-task-queue-dumper")
	if err != nil {
		t.Fatalf("Cannot create temp file: %s", err)
	}
	path := f.Name()
	f.Close()
	defer os.Remove(path)

	t.Logf("tasks queue dump file '%s'\n", path)
	queueWatcher := NewTasksQueueDumper(path, q)
	q.AddWatcher(queueWatcher)
	q.ChangesEnable(true)

	FillQueue4(q)

	// wait for file write
	time.Sleep(100 * time.Millisecond)

	content, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read from '%s': %s", path, err)
	}

	str := string(content)
	assert.Contains(t, str, "global_hook_1", "no first task in dump")
	assert.Contains(t, str, "global_hook_2", "no second task in dump")
	assert.Contains(t, str, "global_hook_3", "no third task in dump")
	assert.Contains(t, str, "global_hook_4", "no fourth task in dump")
}

func FillQueue4(q *TasksQueue) {
	task1 := NewTask(GlobalHookRun, "global_hook_1")
	task2 := NewTask(GlobalHookRun, "global_hook_2")
	task3 := NewTask(GlobalHookRun, "global_hook_3")
	task4 := NewTask(GlobalHookRun, "global_hook_4")
	q.Add(task1)
	q.Add(task2)
	q.Add(task3)
	q.Add(task4)
}
