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

package bootstrap

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
	libretry "github.com/deckhouse/lib-dhctl/pkg/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/immutable"
	"github.com/deckhouse/deckhouse/dhctl/pkg/immutable/immutabletest"
)

// handoffTestLoop keeps the loop real — init_test.go collapses every loop to one
// attempt — and drops the wait, so a rebuild test costs milliseconds. The break
// predicate is the handoff's own; both call sites are covered by their own tests.
func handoffTestLoop(t *testing.T, attempts int) *libretry.Loop {
	t.Helper()

	immutabletest.NoRetryCollapse(t)

	return libretry.NewLoop("waiting in a test", attempts, time.Millisecond).BreakIf(handoffGaveUp)
}

// The machine reboots as part of installing itself; a dial that hangs to gossh's
// deadline ends the accept loop for good, and the listener stays bound, so every
// later attempt reaches a port nobody serves. On a live stand: 348 wasted.
func TestHandoffRebuildsTheChannelAfterItDies(t *testing.T) {
	var opened int
	openChannel := func(context.Context) (string, func(), error) {
		opened++
		return "127.0.0.1:0", func() {}, nil
	}
	attempts := 0
	fetch := func(address string) error {
		attempts++
		if attempts < 3 {
			return errors.New("read tcp 127.0.0.1:44849: connection refused")
		}
		return nil
	}

	if err := retryWithFreshChannel(t.Context(), handoffTestLoop(t, 5), openChannel, fetch); err != nil {
		t.Fatalf("want success on the third attempt, got %v", err)
	}
	if opened != 3 {
		t.Fatalf("the channel must be rebuilt on every attempt: opened %d times", opened)
	}
}

// The channel narrates for as long as it lives, not only while it opens: the
// tunnel reports every connection it forwards and the dial that kills it. A
// buffer replayed once open returned kept only the opening, and reading it
// while the tunnel still wrote was a data race (go test -race).
func TestTheChannelKeepsNarratingAfterItOpens(t *testing.T) {
	var narration bytes.Buffer
	ctx := dhlog.ToContext(t.Context(), dhlog.NewBufferLogger(&narration))

	openChannel := func(ctx context.Context) (string, func(), error) {
		dhlog.FromContext(ctx).InfoContext(ctx, "the channel is open")
		return "127.0.0.1:0", func() { dhlog.FromContext(ctx).InfoContext(ctx, "the channel is closing") }, nil
	}

	if err := retryWithFreshChannel(ctx, handoffTestLoop(t, 1), openChannel, func(string) error { return nil }); err != nil {
		t.Fatal(err)
	}
	for _, said := range []string{"the channel is open", "the channel is closing"} {
		if !strings.Contains(narration.String(), said) {
			t.Fatalf("%q never reached the log; what the channel says after it opened is lost:\n%s", said, narration.String())
		}
	}
}

// One tunnel per attempt is only affordable if it is also closed per attempt:
// 360 leaked listeners and their bastion connections are worse than the dead
// listener this rebuild exists to survive.
func TestHandoffClosesEveryChannelItOpens(t *testing.T) {
	terminal := fmt.Errorf("collect the kubeconfig: %w", immutable.ErrHandoffAlreadyServed)

	cases := []struct {
		name      string
		do        func(string) error
		wantOpens int
		wantErr   bool
	}{
		{"collected on the first attempt", func(string) error { return nil }, 1, false},
		{"every attempt fails", func(string) error { return errors.New("connection refused") }, 3, true},
		{"the node refuses for good", func(string) error { return terminal }, 1, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var opened, closed int
			openChannel := func(context.Context) (string, func(), error) {
				opened++
				return "127.0.0.1:0", func() { closed++ }, nil
			}

			err := retryWithFreshChannel(t.Context(), handoffTestLoop(t, 3), openChannel, c.do)
			if (err != nil) != c.wantErr {
				t.Fatalf("want error %v, got %v", c.wantErr, err)
			}
			if opened != c.wantOpens {
				t.Fatalf("want %d channels opened, got %d", c.wantOpens, opened)
			}
			if closed != opened {
				t.Fatalf("%d of %d channels were left open", opened-closed, opened)
			}
		})
	}
}

// A cancelled wait is the path a defer is easiest to lose: the operator hits
// Ctrl-C and the tunnel outlives the loop that owned it.
func TestHandoffClosesTheChannelWhenTheWaitIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var opened, closed int
	openChannel := func(context.Context) (string, func(), error) {
		opened++
		return "127.0.0.1:0", func() { closed++ }, nil
	}

	err := retryWithFreshChannel(ctx, handoffTestLoop(t, 3), openChannel, func(string) error {
		cancel()
		return errors.New("connection refused")
	})
	if err == nil {
		t.Fatal("a cancelled wait must not report success")
	}
	if opened != 1 || closed != 1 {
		t.Fatalf("opened %d channels and closed %d", opened, closed)
	}
}

