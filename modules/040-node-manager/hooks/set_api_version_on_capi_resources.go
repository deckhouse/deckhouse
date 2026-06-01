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
	capiAPIVersion               = "cluster.x-k8s.io/v1beta1"
	capiInfrastructureAPIVersion = "infrastructure.cluster.x-k8s.io/v1alpha1"
	capiInfraVCDAPIVersion       = "infrastructure.cluster.x-k8s.io/v1beta2"
	capiInfraZvirtAPIVersion     = "infrastructure.cluster.x-k8s.io/v1"
)

var capiMachineTemplateAPIVersions = map[string]string{
	"DeckhouseMachineTemplate":   capiInfrastructureAPIVersion,
	"DynamixMachineTemplate":     capiInfrastructureAPIVersion,
	"HuaweiCloudMachineTemplate": capiInfrastructureAPIVersion,
	"StaticMachineTemplate":      capiInfrastructureAPIVersion,
	"VCDMachineTemplate":         capiInfraVCDAPIVersion,
	"ZvirtMachineTemplate":       capiInfraZvirtAPIVersion,
}

var capiMachineAPIVersions = map[string]string{
	"DeckhouseMachine":   capiInfrastructureAPIVersion,
	"DynamixMachine":     capiInfrastructureAPIVersion,
	"HuaweiCloudMachine": capiInfrastructureAPIVersion,
	"StaticMachine":      capiInfrastructureAPIVersion,
	"VCDMachine":         capiInfraVCDAPIVersion,
	"ZvirtMachine":       capiInfraZvirtAPIVersion,
}

var capiClusterInfraAPIVersions = map[string]string{
	"DeckhouseCluster":   capiInfrastructureAPIVersion,
	"DynamixCluster":     capiInfrastructureAPIVersion,
	"HuaweiCloudCluster": capiInfrastructureAPIVersion,
	"StaticCluster":      capiInfrastructureAPIVersion,
	"VCDCluster":         capiInfraVCDAPIVersion,
	"ZvirtCluster":       capiInfraZvirtAPIVersion,
}

var capiControlPlaneAPIVersions = map[string]string{
	"DeckhouseControlPlane": capiInfrastructureAPIVersion,
}

type capiInfraRef struct {
	Name       string
	Namespace  string
	Kind       string
	APIVersion string
}

type capiClusterRefs struct {
	Name                   string
	Namespace              string
	InfraKind              string
	InfraAPIVersion        string
	ControlPlaneKind       string
	ControlPlaneAPIVersion string
}

func filterTemplateInfraRef(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	kind, _, _ := unstructured.NestedString(obj.Object, "spec", "template", "spec", "infrastructureRef", "kind")
	apiVersion, _, _ := unstructured.NestedString(obj.Object, "spec", "template", "spec", "infrastructureRef", "apiVersion")

	return capiInfraRef{
		Name:       obj.GetName(),
		Namespace:  obj.GetNamespace(),
		Kind:       kind,
		APIVersion: apiVersion,
	}, nil
}

func filterDirectInfraRef(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	kind, _, _ := unstructured.NestedString(obj.Object, "spec", "infrastructureRef", "kind")
	apiVersion, _, _ := unstructured.NestedString(obj.Object, "spec", "infrastructureRef", "apiVersion")

	return capiInfraRef{
		Name:       obj.GetName(),
		Namespace:  obj.GetNamespace(),
		Kind:       kind,
		APIVersion: apiVersion,
	}, nil
}

