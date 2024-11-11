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

package namespace

import (
	"context"
	"fmt"
	"net/http"
	"slices"

	v1 "k8s.io/api/core/v1"

	"sigs.k8s.io/yaml"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func Register(runtimeManager manager.Manager, serviceAccount, deckhouseServiceAccount string) {
	hook := &webhook.Admission{Handler: &validator{client: runtimeManager.GetClient(), serviceAccount: serviceAccount, deckhouseServiceAccount: deckhouseServiceAccount}}
	runtimeManager.GetWebhookServer().Register("/validate/v1/namespaces", hook)
}

type validator struct {
	deckhouseServiceAccount string
	serviceAccount          string
	client                  client.Client
}

func (v *validator) Handle(_ context.Context, req admission.Request) admission.Response {
	namespace := new(v1.Namespace)
	if err := yaml.Unmarshal(req.Object.Raw, namespace); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// allow to create default namespace
	if namespace.Name == "default" {
		return admission.Allowed("")
	}

	// other namespaces can be created only by deckhouse or multitenancy-manager
	if !slices.Contains([]string{v.serviceAccount, v.deckhouseServiceAccount}, req.UserInfo.Username) {
		return admission.Denied(fmt.Sprintf("namespaces can be created only as a part of a Project"))
	}

	return admission.Allowed("")
}