// The bastion is not always there when the master reboots either. An open that
// failed hands back no closer, so calling it would panic, and it is a transient
// the wait has to sit through rather than a reason to end the bootstrap.
func TestHandoffRetriesAChannelThatCannotBeOpened(t *testing.T) {
	var opened, closed, fetched int
	openChannel := func(context.Context) (string, func(), error) {
		opened++
		if opened < 3 {
			return "", nil, errors.New("connect to the bastion host 198.51.100.7: connection refused")
		}
		return "127.0.0.1:0", func() { closed++ }, nil
	}

	err := retryWithFreshChannel(t.Context(), handoffTestLoop(t, 5), openChannel, func(string) error {
		fetched++
		return nil
	})
	if err != nil {
		t.Fatalf("want success once the bastion answers, got %v", err)
	}
	if fetched != 1 {
		t.Fatalf("a channel that never opened must not be fetched through: %d fetches", fetched)
	}
	if opened != 3 || closed != 1 {
		t.Fatalf("opened %d channels and closed %d", opened, closed)
	}
}

// The helper can be right while both callers still open one channel for the whole
// loop, which is the bug itself. Without a bastion nothing observable separates
// the two, so the wiring is guarded at the source.
func TestBothWaitsUseAFreshChannelPerAttempt(t *testing.T) {
	waits := []struct{ file, function string }{
		{"steps_immutable_handoff.go", "func (b *ClusterBootstrapper) collectImmutableKubeconfig"},
		{"steps_immutable.go", "func (b *ClusterBootstrapper) pushImmutablePayload"},
	}

	for _, wait := range waits {
		t.Run(wait.function, func(t *testing.T) {
			src, err := os.ReadFile(wait.file)
			if err != nil {
				t.Fatalf("read %s: %v", wait.file, err)
			}

			body := string(src)
			start := strings.Index(body, wait.function)
			if start < 0 {
				t.Fatalf("%s is gone; this guard needs rewriting against whatever replaced it", wait.function)
			}
			end := strings.Index(body[start:], "\n}\n")
			if end < 0 {
				t.Fatalf("cannot delimit %s", wait.function)
			}
			fn := body[start : start+end]

			if !strings.Contains(fn, "retryWithFreshChannel(") {
				t.Fatal("the wait must run every attempt through a channel of its own")
			}
			if strings.Contains(fn, "defer stop()") {
				t.Fatal("a channel held for the whole wait dies with the machine's boot and every later attempt dials a dead port")
			}
		})
	}
}

// A channel is opened every attempt, so the SSH progress behind it repeats every
// few seconds for a wait that runs for minutes. On a live stand it printed three
// lines per attempt and buried the only line that carried the node's own state.
// The compact terminal drops a record tagged file-only and the debug file keeps
// it (lib-dhctl's routeToTTY), so the tag is the whole contract.
func TestOpeningAChannelIsNotNarratedOnEveryAttempt(t *testing.T) {
	var records bytes.Buffer
	ctx := dhlog.ToContext(context.Background(), dhlog.NewBufferLogger(&records))

	openChannel := func(ctx context.Context) (string, func(), error) {
		dhlog.FromContext(ctx).InfoContext(ctx, "Get SSH client")
		return "127.0.0.1:0", func() {}, nil
	}

	if err := retryWithFreshChannel(ctx, handoffTestLoop(t, 1), openChannel, func(string) error { return nil }); err != nil {
		t.Fatalf("wait: %v", err)
	}

	if !strings.Contains(records.String(), "Get SSH client") {
		t.Fatal("the SSH progress must still reach the debug log; it is how a channel that will not open is diagnosed")
	}
	if !strings.Contains(records.String(), `msg="Get SSH client" file_only=true`) {
		t.Errorf("opening a channel must narrate file-only, off the compact terminal: it repeats every attempt of a minutes-long wait\n%s", records.String())
	}
}

// The node stops answering while kubelet takes the machine over — four minutes of
// it on a live stand — and a wait that says nothing for that long reads as a hang.
func TestTheWaitRepeatsItselfWhileNothingChanges(t *testing.T) {
	var terminal bytes.Buffer
	ctx := dhlog.ToContext(context.Background(), dhlog.NewBufferLogger(&terminal))

	now := time.Date(2026, 8, 25, 20, 0, 0, 0, time.UTC)
	report := waitProgress(ctx, func() time.Time { return now })

	report("master-0 is not answering its bootstrap channel yet")
	now = now.Add(5 * time.Second)
	report("master-0 is not answering its bootstrap channel yet")
	now = now.Add(waitProgressInterval)
	report("master-0 is not answering its bootstrap channel yet")

	if got := strings.Count(terminal.String(), "not answering"); got != 2 {
		t.Errorf("the same message must be repeated once per %s, said it %d times", waitProgressInterval, got)
	}
	if !strings.Contains(terminal.String(), "(35s so far)") {
		t.Errorf("a repeat must carry how long the wait has been running, got:\n%s", terminal.String())
	}
}
