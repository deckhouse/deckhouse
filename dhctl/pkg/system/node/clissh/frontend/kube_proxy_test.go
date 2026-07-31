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

package frontend

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
)

const kubeProxyTestTimeout = 5 * time.Second

func TestKubeProxyRunKubeProxyReturnsCanceledContextBeforeCommandStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	kubeProxy := NewKubeProxy(&session.Session{})

	proxy, port, err := kubeProxy.runKubeProxy(ctx, make(chan error, 1), 1)

	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, proxy)
	require.Empty(t, port)
}

func TestWaitForKubeProxyPortStopsProxyOnTimeout(t *testing.T) {
	oldTimeout := kubeProxyPortReadyTimeout
	kubeProxyPortReadyTimeout = 10 * time.Millisecond
	t.Cleanup(func() {
		kubeProxyPortReadyTimeout = oldTimeout
	})

	stopped := false
	err := waitForKubeProxyPort(
		context.Background(),
		make(chan error, 1),
		make(chan struct{}, 1),
		func() string { return "" },
		func() { stopped = true },
		func(err error) error { return fmt.Errorf("wait error: %w", err) },
		1,
	)

	require.ErrorContains(t, err, "timeout waiting for api proxy port")
	require.True(t, stopped)
}

func TestKubeProxyHealthMonitorStopsTunnelOnContextCancel(t *testing.T) {
	tunnelCmd := exec.Command("sleep", "60")
	require.NoError(t, tunnelCmd.Start())
	t.Cleanup(func() {
		if tunnelCmd.ProcessState == nil {
			_ = tunnelCmd.Process.Kill()
			_ = tunnelCmd.Wait()
		}
	})

	tunnelWaitCh := make(chan error, 1)
	go func() {
		tunnelWaitCh <- tunnelCmd.Wait()
	}()

	startID := 42
	stopCh := make(chan struct{}, 1)
	kubeProxy := NewKubeProxy(&session.Session{})
	kubeProxy.tunnel = NewTunnel(&session.Session{}, "L", "127.0.0.1:2222:127.0.0.1:22")
	kubeProxy.tunnel.sshCmd = tunnelCmd
	kubeProxy.port = "12345"
	kubeProxy.healthMonitorsByStartID[startID] = stopCh

	ctx, cancel := context.WithCancel(context.Background())
	monitorDone := make(chan struct{})
	go func() {
		kubeProxy.healthMonitor(ctx, make(chan error, 1), make(chan error, 1), stopCh, startID)
		close(monitorDone)
	}()

	cancel()

	select {
	case <-monitorDone:
	case <-time.After(kubeProxyTestTimeout):
		require.FailNow(t, "health monitor must stop on context cancellation")
	}

	select {
	case <-tunnelWaitCh:
	case <-time.After(kubeProxyTestTimeout):
		require.FailNow(t, "context cancellation must stop tunnel process")
	}

	require.True(t, kubeProxy.stop)
	require.Nil(t, kubeProxy.tunnel)
	require.Equal(t, "12345", kubeProxy.port)
	require.NotContains(t, kubeProxy.healthMonitorsByStartID, startID)
}
