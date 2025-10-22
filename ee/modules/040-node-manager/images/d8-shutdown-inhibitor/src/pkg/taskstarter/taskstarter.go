/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package taskstarter

import (
	"context"
	"fmt"
	"sync"
)

type Starter struct {
	tasks    []Task
	ctx      context.Context
	cancel   context.CancelFunc
	stopped  bool
	finished chan struct{}
	err      error
}

func NewStarter(tasks ...Task) *Starter {
	return &Starter{
		tasks:    tasks,
		finished: make(chan struct{}),
	}
}

func (s *Starter) Start(ctx context.Context) {
	s.ctx, s.cancel = context.WithCancel(ctx)

	var wg sync.WaitGroup
	errCh := make(chan error, 10)

	for i := range s.tasks {
		wg.Add(1)
		go func(task Task) {
			defer wg.Done()
			task.Run(s.ctx, errCh)
			fmt.Printf("Task %s done\n", task.Name())
		}(s.tasks[i])
	}

	// Error handler: cancel tasks on error, but wait until all tasks are done.
	go func() {
		select {
		case err := <-errCh:
			s.err = err
			s.Stop()
		case <-s.ctx.Done():
			return
		}
	}()

	wg.Wait()

	close(s.finished)
}

func (s *Starter) Stop() {
	if s.stopped {
		return
	}
	fmt.Printf("Stop all tasks...\n")
	// Cancel contexts of all tasks.
	s.cancel()
	s.stopped = true
}

func (s *Starter) Done() <-chan struct{} {
	return s.finished
}

func (s *Starter) Err() error {
	return s.err
}
