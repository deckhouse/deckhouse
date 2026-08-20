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

package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var staticInstanceListGVK = schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "StaticInstanceList"}

// StaticInstanceValidator enforces address uniqueness across StaticInstances
// (reimplementation of the shell hook
// modules/040-node-manager/webhooks/validating/static_instance).
type StaticInstanceValidator struct {
	Reader client.Reader
}

func (w *StaticInstanceValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	var instance struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Spec struct {
			Address string `json:"address"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(req.Object.Raw, &instance); err != nil {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("parse StaticInstance object: %w", err))
	}

	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(staticInstanceListGVK)
	if err := w.Reader.List(ctx, list); err != nil {
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("list StaticInstances: %w", err))
	}

	for _, item := range list.Items {
		if item.GetName() == instance.Metadata.Name {
			continue
		}
		address, _, err := unstructured.NestedString(item.Object, "spec", "address")
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, fmt.Errorf("read spec.address of StaticInstance %s: %w", item.GetName(), err))
		}
		if address == instance.Spec.Address {
			return admission.Denied(fmt.Sprintf(
				"staticinstances.deckhouse.io %q, static instance %q is already using the address %q",
				instance.Metadata.Name, item.GetName(), instance.Spec.Address))
		}
	}

	return admission.Allowed("")
}
