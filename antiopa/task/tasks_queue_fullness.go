package task

type TasksQueueFullnessWatcher struct {
	queue   *TasksQueue
	eventCh chan struct{}
}

func NewTasksQueueFullnessWatcher(queue *TasksQueue) *TasksQueueFullnessWatcher {
	return &TasksQueueFullnessWatcher{
		queue:   queue,
		eventCh: make(chan struct{}, 1),
	}
}

func (t *TasksQueueFullnessWatcher) EventCh() chan struct{} {
	return t.eventCh
}

func (t *TasksQueueFullnessWatcher) QueueChangeCallback() {
	if !t.queue.IsEmpty() {
		t.eventCh <- struct{}{}
	}
}
