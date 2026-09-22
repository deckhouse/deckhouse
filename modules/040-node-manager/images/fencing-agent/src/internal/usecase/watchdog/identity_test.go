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

package watchdog

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
)

const testGuardInterval = time.Millisecond

func TestIdentityGuardStopsTheAgentWhenTheOwnNodeWasRecreated(t *testing.T) {
	state := &stubState{state: Snapshot{Observed: true, UIDMismatch: true}}
	events := &fakeEvents{}

	err := NewIdentityGuard(state, events, testGuardInterval, log.NewNop()).Run(t.Context())
	if !errors.Is(err, errIdentityChanged) {
		t.Fatalf("guard returned %v, want the agent to end so a restart re-reads the identity", err)
	}

	if events.count(reasonIdentityChanged) != 1 {
		t.Errorf("identity events: %d, want 1", events.count(reasonIdentityChanged))
	}
}

// The feed loop starts only after the gossip join, so it cannot be the one that
// catches a Node recreated while the agent is still joining.
func TestIdentityGuardReactsWhileTheAgentIsStillJoining(t *testing.T) {
	state := &stubState{}
	guard := NewIdentityGuard(state, &fakeEvents{}, testGuardInterval, log.NewNop())

	errCh := make(chan error, 1)

	go func() { errCh <- guard.Run(t.Context()) }()

	state.set(Snapshot{Observed: true, UIDMismatch: true})

	select {
	case err := <-errCh:
		if !errors.Is(err, errIdentityChanged) {
			t.Fatalf("guard returned %v, want the recreated Node to end the agent", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the guard did not react to the recreated Node")
	}
}

func TestIdentityGuardStopsWithTheContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := NewIdentityGuard(&stubState{state: Snapshot{Observed: true}}, &fakeEvents{}, testGuardInterval, log.NewNop()).Run(ctx)
	if err != nil {
		t.Fatalf("guard returned %v on shutdown, want nil: a shutdown is not a failure", err)
	}
}
