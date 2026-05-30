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
	"errors"
	"io"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
)

func TestTunnelStopClosesPipesAndUnblocksLineConsumer(t *testing.T) {
	stdoutReadPipe, stdoutWritePipe, err := os.Pipe()
	require.NoError(t, err)
	stdinReadPipe, stdinWritePipe, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = stdoutReadPipe.Close()
		_ = stdoutWritePipe.Close()
		_ = stdinReadPipe.Close()
		_ = stdinWritePipe.Close()
	})

	tun := NewTunnel(&session.Session{}, "L", "127.0.0.1:2222:127.0.0.1:22")
	tun.stdoutReadPipe = stdoutReadPipe
	tun.stdoutWritePipe = stdoutWritePipe
	tun.stdinReadPipe = stdinReadPipe
	tun.stdinWritePipe = stdinWritePipe

	consumerDone := make(chan struct{})
	go func() {
		tun.consumeLines(stdoutReadPipe, nil)
		close(consumerDone)
	}()

	tun.Stop()
	tun.Stop()

	select {
	case <-consumerDone:
	case <-time.After(time.Second):
		require.FailNow(t, "Stop must close stdout read pipe and unblock consumeLines")
	}

	_, err = stdoutWritePipe.Write([]byte("line\n"))
	require.Error(t, err, "Stop must close stdout write pipe")

	_, err = stdinWritePipe.Write([]byte("line\n"))
	require.Error(t, err, "Stop must close stdin write pipe")

	errCh := make(chan error, 1)
	go func() {
		_, readErr := stdinReadPipe.Read(make([]byte, 1))
		errCh <- readErr
	}()

	select {
	case err = <-errCh:
		errIs := errors.Is(err, os.ErrClosed) || errors.Is(err, io.ErrClosedPipe)
		require.True(t, errIs, "Stop must close stdin read pipe, got %v", err)
	case <-time.After(time.Second):
		require.FailNow(t, "Stop must close stdin read pipe")
	}
}

func TestTunnelStopKillsProcessWithoutHealthMonitor(t *testing.T) {
	cmd := exec.Command("sleep", "60")
	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})

	tun := NewTunnel(&session.Session{}, "L", "127.0.0.1:2222:127.0.0.1:22")
	tun.sshCmd = cmd

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	tun.Stop()

	select {
	case <-waitCh:
	case <-time.After(time.Second):
		require.FailNow(t, "Stop must kill ssh process without HealthMonitor")
	}
}

func TestTunnelStopBeforeHealthMonitorStartsKillsProcessAndStopsMonitor(t *testing.T) {
	cmd := exec.Command("sleep", "60")
	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})

	tun := NewTunnel(&session.Session{}, "L", "127.0.0.1:2222:127.0.0.1:22")
	tun.sshCmd = cmd

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	tun.Stop()

	select {
	case err := <-waitCh:
		tun.errorCh <- err
	case <-time.After(time.Second):
		require.FailNow(t, "Stop must kill ssh process before HealthMonitor starts")
	}

	monitorDone := make(chan struct{})
	go func() {
		tun.HealthMonitor(make(chan error))
		close(monitorDone)
	}()

	select {
	case <-monitorDone:
	case <-time.After(time.Second):
		require.FailNow(t, "HealthMonitor must observe earlier Stop and exit")
	}
}

func TestTunnelStopWithoutSessionStopsHealthMonitor(t *testing.T) {
	tun := NewTunnel(nil, "L", "127.0.0.1:2222:127.0.0.1:22")

	monitorDone := make(chan struct{})
	go func() {
		tun.HealthMonitor(make(chan error))
		close(monitorDone)
	}()

	tun.Stop()

	select {
	case <-monitorDone:
	case <-time.After(time.Second):
		require.FailNow(t, "Stop must stop HealthMonitor even when session is missing")
	}
}

func TestTunnelUpResetsErrorChannelBeforeReuse(t *testing.T) {
	sess := session.NewSession(session.Input{
		AvailableHosts: []session.Host{{Host: "127.0.0.1", Name: "localhost"}},
		User:           "nobody",
		Port:           "1",
		ExtraArgs:      "-o ConnectTimeout=1",
	})
	sess.AgentSettings = &session.AgentSettings{}
	tun := NewTunnel(sess, "L", "127.0.0.1:2222:127.0.0.1:22")
	t.Cleanup(tun.Stop)

	oldErrorCh := tun.errorCh
	staleErr := errors.New("stale wait error")
	oldErrorCh <- staleErr

	err := tun.Up()
	require.Error(t, err)
	require.NotContains(t, err.Error(), staleErr.Error())
	require.NotEqual(t, oldErrorCh, tun.errorCh)
}

