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

package scope

import (
	"context"
	"time"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"

	deckhousev1 "caps-controller-manager/api/deckhouse.io/v1alpha2"
	infrav1 "caps-controller-manager/api/infrastructure/v1alpha1"
	"caps-controller-manager/internal/event"
)

// InstanceScope defines a scope defined around an instance and its machine.
type InstanceScope struct {
	*Scope
	MachineScope *MachineScope

	Instance      *deckhousev1.StaticInstance
	Credentials   *deckhousev1.SSHCredentials
	SSHLegacyMode bool
}

// NewInstanceScope creates a new instance scope.
func NewInstanceScope(
	scope *Scope,
	staticInstance *deckhousev1.StaticInstance,
	ctx context.Context,
) (*InstanceScope, error) {
	if scope == nil {
		return nil, errors.New("Scope is required when creating an InstanceScope")
	}
	if staticInstance == nil {
		return nil, errors.New("StaticInstance is required when creating an InstanceScope")
	}

	patchHelper, err := patch.NewHelper(staticInstance, scope.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	scope.PatchHelper = patchHelper

	instanceScope := &InstanceScope{
		Scope:         scope,
		Instance:      staticInstance,
		SSHLegacyMode: true,
	}

	instanceScope.setLoggerContext()

	return instanceScope, nil
}

// LoadSSHCredentials loads the SSHCredentials for the InstanceScope.
func (i *InstanceScope) LoadSSHCredentials(ctx context.Context, recorder *event.Recorder) error {
	credentials := &deckhousev1.SSHCredentials{}
	credentialsKey := k8sClient.ObjectKey{
		Name: i.Instance.Spec.CredentialsRef.Name,
	}

	err := i.Client.Get(ctx, credentialsKey, credentials)
	if err != nil {
		var nodeGroup string

		if i.MachineScope != nil {
			nodeGroup = i.MachineScope.StaticMachine.Labels["node-group"]
		}
		recorder.SendWarningEvent(i.Instance, nodeGroup, "StaticInstanceCredentialsUnavailable", err.Error())
		return errors.Wrap(err, "failed to get StaticInstance credentials")
	}

	i.Credentials = credentials
	if len(i.Credentials.Spec.PrivateSSHKey) == 0 {
		i.SSHLegacyMode = false
	}

	return nil
}

func (i *InstanceScope) AttachMachineScope(machineScope *MachineScope) {
	i.MachineScope = machineScope
	i.setLoggerContext()
}

// GetPhase returns the current phase of the static instance.
func (i *InstanceScope) GetPhase() deckhousev1.StaticInstanceStatusCurrentStatusPhase {
	if i.Instance.Status.CurrentStatus == nil {
		return ""
	}

	return i.Instance.Status.CurrentStatus.Phase
}

// SetPhase sets the current phase of the static instance.
func (i *InstanceScope) SetPhase(phase deckhousev1.StaticInstanceStatusCurrentStatusPhase) {
	prevPhase := i.GetPhase()

	if i.Instance.Status.CurrentStatus == nil {
		i.Instance.Status.CurrentStatus = &deckhousev1.StaticInstanceStatusCurrentStatus{}
	}

	i.Instance.Status.CurrentStatus.Phase = phase
	i.Instance.Status.CurrentStatus.LastUpdateTime = metav1.NewTime(time.Now().UTC())
	i.setLoggerContext()

	if prevPhase != phase {
		i.Logger.Info("StaticInstance phase changed", "from", prevPhase, "to", phase)
	}
}

// Patch updates the StaticInstance resource.
func (i *InstanceScope) Patch(ctx context.Context) error {
	conditions.SetSummary(i.Instance,
		conditions.WithConditions(
			infrav1.StaticInstanceBootstrapSucceededCondition,
		),
		conditions.WithStepCounterIf(i.Instance.ObjectMeta.DeletionTimestamp.IsZero()),
		conditions.WithStepCounter(),
	)

	err := i.PatchHelper.Patch(
		ctx,
		i.Instance,
		patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
			clusterv1.ReadyCondition,
			infrav1.StaticInstanceAddedToNodeGroupCondition,
			infrav1.StaticInstanceBootstrapSucceededCondition,
		}})
	if err != nil {
		return errors.Wrap(err, "failed to patch StaticInstance")
	}

	return nil
}

func (i *InstanceScope) ToPending(ctx context.Context) error {
	i.Instance.Status.MachineRef = nil
	i.Instance.Status.NodeRef = nil
	i.Instance.Status.CurrentStatus = nil
	i.setLoggerContext()

	conditions.MarkFalse(i.Instance, infrav1.StaticInstanceBootstrapSucceededCondition, infrav1.StaticInstanceWaitingForNodeRefReason, clusterv1.ConditionSeverityInfo, "")

	i.SetPhase(deckhousev1.StaticInstanceStatusCurrentStatusPhasePending)

	err := i.Patch(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to set StaticInstance to Pending phase")
	}

	return nil
}

// Close the InstanceScope by updating the instance spec and status.
func (i *InstanceScope) Close(ctx context.Context) error {
	return i.Patch(ctx)
}

func (i *InstanceScope) setLoggerContext() {
	phase := "unknown"
	if i.Instance.Status.CurrentStatus != nil && i.Instance.Status.CurrentStatus.Phase != "" {
		phase = string(i.Instance.Status.CurrentStatus.Phase)
	}

	i.Logger = i.Scope.Logger.WithValues("phase", phase)
}
