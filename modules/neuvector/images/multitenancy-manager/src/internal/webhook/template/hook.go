/*
Copyright 2024 Flant JSC

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

package template

import (
	"context"
	"fmt"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/yaml"

	"controller/apis/deckhouse.io/v1alpha1"
	"controller/apis/deckhouse.io/v1alpha2"
	"controller/internal/validate"
)

func Register(runtimeManager manager.Manager, serviceAccount string) {
	hook := &webhook.Admission{Handler: &validator{client: runtimeManager.GetClient(), serviceAccount: serviceAccount}}
	runtimeManager.GetWebhookServer().Register("/validate/v1alpha1/templates", hook)
}

type validator struct {
	serviceAccount string
	client         client.Client
}

func (v *validator) Handle(_ context.Context, req admission.Request) admission.Response {
	template := new(v1alpha1.ProjectTemplate)
	if req.Operation == admissionv1.Create || req.Operation == admissionv1.Update {
		if err := yaml.Unmarshal(req.Object.Raw, template); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		// cannot create/update invalid template
		if err := validate.ProjectTemplate(template); err != nil {
			return admission.Errored(http.StatusBadRequest, fmt.Errorf("project template validation failed: %v", err))
		}
	}
	if req.Operation == admissionv1.Delete {
		if err := yaml.Unmarshal(req.OldObject.Raw, template); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		// cannot delete template if it is used
		projects := new(v1alpha2.ProjectList)
		if err := v.client.List(context.Background(), projects, client.MatchingLabels{v1alpha2.ResourceLabelTemplate: template.Name}); err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		if len(projects.Items) > 0 {
			msg := fmt.Sprintf("The '%s' project template cannot be deleted, it is used in the '%s' project", template.Name, projects.Items[0].Name)
			return admission.Denied(msg)
		}
	}
	return admission.Allowed("")
}
