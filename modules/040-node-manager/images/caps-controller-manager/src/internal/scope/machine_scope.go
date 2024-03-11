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

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"

	infrav1 "caps-controller-manager/api/infrastructure/v1alpha1"
)

var allowBootstrapRequirement *labels.Requirement

func init() {
	var err error

	allowBootstrapRequirement, err = labels.NewRequirement("node.deckhouse.io/allow-bootstrap", selection.NotIn, []string{"false"})
	if err != nil {
		panic(err.Error())
	}
}

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

// SetReady sets the StaticMachine Ready Status.
func (m *MachineScope) SetReady() {
	m.StaticMachine.Status.Ready = true
}

// SetNotReady sets the StaticMachine Ready Status to false.
func (m *MachineScope) SetNotReady() {
	m.StaticMachine.Status.Ready = false
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

// LabelSelector returns a label selector for the StaticMachine.
func (m *MachineScope) LabelSelector() (labels.Selector, error) {
	if m.StaticMachine.Spec.LabelSelector == nil {
		return labels.NewSelector().Add(*allowBootstrapRequirement), nil
	}

	labelSelector, err := metav1.LabelSelectorAsSelector(m.StaticMachine.Spec.LabelSelector)
	if err != nil {
		return nil, errors.Wrap(err, "unable to convert StaticMachine label selector")
	}

	requirements, _ := labelSelector.Requirements()

	for _, requirement := range requirements {
		if requirement.Key() == allowBootstrapRequirement.Key() {
			return nil, errors.New("label selector requirement for the 'node.deckhouse.io/allow-bootstrap' key can't be added manually")
		}
	}

	return labelSelector.Add(*allowBootstrapRequirement), nil
}

// Close the MachineScope by updating the machine spec and status.
func (m *MachineScope) Close(ctx context.Context) error {
	return m.Patch(ctx)
}
