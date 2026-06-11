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

package project

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/yaml"

	"controller/apis/deckhouse.io/v1alpha1"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/helm"
	projectmanager "controller/internal/manager/project"
	"controller/internal/validate"
)

func Register(runtimeManager manager.Manager, helmClient *helm.Client) {
	hook := &webhook.Admission{Handler: &validator{client: runtimeManager.GetClient(), helmClient: helmClient}}
	runtimeManager.GetWebhookServer().Register("/validate/v1alpha3/projects", hook)
}

type validator struct {
	client     client.Client
	helmClient *helm.Client
}

func (v *validator) Handle(_ context.Context, req admission.Request) admission.Response {
	project := new(v1alpha3.Project)
	if err := yaml.Unmarshal(req.Object.Raw, project); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if req.Operation == admissionv1.Create {
		// pass virtual projects
		if project.Name == projectmanager.DefaultProjectName || project.Name == projectmanager.DeckhouseProjectName {
			return admission.Allowed("")
		}

		if strings.HasPrefix(project.Name, "d8-") || strings.HasPrefix(project.Name, "kube-") {
			return admission.Denied("Projects cannot start with 'd8-' or 'kube-'")
		}

		namespaces := new(corev1.NamespaceList)
		if err := v.client.List(context.Background(), namespaces); err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		for _, namespace := range namespaces.Items {
			if namespace.Name == project.Name {
				if _, ok := namespace.Annotations[v1alpha3.NamespaceAnnotationAdopt]; ok {
					continue
				}

				msg := fmt.Sprintf("The '%s' project cannot be created, a namespace with its name exists", project.Name)
				return admission.Denied(msg)
			}
		}

		// prefix collisions: the "<project>-*" name space is reserved for the additional namespaces
		// of an existing project, so neither "foo-bar" (when "foo" exists) nor "foo" (when "foo-bar"
		// exists) may be created.
		projects := new(v1alpha3.ProjectList)
		if err := v.client.List(context.Background(), projects); err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		for _, existing := range projects.Items {
			if existing.Name == project.Name {
				continue
			}
			if strings.HasPrefix(project.Name, existing.Name+"-") {
				return admission.Denied(fmt.Sprintf(
					"project name %q conflicts with project %q: %q-* names are reserved for additional namespaces of project %q",
					project.Name, existing.Name, existing.Name, existing.Name))
			}
			if strings.HasPrefix(existing.Name, project.Name+"-") {
				return admission.Denied(fmt.Sprintf(
					"project name %q conflicts with project %q: %q-* names are reserved for additional namespaces of project %q",
					project.Name, existing.Name, project.Name, project.Name))
			}
		}
	}

	// validate the standard fields (cheap checks before the OpenAPI validation)
	if denied := validateStandardFields(project); denied != "" {
		return admission.Denied(denied)
	}

	if req.Operation == admissionv1.Update {
		// pass triggered projects
		if annotations := project.Annotations; annotations != nil {
			require, ok := annotations[v1alpha3.ProjectAnnotationRequireSync]
			if ok && require == "true" {
				return admission.Allowed("")
			}
		}

		// pass error projects
		if project.Status.State == v1alpha3.ProjectStateError {
			return admission.Allowed("").WithWarnings("The project skip validation due to the status")
		}
	}

	// skip project with empty template
	if project.Spec.ProjectTemplateName == "" {
		return admission.Allowed("")
	}

	template, err := v.projectTemplateByName(context.Background(), project.Spec.ProjectTemplateName)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	if template == nil {
		return admission.Allowed("").WithWarnings("The project template not found")
	}

	// validate the project against the template
	if err = validate.Project(project, template); err != nil {
		return admission.Denied(fmt.Sprintf("The project '%s' is invalid: %v", project.Name, err))
	}

	// validate helm render
	if err = v.helmClient.ValidateRender(project, template); err != nil {
		// warning errors allow deploying the project
		if errors.Is(err, helm.ErrNamespaceOverride) {
			return admission.Allowed("").WithWarnings(err.Error())
		}

		return admission.Denied(fmt.Sprintf("The project '%s' is invalid: %v", project.Name, err))
	}

	return admission.Allowed("")
}

// validateStandardFields performs cheap validation of the Project standard fields. It returns a
// non-empty denial message when the project is invalid.
func validateStandardFields(project *v1alpha3.Project) string {
	for _, admin := range project.Spec.Administrators {
		if admin.Kind != "User" && admin.Kind != "Group" {
			return fmt.Sprintf("administrator %q has invalid kind %q: must be User or Group", admin.Name, admin.Kind)
		}
		if admin.Name == "" {
			return "administrator name must not be empty"
		}
	}
	for name, quantity := range project.Spec.Quota {
		if _, err := resource.ParseQuantity(quantity.String()); err != nil {
			return fmt.Sprintf("quota %q has invalid value %q: %v", name, quantity.String(), err)
		}
	}
	return ""
}

func (v *validator) projectTemplateByName(ctx context.Context, name string) (*v1alpha1.ProjectTemplate, error) {
	template := new(v1alpha1.ProjectTemplate)
	if err := v.client.Get(ctx, client.ObjectKey{Name: name}, template); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get the '%s' project template: %w", name, err)
	}
	return template, nil
}
