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

// capiMachineDeploymentInfo contains all fields that can be lost during v1beta2 hub pruning.
type capiMachineDeploymentInfo struct {
	Name      string
	Namespace string
	NodeGroup string

	// infrastructureRef fields
	InfraKind       string
	InfraAPIVersion string
	InfraNamespace  string

	// timeout fields
	NodeDrainTimeout        string
	NodeDeletionTimeout     string
	NodeVolumeDetachTimeout string

	// strategy fields
	StrategyType              string
	RollingUpdateMaxSurge     interface{}
	RollingUpdateMaxUnavail   interface{}
}

// capiMachineInfo contains fields that can be lost on Machine resources.
type capiMachineInfo struct {
	Name      string
	Namespace string

	InfraKind       string
	InfraAPIVersion string
	InfraNamespace  string

	NodeDrainTimeout        string
	NodeDeletionTimeout     string
	NodeVolumeDetachTimeout string
}

// capiClusterRefs contains fields that can be lost on Cluster resources.
type capiClusterRefs struct {
	Name      string
	Namespace string

	InfraKind       string
	InfraAPIVersion string
	InfraNamespace  string

	ControlPlaneKind       string
	ControlPlaneAPIVersion string
	ControlPlaneNamespace  string
}

// capiMHCInfo contains fields that can be lost on MachineHealthCheck resources.
type capiMHCInfo struct {
	Name        string
	Namespace   string
	ClusterName string

	NodeStartupTimeout string

	HasUnhealthyConditions bool
}

func filterMachineDeployment(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	labels := obj.GetLabels()

	infraKind, _, _ := unstructured.NestedString(obj.Object, "spec", "template", "spec", "infrastructureRef", "kind")
	infraAPIVersion, _, _ := unstructured.NestedString(obj.Object, "spec", "template", "spec", "infrastructureRef", "apiVersion")
	infraNamespace, _, _ := unstructured.NestedString(obj.Object, "spec", "template", "spec", "infrastructureRef", "namespace")

	nodeDrainTimeout, _, _ := unstructured.NestedString(obj.Object, "spec", "template", "spec", "nodeDrainTimeout")
	nodeDeletionTimeout, _, _ := unstructured.NestedString(obj.Object, "spec", "template", "spec", "nodeDeletionTimeout")
	nodeVolumeDetachTimeout, _, _ := unstructured.NestedString(obj.Object, "spec", "template", "spec", "nodeVolumeDetachTimeout")

	strategyType, _, _ := unstructured.NestedString(obj.Object, "spec", "strategy", "type")
	maxSurge, _, _ := unstructured.NestedFieldNoCopy(obj.Object, "spec", "strategy", "rollingUpdate", "maxSurge")
	maxUnavail, _, _ := unstructured.NestedFieldNoCopy(obj.Object, "spec", "strategy", "rollingUpdate", "maxUnavailable")

	return capiMachineDeploymentInfo{
		Name:                    obj.GetName(),
		Namespace:               obj.GetNamespace(),
		NodeGroup:               labels["node-group"],
		InfraKind:               infraKind,
		InfraAPIVersion:         infraAPIVersion,
		InfraNamespace:          infraNamespace,
		NodeDrainTimeout:        nodeDrainTimeout,
		NodeDeletionTimeout:     nodeDeletionTimeout,
		NodeVolumeDetachTimeout: nodeVolumeDetachTimeout,
		StrategyType:            strategyType,
		RollingUpdateMaxSurge:   maxSurge,
		RollingUpdateMaxUnavail: maxUnavail,
	}, nil
}

func filterMachine(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	infraKind, _, _ := unstructured.NestedString(obj.Object, "spec", "infrastructureRef", "kind")
	infraAPIVersion, _, _ := unstructured.NestedString(obj.Object, "spec", "infrastructureRef", "apiVersion")
	infraNamespace, _, _ := unstructured.NestedString(obj.Object, "spec", "infrastructureRef", "namespace")

	nodeDrainTimeout, _, _ := unstructured.NestedString(obj.Object, "spec", "nodeDrainTimeout")
	nodeDeletionTimeout, _, _ := unstructured.NestedString(obj.Object, "spec", "nodeDeletionTimeout")
	nodeVolumeDetachTimeout, _, _ := unstructured.NestedString(obj.Object, "spec", "nodeVolumeDetachTimeout")

	return capiMachineInfo{
		Name:                    obj.GetName(),
		Namespace:               obj.GetNamespace(),
		InfraKind:               infraKind,
		InfraAPIVersion:         infraAPIVersion,
		InfraNamespace:          infraNamespace,
		NodeDrainTimeout:        nodeDrainTimeout,
		NodeDeletionTimeout:     nodeDeletionTimeout,
		NodeVolumeDetachTimeout: nodeVolumeDetachTimeout,
	}, nil
}

