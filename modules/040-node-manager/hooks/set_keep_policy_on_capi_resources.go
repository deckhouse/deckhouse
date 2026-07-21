// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

const (
	helmResourcePolicyAnnotation = "helm.sh/resource-policy"
	capiNamespace                = "d8-cloud-instance-manager"
	helmManagedSelector          = "app.kubernetes.io/managed-by=Helm"
)

type keepResource struct {
	Group    string
	Resource string
	// versionPreference is tried in order; empty falls back to storedVersionPreference.
	versionPreference []string
}

var capiResources = []keepResource{
	{Group: "cluster.x-k8s.io", Resource: "clusters"},
	{Group: "cluster.x-k8s.io", Resource: "machinehealthchecks"},
	{Group: "cluster.x-k8s.io", Resource: "machinedeployments"},
}

var crdGVR = schema.GroupVersionResource{
	Group:    "apiextensions.k8s.io",
	Version:  "v1",
	Resource: "customresourcedefinitions",
}

var storedVersionPreference = []string{"v1beta1", "v1beta2"}

var mcmStoredVersions = []string{"v1alpha1"}

// migratedTLSSecrets moved from helm to node-controller; keep them from prune.
var migratedTLSSecrets = []string{"capi-webhook-tls", "bashible-api-server-tls"}

var secretGVR = schema.GroupVersionResource{Version: "v1", Resource: "secrets"}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/node-manager/set-keep-policy-on-capi-resources",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
}, dependency.WithExternalDependencies(setKeepPolicyOnCapiResources))

func setKeepPolicyOnCapiResources(_ context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	k8sClient, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("get k8s client: %w", err)
	}
	dynClient := k8sClient.Dynamic()

	patch, _ := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				helmResourcePolicyAnnotation: "keep",
			},
		},
	})

	// MCM MachineDeployment/MachineClass moved from helm to node-controller; keep them from prune.
	resources := append([]keepResource(nil), capiResources...)
	if machineClassKind := input.Values.Get("nodeManager.internal.cloudProvider.machineClassKind").String(); machineClassKind != "" {
		resources = append(resources,
			keepResource{Group: "machine.sapcloud.io", Resource: "machinedeployments", versionPreference: mcmStoredVersions},
			keepResource{Group: "machine.sapcloud.io", Resource: strings.ToLower(machineClassKind) + "es", versionPreference: mcmStoredVersions},
		)
	}

	for _, res := range resources {
		preference := res.versionPreference
		if len(preference) == 0 {
			preference = storedVersionPreference
		}
		version, ok, err := pickStoredVersion(dynClient, res.Group, res.Resource, preference)
		if err != nil {
			return fmt.Errorf("resolve stored version for %s: %w", res.Resource, err)
		}
		if !ok {
			continue
		}
		gvr := schema.GroupVersionResource{Group: res.Group, Version: version, Resource: res.Resource}

		list, err := dynClient.Resource(gvr).Namespace(capiNamespace).List(context.TODO(), metav1.ListOptions{LabelSelector: helmManagedSelector})
		if err != nil {
			if isConversionUnavailable(err) {
				input.Logger.Info("skipping resource, conversion webhook unavailable", slog.String("resource", res.Resource), slog.String("version", version))
				continue
			}
			return fmt.Errorf("list %s/%s: %w", res.Resource, version, err)
		}

		for _, item := range list.Items {
			if item.GetAnnotations()[helmResourcePolicyAnnotation] == "keep" {
				continue
			}
			if _, err := dynClient.Resource(gvr).Namespace(item.GetNamespace()).Patch(
				context.TODO(),
				item.GetName(),
				types.MergePatchType,
				patch,
				metav1.PatchOptions{},
			); err != nil {
				return fmt.Errorf("patch %s/%s: %w", res.Resource, item.GetName(), err)
			}
			input.Logger.Info("stamped keep policy", slog.String("resource", res.Resource), slog.String("name", item.GetName()))
		}

		verify, err := dynClient.Resource(gvr).Namespace(capiNamespace).List(context.TODO(), metav1.ListOptions{LabelSelector: helmManagedSelector})
		if err != nil {
			return fmt.Errorf("verify list %s/%s: %w", res.Resource, version, err)
		}
		for _, item := range verify.Items {
			if item.GetAnnotations()[helmResourcePolicyAnnotation] != "keep" {
				return fmt.Errorf("keep policy not set on %s/%s: refusing to proceed to avoid prune", res.Resource, item.GetName())
			}
		}
	}

	for _, name := range migratedTLSSecrets {
		obj, err := dynClient.Resource(secretGVR).Namespace(capiNamespace).Get(context.TODO(), name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("get secret %s: %w", name, err)
		}
		// node-controller-created Secrets carry no Helm label and are never pruned.
		if obj.GetLabels()["app.kubernetes.io/managed-by"] != "Helm" {
			continue
		}
		if obj.GetAnnotations()[helmResourcePolicyAnnotation] != "keep" {
			if _, err := dynClient.Resource(secretGVR).Namespace(capiNamespace).Patch(
				context.TODO(),
				name,
				types.MergePatchType,
				patch,
				metav1.PatchOptions{},
			); err != nil {
				return fmt.Errorf("patch secret %s: %w", name, err)
			}
			input.Logger.Info("stamped keep policy", slog.String("resource", "secrets"), slog.String("name", name))
		}

		verify, err := dynClient.Resource(secretGVR).Namespace(capiNamespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("verify get secret %s: %w", name, err)
		}
		if verify.GetAnnotations()[helmResourcePolicyAnnotation] != "keep" {
			return fmt.Errorf("keep policy not set on secret %s: refusing to proceed to avoid prune", name)
		}
	}

	return nil
}

func pickStoredVersion(dynClient dynamic.Interface, group, resource string, preference []string) (string, bool, error) {
	crd, err := dynClient.Resource(crdGVR).Get(context.TODO(), resource+"."+group, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return "", false, nil
		}
		return "", false, err
	}
	stored, _, err := unstructured.NestedStringSlice(crd.Object, "status", "storedVersions")
	if err != nil {
		return "", false, err
	}
	for _, want := range preference {
		for _, have := range stored {
			if have == want {
				return want, true, nil
			}
		}
	}
	return "", false, nil
}

func isConversionUnavailable(err error) bool {
	if apierrors.IsServiceUnavailable(err) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "conversion webhook") || strings.Contains(msg, "(re)initializing")
}
