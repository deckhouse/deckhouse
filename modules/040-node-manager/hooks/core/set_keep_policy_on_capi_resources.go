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

package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"time"

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
	{Group: "", Resource: "secrets", keepName: func(name string) bool {
		return IsBootstrapSecretName(name) || name == capiCredentialsSecretName
	}},
}

var crdGVR = schema.GroupVersionResource{
	Group:    "apiextensions.k8s.io",
	Version:  "v1",
	Resource: "customresourcedefinitions",
}

var storedVersionPreference = []string{"v1beta1", "v1beta2"}

// A conversion webhook is unavailable while its Deployment restarts, which is seconds, and this
// upgrade is what restarts it. Retrying costs a short wait; giving up costs the annotation.
var (
	conversionRetryAttempts = 5
	conversionRetryDelay    = 2 * time.Second
)

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
		versions, err := keepResourceVersions(ctx, dynClient, res)
		if err != nil {
			return fmt.Errorf("resolve stored version for %s: %w", res.Resource, err)
		}
		// The CRD is not installed, so the cluster holds no object of this resource and
		// there is nothing for Helm to prune. This is the only skip taken on faith; every
		// other one below is proven against the objects themselves.
		if len(versions) == 0 {
			continue
		}

		version, list, err := listHelmManaged(ctx, dynClient, input.Logger, res, versions)
		if err != nil {
			// Fail closed: Helm prunes whatever this hook failed to annotate on the very next
			// upgrade, so a read error must stop the release instead of being swallowed.
			return fmt.Errorf("list %s: %w", res.Resource, err)
		}
		gvr := schema.GroupVersionResource{Group: res.Group, Version: version, Resource: res.Resource}

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
				// An object listed a moment ago can be gone by now: these are exactly the
				// objects node-controller garbage-collects when a NodeGroup is deleted. What
				// no longer exists cannot be pruned, so it is not this hook's problem.
				if apierrors.IsNotFound(err) {
					continue
				}
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
		// storedVersions is empty on a CRD the provider module has just applied, before anything
		// was written through it. keepResourceVersions falls back to the served versions, so this
		// must not fail the release.
		storedVersions, _, err := unstructured.NestedStringSlice(crd.Object, "status", "storedVersions")
		if err != nil {
			return keepResource{}, false, fmt.Errorf("read stored versions from CRD %s: %w", crd.GetName(), err)
		}
		preference := []string{gv.Version}
		for _, version := range storedVersions {
			if version != gv.Version {
				preference = append(preference, version)
			}
		}

		return keepResource{
			Group:             gv.Group,
			Resource:          plural,
			versionPreference: preference,
			keepName:          keepExactName(name),
		}, true, nil
	}

	return keepResource{}, false, nil
}

// listHelmManaged lists the Helm-owned objects of a resource through the first version that
// answers, and reports which one that was. Reading a custom resource at any version but the
// stored one goes through the provider's conversion webhook, which this very upgrade restarts, so
// the remaining versions and then further attempts are tried. When none of them answers this
// returns an error rather than an empty list: a resource nobody could read is one nobody protected.
func listHelmManaged(
	ctx context.Context,
	dynClient dynamic.Interface,
	logger go_hook.Logger,
	res keepResource,
	versions []string,
) (string, *unstructured.UnstructuredList, error) {
	var lastErr error
	for attempt := 1; attempt <= conversionRetryAttempts; attempt++ {
		for _, version := range versions {
			gvr := schema.GroupVersionResource{Group: res.Group, Version: version, Resource: res.Resource}
			list, err := dynClient.Resource(gvr).Namespace(capiNamespace).List(ctx, metav1.ListOptions{LabelSelector: helmManagedSelector})
			if err == nil {
				return version, list, nil
			}
			if !isConversionUnavailable(res, err) {
				return "", nil, fmt.Errorf("%s: %w", version, err)
			}
			lastErr = err
		}
		if attempt == conversionRetryAttempts {
			break
		}
		logger.Warn("conversion webhook unavailable, retrying",
			slog.String("resource", res.Resource), slog.Any("versions", versions), slog.Int("attempt", attempt))
		select {
		case <-ctx.Done():
			return "", nil, ctx.Err()
		case <-time.After(conversionRetryDelay):
		}
	}
	return "", nil, fmt.Errorf("conversion webhook unavailable on every version %v: %w", versions, lastErr)
}