func filterClusterRefs(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	infraKind, _, _ := unstructured.NestedString(obj.Object, "spec", "infrastructureRef", "kind")
	infraAPIVersion, _, _ := unstructured.NestedString(obj.Object, "spec", "infrastructureRef", "apiVersion")
	infraNamespace, _, _ := unstructured.NestedString(obj.Object, "spec", "infrastructureRef", "namespace")
	cpKind, _, _ := unstructured.NestedString(obj.Object, "spec", "controlPlaneRef", "kind")
	cpAPIVersion, _, _ := unstructured.NestedString(obj.Object, "spec", "controlPlaneRef", "apiVersion")
	cpNamespace, _, _ := unstructured.NestedString(obj.Object, "spec", "controlPlaneRef", "namespace")

	return capiClusterRefs{
		Name:                   obj.GetName(),
		Namespace:              obj.GetNamespace(),
		InfraKind:              infraKind,
		InfraAPIVersion:        infraAPIVersion,
		InfraNamespace:         infraNamespace,
		ControlPlaneKind:       cpKind,
		ControlPlaneAPIVersion: cpAPIVersion,
		ControlPlaneNamespace:  cpNamespace,
	}, nil
}

func filterMHC(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	clusterName, _, _ := unstructured.NestedString(obj.Object, "spec", "clusterName")
	nodeStartupTimeout, _, _ := unstructured.NestedString(obj.Object, "spec", "nodeStartupTimeout")
	conditions, conditionsFound, _ := unstructured.NestedSlice(obj.Object, "spec", "unhealthyConditions")

	return capiMHCInfo{
		Name:                   obj.GetName(),
		Namespace:              obj.GetNamespace(),
		ClusterName:            clusterName,
		NodeStartupTimeout:     nodeStartupTimeout,
		HasUnhealthyConditions: conditionsFound && len(conditions) > 0,
	}, nil
}

// filterTemplateInfraRef is kept for MachineSet and MachinePool which share the same template-based ref structure.
func filterTemplateInfraRef(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	kind, _, _ := unstructured.NestedString(obj.Object, "spec", "template", "spec", "infrastructureRef", "kind")
	apiVersion, _, _ := unstructured.NestedString(obj.Object, "spec", "template", "spec", "infrastructureRef", "apiVersion")
	namespace, _, _ := unstructured.NestedString(obj.Object, "spec", "template", "spec", "infrastructureRef", "namespace")

	return capiInfraRef{
		Name:       obj.GetName(),
		Namespace:  obj.GetNamespace(),
		Kind:       kind,
		APIVersion: apiVersion,
		RefNS:      namespace,
	}, nil
}

type capiInfraRef struct {
	Name       string
	Namespace  string
	Kind       string
	APIVersion string
	RefNS      string
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager/set_fields_on_capi_resources",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                   "capi_machine_deployments",
			ApiVersion:             capiAPIVersion,
			Kind:                   "MachineDeployment",
			WaitForSynchronization: ptr.To(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{capiNamespace},
				},
			},
			FilterFunc: filterMachineDeployment,
		},
		{
			Name:                   "capi_machine_sets",
			ApiVersion:             capiAPIVersion,
			Kind:                   "MachineSet",
			WaitForSynchronization: ptr.To(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{capiNamespace},
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
					MatchNames: []string{capiNamespace},
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
					MatchNames: []string{capiNamespace},
				},
			},
			FilterFunc: filterMachine,
		},
		{
			Name:                   "capi_clusters",
			ApiVersion:             capiAPIVersion,
			Kind:                   "Cluster",
			WaitForSynchronization: ptr.To(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{capiNamespace},
				},
			},
			FilterFunc: filterClusterRefs,
		},
		{
			Name:                   "capi_machine_health_checks",
			ApiVersion:             capiAPIVersion,
			Kind:                   "MachineHealthCheck",
			WaitForSynchronization: ptr.To(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{capiNamespace},
				},
			},
			FilterFunc: filterMHC,
		},
	},
}, handleSetAPIVersionOnCAPIResources)

