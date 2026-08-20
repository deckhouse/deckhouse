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
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	libretry "github.com/deckhouse/lib-dhctl/pkg/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/immutable"
)

// handoffTestLoop keeps the loop real — init_test.go collapses every loop to one
// attempt — and drops the wait, so a rebuild test costs milliseconds. The break
// predicate is the handoff's own; both call sites are covered by their own tests.
func handoffTestLoop(t *testing.T, attempts int) *libretry.Loop {
	t.Helper()

	inTestEnvironment := libretry.InTestEnvironment
	libretry.InTestEnvironment = false
	t.Cleanup(func() { libretry.InTestEnvironment = inTestEnvironment })

	return libretry.NewLoop("waiting in a test", attempts, time.Millisecond).BreakIf(handoffGaveUp)
}

// The machine reboots as part of installing itself; a dial that hangs to gossh's
// deadline ends the accept loop for good, and the listener stays bound, so every
// later attempt reaches a port nobody serves. On a live stand: 348 wasted.
func TestHandoffRebuildsTheChannelAfterItDies(t *testing.T) {
	var opened int
	openChannel := func() (string, func(), error) {
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
			openChannel := func() (string, func(), error) {
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
	openChannel := func() (string, func(), error) {
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
	openChannel := func() (string, func(), error) {
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
