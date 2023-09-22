package agent

import (
	deckhousev1 "cloud-provider-static/api/deckhouse.io/v1alpha1"
	infrav1 "cloud-provider-static/api/infrastructure/v1alpha1"
	"cloud-provider-static/internal/scope"
	"cloud-provider-static/internal/ssh"
	"context"
	"fmt"
	"github.com/pkg/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
)

// Cleanup runs the cleanup script on the static instance.
func (a *Agent) Cleanup(ctx context.Context, instanceScope *scope.InstanceScope) error {
	switch instanceScope.GetPhase() {
	case deckhousev1.StaticInstanceStatusCurrentStatusPhaseRunning:
		err := a.cleanupFromRunningPhase(ctx, instanceScope)
		if err != nil {
			return errors.Wrap(err, "failed to cleanup StaticInstance from running phase")
		}
	case deckhousev1.StaticInstanceStatusCurrentStatusPhaseCleaning:
		err := a.cleanupFromCleaningPhase(ctx, instanceScope)
		if err != nil {
			return errors.Wrap(err, "failed to cleanup StaticInstance from cleaning phase")
		}
	default:
		return errors.New("StaticInstance is not running or cleaning")
	}

	return nil
}

func (a *Agent) cleanupFromRunningPhase(ctx context.Context, instanceScope *scope.InstanceScope) error {
	instanceScope.SetPhase(deckhousev1.StaticInstanceStatusCurrentStatusPhaseCleaning)

	err := instanceScope.Patch(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to patch StaticInstance phase")
	}

	err = a.cleanup(instanceScope)
	if err != nil {
		return err
	}

	return nil
}

// cleanupFromCleaningPhase finishes the cleanup process by checking if the cleanup script was successful and patching the static instance.
func (a *Agent) cleanupFromCleaningPhase(ctx context.Context, instanceScope *scope.InstanceScope) error {
	err := a.cleanup(instanceScope)
	if err != nil {
		return err
	}

	taskResult := a.getTaskResult(instanceScope.MachineScope.StaticMachine.Spec.ProviderID)

	result, _ := taskResult.(bool)
	if result {
		a.deleteTaskResult(instanceScope.MachineScope.StaticMachine.Spec.ProviderID)
	} else {
		return nil
	}

	instanceScope.Instance.Status.MachineRef = nil
	instanceScope.Instance.Status.NodeRef = nil
	instanceScope.Instance.Status.CurrentStatus = nil

	conditions.MarkFalse(instanceScope.Instance, infrav1.StaticInstanceBootstrapSucceededCondition, infrav1.StaticInstanceWaitingForMachineRefReason, clusterv1.ConditionSeverityInfo, "")
	conditions.MarkFalse(instanceScope.Instance, infrav1.StaticInstanceBootstrapSucceededCondition, infrav1.StaticInstanceWaitingForNodeRefReason, clusterv1.ConditionSeverityInfo, "")

	instanceScope.SetPhase(deckhousev1.StaticInstanceStatusCurrentStatusPhasePending)

	err = instanceScope.Patch(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to patch StaticInstance phase")
	}

	return nil
}

func (a *Agent) cleanup(instanceScope *scope.InstanceScope) error {
	a.spawn(instanceScope.MachineScope.StaticMachine.Spec.ProviderID, func() interface{} {
		err := ssh.ExecSSHCommand(instanceScope, fmt.Sprintf("export PROVIDER_ID='%s' && /var/lib/bashible/cleanup-static-node.sh", instanceScope.MachineScope.StaticMachine.Spec.ProviderID), nil)
		if err != nil {
			instanceScope.Logger.Error(err, "Failed to cleanup StaticInstance: failed to exec ssh command")

			return false
		}

		return true
	})

	return nil
}
