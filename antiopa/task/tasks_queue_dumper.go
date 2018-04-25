package task

import (
	"io"
	"os"

	"github.com/romana/rlog"
)

type TasksQueueDumper struct {
	DumpFilePath string
	queue        *TasksQueue
	eventCh      chan struct{}
}

func NewTasksQueueDumper(dumpFilePath string, queue *TasksQueue) *TasksQueueDumper {
	result := &TasksQueueDumper{
		DumpFilePath: dumpFilePath,
		queue:        queue,
		eventCh:      make(chan struct{}, 1),
	}
	go result.WatchQueue()
	return result
}

// При изменении очереди сдампить её в файл
func (t *TasksQueueDumper) QueueChangeCallback() {
	t.eventCh <- struct{}{}
}

func (t *TasksQueueDumper) WatchQueue() {
	for {
		select {
		case <-t.eventCh:
			t.DumpQueue()
		}
	}
}

func (t *TasksQueueDumper) DumpQueue() {
	f, err := os.Create(t.DumpFilePath)
	if err != nil {
		rlog.Errorf("TasksQueueDumper: Cannot open '%s': %s\n", t.DumpFilePath, err)
	}
	_, err = io.Copy(f, t.queue.DumpReader())
	if err != nil {
		rlog.Errorf("TasksQueueDumper: Cannot dump tasks to '%s': %s\n", t.DumpFilePath, err)
	}
	err = f.Close()
	if err != nil {
		rlog.Errorf("TasksQueueDumper: Cannot close '%s': %s\n", t.DumpFilePath, err)
	}
}
