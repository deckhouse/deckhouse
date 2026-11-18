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
	"bytes"
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/pkg/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	deckhousev1 "caps-controller-manager/api/deckhouse.io/v1alpha2"
	infrav1 "caps-controller-manager/api/infrastructure/v1alpha1"
	"caps-controller-manager/internal/ssh"
	"caps-controller-manager/internal/ssh/clissh"
	"caps-controller-manager/internal/ssh/gossh"
)

// The cleanup script returns as soon as it has scheduled the reboot, while the host keeps
// answering for a few more seconds. Handing the instance to another StaticMachine at that
// moment is what starts the bootstrap loop this wait exists to prevent.
const (
	// cleanupNodeDownTimeout bounds the wait for the host to stop answering after the
	// cleanup script has scheduled its reboot. The cleanup timeout in the StaticMachine
	// controller still bounds the whole Cleaning phase; this wait only adds the time the
	// host needs to actually go down.
	cleanupNodeDownTimeout = 3 * time.Minute
	// cleanupNodeDownPollInterval is how often the host is probed while it goes down.
	cleanupNodeDownPollInterval = 2 * time.Second
	// cleanupNodeDownDialTimeout bounds a single TCP probe.
	cleanupNodeDownDialTimeout = 2 * time.Second
)

// dialTimeout is a seam for tests that simulate a host going away.
var dialTimeout = net.DialTimeout

// Cleanup runs the cleanup script on StaticInstance.
func (c *Client) Cleanup(ctx context.Context,
	staticInstance *deckhousev1.StaticInstance,
	staticMachine *infrav1.StaticMachine,
	machine *clusterv1.Machine) error {
	logger := ctrl.LoggerFrom(ctx)
	phase := staticInstance.GetPhase()

	credentials := &deckhousev1.SSHCredentials{}
	if err := c.client.Get(ctx, client.ObjectKey{Name: staticInstance.Spec.CredentialsRef.Name}, credentials); err != nil {
		return fmt.Errorf("failed to load SSHCredentials: %w", err)
	}

	sshLegacyMode := true //nolint:staticcheck
	if len(credentials.Spec.PrivateSSHKey) == 0 {
		sshLegacyMode = false
	}

	switch phase {
	case deckhousev1.StaticInstanceStatusCurrentStatusPhaseBootstrapping, deckhousev1.StaticInstanceStatusCurrentStatusPhaseRunning:
		if err := c.cleanup(ctx, staticInstance, staticMachine, credentials.Spec, sshLegacyMode); err != nil {
			return fmt.Errorf("failed to clean up StaticInstance from running phase: %w", err)
		}
	case deckhousev1.StaticInstanceStatusCurrentStatusPhaseCleaning:
		if err := c.cleanup(ctx, staticInstance, staticMachine, credentials.Spec, sshLegacyMode); err != nil {
			return fmt.Errorf("failed to clean up StaticInstance from cleaning phase: %w", err)
		}
	case deckhousev1.StaticInstanceStatusCurrentStatusPhasePending:
		if !canSkipCleanupForPendingPhase(staticInstance, staticMachine, machine) {
			return errors.New("StaticInstance is pending outside delete flow")
		}
		// During machine deletion, StaticInstance can still be Pending.
		// In this case cleanup is a no-op and deletion should proceed.
		logger.Info("Skipping cleanup for StaticInstance in pending phase during deletion", "phase", phase)
	default:
		return errors.New("StaticInstance is not running or cleaning")
	}
	return nil
}

func canSkipCleanupForPendingPhase(staticInstance *deckhousev1.StaticInstance, staticMachine *infrav1.StaticMachine, machine *clusterv1.Machine) bool {
	if staticMachine == nil || machine == nil {
		return false
	}

	if staticMachine.DeletionTimestamp.IsZero() || machine.DeletionTimestamp.IsZero() {
		return false
	}

	if staticInstance.Status.MachineRef == nil {
		return true
	}

	return staticInstance.Status.MachineRef.UID == staticMachine.UID
}

// waitForNodeDown waits until the host stops accepting TCP connections on the ssh port.
//
// The cleanup script returns as soon as it has scheduled `shutdown -r -t 5`, while the host
// keeps answering for a few more seconds. Returning the StaticInstance to the pool at that
// moment lets the next StaticMachine pass the TCP and ssh checks against a host that is about
// to reboot: the wrapper writes bootstrap-token, the reboot kills the ssh session, and every
// following attempt fails with exit code 1 because the token is still there.
func waitForNodeDown(ctx context.Context, address string) error {
	return waitForNodeDownWith(ctx, address, cleanupNodeDownTimeout, cleanupNodeDownPollInterval)
}