// nodeGroupValues holds the expected field values extracted from input.Values for a specific NodeGroup.
type nodeGroupValues struct {
	NodeDrainTimeout    string
	MaxSurgePerZone     interface{}
	MaxUnavailPerZone   interface{}
}

// getNodeGroupValuesMap builds a map of NodeGroup name -> expected values from input.Values.
func getNodeGroupValuesMap(input *go_hook.HookInput) map[string]nodeGroupValues {
	result := make(map[string]nodeGroupValues)

	nodeGroupsRaw := input.Values.Get("nodeManager.internal.nodeGroups")
	if !nodeGroupsRaw.Exists() {
		return result
	}

	for _, ng := range nodeGroupsRaw.Array() {
		name := ng.Get("name").String()
		if name == "" {
			continue
		}

		ngv := nodeGroupValues{
			NodeDrainTimeout: "10m",
			MaxSurgePerZone:  int64(1),
			MaxUnavailPerZone: int64(0),
		}

		if drainSec := ng.Get("nodeDrainTimeoutSecond"); drainSec.Exists() {
			ngv.NodeDrainTimeout = fmt.Sprintf("%ds", drainSec.Int())
		}

		if ms := ng.Get("cloudInstances.maxSurgePerZone"); ms.Exists() {
			ngv.MaxSurgePerZone = ms.Int()
		}

		if mu := ng.Get("cloudInstances.maxUnavailablePerZone"); mu.Exists() {
			ngv.MaxUnavailPerZone = mu.Int()
		}

		result[name] = ngv
	}

	return result
}

func handleSetAPIVersionOnCAPIResources(_ context.Context, input *go_hook.HookInput) error {
	ngValues := getNodeGroupValuesMap(input)

	if err := repairMachineDeployments(input, ngValues); err != nil {
		return err
	}

	if err := patchInfraRefs(input, input.Snapshots.Get("capi_machine_sets"), capiMachineTemplateAPIVersions, "MachineSet"); err != nil {
		return err
	}

	if err := patchInfraRefs(input, input.Snapshots.Get("capi_machine_pools"), capiMachineTemplateAPIVersions, "MachinePool"); err != nil {
		return err
	}

	if err := repairMachines(input); err != nil {
		return err
	}

	if err := repairClusters(input); err != nil {
		return err
	}

	return repairMHCs(input)
}

// repairMachineDeployments patches all potentially lost fields on MachineDeployment resources.
func repairMachineDeployments(input *go_hook.HookInput, ngValues map[string]nodeGroupValues) error {
	for md, err := range sdkobjectpatch.SnapshotIter[capiMachineDeploymentInfo](input.Snapshots.Get("capi_machine_deployments")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over MachineDeployment snapshots: %w", err)
		}

		patch := make(map[string]interface{})

		// Repair infrastructureRef
		infraPatch := repairRefFields(input, md.InfraKind, md.InfraAPIVersion, md.InfraNamespace, capiMachineTemplateAPIVersions, md.Name, "MachineDeployment")
		if infraPatch != nil {
			setNestedMap(patch, infraPatch, "spec", "template", "spec", "infrastructureRef")
		}

		// Repair timeout fields
		templateSpecPatch := repairTimeouts(md.NodeDrainTimeout, md.NodeDeletionTimeout, md.NodeVolumeDetachTimeout, ngValues, md.NodeGroup)
		if templateSpecPatch != nil {
			for k, v := range templateSpecPatch {
				setNestedValue(patch, v, "spec", "template", "spec", k)
			}
		}

		// Repair strategy fields
		strategyPatch := repairStrategy(md.StrategyType, md.RollingUpdateMaxSurge, md.RollingUpdateMaxUnavail, ngValues, md.NodeGroup)
		if strategyPatch != nil {
			setNestedMap(patch, strategyPatch, "spec", "strategy")
		}

		if len(patch) > 0 {
			input.Logger.Info("repairing MachineDeployment fields", slog.String("name", md.Name))
			input.PatchCollector.PatchWithMerge(patch, capiAPIVersion, "MachineDeployment", md.Namespace, md.Name)
		}
	}
	return nil
}