func TestTunnelWaitForSSHUsesProvidedCommandAndErrorChannel(t *testing.T) {
	waitCmd := exec.Command("sh", "-c", "exit 7")
	require.NoError(t, waitCmd.Start())

	tunnelCmd := exec.Command("sleep", "60")
	require.NoError(t, tunnelCmd.Start())
	t.Cleanup(func() {
		if tunnelCmd.ProcessState == nil {
			_ = tunnelCmd.Process.Kill()
			_ = tunnelCmd.Wait()
		}
	})

	tun := NewTunnel(&session.Session{}, "L", "127.0.0.1:2222:127.0.0.1:22")
	tun.sshCmd = tunnelCmd
	tun.errorCh = make(chan error, 1)
	waitErrorCh := make(chan error, 1)

	tun.waitForSSH(waitCmd, waitErrorCh, tunnelPipes{})

	select {
	case err := <-waitErrorCh:
		require.Error(t, err)
	case <-time.After(time.Second):
		require.FailNow(t, "waitForSSH must send wait result to the provided error channel")
	}

	select {
	case err := <-tun.errorCh:
		require.FailNow(t, "waitForSSH must not send to Tunnel.errorCh", "got %v", err)
	default:
	}

	require.NoError(t, syscall.Kill(tunnelCmd.Process.Pid, 0))
}

func TestTunnelWaitForSSHClosesPipesAndUnblocksLineConsumer(t *testing.T) {
	stdoutReadPipe, stdoutWritePipe, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = stdoutReadPipe.Close()
		_ = stdoutWritePipe.Close()
	})

	waitCmd := exec.Command("sh", "-c", "exit 0")
	require.NoError(t, waitCmd.Start())

	tun := NewTunnel(&session.Session{}, "L", "127.0.0.1:2222:127.0.0.1:22")
	tun.stdoutReadPipe = stdoutReadPipe
	tun.stdoutWritePipe = stdoutWritePipe

	consumerDone := make(chan struct{})
	go func() {
		tun.consumeLines(stdoutReadPipe, nil)
		close(consumerDone)
	}()

	waitErrorCh := make(chan error, 1)
	tun.waitForSSH(waitCmd, waitErrorCh, tunnelPipes{
		stdoutReadPipe:  stdoutReadPipe,
		stdoutWritePipe: stdoutWritePipe,
	})

	select {
	case err = <-waitErrorCh:
		require.NoError(t, err)
	case <-time.After(time.Second):
		require.FailNow(t, "waitForSSH must send wait result")
	}

	select {
	case <-consumerDone:
	case <-time.After(time.Second):
		require.FailNow(t, "waitForSSH must close stdout pipes and unblock consumeLines")
	}
}

func TestTunnelWaitForSSHDoesNotClosePipesFromNextRun(t *testing.T) {
	oldReadPipe, oldWritePipe, err := os.Pipe()
	require.NoError(t, err)
	newReadPipe, newWritePipe, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = oldReadPipe.Close()
		_ = oldWritePipe.Close()
		_ = newReadPipe.Close()
		_ = newWritePipe.Close()
	})

	waitCmd := exec.Command("sh", "-c", "sleep 0.1")
	require.NoError(t, waitCmd.Start())

	tun := NewTunnel(&session.Session{}, "L", "127.0.0.1:2222:127.0.0.1:22")
	tun.stdoutReadPipe = oldReadPipe
	tun.stdoutWritePipe = oldWritePipe

	waitErrorCh := make(chan error, 1)
	tun.waitForSSH(waitCmd, waitErrorCh, tunnelPipes{
		stdoutReadPipe:  oldReadPipe,
		stdoutWritePipe: oldWritePipe,
	})

	tun.pipesMutex.Lock()
	tun.stdoutReadPipe = newReadPipe
	tun.stdoutWritePipe = newWritePipe
	tun.pipesMutex.Unlock()

	select {
	case err = <-waitErrorCh:
		require.NoError(t, err)
	case <-time.After(time.Second):
		require.FailNow(t, "waitForSSH must send wait result")
	}

	_, err = newWritePipe.Write([]byte("line\n"))
	require.NoError(t, err, "old waitForSSH must not close pipes from the next run")
}
