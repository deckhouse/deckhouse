/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package project

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"controller/pkg/apis/deckhouse.io/v1alpha1"
	"controller/pkg/apis/deckhouse.io/v1alpha2"
	"controller/pkg/consts"
	"controller/pkg/helm"
	"controller/pkg/validate"

	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/yaml"
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
		if project.Name == consts.DefaultProjectName || project.Name == consts.DeckhouseProjectName {
			return admission.Allowed("")
		}

		if strings.HasPrefix(project.Name, "d8-") || strings.HasPrefix(project.Name, "kube-") {
			return admission.Denied("Projects cannot start with 'd8-' or 'kube-'")
		}

		namespaces := new(v1.NamespaceList)
		if err := v.client.List(context.Background(), namespaces); err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		for _, namespace := range namespaces.Items {
			if namespace.Name == project.Name {
				msg := fmt.Sprintf("The '%s' project cannot be created, a namespace with its name exists", project.Name)
				return admission.Denied(msg)
			}
		}
	}

	if req.Operation == admissionv1.Update {
		// pass triggered projects
		if annotations := project.Annotations; annotations != nil {
			require, ok := annotations[consts.ProjectRequireSyncAnnotation]
			if ok && require == "true" {
				return admission.Allowed("")
			}
		}

		// pass deploying or error projects
		if project.Status.State == v1alpha2.ProjectStateDeploying || project.Status.State == v1alpha2.ProjectStateError {
			return admission.Allowed("").WithWarnings("The project skip validation due to the status")
		}
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
		return admission.Denied(fmt.Sprintf("The project '%s' is invalid: %s", project.Name, err.Error()))
	}

	// validate helm render
	if err = v.helmClient.ValidateRender(project, template); err != nil {
		return admission.Denied(fmt.Sprintf("The project '%s' is invalid: %s", project.Name, err.Error()))
	}

	return admission.Allowed("")
}

func (v *validator) projectTemplateByName(ctx context.Context, name string) (*v1alpha1.ProjectTemplate, error) {
	template := new(v1alpha1.ProjectTemplate)
	if err := v.client.Get(ctx, types.NamespacedName{Name: name}, template); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return template, nil
}
