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

package checks

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

// shortBudget shrinks the wait for a machine that is booting. The real one is minutes, which is
// right on a cloud and useless in a test.
func shortBudget(t *testing.T, attempts int) {
	t.Helper()

	original := masterAPIBudget
	masterAPIBudget.attempts = attempts
	masterAPIBudget.wait = 10 * time.Millisecond
	masterAPIBudget.dial = 200 * time.Millisecond
	t.Cleanup(func() { masterAPIBudget = original })
}

// listening returns the address of a socket that accepts connections for the duration of the test.
func listening(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	return listener.Addr().String()
}

// closedPort returns an address nothing is listening on.
func closedPort(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	require.NoError(t, listener.Close())

	return address
}

func endpointFunc(endpoint *MasterAPIEndpoint, err error) func(context.Context) (*MasterAPIEndpoint, error) {
	return func(context.Context) (*MasterAPIEndpoint, error) { return endpoint, err }
}

// TestImmutableAPIReachable: a security group that does not allow the API port used to surface as
// the credentials handoff waiting in silence — which is also what a node that is simply still
// installing looks like. Telling those apart is the whole check.
func TestImmutableAPIReachable(t *testing.T) {
	t.Run("the master answers", func(t *testing.T) {
		shortBudget(t, 5)
		stopped := false

		check := ImmutableAPIReachable(endpointFunc(&MasterAPIEndpoint{
			Dial:    listening(t),
			Master:  "10.1.0.5:6443",
			Bastion: "bastion.example.com",
			Stop:    func() { stopped = true },
		}, nil))

		detail, err := check.Run(context.Background())

		require.NoError(t, err)
		assert.Equal(t, "10.1.0.5:6443 answers through the bastion bastion.example.com", detail)
		assert.True(t, stopped, "the tunnel the check opened has to be closed again")
	})

	t.Run("nothing is listening", func(t *testing.T) {
		shortBudget(t, 3)

		check := ImmutableAPIReachable(endpointFunc(&MasterAPIEndpoint{
			Dial:    closedPort(t),
			Master:  "10.1.0.5:6443",
			Bastion: "bastion.example.com",
			Stop:    func() {},
		}, nil))

		_, err := check.Run(context.Background())

		require.Error(t, err)
		var failure *preflight.Failure
		require.ErrorAs(t, err, &failure)

		// The reader needs the machine, the hop, and what to change — none of which the
		// silent wait ever said.
		assert.Contains(t, failure.Checked, "10.1.0.5:6443")
		assert.Contains(t, failure.Checked, "through the bastion bastion.example.com")
		assert.Contains(t, failure.Fix, "security group")
		assert.Contains(t, failure.Fix, "the bastion bastion.example.com")
	})

	t.Run("without a bastion the fix names this host", func(t *testing.T) {
		shortBudget(t, 2)

		check := ImmutableAPIReachable(endpointFunc(&MasterAPIEndpoint{
			Dial:   closedPort(t),
			Master: "10.1.0.5:6443",
			Stop:   func() {},
		}, nil))

		_, err := check.Run(context.Background())

		require.Error(t, err)
		var failure *preflight.Failure
		require.ErrorAs(t, err, &failure)
		assert.NotContains(t, failure.Checked, "bastion")
		assert.Contains(t, failure.Fix, "from this host")
	})

	t.Run("the master is not an immutable one", func(t *testing.T) {
		// Every other cloud bootstrap runs the same suite, and a check that cannot apply must
		// say so rather than pass.
		_, err := ImmutableAPIReachable(nil).Run(context.Background())

		require.ErrorIs(t, err, preflight.ErrNotApplicable)
		assert.Contains(t, err.Error(), "not an immutable one")
	})

	t.Run("the address is not known yet", func(t *testing.T) {
		_, err := ImmutableAPIReachable(endpointFunc(nil, nil)).Run(context.Background())

		require.ErrorIs(t, err, preflight.ErrNotApplicable)
	})

	t.Run("the tunnel could not be opened", func(t *testing.T) {
		opening := errors.New("ssh: tcpip-forward request denied by peer")

		_, err := ImmutableAPIReachable(endpointFunc(nil, opening)).Run(context.Background())

		require.ErrorIs(t, err, opening)
	})

	t.Run("a cancelled run stops waiting", func(t *testing.T) {
		// The budget is minutes; a Ctrl-C must not have to sit through it.
		shortBudget(t, 10_000)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		check := ImmutableAPIReachable(endpointFunc(&MasterAPIEndpoint{
			Dial:   closedPort(t),
			Master: "10.1.0.5:6443",
			Stop:   func() {},
		}, nil))

		done := make(chan error, 1)
		go func() { _, err := check.Run(ctx); done <- err }()

		select {
		case err := <-done:
			require.Error(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("the check kept waiting after its context was cancelled")
		}
	})
}
