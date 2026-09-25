// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package webhooks

import (
	"context"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	cpadmission "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/admission"
	cpwebhook "github.com/deckhouse/deckhouse/go_lib/cloud-provider/webhook"
	dvpicv1aplha1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/api/instanceclass/v1alpha1"
	dvpval "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/validation"
	dvpadmission "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/validation/admission"
)

type NodeGroupValidator struct {
	factory *dvpval.AdmissionStateBuilderFactory
	object  *unstructured.Unstructured
}

var (
	_ admission.Validator[*unstructured.Unstructured] = (*NodeGroupValidator)(nil)
	_ cpwebhook.Registrar                             = (*NodeGroupValidator)(nil)

	nodeGroupLog = logf.Log.WithName("node-group")
)

func NewNodeGroupValidator(factory *dvpval.AdmissionStateBuilderFactory, object *unstructured.Unstructured) *NodeGroupValidator {
	return &NodeGroupValidator{
		factory: factory,
		object:  object,
	}
}

func (v *NodeGroupValidator) Register(manager ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(manager, v.object).
		WithValidator(v).
		Complete()
}

func (v *NodeGroupValidator) ValidateCreate(ctx context.Context, obj *unstructured.Unstructured) (admission.Warnings, error) {
	if !v.shouldValidateNodeGroup(obj) {
		nodeGroupLog.V(2).Info("skipping validation", "reason", "not DVP-relevant NodeGroup", "name", obj.GetName())
		return nil, nil
	}

	return v.validate(ctx, admissionv1.Create, obj)
}

func (v *NodeGroupValidator) ValidateUpdate(ctx context.Context, oldObj, newObj *unstructured.Unstructured) (admission.Warnings, error) {
	if !v.shouldValidateNodeGroupUpdate(oldObj, newObj) {
		nodeGroupLog.V(2).Info("skipping validation", "reason", "not DVP-relevant NodeGroup update", "name", newObj.GetName())
		return nil, nil
	}

	return v.validate(ctx, admissionv1.Update, newObj)
}

func (v *NodeGroupValidator) ValidateDelete(ctx context.Context, obj *unstructured.Unstructured) (admission.Warnings, error) {
	if !v.shouldValidateNodeGroup(obj) {
		nodeGroupLog.V(2).Info("skipping validation", "reason", "not DVP-relevant NodeGroup delete", "name", obj.GetName())
		return nil, nil
	}

	return v.validate(ctx, admissionv1.Delete, obj)
}

func (v *NodeGroupValidator) validate(
	ctx context.Context,
	operation admissionv1.Operation,
	obj *unstructured.Unstructured,
) (admission.Warnings, error) {
	name := obj.GetName()
	nodeGroupLog.Info(
		"validating resource",
		"operation", operation,
		"resource", "NodeGroup",
		"name", name,
		"namespace", obj.GetNamespace(),
	)

	builder := v.factory.CreateBuilder()
	if operation != admissionv1.Delete {
		builder = builder.
			SetNodeGroup(ctx, obj).
			AddAssociatedInstanceClasses(ctx, name)
	}

	state, err := builder.Build(ctx)
	if err != nil {
		nodeGroupLog.Error(err, "failed to build validation state", "name", name)
		return nil, internalBuildError(err)
	}

	if shouldSkipState(state) {
		nodeGroupLog.V(1).Info("skipping validation during migration")
		return nil, nil
	}

	result := dvpadmission.ValidateNodeGroup(state, operation)

	warnings, admissionErr := resultToAdmission(result)
	if admissionErr != nil {
		errorViolations := result.Errors()
		warningViolations := result.Warnings()

		nodeGroupLog.Info("validation denied", "errors", len(errorViolations), "warnings", len(warningViolations))
		nodeGroupLog.V(1).Info("validation errors", "errors", violationMessages(errorViolations), "warnings", violationMessages(warningViolations))

		return warnings, admissionErr
	}

	nodeGroupLog.Info(
		"validation allowed",
		"operation", operation,
		"resource", "NodeGroup",
		"name", name,
		"namespace", obj.GetNamespace(),
	)

	return warnings, nil
}

func (v *NodeGroupValidator) shouldValidateNodeGroup(obj *unstructured.Unstructured) bool {
	if obj == nil {
		return true
	}

	nodeGroup, err := cpadmission.DecodeNodeGroupObject(obj)
	if err != nil {
		return true
	}

	return v.isDVPRelevantNodeGroup(nodeGroup)
}

func (v *NodeGroupValidator) shouldValidateNodeGroupUpdate(oldObj, newObj *unstructured.Unstructured) bool {
	if v.shouldValidateNodeGroup(newObj) {
		return true
	}

	if oldObj != nil && v.shouldValidateNodeGroup(oldObj) {
		return true
	}

	return false
}

func (v *NodeGroupValidator) isDVPRelevantNodeGroup(nodeGroup *cpapi.NodeGroup) bool {
	if nodeGroup.Spec.NodeType == cpapi.NodeTypeCloudPermanent {
		return true
	}

	if nodeGroup.Spec.CloudInstances == nil || nodeGroup.Spec.CloudInstances.ClassReference == nil {
		return false
	}

	return nodeGroup.Spec.CloudInstances.ClassReference.Kind == dvpicv1aplha1.GroupVersionKind.Kind
}