func filterClusterRefs(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	infraKind, _, _ := unstructured.NestedString(obj.Object, "spec", "infrastructureRef", "kind")
	infraAPIVersion, _, _ := unstructured.NestedString(obj.Object, "spec", "infrastructureRef", "apiVersion")
	cpKind, _, _ := unstructured.NestedString(obj.Object, "spec", "controlPlaneRef", "kind")
	cpAPIVersion, _, _ := unstructured.NestedString(obj.Object, "spec", "controlPlaneRef", "apiVersion")

	return capiClusterRefs{
		Name:                   obj.GetName(),
		Namespace:              obj.GetNamespace(),
		InfraKind:              infraKind,
		InfraAPIVersion:        infraAPIVersion,
		ControlPlaneKind:       cpKind,
		ControlPlaneAPIVersion: cpAPIVersion,
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

		apiVersion, ok := kindMap[ref.Kind]
		if !ok {
			input.Logger.Warn("unknown infrastructure kind", slog.String("resource", resourceKind), slog.String("name", ref.Name), slog.String("kind", ref.Kind))
			continue
		}

		if ref.APIVersion == apiVersion {
			continue
		}

		if ref.APIVersion != "" {
			input.Logger.Debug("infrastructureRef.apiVersion already set", slog.String("resource", resourceKind), slog.String("name", ref.Name), slog.String("apiVersion", ref.APIVersion))
			continue
		}

		input.PatchCollector.PatchWithMerge(buildPatch(apiVersion), capiAPIVersion, resourceKind, ref.Namespace, ref.Name)
	}
	return nil
}

func templateInfraRefPatch(apiVersion string) map[string]interface{} {
	return map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"infrastructureRef": map[string]interface{}{
						"apiVersion": apiVersion,
					},
				},
			},
		},
	}
}

func directInfraRefPatch(apiVersion string) map[string]interface{} {
	return map[string]interface{}{
		"spec": map[string]interface{}{
			"infrastructureRef": map[string]interface{}{
				"apiVersion": apiVersion,
			},
		},
	}
}

func controlPlaneRefPatch(apiVersion string) map[string]interface{} {
	return map[string]interface{}{
		"spec": map[string]interface{}{
			"controlPlaneRef": map[string]interface{}{
				"apiVersion": apiVersion,
			},
		},
	}
}

func patchClusterRefs(input *go_hook.HookInput, snaps []pkg.Snapshot) error {
	for cluster, err := range sdkobjectpatch.SnapshotIter[capiClusterRefs](snaps) {
		if err != nil {
			return fmt.Errorf("failed to iterate over Cluster snapshots: %w", err)
		}

		patchRefIfEmpty(input, cluster.Name, cluster.Namespace, cluster.InfraKind, cluster.InfraAPIVersion, capiClusterInfraAPIVersions, directInfraRefPatch)
		patchRefIfEmpty(input, cluster.Name, cluster.Namespace, cluster.ControlPlaneKind, cluster.ControlPlaneAPIVersion, capiControlPlaneAPIVersions, controlPlaneRefPatch)
	}
	return nil
}

func patchRefIfEmpty(input *go_hook.HookInput, name, namespace, kind, currentAPIVersion string, kindMap map[string]string, buildPatch func(string) map[string]interface{}) {
	if kind == "" {
		return
	}

	expectedAPIVersion, ok := kindMap[kind]
	if !ok {
		input.Logger.Warn("unknown kind", slog.String("cluster", name), slog.String("kind", kind))
		return
	}

	if currentAPIVersion == expectedAPIVersion {
		return
	}

	if currentAPIVersion != "" {
		input.Logger.Debug("apiVersion already set", slog.String("cluster", name), slog.String("apiVersion", currentAPIVersion))
		return
	}

	input.PatchCollector.PatchWithMerge(buildPatch(expectedAPIVersion), capiAPIVersion, "Cluster", namespace, name)
}

func handleSetAPIVersionOnCAPIResources(_ context.Context, input *go_hook.HookInput) error {
	if err := patchInfraRefs(input, input.Snapshots.Get("capi_machine_deployments"), capiMachineTemplateAPIVersions, "MachineDeployment", templateInfraRefPatch); err != nil {
		return err
	}

	if err := patchInfraRefs(input, input.Snapshots.Get("capi_machine_sets"), capiMachineTemplateAPIVersions, "MachineSet", templateInfraRefPatch); err != nil {
		return err
	}

	if err := patchInfraRefs(input, input.Snapshots.Get("capi_machine_pools"), capiMachineTemplateAPIVersions, "MachinePool", templateInfraRefPatch); err != nil {
		return err
	}

	if err := patchInfraRefs(input, input.Snapshots.Get("capi_machines"), capiMachineAPIVersions, "Machine", directInfraRefPatch); err != nil {
		return err
	}

	return patchClusterRefs(input, input.Snapshots.Get("capi_clusters"))
}
