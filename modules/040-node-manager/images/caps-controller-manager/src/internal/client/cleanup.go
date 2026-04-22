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
	"fmt"

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

	sshLegacyMode := true
	if len(credentials.Spec.PrivateSSHKey) == 0 {
		sshLegacyMode = false
	}

	switch phase {
	case
		deckhousev1.StaticInstanceStatusCurrentStatusPhaseBootstrapping,
		deckhousev1.StaticInstanceStatusCurrentStatusPhaseRunning:
		err := c.cleanup(ctx, staticInstance, staticMachine, credentials.Spec, sshLegacyMode)
		if err != nil {
			return fmt.Errorf("failed to clean up StaticInstance from running phase: %w", err)
		}
	case deckhousev1.StaticInstanceStatusCurrentStatusPhaseCleaning:
		err := c.cleanup(ctx, staticInstance, staticMachine, credentials.Spec, sshLegacyMode)
		if err != nil {
			return fmt.Errorf("failed to clean up StaticInstance from cleaning phase: %w", err)
		}
	case
		deckhousev1.StaticInstanceStatusCurrentStatusPhasePending:
		if !canSkipCleanupForPendingPhase(staticInstance, staticMachine, machine) {
			return errors.New("StaticInstance is pending outside delete flow")
		}
		// During machine deletion, StaticInstance can still be Pending.
		// In this case cleanup is a no-op and deletion should proceed.
		logger.V(1).Info("Skipping cleanup for StaticInstance in pending phase during deletion", "phase", phase)
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

func (c *Client) cleanup(ctx context.Context,
	staticInstance *deckhousev1.StaticInstance,
	staticMachine *infrav1.StaticMachine,
	credentials deckhousev1.SSHCredentialsSpec,
	sshLegacyMode bool) error {
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
			tLogger.V(1).Info("using clissh")
			sshCl = clissh.CreateSSHClient(dataStr.address, dataStr.credentials)
		} else {
			tLogger.V(1).Info("using gossh")
			sshCl, err = gossh.CreateSSHClient(dataStr.address, dataStr.credentials)
		}
		if err != nil {
			tLogger.Error(err, "failed to create ssh client")
			return fmt.Errorf("failed to create ssh client: %w", err)
		}
		err = sshCl.ExecSSHCommand("if [ ! -f /var/lib/bashible/cleanup_static_node.sh ]; then rm -rf /var/lib/bashible; (sleep 5 && shutdown -r now) & else bash /var/lib/bashible/cleanup_static_node.sh --yes-i-am-sane-and-i-understand-what-i-am-doing; fi", nil, nil)
		if err != nil {
			tLogger.Error(err, "failed to exec ssh command")
			return fmt.Errorf("failed to exec ssh command: %w", err)
		}
		return nil
	}

	staticInstance.SetPhase(deckhousev1.StaticInstanceStatusCurrentStatusPhaseCleaning)

	taskData := taskDataStr{
		address:       staticInstance.Spec.Address,
		credentials:   credentials,
		sshLegacyMode: sshLegacyMode,
	}

	taskCtx := ctrl.LoggerInto(c.taskManagerCtx, ctrl.LoggerFrom(ctx))
	err, finished := c.taskManager.Spawn(taskCtx, string(staticMachine.Spec.ProviderID), "cleanup", taskData, taskFunc)
	if err != nil {
		return err
	}

	logger := ctrl.LoggerFrom(ctx)
	if finished {
		logger.V(1).Info("Cleanup script executed successfully")
		c.recorder.SendNormalEvent(staticInstance, staticMachine.Labels["node-group"], "CleanupScriptSucceeded", "Cleanup script executed successfully")
		staticInstance.ToPending()
		return nil
	}

	logger.V(1).Info("Cleaning is not finished yet, waiting...")
	return nil
}
