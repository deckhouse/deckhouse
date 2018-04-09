package task

import "fmt"

// Слежка за заполненной очередью.
// Не удалось, потому что в итоге в канал приходило несколько событий.
// Это должно быть отдельным вотчером, а не Change, т.к. сама очередь засчёт локов
// сможет установить факт, что была длина 0, и вдруг что-то появилось.

type TasksQueueFullnessWatcher struct {
	queue           *TasksQueue
	eventCh         chan struct{}
	lastQueueLength int
}

func NewTasksQueueFullnessWatcher(queue *TasksQueue) *TasksQueueFullnessWatcher {
	return &TasksQueueFullnessWatcher{
		queue:           queue,
		eventCh:         make(chan struct{}, 1),
		lastQueueLength: 0,
	}
}

func (t *TasksQueueFullnessWatcher) EventCh() chan struct{} {
	return t.eventCh
}

// TODO! сигнал посылается при любых изменениях, а нужно только когда длина очереди изменилась с 0 на что-то другое.
func (t *TasksQueueFullnessWatcher) QueueChangeCallback() {
	fmt.Println("QueueChangeCallback")
	//lastLen := t.lastQueueLength
	qLen := t.queue.Length()
	//t.lastQueueLength = qLen
	//if lastLen == 0 && qLen > 0 {
	if qLen > 0 {
		fmt.Println("NotEmpty")
		t.eventCh <- struct{}{}
	}
}
