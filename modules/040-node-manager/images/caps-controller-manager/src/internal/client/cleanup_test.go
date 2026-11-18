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

package client

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// nopConn is enough for the wait loop, which only closes successful probes.
type nopConn struct{ net.Conn }

func (nopConn) Close() error { return nil }

func stubDialer(t *testing.T, dial func() (net.Conn, error)) {
	t.Helper()

	original := dialTimeout
	t.Cleanup(func() { dialTimeout = original })

	dialTimeout = func(string, string, time.Duration) (net.Conn, error) {
		return dial()
	}
}

// The host must be handed over only after it stops answering: a probe that still connects
// means the reboot has not happened yet and the next StaticMachine would bootstrap on a live
// host.
func TestWaitForNodeDownWaitsUntilTheHostStopsAnswering(t *testing.T) {
	attempts := 0
	stubDialer(t, func() (net.Conn, error) {
		attempts++
		if attempts < 4 {
			return nopConn{}, nil
		}

		return nil, errors.New("connection refused")
	})

	require.NoError(t, waitForNodeDownWith(context.Background(), "192.168.0.1:22", time.Second, time.Millisecond))
	require.Equal(t, 4, attempts)
}

func TestWaitForNodeDownFailsWhenTheHostNeverGoesDown(t *testing.T) {
	stubDialer(t, func() (net.Conn, error) {
		return nopConn{}, nil
	})

	err := waitForNodeDownWith(context.Background(), "192.168.0.1:22", 20*time.Millisecond, 5*time.Millisecond)

	require.Error(t, err)
	require.Contains(t, err.Error(), "still reachable")
}

func TestWaitForNodeDownReturnsWhenTheContextIsCancelled(t *testing.T) {
	stubDialer(t, func() (net.Conn, error) {
		return nopConn{}, nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := waitForNodeDownWith(ctx, "192.168.0.1:22", time.Second, time.Millisecond)

	require.ErrorIs(t, err, context.Canceled)
}
