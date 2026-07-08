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
	"encoding/json"
	"fmt"

	cljson "github.com/clarketm/json"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/set"
	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
)

type NodeGroupCrdInfo struct {
	Name   string
	Spec   ngv1.NodeGroupSpec
	Engine ngv1.NodeGroupEngine
	UseMCM bool
}

const useMCMAnnotation = "node.deckhouse.io/use-mcm"

// applyNodeGroupCrdFilter returns name, spec, status.engine and use-mcm annotation from the NodeGroup.
func applyNodeGroupCrdFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var nodeGroup ngv1.NodeGroup
	err := sdk.FromUnstructured(obj, &nodeGroup)
	if err != nil {
		return nil, err
	}

	return NodeGroupCrdInfo{
		Name:   nodeGroup.GetName(),
		Spec:   nodeGroup.Spec,
		Engine: nodeGroup.Status.Engine,
		UseMCM: nodeGroup.GetAnnotations()[useMCMAnnotation] != "",
	}, nil
}

type MachineDeploymentCrdInfo struct {
	Name string
	Zone string
}

func applyMachineDeploymentCrdFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return MachineDeploymentCrdInfo{
		Name: obj.GetName(),
		Zone: obj.GetAnnotations()["zone"],
	}, nil
}

func applyCloudProviderSecretKindZonesFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secretData, err := decodeDataFromSecret(obj)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"instanceClassKind": secretData["instanceClassKind"],
		"zones":             secretData["zones"],
	}, nil
}

var getCRDsHookConfig = &go_hook.HookConfig{
	Queue:        "/modules/node-manager",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "ngs",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "NodeGroup",
			FilterFunc: applyNodeGroupCrdFilter,
		},
		{
			Name:       "machine_deployments",
			ApiVersion: "machine.sapcloud.io/v1alpha1",
			Kind:       "MachineDeployment",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-instance-manager"},
				},
			},
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "heritage",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"deckhouse"},
					},
				},
			},
			FilterFunc: applyMachineDeploymentCrdFilter,
		},
		// kube-system/Secret/d8-node-manager-cloud-provider
		{
			Name:       "cloud_provider_secret",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-node-manager-cloud-provider"},
			},
			FilterFunc: applyCloudProviderSecretKindZonesFilter,
		},
	},
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "sync",
			Crontab: "*/10 * * * *",
		},
	},
}

var _ = sdk.RegisterFunc(getCRDsHookConfig, getCRDsHandler)

// getCRDsHandler builds the thin nodeManager.internal.nodeGroups blob. It is a passthrough
// of the NodeGroup spec enriched with name, engine and defaulted cloudInstances.zones.
// All validation, status, instanceClass overlay, capacity, CRI/kubernetesVersion resolution,
// updateEpoch and serialized labels/taints are owned by node-controller now. The only extra
// responsibility kept here is the node_group_info metric that feeds the module 340
// UnsupportedContainerRuntimeVersion alert.
func getCRDsHandler(_ context.Context, input *go_hook.HookInput) error {
	// Default zones. Take them from machine_deployments and cloud_provider_secret.zones.
	defaultZones := set.New()
	for machineInfo, err := range sdkobjectpatch.SnapshotIter[MachineDeploymentCrdInfo](input.Snapshots.Get("machine_deployments")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'machine_deployments' snapshots: %w", err)
		}

		defaultZones.Add(machineInfo.Zone)
	}

	cloudProviderSecrets, err := sdkobjectpatch.UnmarshalToStruct[map[string]interface{}](input.Snapshots, "cloud_provider_secret")
	if err != nil {
		return fmt.Errorf("failed to unmarshal 'cloud_provider_secret' snapshot: %w", err)
	}
	if len(cloudProviderSecrets) > 0 {
		switch v := cloudProviderSecrets[0]["zones"].(type) {
		case []string:
			defaultZones.Add(v...)
		case []interface{}:
			for _, zoneUntyped := range v {
				if s, ok := zoneUntyped.(string); ok {
					defaultZones.Add(s)
				}
			}
		case string:
			defaultZones.Add(v)
		}
	}

	finalNodeGroups := make([]interface{}, 0)

	for nodeGroup, err := range sdkobjectpatch.SnapshotIter[NodeGroupCrdInfo](input.Snapshots.Get("ngs")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'ngs' snapshots: %w", err)
		}

		ngForValues := nodeGroupForValues(nodeGroup.Spec.DeepCopy())
		ngForValues["name"] = nodeGroup.Name
		ngForValues["engine"] = string(calculateNodeGroupEngine(input, nodeGroup))

		if nodeGroup.Spec.NodeType == ngv1.NodeTypeStatic {
			if staticValue, has := input.Values.GetOk("nodeManager.internal.static"); has {
				if len(staticValue.Map()) > 0 {
					ngForValues["static"] = staticValue.Value()
				}
			}
		}

		if nodeGroup.Spec.NodeType == ngv1.NodeTypeCloudEphemeral {
			zones := nodeGroup.Spec.CloudInstances.Zones
			if zones == nil {
				zones = defaultZones.Slice()
			}

			if ngForValues["cloudInstances"] == nil {
				ngForValues["cloudInstances"] = ngv1.CloudInstances{}
			}
			cloudInstances := ngForValues["cloudInstances"].(ngv1.CloudInstances)
			cloudInstances.Zones = zones
			ngForValues["cloudInstances"] = cloudInstances
		}

		ngBytes, _ := cljson.Marshal(ngForValues)
		finalNodeGroups = append(finalNodeGroups, json.RawMessage(ngBytes))
	}

	if !input.Values.Exists("nodeManager.internal") {
		input.Values.Set("nodeManager.internal", map[string]interface{}{})
	}

	input.Values.Set("nodeManager.internal.nodeGroups", finalNodeGroups)
	return nil
}

