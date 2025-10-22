// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package infrastructure

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/fsnotify/fsnotify"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/fs"
)

type SaverDestination interface {
	SaveState(outputs *PipelineOutputs) error
}

type StateSaver struct {
	saversLock         sync.RWMutex
	saversDestinations []SaverDestination

	runner  *Runner
	watcher *fsnotify.Watcher
	doneCh  chan struct{}
	stopped bool
}

func NewStateSaver(destinations []SaverDestination) *StateSaver {
	return &StateSaver{
		saversDestinations: destinations,
	}
}

// Start creates a new file watcher for r.statePath and
// a chan to stop it.
func (s *StateSaver) Start(runner *Runner) error {
	if s.stopped {
		return nil
	}

	s.saversLock.RLock()
	defer s.saversLock.RUnlock()

	if len(s.saversDestinations) == 0 {
		return nil
	}

	if s.watcher != nil {
		return nil
	}

	if runner == nil {
		return nil
	}
	s.runner = runner

	if err := fs.TouchFile(s.runner.statePath); err != nil {
		return err
	}

	var err error
	s.doneCh = make(chan struct{})
	s.watcher, err = fs.StartFileWatcher(s.runner.statePath, s.FsEventHandler, s.doneCh, s.runner.logger)
	if err != nil {
		return fmt.Errorf("fs watcher for intermediate infrastructure state file: %s: %v", s.runner.statePath, err)
	}

	return nil
}

// Stop is blocked until doneCh is closed.
func (s *StateSaver) Stop() {
	if s.stopped {
		return
	}

	s.stopped = true
	if s.watcher != nil {
		err := s.watcher.Close()
		if err != nil {
			log.DebugF("State file watcher did not close: %v \n", err)
		}
		// Wait until saves are completed.
		<-s.doneCh
	}
}

func (s *StateSaver) IsStarted() bool {
	return s.watcher != nil
}

func (s *StateSaver) DoneCh() chan struct{} {
	return s.doneCh
}

func (s *StateSaver) FsEventHandler(event fsnotify.Event) {
	s.saversLock.RLock()
	defer s.saversLock.RUnlock()

	if s.runner == nil {
		log.ErrorF("Possible bug!!! The state watcher got fs event while not started!")
	}

	if event.Op&fsnotify.Write != fsnotify.Write {
		return
	}
	log.DebugF("modified state file: %s\n", event.Name)
	if app.IsDebug {
		fs.CreateFileBackup(event.Name)
	}

	if len(s.saversDestinations) == 0 {
		log.DebugF("Not found state saversDestinations. Skip. %s\n", event.Name)
		return
	}

	outputs, err := OnlyState(context.Background(), s.runner)
	if err != nil {
		log.ErrorF("Parse intermediate state: %v\n", err)
		return
	}

	log.DebugLn("Save intermediate state...")
	wg := &sync.WaitGroup{}
	hasError := int32(0)
	for _, saver := range s.saversDestinations {
		svr := saver
		wg.Add(1)
		go func() {
			defer wg.Done()

			err = svr.SaveState(outputs)
			if err != nil {
				log.ErrorF("Save intermediate state error: %v\n", err)
				atomic.StoreInt32(&hasError, 1)
				return
			}
		}()
	}

	wg.Wait()

	if (s.stopped || s.runner.stopped) && hasError == 0 {
		log.DebugF("Infrastructure state is saved.\n")
	}
}

func (s *StateSaver) addDestinations(destinations ...SaverDestination) {
	s.saversLock.Lock()
	defer s.saversLock.Unlock()

	s.saversDestinations = append(s.saversDestinations, destinations...)
}

var _ SaverDestination = &cacheDestination{}

type cacheDestination struct {
	runner *Runner
}

func (d *cacheDestination) SaveState(outputs *PipelineOutputs) error {
	if len(outputs.InfrastructureState) == 0 {
		log.DebugF("state is empty. Skip\n")
		return nil
	}
	name := d.runner.stateName()
	log.DebugF("Intermediate save state %s in cache...\n", name)
	err := d.runner.stateCache.Save(name, outputs.InfrastructureState)
	msg := fmt.Sprintf("Intermediate state %s in cache was saved\n", name)
	if err != nil {
		msg = fmt.Sprintf("Intermediate state %s in cache was not saved: %v\n", name, err)
	}
	log.DebugF(msg)
	return err
}

func getCacheDestination(runner *Runner) *cacheDestination {
	if !runner.stateCache.NeedIntermediateSave() {
		return nil
	}

	return &cacheDestination{
		runner: runner,
	}
}