// repairMachines patches all potentially lost fields on Machine resources.
func repairMachines(input *go_hook.HookInput) error {
	for m, err := range sdkobjectpatch.SnapshotIter[capiMachineInfo](input.Snapshots.Get("capi_machines")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over Machine snapshots: %w", err)
		}

		patch := make(map[string]interface{})

		infraPatch := repairRefFields(input, m.InfraKind, m.InfraAPIVersion, m.InfraNamespace, capiMachineAPIVersions, m.Name, "Machine")
		if infraPatch != nil {
			setNestedMap(patch, infraPatch, "spec", "infrastructureRef")
		}

		timeoutPatch := repairTimeouts(m.NodeDrainTimeout, m.NodeDeletionTimeout, m.NodeVolumeDetachTimeout, nil, "")
		if timeoutPatch != nil {
			for k, v := range timeoutPatch {
				setNestedValue(patch, v, "spec", k)
			}
		}

		if len(patch) > 0 {
			input.Logger.Info("repairing Machine fields", slog.String("name", m.Name))
			input.PatchCollector.PatchWithMerge(patch, capiAPIVersion, "Machine", m.Namespace, m.Name)
		}
	}
	return nil
}

// repairClusters patches all potentially lost fields on Cluster resources.
func repairClusters(input *go_hook.HookInput) error {
	for cluster, err := range sdkobjectpatch.SnapshotIter[capiClusterRefs](input.Snapshots.Get("capi_clusters")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over Cluster snapshots: %w", err)
		}

		patch := make(map[string]interface{})

		infraPatch := repairRefFields(input, cluster.InfraKind, cluster.InfraAPIVersion, cluster.InfraNamespace, capiClusterInfraAPIVersions, cluster.Name, "Cluster")
		if infraPatch != nil {
			setNestedMap(patch, infraPatch, "spec", "infrastructureRef")
		}

		cpPatch := repairRefFields(input, cluster.ControlPlaneKind, cluster.ControlPlaneAPIVersion, cluster.ControlPlaneNamespace, capiControlPlaneAPIVersions, cluster.Name, "Cluster")
		if cpPatch != nil {
			setNestedMap(patch, cpPatch, "spec", "controlPlaneRef")
		}

		if len(patch) > 0 {
			input.Logger.Info("repairing Cluster fields", slog.String("name", cluster.Name))
			input.PatchCollector.PatchWithMerge(patch, capiAPIVersion, "Cluster", cluster.Namespace, cluster.Name)
		}
	}
	return nil
}

// repairMHCs patches all potentially lost fields on MachineHealthCheck resources.
func repairMHCs(input *go_hook.HookInput) error {
	for mhc, err := range sdkobjectpatch.SnapshotIter[capiMHCInfo](input.Snapshots.Get("capi_machine_health_checks")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over MachineHealthCheck snapshots: %w", err)
		}

		patch := make(map[string]interface{})

		if mhc.NodeStartupTimeout == "" {
			setNestedValue(patch, "20m", "spec", "nodeStartupTimeout")
		}

		if !mhc.HasUnhealthyConditions {
			// Static clusters (CAPS) use very long timeout (effectively disabled),
			// cloud clusters use 5m.
			timeout := "5m"
			if mhc.ClusterName == "static" {
				timeout = "876000h"
			}
			conditions := []interface{}{
				map[string]interface{}{
					"type":    "Ready",
					"status":  "Unknown",
					"timeout": timeout,
				},
				map[string]interface{}{
					"type":    "Ready",
					"status":  "False",
					"timeout": timeout,
				},
			}
			setNestedValue(patch, conditions, "spec", "unhealthyConditions")
		}

		if len(patch) > 0 {
			input.Logger.Info("repairing MachineHealthCheck fields", slog.String("name", mhc.Name))
			input.PatchCollector.PatchWithMerge(patch, capiAPIVersion, "MachineHealthCheck", mhc.Namespace, mhc.Name)
		}
	}
	return nil
}

// patchInfraRefs handles MachineSet and MachinePool — patches apiVersion and namespace in template-based infrastructureRef.
func patchInfraRefs(input *go_hook.HookInput, snaps []pkg.Snapshot, kindMap map[string]string, resourceKind string) error {
	for ref, err := range sdkobjectpatch.SnapshotIter[capiInfraRef](snaps) {
		if err != nil {
			return fmt.Errorf("failed to iterate over %s snapshots: %w", resourceKind, err)
		}

		patch := repairRefFields(input, ref.Kind, ref.APIVersion, ref.RefNS, kindMap, ref.Name, resourceKind)
		if patch != nil {
			input.Logger.Info("repairing ref fields", slog.String("resource", resourceKind), slog.String("name", ref.Name))
			mergePatch := map[string]interface{}{
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"infrastructureRef": patch,
						},
					},
				},
			}
			input.PatchCollector.PatchWithMerge(mergePatch, capiAPIVersion, resourceKind, ref.Namespace, ref.Name)
		}
	}
	return nil
}

