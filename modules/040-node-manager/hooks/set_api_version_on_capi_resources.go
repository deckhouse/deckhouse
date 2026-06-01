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
	capiAPIVersion             = "cluster.x-k8s.io/v1beta1"
	capiInfrastructureAPIVersion = "infrastructure.cluster.x-k8s.io/v1alpha1"
)

const (
	capiInfraVCDAPIVersion   = "infrastructure.cluster.x-k8s.io/v1beta2"
	capiInfraZvirtAPIVersion = "infrastructure.cluster.x-k8s.io/v1"
)

// kind→apiVersion for MachineDeployment, MachineSet, MachinePool (infrastructureRef.kind = *Template)
var capiMachineTemplateAPIVersions = map[string]string{
	"DeckhouseMachineTemplate":   capiInfrastructureAPIVersion,
	"DynamixMachineTemplate":     capiInfrastructureAPIVersion,
	"HuaweiCloudMachineTemplate": capiInfrastructureAPIVersion,
	"StaticMachineTemplate":      capiInfrastructureAPIVersion,
	"VCDMachineTemplate":         capiInfraVCDAPIVersion,
	"ZvirtMachineTemplate":       capiInfraZvirtAPIVersion,
}

// kind→apiVersion for Machine (infrastructureRef.kind = *Machine)
var capiMachineAPIVersions = map[string]string{
	"DeckhouseMachine":   capiInfrastructureAPIVersion,
	"DynamixMachine":     capiInfrastructureAPIVersion,
	"HuaweiCloudMachine": capiInfrastructureAPIVersion,
	"StaticMachine":      capiInfrastructureAPIVersion,
	"VCDMachine":         capiInfraVCDAPIVersion,
	"ZvirtMachine":       capiInfraZvirtAPIVersion,
}

// kind→apiVersion for Cluster (infrastructureRef.kind = *Cluster)
var capiClusterInfraAPIVersions = map[string]string{
	"DeckhouseCluster":   capiInfrastructureAPIVersion,
	"DynamixCluster":     capiInfrastructureAPIVersion,
	"HuaweiCloudCluster": capiInfrastructureAPIVersion,
	"StaticCluster":      capiInfrastructureAPIVersion,
	"VCDCluster":         capiInfraVCDAPIVersion,
	"ZvirtCluster":       capiInfraZvirtAPIVersion,
}

// kind→apiVersion for Cluster (controlPlaneRef.kind)
var capiControlPlaneAPIVersions = map[string]string{
	"DeckhouseControlPlane": capiInfrastructureAPIVersion,
}

// capiInfraRef is a common structure for resources with a single infrastructureRef.
type capiInfraRef struct {
	Name       string
	Namespace  string
	Kind       string
	APIVersion string
}

// capiClusterRefs holds both infrastructureRef and controlPlaneRef from a Cluster resource.
type capiClusterRefs struct {
	Name                      string
	Namespace                 string
	InfraKind                 string
	InfraAPIVersion           string
	ControlPlaneKind          string
	ControlPlaneAPIVersion    string
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
}, handleSetInfraRefAPIVersion)

// patchTemplateInfraRefs patches spec.template.spec.infrastructureRef.apiVersion for resources
// like MachineDeployment, MachineSet, MachinePool.
func patchTemplateInfraRefs(input *go_hook.HookInput, snaps []pkg.Snapshot, kindMap map[string]string, resourceKind string) error {
	for ref, err := range sdkobjectpatch.SnapshotIter[capiInfraRef](snaps) {
		if err != nil {
			return fmt.Errorf("failed to iterate over %s snapshots: %w", resourceKind, err)
		}

		apiVersion, ok := kindMap[ref.Kind]
		if !ok {
			input.Logger.Warn("unknown infrastructure template kind", slog.String("resource", resourceKind), slog.String("name", ref.Name), slog.String("kind", ref.Kind))
			continue
		}

		if ref.APIVersion == apiVersion {
			continue
		}

		if ref.APIVersion != "" {
			input.Logger.Debug("infrastructureRef.apiVersion already set", slog.String("resource", resourceKind), slog.String("name", ref.Name), slog.String("apiVersion", ref.APIVersion))
			continue
		}

		patch := map[string]interface{}{
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

		input.PatchCollector.PatchWithMerge(patch, capiAPIVersion, resourceKind, ref.Namespace, ref.Name)
	}
	return nil
}

// patchDirectInfraRefs patches spec.infrastructureRef.apiVersion for resources like Machine.
func patchDirectInfraRefs(input *go_hook.HookInput, snaps []pkg.Snapshot, kindMap map[string]string, resourceKind string) error {
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

		patch := map[string]interface{}{
			"spec": map[string]interface{}{
				"infrastructureRef": map[string]interface{}{
					"apiVersion": apiVersion,
				},
			},
		}

		input.PatchCollector.PatchWithMerge(patch, capiAPIVersion, resourceKind, ref.Namespace, ref.Name)
	}
	return nil
}

