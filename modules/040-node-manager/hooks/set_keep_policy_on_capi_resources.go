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
	{Group: "infrastructure.cluster.x-k8s.io", Resource: "staticmachinetemplates", versionPreference: []string{"v1alpha1"}},
	// Bootstrap secrets move from helm to node-controller in 1.79; the annotation
	// must land before helm stops rendering them, or the release prunes them
	// between the two steps. Remove together with this hook.
	{Group: "", Resource: "secrets"},
}

var crdGVR = schema.GroupVersionResource{
	Group:    "apiextensions.k8s.io",
	Version:  "v1",
	Resource: "customresourcedefinitions",
}

var storedVersionPreference = []string{"v1beta1", "v1beta2"}

var mcmStoredVersions = []string{"v1alpha1"}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/node-manager/set-keep-policy-on-capi-resources",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
}, dependency.WithExternalDependencies(setKeepPolicyOnCapiResources))

// Remove in 1.81: the hook only protects objects adopted during the #21372 migration, and an
// upgrade from a pre-migration release is impossible once 1.81 is the oldest supported hop.
func setKeepPolicyOnCapiResources(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
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

	// MCM MachineDeployment/MachineClass (machine.sapcloud.io/v1alpha1) are no longer
	// rendered by helm after the get_crds→node-controller migration; keep them from prune.
	resources := append([]keepResource(nil), capiResources...)
	if machineClassKind := input.Values.Get("nodeManager.internal.cloudProvider.machineClassKind").String(); machineClassKind != "" {
		resources = append(resources,
			keepResource{Group: "machine.sapcloud.io", Resource: "machinedeployments", versionPreference: mcmStoredVersions},
			keepResource{Group: "machine.sapcloud.io", Resource: strings.ToLower(machineClassKind) + "es", versionPreference: mcmStoredVersions},
		)
	}

	for _, res := range resources {
		version, ok, err := keepResourceVersion(ctx, dynClient, res)
		if err != nil {
			return fmt.Errorf("resolve stored version for %s: %w", res.Resource, err)
		}
		if !ok {
			continue
		}
		gvr := schema.GroupVersionResource{Group: res.Group, Version: version, Resource: res.Resource}

		list, err := dynClient.Resource(gvr).Namespace(capiNamespace).List(ctx, metav1.ListOptions{LabelSelector: helmManagedSelector})
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
				ctx,
				item.GetName(),
				types.MergePatchType,
				patch,
				metav1.PatchOptions{},
			); err != nil {
				return fmt.Errorf("patch %s/%s: %w", res.Resource, item.GetName(), err)
			}
			input.Logger.Info("stamped keep policy", slog.String("resource", res.Resource), slog.String("name", item.GetName()))
		}

		verify, err := dynClient.Resource(gvr).Namespace(capiNamespace).List(ctx, metav1.ListOptions{LabelSelector: helmManagedSelector})
		if err != nil {
			return fmt.Errorf("verify list %s/%s: %w", res.Resource, version, err)
		}
		for _, item := range verify.Items {
			if item.GetAnnotations()[helmResourcePolicyAnnotation] != "keep" {
				return fmt.Errorf("keep policy not set on %s/%s: refusing to proceed to avoid prune", res.Resource, item.GetName())
			}
		}
	}

	return nil
}

// keepResourceVersion resolves the version to patch a resource through. A core
// resource has no CRD to read a stored version from, and v1 is the only version
// the group has ever served.
func keepResourceVersion(ctx context.Context, dynClient dynamic.Interface, res keepResource) (string, bool, error) {
	if res.Group == "" {
		return "v1", true, nil
	}
	preference := res.versionPreference
	if len(preference) == 0 {
		preference = storedVersionPreference
	}
	return pickStoredVersion(ctx, dynClient, res.Group, res.Resource, preference)
}

func pickStoredVersion(ctx context.Context, dynClient dynamic.Interface, group, resource string, preference []string) (string, bool, error) {
	crd, err := dynClient.Resource(crdGVR).Get(ctx, resource+"."+group, metav1.GetOptions{})
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
