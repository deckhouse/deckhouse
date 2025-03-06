/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
	errCh := make(chan error)

	for i, item := range s.tasks {
		wg.Add(1)
		task := item
		go func() {
			defer wg.Done()
			task.Run(s.ctx, errCh)
			fmt.Printf("Task %d done\n", i)
		}()
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
