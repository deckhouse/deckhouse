// Copyright 2025 Flant JSC
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

package cron

import (
	"sync"
	"testing"
	"time"

	smtypes "github.com/flant/shell-operator/pkg/schedule_manager/types"
	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/deckhouse/pkg/log"
)

func newTestManager() *Manager {
	return NewManager(log.NewNop())
}

func TestAddAndRemoveEntry(t *testing.T) {
	mgr := newTestManager()
	e := smtypes.ScheduleEntry{Crontab: "* * * * *", Id: "id1"}

	mgr.Add(e)
	assert.Contains(t, mgr.entries, e.Crontab)
	assert.Contains(t, mgr.entries[e.Crontab].ids, e.Id)

	mgr.Remove(e)
	assert.NotContains(t, mgr.entries, e.Crontab)
}

func TestAddDuplicateId(t *testing.T) {
	mgr := newTestManager()
	e := smtypes.ScheduleEntry{Crontab: "* * * * *", Id: "id1"}
	mgr.Add(e)
	mgr.Add(e) // duplicate
	assert.Equal(t, 1, len(mgr.entries[e.Crontab].ids))
}

func TestAddMultipleIdsSameCrontab(t *testing.T) {
	mgr := newTestManager()
	entry1 := smtypes.ScheduleEntry{Crontab: "* * * * *", Id: "id1"}
	entry2 := smtypes.ScheduleEntry{Crontab: "* * * * *", Id: "id2"}
	mgr.Add(entry1)
	mgr.Add(entry2)
	assert.Contains(t, mgr.entries[entry1.Crontab].ids, entry1.Id)
	assert.Contains(t, mgr.entries[entry2.Crontab].ids, entry2.Id)
	assert.Equal(t, 2, len(mgr.entries[entry1.Crontab].ids))

	mgr.Remove(entry1)
	assert.NotContains(t, mgr.entries[entry1.Crontab].ids, entry1.Id)
	assert.Contains(t, mgr.entries[entry1.Crontab].ids, entry2.Id)
	mgr.Remove(entry2)
	assert.NotContains(t, mgr.entries, entry1.Crontab)
}

func TestRemoveNonExistentEntry(_ *testing.T) {
	mgr := newTestManager()
	e := smtypes.ScheduleEntry{Crontab: "* * * * *", Id: "id1"}
	mgr.Remove(e) // should not panic or error
}

func TestRemoveNonExistentId(t *testing.T) {
	mgr := newTestManager()
	entry1 := smtypes.ScheduleEntry{Crontab: "* * * * *", Id: "id1"}
	entry2 := smtypes.ScheduleEntry{Crontab: "* * * * *", Id: "id2"}
	mgr.Add(entry1)
	mgr.Remove(entry2) // should not remove entry1
	assert.Contains(t, mgr.entries[entry1.Crontab].ids, entry1.Id)
}

func TestAddEmptyCrontab(t *testing.T) {
	mgr := newTestManager()
	e := smtypes.ScheduleEntry{Crontab: "", Id: "id1"}
	mgr.Add(e)
	// Should not panic, and entry is not added
	assert.NotContains(t, mgr.entries, "")
}

func TestStartAndStop(_ *testing.T) {
	mgr := newTestManager()
	mgr.Start()
	mgr.Stop()
}

func TestChReturnsChannel(t *testing.T) {
	mgr := newTestManager()
	ch := mgr.Ch()
	assert.NotNil(t, ch)
}

func TestScheduleFires(t *testing.T) {
	mgr := newTestManager()
	e := smtypes.ScheduleEntry{Crontab: "@every 0.1s", Id: "id1"}
	mgr.Add(e)
	mgr.Start()
	defer mgr.Stop()

	select {
	case v := <-mgr.Ch():
		assert.Equal(t, e.Crontab, v)
	case <-time.After(2 * time.Second):
		t.Fatal("schedule did not fire in time")
	}
}

func TestConcurrentAddRemove(_ *testing.T) {
	mgr := newTestManager()
	mgr.Start()
	defer mgr.Stop()
	e := smtypes.ScheduleEntry{Crontab: "@every 0.2s", Id: "id1"}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		for i := 0; i < 10; i++ {
			mgr.Add(smtypes.ScheduleEntry{Crontab: e.Crontab, Id: "id" + string(rune(i))})
		}
		wg.Done()
	}()
	go func() {
		for i := 0; i < 10; i++ {
			mgr.Remove(smtypes.ScheduleEntry{Crontab: e.Crontab, Id: "id" + string(rune(i))})
		}
		wg.Done()
	}()
	wg.Wait()
}
