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

package hooks

import (
	"context"
	"fmt"
	"slices"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/module"
)

// The ClusterIssuer grant registration (GrantableClusterResourceDefinition "clusterissuers" and the
// GrantableClusterResourceReferences "clusterissuers-certificate" and "clusterissuers-ingress") moved
// from this module to the cert-manager module under the same names: a ClusterResourceGrantPolicy
// refers to the definition by name.
//
// The two releases run in no fixed order: neither module is critical, so addon-operator runs them
// in parallel queues, ordered only by module dependencies, not by weight. If this release runs first
// on the new chart, it prunes the objects it no longer renders, and until cert-manager recreates them
// the controller deletes the "clusterissuers" AvailableClusterResource from every project namespace
// and the grant webhooks stop checking Certificates and Ingresses. nelm leaves an object alone once
// another release has adopted it, but its helm3lib fallback deletes without checking the owner. Both
// engines skip an object whose live copy carries `helm.sh/resource-policy: keep`, so the objects get
// that annotation before the release runs and lose it after the release has succeeded
// (migrate_clusterissuers_grants_after_helm.go): from then on the release no longer lists them, and
// the cert-manager release, which adopts objects of other releases, owns them without a leftover
// keep policy.
//
// Nothing is protected while cert-manager is disabled: then the objects should go, as before.
//
// Two paths leave objects behind; both need a manual fix and are not worth a marker annotation for
// hooks that live only for a limited number of releases:
//   - The release fails after this hook, and the module is disabled before a successful run, so the
//     after-helm hook never removes the keep policy. The objects then survive cert-manager no longer
//     rendering them. Remove the helm.sh/resource-policy annotation from the three objects.
//   - cert-manager is disabled while a run of this module is between this hook and the after-helm
//     one: the cert-manager release is uninstalled while the objects carry keep, so they stay with
//     no release rendering them. Delete the GrantableClusterResourceDefinition "clusterissuers" and
//     the two GrantableClusterResourceReferences by hand.
//
// TODO: remove together with migrate_clusterissuers_grants_after_helm.go in the first minor release
// after the next LTS release following v1.77. LTS clusters jump up to 10 minors
// (deckhouse-controller/pkg/releaseupdater/task_calculator.go, maxMinorVersionDiffForLTS), so a
// cluster on the 1.77 LTS goes straight to the next LTS, and that LTS must still run these hooks.
// Also make sure no update constraint in release metadata skips the release that introduced them.

const (
	clusterIssuersDefinitionsSnapshot = "clusterissuers_definitions"
	clusterIssuersReferencesSnapshot  = "clusterissuers_references"

	resourcePolicyAnnotation = "helm.sh/resource-policy"
	resourcePolicyKeep       = "keep"
)

var (
	clusterIssuersDefinitionNames = []string{"clusterissuers"}
	clusterIssuersReferenceNames  = []string{"clusterissuers-certificate", "clusterissuers-ingress"}
)

var clusterIssuersGrantKubernetes = []go_hook.KubernetesConfig{
	{
		Name:                         clusterIssuersDefinitionsSnapshot,
		ApiVersion:                   "multitenancy.deckhouse.io/v1alpha1",
		Kind:                         "GrantableClusterResourceDefinition",
		NameSelector:                 &types.NameSelector{MatchNames: clusterIssuersDefinitionNames},
		ExecuteHookOnEvents:          ptr.To(false),
		ExecuteHookOnSynchronization: ptr.To(false),
		FilterFunc:                   filterObjectName,
	},
	{
		Name:                         clusterIssuersReferencesSnapshot,
		ApiVersion:                   "multitenancy.deckhouse.io/v1alpha1",
		Kind:                         "GrantableClusterResourceReference",
		NameSelector:                 &types.NameSelector{MatchNames: clusterIssuersReferenceNames},
		ExecuteHookOnEvents:          ptr.To(false),
		ExecuteHookOnSynchronization: ptr.To(false),
		FilterFunc:                   filterObjectName,
	},
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/160-multitenancy-manager",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 1},
	Kubernetes:   clusterIssuersGrantKubernetes,
}, keepClusterIssuersGrants)

func filterObjectName(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

type grantObject struct {
	kind string
	name string
}

func clusterIssuersGrantObjects(input *go_hook.HookInput) ([]grantObject, error) {
	var objects []grantObject
	for _, s := range []struct {
		snapshot, kind string
		names          []string
	}{
		{clusterIssuersDefinitionsSnapshot, "GrantableClusterResourceDefinition", clusterIssuersDefinitionNames},
		{clusterIssuersReferencesSnapshot, "GrantableClusterResourceReference", clusterIssuersReferenceNames},
	} {
		names, err := sdkobjectpatch.UnmarshalToStruct[string](input.Snapshots, s.snapshot)
		if err != nil {
			return nil, fmt.Errorf("unmarshal %s snapshot: %w", s.snapshot, err)
		}
		for _, name := range names {
			// The name selector already narrows the snapshot; this keeps any other object of the
			// kind out of the patches whatever the binding delivers.
			if slices.Contains(s.names, name) {
				objects = append(objects, grantObject{kind: s.kind, name: name})
			}
		}
	}
	return objects, nil
}

func keepClusterIssuersGrants(_ context.Context, input *go_hook.HookInput) error {
	if !module.IsEnabled("cert-manager", input) {
		return nil
	}
	return patchResourcePolicy(input, resourcePolicyKeep)
}

// patchResourcePolicy sets the helm.sh/resource-policy annotation of every existing object to value,
// or removes it when value is nil. A patch that changes nothing is not written by the API server.
func patchResourcePolicy(input *go_hook.HookInput, value any) error {
	objects, err := clusterIssuersGrantObjects(input)
	if err != nil {
		return err
	}
	patch := map[string]any{
		"metadata": map[string]any{"annotations": map[string]any{resourcePolicyAnnotation: value}},
	}
	for _, obj := range objects {
		input.PatchCollector.PatchWithMerge(patch, "multitenancy.deckhouse.io/v1alpha1", obj.kind, "", obj.name,
			object_patch.WithIgnoreMissingObject())
	}
	return nil
}
