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

// Package systemnetwork checks that the SystemNetwork named in the ModuleConfig node-manager exists.
package systemnetwork

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Path must not change: the webhook configuration of the module names it.
const Path = "/validate-deckhouse-io-v1alpha1-moduleconfig-node-manager"

const readTimeout = 5 * time.Second

var systemNetworkGVK = schema.GroupVersionKind{Group: "network.deckhouse.io", Version: "v1alpha1", Kind: "SystemNetwork"}

// Register adds the webhook to the server of the manager.
func Register(mgr ctrl.Manager) {
	mgr.GetWebhookServer().Register(Path, &webhook.Admission{
		// The API reader, not the cache: an informer does not start when the CRD is absent.
		Handler: &validator{reader: mgr.GetAPIReader()},
	})
}

type validator struct {
	reader client.Reader
}

// Handle allows the request only when the SystemNetwork exists. The webhook configuration
// calls it only when the name changes to a value that is not empty.
//
//nolint:nonamedreturns // The named result lets recover set the answer.
func (v *validator) Handle(ctx context.Context, req admission.Request) (resp admission.Response) {
	defer func() {
		if r := recover(); r != nil {
			resp = admission.Denied(fmt.Sprintf("internal error in the SystemNetwork check: %v", r))
		}
	}()

	name, err := networkName(req.Object.Raw)
	if err != nil {
		return admission.Denied(err.Error())
	}
	// The old value passed the check before, or it has never been a string.
	oldName, _ := networkName(req.OldObject.Raw)
	if name == "" || name == oldName {
		return admission.Allowed("")
	}

	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	network := &unstructured.Unstructured{}
	network.SetGroupVersionKind(systemNetworkGVK)
	err = v.reader.Get(ctx, client.ObjectKey{Name: name}, network)
	switch {
	case err == nil:
		return admission.Allowed("")
	case isNotInstalled(err):
		return admission.Denied("SystemNetwork is not installed (is the sdn module disabled?)")
	case apierrors.IsNotFound(err):
		return admission.Denied(fmt.Sprintf("SystemNetwork `%s` not found", name))
	default:
		return admission.Denied(fmt.Sprintf("cannot check SystemNetwork `%s`: %v", name, err))
	}
}

// isNotInstalled tells a resource that the API does not serve from an object that does not exist.
// The error of a missing object carries the name from the request, so the details cannot tell them
// apart. The cause can: the API answers with an unexpected response only for a missing resource.
// Such an error also comes after the CRD was deleted while the mapper of the reader still knows it.
func isNotInstalled(err error) bool {
	return meta.IsNoMatchError(err) ||
		runtime.IsNotRegisteredError(err) ||
		(apierrors.IsNotFound(err) && apierrors.HasStatusCause(err, metav1.CauseTypeUnexpectedServerResponse))
}

// networkName reads the name of the SystemNetwork from the ModuleConfig. An empty string means
// that the name is not set.
func networkName(raw []byte) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}
	var moduleConfig map[string]any
	if err := json.Unmarshal(raw, &moduleConfig); err != nil {
		return "", fmt.Errorf("cannot read ModuleConfig: %w", err)
	}
	value, found, err := unstructured.NestedFieldNoCopy(moduleConfig,
		"spec", "settings", "fencing", "systemNetworkFallback", "systemNetworkName")
	if err != nil {
		return "", err
	}
	if !found || value == nil {
		return "", nil
	}
	name, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("systemNetworkName: expected a string, got %T", value)
	}
	return name, nil
}
