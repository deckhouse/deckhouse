package scope

import (
	infrav1 "caps-controller-manager/api/infrastructure/v1alpha1"
	"context"
	"github.com/pkg/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
)

// MachineScope defines a scope defined around a machine and its cluster.
type MachineScope struct {
	*Scope
	ClusterScope *ClusterScope

	Machine       *clusterv1.Machine
	StaticMachine *infrav1.StaticMachine
}

// NewMachineScope creates a new machine scope.
func NewMachineScope(
	scope *Scope,
	clusterScope *ClusterScope,
	machine *clusterv1.Machine,
	staticMachine *infrav1.StaticMachine,
) (*MachineScope, error) {
	if scope == nil {
		return nil, errors.New("Scope is required when creating a MachineScope")
	}
	if clusterScope == nil {
		return nil, errors.New("ClusterScope is required when creating a MachineScope")
	}
	if machine == nil {
		return nil, errors.New("Machine is required when creating a MachineScope")
	}
	if staticMachine == nil {
		return nil, errors.New("StaticMachine is required when creating a MachineScope")
	}

	patchHelper, err := patch.NewHelper(staticMachine, scope.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	scope.PatchHelper = patchHelper

	return &MachineScope{
		Scope:         scope,
		ClusterScope:  clusterScope,
		Machine:       machine,
		StaticMachine: staticMachine,
	}, nil
}

// Patch updates the StaticMachine resource.
func (m *MachineScope) Patch(ctx context.Context) error {
	conditions.SetSummary(m.StaticMachine,
		conditions.WithConditions(infrav1.StaticMachineStaticInstanceReadyCondition),
		conditions.WithStepCounterIf(m.StaticMachine.ObjectMeta.DeletionTimestamp.IsZero()),
		conditions.WithStepCounter(),
	)

	err := m.PatchHelper.Patch(
		ctx,
		m.StaticMachine,
		patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
			clusterv1.ReadyCondition,
			infrav1.StaticMachineStaticInstanceReadyCondition,
		}})
	if err != nil {
		return errors.Wrap(err, "failed to patch StaticMachine")
	}

	return nil
}

// Fail marks the StaticMachine as failed.
func (m *MachineScope) Fail(reason capierrors.MachineStatusError, err error) {
	m.StaticMachine.Status.FailureReason = &reason

	failureMessage := err.Error()
	m.StaticMachine.Status.FailureMessage = &failureMessage
}

// HasFailed returns the failure state of the machine scope.
func (m *MachineScope) HasFailed() bool {
	return m.StaticMachine.Status.FailureReason != nil || m.StaticMachine.Status.FailureMessage != nil
}

// Close the MachineScope by updating the machine spec and status.
func (m *MachineScope) Close(ctx context.Context) error {
	return m.Patch(ctx)
}
