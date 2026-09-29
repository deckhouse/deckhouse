// Copyright 2026 Flant JSC
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

package nelm

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/werf/nelm/pkg/legacy/progrep"
)

func TestForwardReports(t *testing.T) {
	const release = "release"

	report := func(id string) progrep.ProgressReport {
		return progrep.ProgressReport{Operations: []progrep.Operation{{ID: id}}}
	}

	// A send the forwarder never takes blocks the nelm action that makes it.
	send := func(t *testing.T, ch chan<- progrep.ProgressReport, r progrep.ProgressReport) {
		t.Helper()

		select {
		case ch <- r:
		case <-time.After(time.Second):
			t.Fatal("the forwarder did not take the report")
		}
	}

	// The action defers stop, so a stop that hangs hangs the action.
	stopWithin := func(t *testing.T, stop func()) {
		t.Helper()

		stopped := make(chan struct{})
		go func() {
			stop()
			close(stopped)
		}()

		select {
		case <-stopped:
		case <-time.After(time.Second):
			t.Fatal("stop did not return")
		}
	}

	t.Run("forwards reports in order and leaves on channel close", func(t *testing.T) {
		seen := make(chan progrep.ProgressReport, 10)
		reportCh, stop := forwardReports(release, func(name string, r progrep.ProgressReport) {
			assert.Equal(t, release, name)
			seen <- r
		})

		send(t, reportCh, report("a"))
		send(t, reportCh, report("b"))

		for _, want := range []progrep.ProgressReport{report("a"), report("b")} {
			select {
			case got := <-seen:
				assert.Equal(t, want, got)
			case <-time.After(time.Second):
				t.Fatalf("report %v was not forwarded", want)
			}
		}

		// What the nelm action does when it returns. Without the ok check the
		// forwarder would spin on the closed channel and forward empty reports.
		close(reportCh)
		stopWithin(t, stop)

		assert.Len(t, seen, 0, "reports forwarded after the channel was closed")
	})

	t.Run("stop ends the forwarder while the channel stays open", func(t *testing.T) {
		_, stop := forwardReports(release, func(string, progrep.ProgressReport) {})

		stopWithin(t, stop)
	})

	t.Run("nil callback still drains the channel", func(t *testing.T) {
		reportCh, stop := forwardReports(release, nil)

		for _, id := range []string{"a", "b", "c"} {
			send(t, reportCh, report(id))
		}

		close(reportCh)
		stopWithin(t, stop)
	})
}
