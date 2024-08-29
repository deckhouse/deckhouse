/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package project

import (
	"context"
	"controller/pkg/consts"
	"fmt"
	"net/http"
	"strings"

	"controller/pkg/apis/deckhouse.io/v1alpha2"

	v1 "k8s.io/api/core/v1"

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

func (v *validator) Handle(_ context.Context, req admission.Request) admission.Response {
	project := new(v1alpha2.Project)
	if err := yaml.Unmarshal(req.Object.Raw, project); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	if strings.HasPrefix(project.Name, "d8-") || strings.HasPrefix(project.Name, "kube-") {
		return admission.Denied("Projects cannot start with 'd8-' or 'kube-'")
	}

	// allow virtual projects
	if project.Name == consts.DefaultProjectName || project.Name == consts.DeckhouseProjectName {
		return admission.Allowed("")
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

	return admission.Allowed("")
}
