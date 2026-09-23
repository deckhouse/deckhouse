/*
Copyright 2026 Flant JSC

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

package decision

import (
	"testing"
	"time"
)

func TestRestrictionLog(t *testing.T) {
	t.Parallel()
	now := time.Now()
	l := &RestrictionLog{now: func() time.Time { return now }}

	if !l.Allow("late") {
		t.Fatal("the first time a rule restricts is worth a line")
	}
	// Every request of every subject the rule names comes through here.
	for i := 0; i < 1000; i++ {
		if l.Allow("late") {
			t.Fatalf("request %d wrote a second line for the same rule within the hour", i)
		}
	}
	// A different rule is a different event.
	if !l.Allow("other") {
		t.Error("a second rule must be reported on its own")
	}

	// While it goes on, it is repeated - but hourly, not per request.
	now = now.Add(restrictionRepeat)
	if !l.Allow("late") {
		t.Error("a rule that is still restricting an hour later is worth saying again")
	}

	// And once it stops, the memo lets go, so the next episode is a first one again.
	now = now.Add(restrictionForget)
	if !l.Allow("late") {
		t.Error("a new episode must be reported, not swallowed by the previous one")
	}

	l.mu.Lock()
	held := len(l.reported)
	l.mu.Unlock()
	if held != 1 {
		t.Errorf("the memo holds %d rules; the ones that stopped restricting must be dropped", held)
	}
}

func TestRestrictionLog_NilIsSilent(t *testing.T) {
	t.Parallel()
	var l *RestrictionLog
	if l.Allow("late") {
		t.Error("a nil log allows nothing rather than panicking")
	}
}
