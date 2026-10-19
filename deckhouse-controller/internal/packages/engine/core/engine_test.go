// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package core_test

import (
	"context"
	"sync"
	"testing"

	"github.com/Masterminds/semver/v3"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/core"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
)

// fakeScheduler is a converged scheduler without nodes.
type fakeScheduler struct {
	wake chan struct{}
	once sync.Once
}

func (f *fakeScheduler) CheckConstraints(string, schedule.Constraints) error { return nil }
func (f *fakeScheduler) CheckDependents(string, *semver.Version) error       { return nil }
func (f *fakeScheduler) AddNode(schedule.Node) error                         { return nil }
func (f *fakeScheduler) RemoveNode(string)                                   {}
func (f *fakeScheduler) Publish(string, schedule.Facts)                      {}
func (f *fakeScheduler) SetIntent(string, *bool)                             {}
func (f *fakeScheduler) Resume()                                             {}
func (f *fakeScheduler) Recheck()                                            {}
func (f *fakeScheduler) Wake() <-chan struct{}                               { return f.wake }
func (f *fakeScheduler) Take() []schedule.Event                              { return nil }
func (f *fakeScheduler) Converged() bool                                     { return true }
func (f *fakeScheduler) Stop()                                               { f.once.Do(func() { close(f.wake) }) }

func TestReadyIgnoresModulesBootDidNotRestore(t *testing.T) {
	e := core.New(core.Config{}, &fakeScheduler{wake: make(chan struct{})}, core.Deps{})
	t.Cleanup(e.Shutdown)

	// A first install: passed to Boot without a version, so nothing is restored, and never applied.
	err := e.Boot(context.Background(), nil, []model.BootPackage{{Name: "first-install", Kind: model.KindModule}})
	if err != nil {
		t.Fatalf("Boot() = %v", err)
	}

	e.Resume()

	if !e.Ready() {
		t.Error("Ready() = false, want true: a module Boot did not restore must not hold readiness")
	}
}
