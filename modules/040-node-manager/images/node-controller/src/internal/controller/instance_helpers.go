/*
Copyright 2025 Flant JSC

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

package controller

import (
	"context"
	"fmt"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var MachineNamespace = "d8-cloud-instance-manager"

type instanceSourceType string

const (
	instanceSourceNone    instanceSourceType = ""
	instanceSourceMachine instanceSourceType = "machine"
	instanceSourceNode    instanceSourceType = "node"
)

type instanceSource struct {
	Type       instanceSourceType
	MachineRef *deckhousev1alpha2.MachineRef
	NodeName   string
}

type BashibleStatusFactory interface {
	FromConditions(conditions []deckhousev1alpha2.InstanceCondition) deckhousev1alpha2.BashibleStatus
}

type bashibleStatusFactory struct{}

func NewBashibleStatusFactory() BashibleStatusFactory {
	return &bashibleStatusFactory{}
}

func ensureInstanceExists(
	ctx context.Context,
	c client.Client,
	name string,
	spec deckhousev1alpha2.InstanceSpec,
) (*deckhousev1alpha2.Instance, error) {
	instance := &deckhousev1alpha2.Instance{}
	if err := c.Get(ctx, types.NamespacedName{Name: name}, instance); err == nil {
		return instance, nil
	} else if !apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("get instance %q: %w", name, err)
	}

	newInstance := &deckhousev1alpha2.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: spec,
	}

	if err := c.Create(ctx, newInstance); err != nil {
		if apierrors.IsAlreadyExists(err) {
			if err := c.Get(ctx, types.NamespacedName{Name: name}, instance); err != nil {
				return nil, fmt.Errorf("get instance %q after already exists: %w", name, err)
			}
			return instance, nil
		}

		return nil, fmt.Errorf("create instance %q: %w", name, err)
	}

	return newInstance, nil
}

func getInstanceSource(instance *deckhousev1alpha2.Instance) instanceSource {
	if instance.Spec.MachineRef != nil && instance.Spec.MachineRef.Name != "" {
		return instanceSource{
			Type:       instanceSourceMachine,
			MachineRef: instance.Spec.MachineRef,
		}
	}

	nodeName := instance.Spec.NodeRef.Name
	if nodeName == "" {
		return instanceSource{Type: instanceSourceNone}
	}

	return instanceSource{
		Type:     instanceSourceNode,
		NodeName: nodeName,
	}
}

func (f *bashibleStatusFactory) FromConditions(
	conditions []deckhousev1alpha2.InstanceCondition,
) deckhousev1alpha2.BashibleStatus {
	var bashibleReady *deckhousev1alpha2.InstanceCondition
	var waitingApproval *deckhousev1alpha2.InstanceCondition
	var waitingDisruptionApproval *deckhousev1alpha2.InstanceCondition

	for i := range conditions {
		condition := &conditions[i]
		switch condition.Type {
		case deckhousev1alpha2.InstanceConditionTypeBashibleReady:
			bashibleReady = condition
		case deckhousev1alpha2.InstanceConditionTypeWaitingApproval:
			waitingApproval = condition
		case deckhousev1alpha2.InstanceConditionTypeWaitingDisruptionApproval:
			waitingDisruptionApproval = condition
		}
	}

	if isConditionTrue(waitingApproval) || isConditionTrue(waitingDisruptionApproval) {
		return deckhousev1alpha2.BashibleStatusWaitingApproval
	}

	if bashibleReady == nil {
		return deckhousev1alpha2.BashibleStatusUnknown
	}

	switch bashibleReady.Status {
	case metav1.ConditionTrue:
		return deckhousev1alpha2.BashibleStatusReady
	case metav1.ConditionFalse:
		return deckhousev1alpha2.BashibleStatusError
	default:
		return deckhousev1alpha2.BashibleStatusUnknown
	}
}

func isConditionTrue(condition *deckhousev1alpha2.InstanceCondition) bool {
	return condition != nil && condition.Status == metav1.ConditionTrue
}

func (r *CAPIMachineReconciler) deleteInstanceIfExists(ctx context.Context, name string) (bool, error) {
	instance := &deckhousev1alpha2.Instance{ObjectMeta: metav1.ObjectMeta{Name: name}}
	if err := r.Delete(ctx, instance); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return false, nil
		}
		return false, fmt.Errorf("delete instance %q: %w", name, err)
	}

	return true, nil
}
