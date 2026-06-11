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

package clusterprojectrolebinding

import (
	"context"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/yaml"

	"controller/apis/deckhouse.io/v1alpha3"
	rolebindingwebhook "controller/internal/webhook/rolebinding"
)

// Register installs the ClusterProjectRoleBinding validating webhook.
func Register(runtimeManager manager.Manager) {
	hook := &webhook.Admission{Handler: &validator{client: runtimeManager.GetClient()}}
	runtimeManager.GetWebhookServer().Register("/validate/v1alpha3/clusterprojectrolebindings", hook)
}

type validator struct {
	client client.Client
}

func (v *validator) Handle(ctx context.Context, req admission.Request) admission.Response {
	cprb := new(v1alpha3.ClusterProjectRoleBinding)
	raw := req.Object.Raw
	if req.Operation == admissionv1.Delete {
		raw = req.OldObject.Raw
	}
	if err := yaml.Unmarshal(raw, cprb); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	return rolebindingwebhook.Validate(ctx, v.client, req, rolebindingwebhook.Input{
		RoleRefKind: cprb.Spec.RoleRef.Kind,
		RoleRefName: cprb.Spec.RoleRef.Name,
		Namespace:   "",
		ManagedBy:   cprb.Labels[v1alpha3.ResourceLabelManagedBy],
	})
}