func nodeGroupForValues(nodeGroupSpec *ngv1.NodeGroupSpec) map[string]interface{} {
	res := make(map[string]interface{})

	res["nodeType"] = nodeGroupSpec.NodeType
	if !nodeGroupSpec.CRI.IsEmpty() {
		res["cri"] = nodeGroupSpec.CRI
	}
	if !nodeGroupSpec.GPU.IsEmpty() {
		res["gpu"] = nodeGroupSpec.GPU
	}
	if nodeGroupSpec.StaticInstances != nil {
		res["staticInstances"] = *nodeGroupSpec.StaticInstances
	}
	if !nodeGroupSpec.CloudInstances.IsEmpty() {
		res["cloudInstances"] = nodeGroupSpec.CloudInstances
	}
	if !nodeGroupSpec.NodeTemplate.IsEmpty() {
		res["nodeTemplate"] = nodeGroupSpec.NodeTemplate
	}
	if !nodeGroupSpec.Chaos.IsEmpty() {
		res["chaos"] = nodeGroupSpec.Chaos
	}
	if !nodeGroupSpec.OperatingSystem.IsEmpty() {
		res["operatingSystem"] = nodeGroupSpec.OperatingSystem
	}
	if !nodeGroupSpec.Disruptions.IsEmpty() {
		res["disruptions"] = nodeGroupSpec.Disruptions
	}
	if !nodeGroupSpec.Kubelet.IsEmpty() {
		res["kubelet"] = nodeGroupSpec.Kubelet
	}
	if !nodeGroupSpec.Fencing.IsEmpty() {
		res["fencing"] = nodeGroupSpec.Fencing
	}
	if nodeGroupSpec.NodeDrainTimeoutSecond != nil {
		res["nodeDrainTimeoutSecond"] = nodeGroupSpec.NodeDrainTimeoutSecond
	}
	return res
}

var detectInstanceClassKind = func(input *go_hook.HookInput, config *go_hook.HookConfig) (string, string) {
	var fromSecret string
	secretInfoSnapshots := input.Snapshots.Get("cloud_provider_secret")

	if len(secretInfoSnapshots) > 0 {
		var secretInfo map[string]interface{}
		err := secretInfoSnapshots[0].UnmarshalTo(&secretInfo)
		if err == nil {
			if kind, ok := secretInfo["instanceClassKind"].(string); ok {
				fromSecret = kind
			}
		}
	}

	return config.Kubernetes[0].Kind, fromSecret
}

func calculateNodeGroupEngine(input *go_hook.HookInput, nodeGroup NodeGroupCrdInfo) ngv1.NodeGroupEngine {
	if nodeGroup.Engine != "" {
		return nodeGroup.Engine
	}

	defaultEngine := defaultCloudEphemeralNodeGroupEngineForNewNodeGroups(input, nodeGroup.UseMCM)

	switch nodeGroup.Spec.NodeType {
	case ngv1.NodeTypeCloudEphemeral:
		return defaultEngine
	case ngv1.NodeTypeStatic:
		if nodeGroup.Spec.StaticInstances != nil {
			return ngv1.NodeGroupEngineCAPI
		}
		return ngv1.NodeGroupEngineNone
	default:
		return ngv1.NodeGroupEngineNone
	}
}

func defaultCloudEphemeralNodeGroupEngineForNewNodeGroups(input *go_hook.HookInput, useMCM bool) ngv1.NodeGroupEngine {
	hasMCM := valueExistsAndNotEmpty(input, "nodeManager.internal.cloudProvider.machineClassKind")
	hasCAPI := valueExistsAndNotEmpty(input, "nodeManager.internal.cloudProvider.capiClusterKind")

	switch {
	case hasMCM && hasCAPI:
		if useMCM {
			return ngv1.NodeGroupEngineMCM
		}
		return ngv1.NodeGroupEngineCAPI
	case hasMCM:
		return ngv1.NodeGroupEngineMCM
	case hasCAPI:
		return ngv1.NodeGroupEngineCAPI
	default:
		return ngv1.NodeGroupEngineNone
	}
}
