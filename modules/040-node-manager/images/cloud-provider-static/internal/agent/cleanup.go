package agent

import (
	deckhousev1 "cloud-provider-static/api/deckhouse.io/v1alpha1"
	infrav1 "cloud-provider-static/api/infrastructure/v1alpha1"
	"cloud-provider-static/internal/providerid"
	"cloud-provider-static/internal/scope"
	"cloud-provider-static/internal/ssh"
	"context"
	"encoding/base64"
	"fmt"
	"github.com/pkg/errors"
	"os"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
)

// Cleanup runs the cleanup script on the static instance.
func (a *Agent) Cleanup(ctx context.Context, instanceScope *scope.InstanceScope) error {
	if instanceScope.GetPhase() != deckhousev1.StaticInstanceStatusCurrentStatusPhaseRunning {
		return errors.New("StaticInstance is not running")
	}

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

// FinishCleaning finishes the cleanup process by checking if the cleanup script was successful and patching the static instance.
func (a *Agent) FinishCleaning(ctx context.Context, instanceScope *scope.InstanceScope) error {
	err := a.cleanup(instanceScope)
	if err != nil {
		return err
	}

	bashibleDirExists, err := ssh.ExecSSHCommandToString(instanceScope, "test ! -d /var/lib/bashible && echo 'true'")
	if err != nil {
		return errors.Wrap(err, "failed to check Bashible directory")
	}

	if bashibleDirExists != "true" {
		return errors.New("Bashible directory exist")
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
	cleanupScript, err := os.ReadFile("cleanup_static_node.sh")
	if err != nil {
		return errors.Wrap(err, "failed to read cleanup script")
	}

	a.lock(providerid.ProviderID(instanceScope.MachineScope.StaticMachine.Spec.ProviderID), func() {
		err := ssh.ExecSSHCommand(instanceScope, fmt.Sprintf("export PROVIDER_ID='%s' && echo '%s' | base64 -d | bash", instanceScope.MachineScope.StaticMachine.Spec.ProviderID, base64.StdEncoding.EncodeToString(cleanupScript)), nil)
		if err != nil {
			instanceScope.Logger.Error(err, "Failed to cleanup StaticInstance: failed to exec ssh command")
		}
	})

	return nil
}
