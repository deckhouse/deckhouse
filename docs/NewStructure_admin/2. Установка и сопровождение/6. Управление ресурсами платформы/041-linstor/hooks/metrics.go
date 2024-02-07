/*
Copyright 2023 Flant JSC

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
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/telemetry"
)

const (
	linstorNodesSnapshot     = "linstor_nodes"
	linstorSPsSnapshot       = "linstor_storagepools"
	linstorRDsSnapshot       = "linstor_resource_definitions"
	linstorResourcesSnapshot = "linstor_resources"
	linstorCRDsSnapshot      = "linstor_crds"
	nodeTypeSatellite        = 2
)

type linstorNodeSnapshot struct {
	Type int64
}

type linstorSPSnapshot struct {
	Driver string
}

type linstorRDSnapshot struct {
	IsSnapshot bool
}

type linstorResourceSnapshot struct {
	IsSnapshot bool
}

type linstorObj struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Spec              map[string]interface{}
}

var linstorMetricsHookConfig = &go_hook.HookConfig{
	Queue: "/modules/linstor/metrics",
	Kubernetes: []go_hook.KubernetesConfig{
		// A bindings with dynamic kind have index 0-4 for simplicity.
		{
			Name:       linstorNodesSnapshot,
			ApiVersion: "",
			Kind:       "",
			FilterFunc: applyLinstorNodeFilter,
		},
		{
			Name:       linstorSPsSnapshot,
			ApiVersion: "",
			Kind:       "",
			FilterFunc: applyLinstorSPFilter,
		},
		{
			Name:       linstorRDsSnapshot,
			ApiVersion: "",
			Kind:       "",
			FilterFunc: applyLinstorRDFilter,
		},
		{
			Name:       linstorResourcesSnapshot,
			ApiVersion: "",
			Kind:       "",
			FilterFunc: applyLinstorResourceFilter,
		},
		{
			Name:       linstorCRDsSnapshot,
			ApiVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
			NameSelector: &types.NameSelector{
				MatchNames: []string{
					"nodes.internal.linstor.linbit.com",
					"nodestorpool.internal.linstor.linbit.com",
					"resourcedefinitions.internal.linstor.linbit.com",
					"resources.internal.linstor.linbit.com"},
			},
			FilterFunc: applyCRDVersionsFilter,
		},
	},
}

var _ = sdk.RegisterFunc(linstorMetricsHookConfig, collectLinstorMetrics)

func applyCRDVersionsFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var s apiextensions.CustomResourceDefinition
	err := sdk.FromUnstructured(obj, &s)
	if err != nil {
		return "", err
	}

	for _, v := range s.Spec.Versions {
		if v.Storage {
			return v.Name, nil
		}
	}
	return "", fmt.Errorf("Can not find storage version for %s", s.Name)
}

func applyLinstorNodeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var s linstorObj
	err := sdk.FromUnstructured(obj, &s)
	if err != nil {
		return "", err
	}

	nodeType, ok := s.Spec["node_type"].(int64)
	if !ok {
		nodeTypeFloat64, _ := s.Spec["node_type"].(float64)
		nodeType = int64(nodeTypeFloat64)
	}

	return linstorNodeSnapshot{
		Type: nodeType,
	}, nil
}

func applyLinstorSPFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var s linstorObj
	err := sdk.FromUnstructured(obj, &s)
	if err != nil {
		return "", err
	}

	driverName, _ := s.Spec["driver_name"].(string)

	return linstorSPSnapshot{
		Driver: driverName,
	}, nil
}

func applyLinstorRDFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var s linstorObj
	err := sdk.FromUnstructured(obj, &s)
	if err != nil {
		return "", err
	}

	snapshotName, _ := s.Spec["snapshot_name"].(string)

	return linstorRDSnapshot{
		IsSnapshot: snapshotName != "",
	}, nil
}

func applyLinstorResourceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var s linstorObj
	err := sdk.FromUnstructured(obj, &s)
	if err != nil {
		return "", err
	}

	snapshotName, _ := s.Spec["snapshot_name"].(string)

	return linstorResourceSnapshot{
		IsSnapshot: snapshotName != "",
	}, nil
}

// collectLinstorMetrics
//
// synopsis:
//
//	Waits for internal LINSTOR resources be installed then
//	collects general statistics from them
func collectLinstorMetrics(input *go_hook.HookInput) error {
	input.MetricsCollector.Set(telemetry.WrapName("linstor_enabled"), 1, map[string]string{})

	// LINSTOR manages it's own internal CRDs, so we need to wait for them before starting the watch
	if linstorMetricsHookConfig.Kubernetes[0].Kind == "" {
		var version string
		for _, sRaw := range input.Snapshots[linstorCRDsSnapshot] {
			discoveredVersion := sRaw.(string)
			if version == "" {
				version = discoveredVersion
			} else if version != discoveredVersion {
				return fmt.Errorf("LINSTOR internal CRDs have different storage versions")
			}
		}

		if len(input.Snapshots[linstorCRDsSnapshot]) >= 4 {
			// LINSTOR installed
			input.LogEntry.Infof("LINSTOR internal CRDs installed, update kind for binding linstor resources to collect metrics")
			apiVersion := "internal.linstor.linbit.com/" + version
			*input.BindingActions = append(*input.BindingActions, go_hook.BindingAction{
				Name:       linstorNodesSnapshot,
				Action:     "UpdateKind",
				ApiVersion: apiVersion,
				Kind:       "Nodes",
			})
			*input.BindingActions = append(*input.BindingActions, go_hook.BindingAction{
				Name:       linstorSPsSnapshot,
				Action:     "UpdateKind",
				ApiVersion: apiVersion,
				Kind:       "NodeStorPool",
			})
			*input.BindingActions = append(*input.BindingActions, go_hook.BindingAction{
				Name:       linstorRDsSnapshot,
				Action:     "UpdateKind",
				ApiVersion: apiVersion,
				Kind:       "ResourceDefinitions",
			})
			*input.BindingActions = append(*input.BindingActions, go_hook.BindingAction{
				Name:       linstorResourcesSnapshot,
				Action:     "UpdateKind",
				ApiVersion: apiVersion,
				Kind:       "Resources",
			})
			// Save new kind as current kind.
			linstorMetricsHookConfig.Kubernetes[0].Kind = "Nodes"
			linstorMetricsHookConfig.Kubernetes[0].ApiVersion = apiVersion
			linstorMetricsHookConfig.Kubernetes[1].Kind = "NodeStorPool"
			linstorMetricsHookConfig.Kubernetes[1].ApiVersion = apiVersion
			linstorMetricsHookConfig.Kubernetes[2].Kind = "ResourceDefinitions"
			linstorMetricsHookConfig.Kubernetes[2].ApiVersion = apiVersion
			linstorMetricsHookConfig.Kubernetes[3].Kind = "Resources"
			linstorMetricsHookConfig.Kubernetes[3].ApiVersion = apiVersion
			// Binding changed, hook will be restarted with new objects in snapshot.
			return nil
		}
		// LINSTOR is not yet installed, do nothing
		return nil
	}

	// Start main hook logic

	// Collect general amount of lisntor nodes
	var linstorSatellitesCount float64
	for _, sRaw := range input.Snapshots[linstorNodesSnapshot] {
		s := sRaw.(linstorNodeSnapshot)

		if s.Type == nodeTypeSatellite {
			linstorSatellitesCount++
		}
	}
	input.MetricsCollector.Set(telemetry.WrapName("linstor_satellites"), linstorSatellitesCount, map[string]string{})

	// Collect general amount of lisntor storage pools by type
	linstorStoragePoolsCount := make(map[string]float64)
	for _, sRaw := range input.Snapshots[linstorSPsSnapshot] {
		s := sRaw.(linstorSPSnapshot)
		if s.Driver != "" {
			linstorStoragePoolsCount[s.Driver]++
		}
	}
	for driver, count := range linstorStoragePoolsCount {
		input.MetricsCollector.Set(telemetry.WrapName("linstor_storage_pools"), count, map[string]string{
			"driver": driver,
		})
	}

	// Collect general amount of lisntor resource definitions and snapshot definitions
	var linstorResourceDefinitionsCount float64
	var linstorSnapshotDefinitionsCount float64
	for _, sRaw := range input.Snapshots[linstorRDsSnapshot] {
		s := sRaw.(linstorRDSnapshot)
		if s.IsSnapshot {
			linstorSnapshotDefinitionsCount++
		} else {
			linstorResourceDefinitionsCount++
		}
	}
	input.MetricsCollector.Set(telemetry.WrapName("linstor_resource_definitions"), linstorResourceDefinitionsCount, map[string]string{})
	input.MetricsCollector.Set(telemetry.WrapName("linstor_snapshot_definitions"), linstorSnapshotDefinitionsCount, map[string]string{})

	// Collect general amount of lisntor resources
	var linstorResourcesCount float64
	for _, sRaw := range input.Snapshots[linstorResourcesSnapshot] {
		s := sRaw.(linstorResourceSnapshot)
		if !s.IsSnapshot {
			linstorResourcesCount++
		}
	}
	input.MetricsCollector.Set(telemetry.WrapName("linstor_resources"), linstorResourcesCount, map[string]string{})

	return nil
}
