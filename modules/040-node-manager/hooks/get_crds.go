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
	"log/slog"
	"strings"

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
	Name            string
	Spec            ngv1.NodeGroupSpec
	Engine          ngv1.NodeGroupEngine
	UseMCM          bool
	ManualRolloutID string
}

const (
	useMCMAnnotation          = "node.deckhouse.io/use-mcm"
	manualRolloutIDAnnotation = "manual-rollout-id"
)

// CloudFillerFunc fills provider-specific defaults into the instanceClass spec map
// (used only to keep the bootstrap-secret name checksum byte-parity with helm).
type CloudFillerFunc func(cloudVariables map[string]interface{}, instanceClass map[string]interface{}) error

var fillCloudSpecificDefaults = map[string][]CloudFillerFunc{
	"vsphere": {fillVsphereMainNewtork},
}

// InstanceClassCrdInfo is a name+spec of a cloud InstanceClass CRD (kind is dynamic).
type InstanceClassCrdInfo struct {
	Name string
	Spec interface{}
}

func applyInstanceClassCrdFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return InstanceClassCrdInfo{
		Name: obj.GetName(),
		Spec: obj.Object["spec"],
	}, nil
}

// applyNodeGroupCrdFilter returns name, spec, status.engine and use-mcm annotation from the NodeGroup.
func applyNodeGroupCrdFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var nodeGroup ngv1.NodeGroup
	err := sdk.FromUnstructured(obj, &nodeGroup)
	if err != nil {
		return nil, err
	}

	return NodeGroupCrdInfo{
		Name:            nodeGroup.GetName(),
		Spec:            nodeGroup.Spec,
		Engine:          nodeGroup.Status.Engine,
		UseMCM:          nodeGroup.GetAnnotations()[useMCMAnnotation] != "",
		ManualRolloutID: nodeGroup.GetAnnotations()[manualRolloutIDAnnotation],
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
		// ics MUST stay at index 0: detectInstanceClassKind reads config.Kubernetes[0].Kind.
		{
			Name:       "ics",
			ApiVersion: "",
			Kind:       "",
			FilterFunc: applyInstanceClassCrdFilter,
		},
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
// updateEpoch, serialized labels/taints and the node_group_info metric are owned by
// node-controller now.
func getCRDsHandler(_ context.Context, input *go_hook.HookInput) error {
	// Dynamically bind the 'ics' snapshot to the InstanceClass kind advertised by the
	// cloud-provider secret. On a kind change we adjust the binding and re-run.
	kindInUse, kindFromSecret := detectInstanceClassKind(input, getCRDsHookConfig)
	if kindInUse != kindFromSecret {
		if kindFromSecret == "" {
			input.Logger.Info("InstanceClassKind has changed: disable binding 'ics'")
			*input.BindingActions = append(*input.BindingActions, go_hook.BindingAction{
				Name: "ics", Action: "Disable", Kind: "", ApiVersion: "",
			})
		} else {
			input.Logger.Info("InstanceClassKind has changed: update kind for binding 'ics'",
				slog.String("from", kindInUse), slog.String("to", kindFromSecret))
			*input.BindingActions = append(*input.BindingActions, go_hook.BindingAction{
				Name: "ics", Action: "UpdateKind", Kind: kindFromSecret, ApiVersion: "",
			})
		}
		getCRDsHookConfig.Kubernetes[0].Kind = kindFromSecret
		return nil
	}

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

	// instanceClass specs, keyed by name. Only used to keep the CAPI bootstrap-secret
	// name checksum byte-parity with helm (spec is passed through to values verbatim,
	// aside from provider-specific defaults filled by applyCloudSpecificDefaults).
	instanceClasses := make(map[string]interface{})
	for ic, err := range sdkobjectpatch.SnapshotIter[InstanceClassCrdInfo](input.Snapshots.Get("ics")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'ics' snapshots: %w", err)
		}
		instanceClasses[ic.Name] = ic.Spec
	}

	finalNodeGroups := make([]interface{}, 0)

	for nodeGroup, err := range sdkobjectpatch.SnapshotIter[NodeGroupCrdInfo](input.Snapshots.Get("ngs")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'ngs' snapshots: %w", err)
		}

		ngForValues := nodeGroupForValues(nodeGroup.Spec.DeepCopy())
		ngForValues["name"] = nodeGroup.Name
		ngForValues["engine"] = string(calculateNodeGroupEngine(input, nodeGroup))
		ngForValues["manualRolloutID"] = nodeGroup.ManualRolloutID

		// Overlay the raw instanceClass spec so helm can recompute the CAPI bootstrap
		// secret name via capi/<type>/instance-class.checksum (byte-parity with main).
		if nodeGroup.Spec.NodeType == ngv1.NodeTypeCloudEphemeral && kindInUse != "" {
			nodeGroupInstanceClassName := nodeGroup.Spec.CloudInstances.ClassReference.Name
			if instanceClassSpec, ok := instanceClasses[nodeGroupInstanceClassName]; ok {
				providerName := strings.ToLower(input.Values.Get("nodeManager.internal.cloudProvider.type").String())
				updatedSpecMap, err := applyCloudSpecificDefaults(input, providerName, instanceClassSpec)
				if err != nil {
					return fmt.Errorf("failed to fill cloud specific defaults for %s: %w", providerName, err)
				}
				ngForValues["instanceClass"] = updatedSpecMap
			}
		}

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
	if nodeGroupSpec.OSType != "" {
		res["osType"] = nodeGroupSpec.OSType
	}
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

func fillVsphereMainNewtork(cloudVariables map[string]interface{}, instanceClass map[string]interface{}) error {
	if _, ok := instanceClass["mainNetwork"]; ok {
		return nil
	}
	instancesRaw, ok := cloudVariables["instances"]
	if !ok {
		return nil
	}
	instancesMap, ok := instancesRaw.(map[string]interface{})
	if !ok {
		return fmt.Errorf("cloudVariables.instances: expected map[string]interface{}, got %T", instancesRaw)
	}
	val, ok := instancesMap["mainNetwork"]
	if !ok {
		return nil
	}
	mn, ok := val.(string)
	if !ok {
		return fmt.Errorf("instances.mainNetwork: expected string, got %T", val)
	}
	instanceClass["mainNetwork"] = mn
	return nil
}

func applyCloudSpecificDefaults(input *go_hook.HookInput, providerName string, instanceClassSpec interface{}) (interface{}, error) {
	specMap, ok := instanceClassSpec.(map[string]interface{})
	if !ok {
		return instanceClassSpec, nil
	}
	raw, ok := input.Values.GetOk("nodeManager.internal.cloudProvider." + providerName)
	if !ok || !raw.IsObject() {
		return specMap, nil
	}
	cloudVariables, ok := raw.Value().(map[string]interface{})
	if !ok {
		return specMap, nil
	}
	for _, fillFn := range fillCloudSpecificDefaults[providerName] {
		if err := fillFn(cloudVariables, specMap); err != nil {
			return nil, fmt.Errorf("fill %s defaults: %w", providerName, err)
		}
	}
	return specMap, nil
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
