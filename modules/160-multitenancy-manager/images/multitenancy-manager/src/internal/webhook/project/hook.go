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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/yaml"

	"controller/apis/deckhouse.io/v1alpha1"
	"controller/apis/deckhouse.io/v1alpha2"
	"controller/internal/helm"
	projectmanager "controller/internal/manager/project"
	"controller/internal/validate"
)

func Register(runtimeManager manager.Manager, helmClient *helm.Client) {
	hook := &webhook.Admission{Handler: &validator{client: runtimeManager.GetClient(), helmClient: helmClient}}
	runtimeManager.GetWebhookServer().Register("/validate/v1alpha2/projects", hook)
}

type validator struct {
	client     client.Client
	helmClient *helm.Client
}

func (v *validator) Handle(_ context.Context, req admission.Request) admission.Response {
	project := new(v1alpha2.Project)
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
				if _, ok := namespace.Annotations[v1alpha2.NamespaceAnnotationAdopt]; ok {
					continue
				}

				msg := fmt.Sprintf("The '%s' project cannot be created, a namespace with its name exists", project.Name)
				return admission.Denied(msg)
			}
		}
	}

	if req.Operation == admissionv1.Update {
		// pass triggered projects
		if annotations := project.Annotations; annotations != nil {
			require, ok := annotations[v1alpha2.ProjectAnnotationRequireSync]
			if ok && require == "true" {
				return admission.Allowed("")
			}
		}

		// pass error projects
		if project.Status.State == v1alpha2.ProjectStateError {
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