func waitForNodeDownWith(ctx context.Context, address string, timeout, interval time.Duration) error {
	logger := ctrl.LoggerFrom(ctx).WithValues("address", address)
	deadline := time.Now().Add(timeout)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for attempt := 1; ; attempt++ {
		conn, err := dialTimeout("tcp", address, cleanupNodeDownDialTimeout)
		if err != nil {
			logger.Info("Host stopped answering, cleanup is complete", "attempt", attempt)
			return nil
		}
		_ = conn.Close()

		if time.Now().After(deadline) {
			return fmt.Errorf("host %s is still reachable after %s", address, timeout)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (c *Client) cleanup(ctx context.Context,
	staticInstance *deckhousev1.StaticInstance,
	staticMachine *infrav1.StaticMachine,
	credentials deckhousev1.SSHCredentialsSpec,
	sshLegacyMode bool) error {
	logger := ctrl.LoggerFrom(ctx)

	type taskDataStr struct {
		address       string
		credentials   deckhousev1.SSHCredentialsSpec
		sshLegacyMode bool
	}

	taskFunc := func(tCtx context.Context, data any) error {
		tLogger := ctrl.LoggerFrom(tCtx)
		dataStr, ok := data.(taskDataStr)
		if !ok {
			return errors.New("invalid task data")
		}

		var sshCl ssh.SSH
		var err error
		if dataStr.sshLegacyMode {
			tLogger.Info("using clissh")
			sshCl = clissh.CreateSSHClient(dataStr.address, dataStr.credentials)
		} else {
			tLogger.Info("using gossh")
			sshCl, err = gossh.CreateSSHClient(dataStr.address, dataStr.credentials)
		}
		if err != nil {
			tLogger.Error(err, "failed to create ssh client")
			return fmt.Errorf("failed to create ssh client: %w", err)
		}

		var stdout, stderr bytes.Buffer
		err = sshCl.ExecSSHCommand(tCtx, "if [ ! -f /var/lib/bashible/cleanup_static_node.sh ]; then rm -rf /var/lib/bashible; (sleep 5 && shutdown -r now) & else bash /var/lib/bashible/cleanup_static_node.sh --yes-i-am-sane-and-i-understand-what-i-am-doing; fi", &stdout, &stderr)
		if err != nil {
			output := tailOutput(combinedOutput(stdout.String(), stderr.String()))
			tLogger.Error(err, "failed to exec ssh command", "output", output)
			return fmt.Errorf("failed to exec ssh command: %w", &remoteCommandError{err: err, output: output})
		}

		address := net.JoinHostPort(dataStr.address, strconv.Itoa(dataStr.credentials.SSHPort))
		tLogger.Info("Waiting for the host to stop answering after cleanup",
			"address", address, "timeout", cleanupNodeDownTimeout.String())
		if err := waitForNodeDown(tCtx, address); err != nil {
			tLogger.Error(err, "host did not stop answering after cleanup")
			return fmt.Errorf("failed to wait for the host to stop answering: %w", err)
		}

		return nil
	}

	staticInstance.SetPhase(deckhousev1.StaticInstanceStatusCurrentStatusPhaseCleaning)

	taskData := taskDataStr{
		address:       staticInstance.Spec.Address,
		credentials:   credentials,
		sshLegacyMode: sshLegacyMode,
	}

	logger = logger.WithValues("taskID", string(staticMachine.UID))
	logger.Info("Running cleanup task")
	err, finished := c.taskManager.Spawn(c.taskManagerCtx, string(staticMachine.UID), "cleanup", taskData, taskFunc)
	if err != nil {
		var cmdErr *remoteCommandError
		if errors.As(err, &cmdErr) {
			c.recorder.SendWarningEvent(staticInstance, staticMachine.Labels["node-group"], "CleanupScriptFailed", cmdErr.EventMessage())
		}

		return err
	}

	if !finished {
		logger.Info("Cleaning is not finished yet, waiting...")
		return nil
	}

	logger.Info("Cleanup completed, the host is not reachable")
	c.recorder.SendNormalEvent(staticInstance, staticMachine.Labels["node-group"], "CleanupScriptSucceeded", "Cleanup completed, the host is not reachable")
	staticInstance.ToPending()

	return nil
}
