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
	"regexp"
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
	manualBootstrapSecretPrefix  = "manual-bootstrap-for-"
	capiCredentialsSecretName    = "capi-user-credentials"
)

// zoneHashedSecretName matches <ng>-<sha256(clusterUUID+zone)[:8]>, the name both the CAPI
// bootstrap and the machine-class Secret carry — the shape helm gave them and the one
// node-controller keeps writing them under.
var zoneHashedSecretName = regexp.MustCompile(`-[0-9a-f]{8}$`)

type keepResource struct {
	Group    string
	Resource string
	// version is used when the provider registration declares the exact served version.
	version string
	// versionPreference is tried in order; empty falls back to storedVersionPreference.
	versionPreference []string
	// keepName picks the objects of a resource whose namespace is shared with other
	// owners. nil keeps every helm-managed object of the resource, which is right for
	// the CAPI kinds: in d8-cloud-instance-manager they are all this module's.
	keepName func(string) bool
}

var capiResources = []keepResource{
	{Group: "cluster.x-k8s.io", Resource: "clusters"},
	{Group: "cluster.x-k8s.io", Resource: "machinehealthchecks"},
	{Group: "cluster.x-k8s.io", Resource: "machinedeployments"},
	{Group: "infrastructure.cluster.x-k8s.io", Resource: "staticmachinetemplates", versionPreference: []string{"v1alpha1"}},
	// The bootstrap Secrets are node-controller's from 1.79 on. This hook runs before
	// helm, so on the upgrade that stops rendering them the annotation is already there
	// and the release leaves them alone. Remove together with this hook.
	{Group: "", Resource: "secrets", keepName: IsBootstrapSecretName},
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

// Remove in 1.81: the hook only protects objects adopted during the #21372 and #22899
// migrations, and an upgrade from a pre-migration release is impossible once 1.81 is the
// oldest supported hop.
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
	if clusterKind := input.Values.Get("nodeManager.internal.cloudProvider.capiClusterKind").String(); clusterKind != "" {
		clusterName := input.Values.Get("nodeManager.internal.cloudProvider.capiClusterName").String()
		if clusterName != "" {
			resources = append(resources,
				keepResource{
					Group:             "infrastructure.cluster.x-k8s.io",
					Resource:          "deckhousecontrolplanes",
					versionPreference: []string{"v1alpha1"},
					keepName:          keepExactName(clusterName + "-control-plane"),
				},
				keepResource{
					Group:    "",
					Resource: "secrets",
					keepName: keepExactName(capiCredentialsSecretName),
				},
			)

			apiVersion := input.Values.Get("nodeManager.internal.cloudProvider.capiClusterAPIVersion").String()
			if apiVersion == "" {
				apiVersion = "infrastructure.cluster.x-k8s.io/v1alpha1"
			}
			providerResource, found, err := resolveKeepResourceForGVK(ctx, dynClient, apiVersion, clusterKind, clusterName)
			if err != nil {
				return fmt.Errorf("resolve provider infrastructure resource: %w", err)
			}
			if found {
				resources = append(resources, providerResource)
			}
		}
	}
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
			if res.keepName != nil && !res.keepName(item.GetName()) {
				continue
			}
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
			if res.keepName != nil && !res.keepName(item.GetName()) {
				continue
			}
			if item.GetAnnotations()[helmResourcePolicyAnnotation] != "keep" {
				return fmt.Errorf("keep policy not set on %s/%s: refusing to proceed to avoid prune", res.Resource, item.GetName())
			}
		}
	}

	return nil
}

// IsBootstrapSecretName reports whether a Secret of d8-cloud-instance-manager is one this
// migration takes over. Two shapes are at prune risk: manual-bootstrap-for-<ng>, and the
// zone-hashed <ng>-<sha256(clusterUUID+zone)[:8]> that both the MCM machine-class Secret and
// the CAPI bootstrap Secret carry. A cluster upgrading into 1.79 holds all of them as
// helm-managed objects the release would otherwise prune.
//
// Selecting by name because the namespace is shared and the labels do not separate them:
// deckhouse-registry, bashible-bashbooster and bashible-api-server-tls carry the same
// heritage/module pair and no other, and four registry-packages-proxy Secrets live here too.
//
// Exported for the template test that binds these shapes to the names the chart still renders.
func IsBootstrapSecretName(name string) bool {
	return strings.HasPrefix(name, manualBootstrapSecretPrefix) || zoneHashedSecretName.MatchString(name)
}

func keepExactName(expected string) func(string) bool {
	return func(name string) bool { return name == expected }
}

// resolveKeepResourceForGVK gets the provider resource plural from its CRD. The registration
// contract exposes apiVersion and kind, while Kubernetes clients address resources by plural;
// reading spec.names.plural avoids encoding provider names in node-manager.
func resolveKeepResourceForGVK(
	ctx context.Context,
	dynClient dynamic.Interface,
	apiVersion string,
	kind string,
	name string,
) (keepResource, bool, error) {
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return keepResource{}, false, fmt.Errorf("parse apiVersion %q: %w", apiVersion, err)
	}

	crds, err := dynClient.Resource(crdGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return keepResource{}, false, fmt.Errorf("list CRDs: %w", err)
	}
	for _, crd := range crds.Items {
		group, _, _ := unstructured.NestedString(crd.Object, "spec", "group")
		crdKind, _, _ := unstructured.NestedString(crd.Object, "spec", "names", "kind")
		if group != gv.Group || crdKind != kind {
			continue
		}

		plural, found, err := unstructured.NestedString(crd.Object, "spec", "names", "plural")
		if err != nil {
			return keepResource{}, false, fmt.Errorf("read plural from CRD %s: %w", crd.GetName(), err)
		}
		if !found || plural == "" {
			return keepResource{}, false, fmt.Errorf("CRD %s has no spec.names.plural", crd.GetName())
		}

		return keepResource{
			Group:    gv.Group,
			Resource: plural,
			version:  gv.Version,
			keepName: keepExactName(name),
		}, true, nil
	}

	return keepResource{}, false, nil
}

// keepResourceVersion resolves the version to patch a resource through. A core
// resource has no CRD to read a stored version from, and v1 is the only version
// the group has ever served.
func keepResourceVersion(ctx context.Context, dynClient dynamic.Interface, res keepResource) (string, bool, error) {
	if res.version != "" {
		return res.version, true, nil
	}
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
