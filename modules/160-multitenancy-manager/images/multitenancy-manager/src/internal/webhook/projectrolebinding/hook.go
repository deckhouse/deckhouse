/*
Copyright 2026 Flant JSC

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

package projectrolebinding

import (
	"context"
	"fmt"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/yaml"

	"controller/apis/deckhouse.io/v1alpha3"
	rolebindingwebhook "controller/internal/webhook/rolebinding"
)

// Register installs the ProjectRoleBinding validating webhook.
func Register(runtimeManager manager.Manager) {
	hook := &webhook.Admission{Handler: &validator{client: runtimeManager.GetClient()}}
	runtimeManager.GetWebhookServer().Register("/validate/v1alpha3/projectrolebindings", hook)
}

type validator struct {
	client client.Client
}

func (v *validator) Handle(ctx context.Context, req admission.Request) admission.Response {
	prb := new(v1alpha3.ProjectRoleBinding)
	raw := req.Object.Raw
	if req.Operation == admissionv1.Delete {
		raw = req.OldObject.Raw
	}
	if err := yaml.Unmarshal(raw, prb); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// The "binding must live in the main namespace of an existing, non-virtual project" check only
	// guards the *placement* of a binding, so it is enforced on CREATE alone. Enforcing it on UPDATE
	// would deadlock project teardown: the controller removes the PRB finalizer with an Update, and
	// by then the owning project is terminating or already gone, so the lookup below would deny the
	// finalizer removal and the project's main namespace could never be cleaned up. An UPDATE can
	// never move a binding into a different namespace, and the shared validator still applies the
	// managed-by, role-scope and privilege-escalation checks on every operation.
	if req.Operation == admissionv1.Create {
		if req.Namespace == "default" || req.Namespace == "deckhouse" {
			return admission.Denied("ProjectRoleBinding cannot be created in a virtual project namespace")
		}
		project := new(v1alpha3.Project)
		if err := v.client.Get(ctx, client.ObjectKey{Name: req.Namespace}, project); err != nil {
			if apierrors.IsNotFound(err) {
				return admission.Denied(fmt.Sprintf("namespace %q is not the main namespace of a project", req.Namespace))
			}
			return admission.Errored(http.StatusInternalServerError, err)
		}
		if project.Labels[v1alpha3.ProjectLabelVirtualProject] == "true" {
			return admission.Denied("ProjectRoleBinding cannot be created in a virtual project namespace")
		}
	}

	return rolebindingwebhook.Validate(ctx, v.client, req, rolebindingwebhook.Input{
		RoleRefKind: prb.Spec.RoleRef.Kind,
		RoleRefName: prb.Spec.RoleRef.Name,
		Subjects:    prb.Spec.Subjects,
		Namespace:   req.Namespace,
		ManagedBy:   prb.Labels[v1alpha3.ResourceLabelManagedBy],
	})
}
