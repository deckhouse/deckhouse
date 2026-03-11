/*
Copyright 2023 Flant JSC

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

	"github.com/pkg/errors"

	"caps-controller-manager/internal/scope"
	"caps-controller-manager/internal/ssh"
	"caps-controller-manager/internal/ssh/clissh"
	"caps-controller-manager/internal/ssh/gossh"

	deckhousev1 "caps-controller-manager/api/deckhouse.io/v1alpha2"
)

// Cleanup runs the cleanup script on StaticInstance.
func (c *Client) Cleanup(ctx context.Context, instanceScope *scope.InstanceScope) error {
	phase := instanceScope.GetPhase()

	switch phase {
	case
		deckhousev1.StaticInstanceStatusCurrentStatusPhaseBootstrapping,
		deckhousev1.StaticInstanceStatusCurrentStatusPhaseRunning:
		err := c.cleanupFromBootstrappingOrRunningPhase(ctx, instanceScope)
		if err != nil {
			return errors.Wrap(err, "failed to clean up StaticInstance from running phase")
		}
	case deckhousev1.StaticInstanceStatusCurrentStatusPhaseCleaning:
		err := c.cleanupFromCleaningPhase(ctx, instanceScope)
		if err != nil {
			return errors.Wrap(err, "failed to clean up StaticInstance from cleaning phase")
		}
	case
		deckhousev1.StaticInstanceStatusCurrentStatusPhasePending:
		if !canSkipCleanupForPendingPhase(instanceScope) {
			return errors.New("StaticInstance is pending outside delete flow")
		}
		// During machine deletion, StaticInstance can still be Pending.
		// In this case cleanup is a no-op and deletion should proceed.
		instanceScope.Logger.V(1).Info("Skipping cleanup for StaticInstance in pending phase during deletion", "phase", phase)
	default:
		return errors.New("StaticInstance is not running or cleaning")
	}

	return nil
}

func canSkipCleanupForPendingPhase(instanceScope *scope.InstanceScope) bool {
	if instanceScope.MachineScope == nil || instanceScope.MachineScope.StaticMachine == nil || instanceScope.MachineScope.Machine == nil {
		return false
	}

	if instanceScope.MachineScope.StaticMachine.DeletionTimestamp.IsZero() || instanceScope.MachineScope.Machine.DeletionTimestamp.IsZero() {
		return false
	}

	if instanceScope.Instance.Status.MachineRef == nil {
		return true
	}

	return instanceScope.Instance.Status.MachineRef.UID == instanceScope.MachineScope.StaticMachine.UID
}

func (c *Client) cleanupFromBootstrappingOrRunningPhase(ctx context.Context, instanceScope *scope.InstanceScope) error {
	instanceScope.SetPhase(deckhousev1.StaticInstanceStatusCurrentStatusPhaseCleaning)

	err := instanceScope.Patch(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to patch StaticInstance phase")
	}

	c.cleanup(instanceScope)

	return nil
}

// cleanupFromCleaningPhase finishes the cleanup process by checking if the cleanup script was successfully executed and patching StaticInstance.
func (c *Client) cleanupFromCleaningPhase(ctx context.Context, instanceScope *scope.InstanceScope) error {
	done := c.cleanup(instanceScope)
	if !done {
		return nil
	}

	err := instanceScope.ToPending(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to set StaticInstance to Pending phase")
	}

	return nil
}

func (c *Client) cleanup(instanceScope *scope.InstanceScope) bool {
	done := c.cleanupTaskManager.spawn(taskID(instanceScope.MachineScope.StaticMachine.Spec.ProviderID), func() bool {
		var sshCl ssh.SSH
		var err error
		if instanceScope.SSHLegacyMode {
			instanceScope.Logger.V(1).Info("using clissh")
			sshCl, err = clissh.CreateSSHClient(instanceScope)
		} else {
			instanceScope.Logger.V(1).Info("using gossh")
			sshCl, err = gossh.CreateSSHClient(instanceScope)
		}
		if err != nil {
			instanceScope.Logger.Error(err, "Failed to clean up StaticInstance: failed to create ssh client")
			return false
		}
		err = sshCl.ExecSSHCommand(instanceScope, "if [ ! -f /var/lib/bashible/cleanup_static_node.sh ]; then rm -rf /var/lib/bashible; (sleep 5 && shutdown -r now) & else bash /var/lib/bashible/cleanup_static_node.sh --yes-i-am-sane-and-i-understand-what-i-am-doing; fi", nil, nil)
		if err != nil {
			instanceScope.Logger.Error(err, "Failed to clean up StaticInstance: failed to exec ssh command")
			return false
		}
		return true
	})
	if done != nil && *done {
		c.recorder.SendNormalEvent(instanceScope.Instance, instanceScope.MachineScope.StaticMachine.Labels["node-group"], "CleanupScriptSucceeded", "Cleanup script executed successfully")
		return true
	}
	instanceScope.Logger.V(1).Info("Cleaning is not finished yet, waiting...")
	return false
}
