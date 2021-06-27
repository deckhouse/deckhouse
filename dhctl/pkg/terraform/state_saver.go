// Copyright 2021 Flant CJSC
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

package terraform

import (
	"fmt"

	"github.com/fsnotify/fsnotify"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/fs"
)

type StateSaver struct {
	runner      *Runner
	saveStateFn func(outputs *PipelineOutputs) error
	watcher     *fsnotify.Watcher
	doneCh      chan struct{}
	stopped     bool
}

func NewStateSaver(saveStateFn func(outputs *PipelineOutputs) error) *StateSaver {
	return &StateSaver{
		saveStateFn: saveStateFn,
	}
}

// Start creates a new file watcher for r.statePath and
// a chan to stop it.
func (s *StateSaver) Start(runner *Runner) error {
	if s.stopped {
		return nil
	}
	if s.saveStateFn == nil {
		return nil
	}
	if s.watcher != nil {
		return nil
	}

	if runner == nil {
		return nil
	}
	s.runner = runner

	var err error
	s.doneCh = make(chan struct{})
	s.watcher, err = fs.StartFileWatcher(s.runner.statePath, s.FsEventHandler, s.doneCh)
	if err != nil {
		return fmt.Errorf("fs watcher for intermediate terraform state file: %s: %v", s.runner.statePath, err)
	}

	return nil
}

// Stop is blocked until doneCh is closed.
func (s *StateSaver) Stop() {
	s.stopped = true
	if s.watcher != nil {
		s.watcher.Close()
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
	if s.saveStateFn == nil {
		return
	}

	outputs, err := OnlyState(s.runner)
	if err != nil {
		log.ErrorF("Parse intermediate state: %v\n", err)
		return
	}

	log.DebugLn("Save intermediate state...")

	err = s.saveStateFn(outputs)
	if err != nil {
		log.ErrorF("Save intermediate state: %v\n", err)
		return
	}

	if s.stopped || s.runner.stopped {
		log.InfoF("Terraform state is saved.\n")
	}
}