// isConversionUnavailable reports the one failure that says nothing about the objects: the
// provider's conversion webhook is restarting. It is only possible for a custom resource, so a
// core resource never qualifies, and every other read failure is a real one.
func isConversionUnavailable(res keepResource, err error) bool {
	if res.Group == "" {
		return false
	}
	if apierrors.IsServiceUnavailable(err) {
		return true
	}
	message := err.Error()
	return strings.Contains(message, "conversion webhook") || strings.Contains(message, "(re)initializing")
}

func keepResourceVersion(ctx context.Context, dynClient dynamic.Interface, res keepResource) (string, bool, error) {
	versions, err := keepResourceVersions(ctx, dynClient, res)
	if err != nil || len(versions) == 0 {
		return "", false, err
	}
	return versions[0], true, nil
}

// keepResourceVersions returns the versions the resource's objects can be read through, best
// first. An empty result means the CRD is not installed, the only case where skipping a resource
// proves by itself that nothing is left unprotected.
func keepResourceVersions(ctx context.Context, dynClient dynamic.Interface, res keepResource) ([]string, error) {
	// A core resource has no CRD to read a stored version from, and v1 is the only version
	// the group has ever served.
	if res.Group == "" {
		return []string{"v1"}, nil
	}
	preference := res.versionPreference
	if len(preference) == 0 {
		preference = storedVersionPreference
	}
	crd, err := dynClient.Resource(crdGVR).Get(ctx, res.Resource+"."+res.Group, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return orderedVersions(crd, preference)
}

func orderedVersions(crd *unstructured.Unstructured, preference []string) ([]string, error) {
	stored, _, err := unstructured.NestedStringSlice(crd.Object, "status", "storedVersions")
	if err != nil {
		return nil, err
	}
	served, storage, err := servedVersions(crd)
	if err != nil {
		return nil, err
	}
	available := append(append([]string(nil), stored...), served...)

	ordered := make([]string, 0, len(available))
	add := func(version string) {
		if version == "" || slices.Contains(ordered, version) {
			return
		}
		ordered = append(ordered, version)
	}
	// Storage version first, ahead of the caller's preference: reading any other version routes
	// through the provider's conversion webhook, which this very upgrade restarts. The preference
	// decides everything the storage version cannot answer.
	if slices.Contains(available, storage) {
		add(storage)
	}
	for _, want := range preference {
		if slices.Contains(available, want) {
			add(want)
		}
	}
	for _, version := range available {
		add(version)
	}
	return ordered, nil
}

// servedVersions returns the versions the CRD serves, and the one it stores objects in.
func servedVersions(crd *unstructured.Unstructured) ([]string, string, error) {
	entries, _, err := unstructured.NestedSlice(crd.Object, "spec", "versions")
	if err != nil {
		return nil, "", err
	}
	names := make([]string, 0, len(entries))
	storage := ""
	for _, entry := range entries {
		version, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		name, _, _ := unstructured.NestedString(version, "name")
		if name == "" {
			continue
		}
		if isStorage, _, _ := unstructured.NestedBool(version, "storage"); isStorage {
			storage = name
		}
		if served, _, _ := unstructured.NestedBool(version, "served"); served {
			names = append(names, name)
		}
	}
	return names, storage, nil
}

func pickStoredVersion(ctx context.Context, dynClient dynamic.Interface, group, resource string, preference []string) (string, bool, error) {
	return keepResourceVersion(ctx, dynClient, keepResource{Group: group, Resource: resource, versionPreference: preference})
}
