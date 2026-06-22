/*
Copyright 2021 Flant JSC

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
	"log/slog"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/module-sdk/pkg"
	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const (
	// CAPI v1.12 uses v1beta2 as the hub (storage) version. All object refs use apiGroup, not apiVersion.
	capiAPIVersion             = "cluster.x-k8s.io/v1beta2"
	capiInfrastructureAPIGroup = "infrastructure.cluster.x-k8s.io"
)

var capiMachineTemplateAPIGroups = map[string]string{
	"DeckhouseMachineTemplate":   capiInfrastructureAPIGroup,
	"DynamixMachineTemplate":     capiInfrastructureAPIGroup,
	"HuaweiCloudMachineTemplate": capiInfrastructureAPIGroup,
	"StaticMachineTemplate":      capiInfrastructureAPIGroup,
	"VCDMachineTemplate":         capiInfrastructureAPIGroup,
	"ZvirtMachineTemplate":       capiInfrastructureAPIGroup,
}

var capiMachineAPIGroups = map[string]string{
	"DeckhouseMachine":   capiInfrastructureAPIGroup,
	"DynamixMachine":     capiInfrastructureAPIGroup,
	"HuaweiCloudMachine": capiInfrastructureAPIGroup,
	"StaticMachine":      capiInfrastructureAPIGroup,
	"VCDMachine":         capiInfrastructureAPIGroup,
	"ZvirtMachine":       capiInfrastructureAPIGroup,
}

var capiClusterInfraAPIGroups = map[string]string{
	"DeckhouseCluster":   capiInfrastructureAPIGroup,
	"DynamixCluster":     capiInfrastructureAPIGroup,
	"HuaweiCloudCluster": capiInfrastructureAPIGroup,
	"StaticCluster":      capiInfrastructureAPIGroup,
	"VCDCluster":         capiInfrastructureAPIGroup,
	"ZvirtCluster":       capiInfrastructureAPIGroup,
}

var capiControlPlaneAPIGroups = map[string]string{
	"DeckhouseControlPlane": capiInfrastructureAPIGroup,
}

type capiInfraRef struct {
	Name      string
	Namespace string
	Kind      string
	APIGroup  string
}

type capiClusterRefs struct {
	Name                 string
	Namespace            string
	InfraKind            string
	InfraAPIGroup        string
	ControlPlaneKind     string
	ControlPlaneAPIGroup string
}

func filterTemplateInfraRef(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	kind, _, _ := unstructured.NestedString(obj.Object, "spec", "template", "spec", "infrastructureRef", "kind")
	apiGroup, _, _ := unstructured.NestedString(obj.Object, "spec", "template", "spec", "infrastructureRef", "apiGroup")

	return capiInfraRef{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Kind:      kind,
		APIGroup:  apiGroup,
	}, nil
}

func filterDirectInfraRef(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	kind, _, _ := unstructured.NestedString(obj.Object, "spec", "infrastructureRef", "kind")
	apiGroup, _, _ := unstructured.NestedString(obj.Object, "spec", "infrastructureRef", "apiGroup")

	return capiInfraRef{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Kind:      kind,
		APIGroup:  apiGroup,
	}, nil
}

func filterClusterRefs(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	infraKind, _, _ := unstructured.NestedString(obj.Object, "spec", "infrastructureRef", "kind")
	infraAPIGroup, _, _ := unstructured.NestedString(obj.Object, "spec", "infrastructureRef", "apiGroup")
	cpKind, _, _ := unstructured.NestedString(obj.Object, "spec", "controlPlaneRef", "kind")
	cpAPIGroup, _, _ := unstructured.NestedString(obj.Object, "spec", "controlPlaneRef", "apiGroup")

	return capiClusterRefs{
		Name:                 obj.GetName(),
		Namespace:            obj.GetNamespace(),
		InfraKind:            infraKind,
		InfraAPIGroup:        infraAPIGroup,
		ControlPlaneKind:     cpKind,
		ControlPlaneAPIGroup: cpAPIGroup,
	}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager/set_api_version_on_capi_resources",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                   "capi_machine_deployments",
			ApiVersion:             capiAPIVersion,
			Kind:                   "MachineDeployment",
			WaitForSynchronization: ptr.To(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-instance-manager"},
				},
			},
			FilterFunc: filterTemplateInfraRef,
		},
		{
			Name:                   "capi_machine_sets",
			ApiVersion:             capiAPIVersion,
			Kind:                   "MachineSet",
			WaitForSynchronization: ptr.To(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-instance-manager"},
				},
			},
			FilterFunc: filterTemplateInfraRef,
		},
		{
			Name:                   "capi_machine_pools",
			ApiVersion:             capiAPIVersion,
			Kind:                   "MachinePool",
			WaitForSynchronization: ptr.To(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-instance-manager"},
				},
			},
			FilterFunc: filterTemplateInfraRef,
		},
		{
			Name:                   "capi_machines",
			ApiVersion:             capiAPIVersion,
			Kind:                   "Machine",
			WaitForSynchronization: ptr.To(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-instance-manager"},
				},
			},
			FilterFunc: filterDirectInfraRef,
		},
		{
			Name:                   "capi_clusters",
			ApiVersion:             capiAPIVersion,
			Kind:                   "Cluster",
			WaitForSynchronization: ptr.To(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-instance-manager"},
				},
			},
			FilterFunc: filterClusterRefs,
		},
	},
}, handleSetAPIVersionOnCAPIResources)

func patchInfraRefs(input *go_hook.HookInput, snaps []pkg.Snapshot, kindMap map[string]string, resourceKind string, buildPatch func(string) map[string]interface{}) error {
	for ref, err := range sdkobjectpatch.SnapshotIter[capiInfraRef](snaps) {
		if err != nil {
			return fmt.Errorf("failed to iterate over %s snapshots: %w", resourceKind, err)
		}

		apiGroup, ok := kindMap[ref.Kind]
		if !ok {
			input.Logger.Warn("unknown infrastructure kind", slog.String("resource", resourceKind), slog.String("name", ref.Name), slog.String("kind", ref.Kind))
			continue
		}

		if ref.APIGroup == apiGroup {
			continue
		}

		if ref.APIGroup != "" {
			input.Logger.Debug("infrastructureRef.apiGroup already set", slog.String("resource", resourceKind), slog.String("name", ref.Name), slog.String("apiGroup", ref.APIGroup))
			continue
		}

		input.PatchCollector.PatchWithMerge(buildPatch(apiGroup), capiAPIVersion, resourceKind, ref.Namespace, ref.Name)
	}
	return nil
}

func templateInfraRefPatch(apiGroup string) map[string]interface{} {
	return map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"infrastructureRef": map[string]interface{}{
						"apiGroup": apiGroup,
					},
				},
			},
		},
	}
}

func directInfraRefPatch(apiGroup string) map[string]interface{} {
	return map[string]interface{}{
		"spec": map[string]interface{}{
			"infrastructureRef": map[string]interface{}{
				"apiGroup": apiGroup,
			},
		},
	}
}

func controlPlaneRefPatch(apiGroup string) map[string]interface{} {
	return map[string]interface{}{
		"spec": map[string]interface{}{
			"controlPlaneRef": map[string]interface{}{
				"apiGroup": apiGroup,
			},
		},
	}
}

func patchClusterRefs(input *go_hook.HookInput, snaps []pkg.Snapshot) error {
	for cluster, err := range sdkobjectpatch.SnapshotIter[capiClusterRefs](snaps) {
		if err != nil {
			return fmt.Errorf("failed to iterate over Cluster snapshots: %w", err)
		}

		patchRefIfEmpty(input, cluster.Name, cluster.Namespace, cluster.InfraKind, cluster.InfraAPIGroup, capiClusterInfraAPIGroups, directInfraRefPatch)
		patchRefIfEmpty(input, cluster.Name, cluster.Namespace, cluster.ControlPlaneKind, cluster.ControlPlaneAPIGroup, capiControlPlaneAPIGroups, controlPlaneRefPatch)
	}
	return nil
}

func patchRefIfEmpty(input *go_hook.HookInput, name, namespace, kind, currentAPIGroup string, kindMap map[string]string, buildPatch func(string) map[string]interface{}) {
	if kind == "" {
		return
	}

	expectedAPIGroup, ok := kindMap[kind]
	if !ok {
		input.Logger.Warn("unknown kind", slog.String("cluster", name), slog.String("kind", kind))
		return
	}

	if currentAPIGroup == expectedAPIGroup {
		return
	}

	if currentAPIGroup != "" {
		input.Logger.Debug("apiGroup already set", slog.String("cluster", name), slog.String("apiGroup", currentAPIGroup))
		return
	}

	input.PatchCollector.PatchWithMerge(buildPatch(expectedAPIGroup), capiAPIVersion, "Cluster", namespace, name)
}

func handleSetAPIVersionOnCAPIResources(_ context.Context, input *go_hook.HookInput) error {
	if err := patchInfraRefs(input, input.Snapshots.Get("capi_machine_deployments"), capiMachineTemplateAPIGroups, "MachineDeployment", templateInfraRefPatch); err != nil {
		return err
	}

	if err := patchInfraRefs(input, input.Snapshots.Get("capi_machine_sets"), capiMachineTemplateAPIGroups, "MachineSet", templateInfraRefPatch); err != nil {
		return err
	}

	if err := patchInfraRefs(input, input.Snapshots.Get("capi_machine_pools"), capiMachineTemplateAPIGroups, "MachinePool", templateInfraRefPatch); err != nil {
		return err
	}

	if err := patchInfraRefs(input, input.Snapshots.Get("capi_machines"), capiMachineAPIGroups, "Machine", directInfraRefPatch); err != nil {
		return err
	}

	return patchClusterRefs(input, input.Snapshots.Get("capi_clusters"))
}