// repairRefFields returns a partial patch for a single ObjectReference if any fields are missing.
// Returns nil if no repair is needed.
func repairRefFields(input *go_hook.HookInput, kind, apiVersion, namespace string, kindMap map[string]string, objName, resourceKind string) map[string]interface{} {
	if kind == "" {
		return nil
	}

	patch := make(map[string]interface{})

	// Repair apiVersion
	expectedAPIVersion, ok := kindMap[kind]
	if !ok {
		input.Logger.Warn("unknown kind", slog.String("resource", resourceKind), slog.String("name", objName), slog.String("kind", kind))
	} else if apiVersion == "" {
		patch["apiVersion"] = expectedAPIVersion
	}

	// Repair namespace
	if namespace == "" {
		patch["namespace"] = capiNamespace
	}

	if len(patch) == 0 {
		return nil
	}
	return patch
}

// repairTimeouts returns a map of timeout fields that need to be patched.
// For MachineDeployment: uses NodeGroup values for nodeDrainTimeout.
// For Machine: uses defaults only (no NodeGroup link).
func repairTimeouts(nodeDrainTimeout, nodeDeletionTimeout, nodeVolumeDetachTimeout string, ngValues map[string]nodeGroupValues, nodeGroup string) map[string]interface{} {
	patch := make(map[string]interface{})

	if nodeDrainTimeout == "" {
		expectedDrainTimeout := "10m"
		if ngValues != nil && nodeGroup != "" {
			if ngv, ok := ngValues[nodeGroup]; ok {
				expectedDrainTimeout = ngv.NodeDrainTimeout
			}
		}
		patch["nodeDrainTimeout"] = expectedDrainTimeout
	}

	if nodeDeletionTimeout == "" {
		patch["nodeDeletionTimeout"] = "10m"
	}

	if nodeVolumeDetachTimeout == "" {
		patch["nodeVolumeDetachTimeout"] = "10m"
	}

	if len(patch) == 0 {
		return nil
	}
	return patch
}

// repairStrategy returns a partial patch for spec.strategy if fields are missing.
func repairStrategy(strategyType string, maxSurge, maxUnavail interface{}, ngValues map[string]nodeGroupValues, nodeGroup string) map[string]interface{} {
	if strategyType != "" && maxSurge != nil && maxUnavail != nil {
		return nil
	}

	patch := make(map[string]interface{})

	if strategyType == "" {
		patch["type"] = "RollingUpdate"
	}

	if maxSurge == nil || maxUnavail == nil {
		expectedMaxSurge := interface{}(int64(1))
		expectedMaxUnavail := interface{}(int64(0))

		if ngValues != nil && nodeGroup != "" {
			if ngv, ok := ngValues[nodeGroup]; ok {
				expectedMaxSurge = ngv.MaxSurgePerZone
				expectedMaxUnavail = ngv.MaxUnavailPerZone
			}
		}

		rollingUpdate := make(map[string]interface{})
		if maxSurge == nil {
			rollingUpdate["maxSurge"] = expectedMaxSurge
		}
		if maxUnavail == nil {
			rollingUpdate["maxUnavailable"] = expectedMaxUnavail
		}
		patch["rollingUpdate"] = rollingUpdate
	}

	return patch
}

// setNestedValue sets a value at the given path in a nested map structure.
func setNestedValue(m map[string]interface{}, value interface{}, path ...string) {
	current := m
	for i := 0; i < len(path)-1; i++ {
		next, ok := current[path[i]].(map[string]interface{})
		if !ok {
			next = make(map[string]interface{})
			current[path[i]] = next
		}
		current = next
	}
	current[path[len(path)-1]] = value
}

// setNestedMap merges a source map into a nested path in the target map.
func setNestedMap(m map[string]interface{}, source map[string]interface{}, path ...string) {
	current := m
	for _, p := range path {
		next, ok := current[p].(map[string]interface{})
		if !ok {
			next = make(map[string]interface{})
			current[p] = next
		}
		current = next
	}
	for k, v := range source {
		current[k] = v
	}
}