// patchClusterRefs patches both spec.infrastructureRef.apiVersion and spec.controlPlaneRef.apiVersion
// on Cluster resources.
func patchClusterRefs(input *go_hook.HookInput, snaps []pkg.Snapshot) error {
	for cluster, err := range sdkobjectpatch.SnapshotIter[capiClusterRefs](snaps) {
		if err != nil {
			return fmt.Errorf("failed to iterate over Cluster snapshots: %w", err)
		}

		// Patch infrastructureRef
		if infraAPIVersion, ok := capiClusterInfraAPIVersions[cluster.InfraKind]; ok {
			if cluster.InfraAPIVersion == "" {
				patch := map[string]interface{}{
					"spec": map[string]interface{}{
						"infrastructureRef": map[string]interface{}{
							"apiVersion": infraAPIVersion,
						},
					},
				}
				input.PatchCollector.PatchWithMerge(patch, capiAPIVersion, "Cluster", cluster.Namespace, cluster.Name)
			} else if cluster.InfraAPIVersion != infraAPIVersion {
				input.Logger.Debug("cluster infrastructureRef.apiVersion already set", slog.String("cluster", cluster.Name), slog.String("apiVersion", cluster.InfraAPIVersion))
			}
		} else if cluster.InfraKind != "" {
			input.Logger.Warn("unknown infrastructure cluster kind", slog.String("cluster", cluster.Name), slog.String("kind", cluster.InfraKind))
		}

		// Patch controlPlaneRef
		if cpAPIVersion, ok := capiControlPlaneAPIVersions[cluster.ControlPlaneKind]; ok {
			if cluster.ControlPlaneAPIVersion == "" {
				patch := map[string]interface{}{
					"spec": map[string]interface{}{
						"controlPlaneRef": map[string]interface{}{
							"apiVersion": cpAPIVersion,
						},
					},
				}
				input.PatchCollector.PatchWithMerge(patch, capiAPIVersion, "Cluster", cluster.Namespace, cluster.Name)
			} else if cluster.ControlPlaneAPIVersion != cpAPIVersion {
				input.Logger.Debug("cluster controlPlaneRef.apiVersion already set", slog.String("cluster", cluster.Name), slog.String("apiVersion", cluster.ControlPlaneAPIVersion))
			}
		} else if cluster.ControlPlaneKind != "" {
			input.Logger.Warn("unknown control plane kind", slog.String("cluster", cluster.Name), slog.String("kind", cluster.ControlPlaneKind))
		}
	}
	return nil
}

func handleSetInfraRefAPIVersion(_ context.Context, input *go_hook.HookInput) error {
	if err := patchTemplateInfraRefs(input, input.Snapshots.Get("capi_machine_deployments"), capiMachineTemplateAPIVersions, "MachineDeployment"); err != nil {
		return err
	}

	if err := patchTemplateInfraRefs(input, input.Snapshots.Get("capi_machine_sets"), capiMachineTemplateAPIVersions, "MachineSet"); err != nil {
		return err
	}

	if err := patchTemplateInfraRefs(input, input.Snapshots.Get("capi_machine_pools"), capiMachineTemplateAPIVersions, "MachinePool"); err != nil {
		return err
	}

	if err := patchDirectInfraRefs(input, input.Snapshots.Get("capi_machines"), capiMachineAPIVersions, "Machine"); err != nil {
		return err
	}

	if err := patchClusterRefs(input, input.Snapshots.Get("capi_clusters")); err != nil {
		return err
	}

	return nil
}
