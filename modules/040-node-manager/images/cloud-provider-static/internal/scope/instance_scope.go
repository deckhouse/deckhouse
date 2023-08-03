package scope

import (
	infrav1 "cloud-provider-static/api/v1alpha1"
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"

	"github.com/pkg/errors"
	"sigs.k8s.io/cluster-api/util/patch"
)

// InstanceScope defines a scope defined around an instance and its machine.
type InstanceScope struct {
	*Scope
	MachineScope *MachineScope

	Instance    *infrav1.StaticInstance
	Credentials *infrav1.StaticInstanceCredentials
}

// NewInstanceScope creates a new instance scope.
func NewInstanceScope(
	scope *Scope,
	staticInstance *infrav1.StaticInstance,
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

	return &InstanceScope{
		Scope:    scope,
		Instance: staticInstance,
	}, nil
}

// GetPhase returns the current phase of the static instance.
func (i *InstanceScope) GetPhase() infrav1.StaticInstanceStatusCurrentStatusPhase {
	if i.Instance.Status.CurrentStatus == nil {
		return ""
	}

	return i.Instance.Status.CurrentStatus.Phase
}

// SetPhase sets the current phase of the static instance.
func (i *InstanceScope) SetPhase(phase infrav1.StaticInstanceStatusCurrentStatusPhase) {
	if i.Instance.Status.CurrentStatus == nil {
		i.Instance.Status.CurrentStatus = &infrav1.StaticInstanceStatusCurrentStatus{}
	}

	i.Instance.Status.CurrentStatus.Phase = phase
	i.Instance.Status.CurrentStatus.LastUpdateTime = metav1.NewTime(time.Now().UTC())
}

// Patch updates the StaticInstance resource.
func (i *InstanceScope) Patch(ctx context.Context) error {
	conditions.SetSummary(i.Instance,
		conditions.WithConditions(
			infrav1.StaticInstanceAddedToNodeGroupCondition,
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

// Close the InstanceScope by updating the instance spec and status.
func (i *InstanceScope) Close(ctx context.Context) error {
	return i.Patch(ctx)
}
