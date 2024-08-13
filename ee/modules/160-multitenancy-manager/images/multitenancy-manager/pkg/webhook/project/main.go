/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package project

import (
	"context"
	"controller/pkg/validate"
	"fmt"
	"net/http"

	"controller/pkg/apis/deckhouse.io/v1alpha1"
	"controller/pkg/apis/deckhouse.io/v1alpha2"
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

func Register(runtimeManager manager.Manager) {
	hook := &webhook.Admission{Handler: &validator{client: runtimeManager.GetClient()}}
	runtimeManager.GetWebhookServer().Register("/validate/v1alpha2/projects", hook)
}

type validator struct {
	client client.Client
}

func (v *validator) Handle(ctx context.Context, req admission.Request) admission.Response {
	project := new(v1alpha2.Project)
	if err := yaml.Unmarshal(req.Object.Raw, project); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// cannot create/update without template
	if project.Spec.ProjectTemplateName == "" {
		return admission.Denied("project template name is required")
	}

	// cannot create/update project if its template does not exist
	projectTemplate, err := v.projectTemplateByName(ctx, project.Spec.ProjectTemplateName)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	if projectTemplate == nil {
		msg := fmt.Sprintf("The '%s' project template not found", project.Spec.ProjectTemplateName)
		return admission.Denied(msg)
	}

	// cannot create/update invalid project
	if err = validate.Project(project, projectTemplate); err != nil {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("project validation failed: %v", err))
	}

	// cannot create project if a namespace with its name exists
	if req.Operation == admissionv1.Create {
		namespaces := new(v1.NamespaceList)
		if err = v.client.List(context.Background(), namespaces); err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		for _, namespace := range namespaces.Items {
			if namespace.Name == project.Name {
				msg := fmt.Sprintf("The '%s' project cannot be created, a namespace with its name exists", project.Name)
				return admission.Denied(msg)
			}
		}
	}

	// cannot change project template in project
	if req.Operation == admissionv1.Update {
		oldProject := new(v1alpha2.Project)
		if err = yaml.Unmarshal(req.OldObject.Raw, oldProject); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		if oldProject.Spec.ProjectTemplateName != project.Spec.ProjectTemplateName {
			msg := fmt.Sprintf("The '%s' project template cannot be updated", project.Spec.ProjectTemplateName)
			return admission.Denied(msg)
		}
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
