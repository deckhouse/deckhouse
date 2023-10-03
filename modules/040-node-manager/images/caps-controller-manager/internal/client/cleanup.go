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
	deckhousev1 "caps-controller-manager/api/deckhouse.io/v1alpha1"
	infrav1 "caps-controller-manager/api/infrastructure/v1alpha1"
	"caps-controller-manager/internal/scope"
	"caps-controller-manager/internal/ssh"
	"context"
	"github.com/pkg/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
)

// Cleanup runs the cleanup script on StaticInstance.
func (c *Client) Cleanup(ctx context.Context, instanceScope *scope.InstanceScope) error {
	switch instanceScope.GetPhase() {
	case deckhousev1.StaticInstanceStatusCurrentStatusPhaseRunning:
		err := c.cleanupFromRunningPhase(ctx, instanceScope)
		if err != nil {
			return errors.Wrap(err, "failed to clean up StaticInstance from running phase")
		}
	case deckhousev1.StaticInstanceStatusCurrentStatusPhaseCleaning:
		err := c.cleanupFromCleaningPhase(ctx, instanceScope)
		if err != nil {
			return errors.Wrap(err, "failed to clean up StaticInstance from cleaning phase")
		}
	default:
		return errors.New("StaticInstance is not running or cleaning")
	}

	return nil
}

func (c *Client) cleanupFromRunningPhase(ctx context.Context, instanceScope *scope.InstanceScope) error {
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

	instanceScope.Instance.Status.MachineRef = nil
	instanceScope.Instance.Status.NodeRef = nil
	instanceScope.Instance.Status.CurrentStatus = nil

	conditions.MarkFalse(instanceScope.Instance, infrav1.StaticInstanceBootstrapSucceededCondition, infrav1.StaticInstanceWaitingForNodeRefReason, clusterv1.ConditionSeverityInfo, "")

	instanceScope.SetPhase(deckhousev1.StaticInstanceStatusCurrentStatusPhasePending)

	err := instanceScope.Patch(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to patch StaticInstance phase")
	}

	return nil
}

func (c *Client) cleanup(instanceScope *scope.InstanceScope) bool {
	done := c.spawn(instanceScope.MachineScope.StaticMachine.Spec.ProviderID, func() bool {
		err := ssh.ExecSSHCommand(instanceScope, "test -d /opt/deckhouse || exit 0 && bash /var/lib/bashible/cleanup_static_node.sh --yes-i-am-sane-and-i-understand-what-i-am-doing", nil)
		if err != nil {
			instanceScope.Logger.Error(err, "Failed to clean up StaticInstance: failed to exec ssh command")

			return false
		}

		return true
	})
	if !done {
		instanceScope.Logger.Info("Cleaning is not finished yet, waiting...")
	}

	return done
}
