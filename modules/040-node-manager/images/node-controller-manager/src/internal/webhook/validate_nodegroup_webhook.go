/*
Copyright 2025 Flant JSC

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

// Package webhook contains the NodeGroup webhook implementation.
// This is a complete reimplementation of two bash/python hooks:
// - modules/040-node-manager/hooks/node_group (validation)
// - modules/040-node-manager/hooks/node_group.py (conversion)
package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

var webhookLog = logf.Log.WithName("nodegroup-webhook")

// NodeGroupValidator handles validation for NodeGroup resources.
// It has access to cluster state via Client.
type NodeGroupValidator struct {
	Client  client.Client
	decoder admission.Decoder
}

// NodeGroupDefaulter handles defaulting for NodeGroup resources.
type NodeGroupDefaulter struct {
	decoder admission.Decoder
}

// SetupWithManager registers the webhooks with the manager.
func SetupWithManager(mgr ctrl.Manager) error {
	hookServer := mgr.GetWebhookServer()

	// Create decoder
	decoder := admission.NewDecoder(mgr.GetScheme())

	// Register validating webhook
	hookServer.Register("/validate-deckhouse-io-v1-nodegroup", &webhook.Admission{
		Handler: &NodeGroupValidator{
			Client:  mgr.GetClient(),
			decoder: decoder,
		},
	})

	// Register mutating webhook for defaults
	hookServer.Register("/mutate-deckhouse-io-v1-nodegroup", &webhook.Admission{
		Handler: &NodeGroupDefaulter{
			decoder: decoder,
		},
	})

	return nil
}

// Handle implements admission.Handler for validation.
func (w *NodeGroupValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	webhookLog.Info("validating nodegroup", "name", req.Name, "operation", req.Operation)

	ng := &v1.NodeGroup{}
	if err := w.decoder.Decode(req, ng); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	var oldNG *v1.NodeGroup
	if req.Operation == "UPDATE" {
		oldNG = &v1.NodeGroup{}
		if err := w.decoder.DecodeRaw(req.OldObject, oldNG); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
	}

	// Load cluster configuration for advanced validations
	clusterConfig := w.loadClusterConfig(ctx)
	providerConfig := w.loadProviderClusterConfig(ctx)

	var warnings []string

	if err := validateNodeGroupName(ng.Name); err != nil {
		return admission.Denied(err.Error())
	}

	if req.Operation == "CREATE" && clusterConfig.ClusterType == "Cloud" {
		maxAllowed := 63 - clusterConfig.ClusterPrefixLen - 1 - 21
		if len(ng.Name) > maxAllowed {
			return admission.Denied(fmt.Sprintf(
				"it is forbidden for this cluster to set (cluster prefix + node group name) longer than 42 symbols; "+
					"max nodegroup name length for this cluster is %d", maxAllowed))
		}
	}

	validNodeTypes := map[v1.NodeType]bool{
		v1.NodeTypeCloudEphemeral: true,
		v1.NodeTypeCloudPermanent: true,
		v1.NodeTypeCloudStatic:    true,
		v1.NodeTypeStatic:         true,
	}
	if !validNodeTypes[ng.Spec.NodeType] {
		return admission.Denied(fmt.Sprintf(
			"invalid nodeType %q, must be one of: CloudEphemeral, CloudPermanent, CloudStatic, Static",
			ng.Spec.NodeType))
	}

	if req.Operation == "UPDATE" && oldNG != nil {
		if oldNG.Spec.NodeType != ng.Spec.NodeType {
			return admission.Denied(".spec.nodeType field is immutable")
		}
	}

	if ng.Spec.CloudInstances != nil {
		if ng.Spec.CloudInstances.MaxPerZone < ng.Spec.CloudInstances.MinPerZone {
			return admission.Denied("it is forbidden to set maxPerZone lower than minPerZone for NodeGroup")
		}
	}

	// Validation 6: CloudEphemeral requires cloudInstances
	if ng.Spec.NodeType == v1.NodeTypeCloudEphemeral && ng.Spec.CloudInstances == nil {
		return admission.Denied("cloudInstances is required for nodeType CloudEphemeral")
	}

	if ng.Spec.NodeType == v1.NodeTypeStatic && ng.Spec.CloudInstances != nil {
		return admission.Denied("cloudInstances must not be set for nodeType Static")
	}

	if ng.Spec.NodeType == v1.NodeTypeCloudEphemeral && ng.Spec.CloudInstances != nil {
		if ng.Spec.CloudInstances.ClassReference.Kind == "" {
			return admission.Denied("cloudInstances.classReference.kind is required for CloudEphemeral")
		}
		if ng.Spec.CloudInstances.ClassReference.Name == "" {
			return admission.Denied("cloudInstances.classReference.name is required for CloudEphemeral")
		}
	}

	if ng.Spec.Kubelet != nil && ng.Spec.Kubelet.MaxPods != nil {
		maxPods := *ng.Spec.Kubelet.MaxPods
		prefix := clusterConfig.PodSubnetNodeCIDRPrefix
		if prefix == 0 {
			prefix = 24
		}
		availableIPs := (1 << (32 - prefix)) - 3
		if 2*int(maxPods) > availableIPs {
			warnings = append(warnings, fmt.Sprintf(
				".spec.kubelet.maxPods (%d) is too high: may lead to IP exhaustion", maxPods))
		}
	}

	if ng.Spec.CloudInstances != nil && len(providerConfig.Zones) > 0 && len(ng.Spec.CloudInstances.Zones) > 0 {
		allowedZones := make(map[string]bool)
		for _, z := range providerConfig.Zones {
			allowedZones[z] = true
		}
		for _, zone := range ng.Spec.CloudInstances.Zones {
			if !allowedZones[zone] {
				return admission.Denied(fmt.Sprintf("unknown zone %q", zone))
			}
		}
	}

	if ng.Spec.CRI != nil && ng.Spec.CRI.Type == v1.CRITypeDocker {
		return admission.Denied("it is forbidden to set cri type to Docker")
	}

	if ng.Spec.CRI != nil {
		if ng.Spec.CRI.Containerd != nil && ng.Spec.CRI.Type != "" && ng.Spec.CRI.Type != v1.CRITypeContainerd {
			return admission.Denied("it is forbidden to set .spec.cri.containerd without .spec.cri.type=\"Containerd\"")
		}
		if ng.Spec.CRI.ContainerdV2 != nil && ng.Spec.CRI.Type != "" && ng.Spec.CRI.Type != v1.CRITypeContainerdV2 {
			return admission.Denied("it is forbidden to set .spec.cri.containerdV2 without .spec.cri.type=\"ContainerdV2\"")
		}
		if ng.Spec.CRI.Docker != nil && ng.Spec.CRI.Type != "" && ng.Spec.CRI.Type != v1.CRITypeDocker {
			return admission.Denied("it is forbidden to set .spec.cri.docker without .spec.cri.type=\"Docker\"")
		}
	}

		oldCRIType := getCRIType(oldNG, clusterConfig.DefaultCRI)
		newCRIType := getCRIType(ng, clusterConfig.DefaultCRI)
		if oldCRIType != newCRIType {
			endpointsCount := w.getKubernetesEndpointsCount(ctx)
			if endpointsCount < 3 {
				warnings = append(warnings,
					"it is disruptive to change cri.type in master node group for cluster with apiserver endpoints < 3")
			}
		}
	}

	if ng.Spec.NodeTemplate != nil && len(ng.Spec.NodeTemplate.Taints) > 0 {
		customKeys := w.loadCustomTolerationKeys(ctx)
		standardTaints := map[string]bool{
			"dedicated":                             true,
			"dedicated.deckhouse.io":                true,
			"node-role.kubernetes.io/control-plane": true,
			"node-role.kubernetes.io/master":        true,
			"node.deckhouse.io/etcd-arbiter":        true,
		}
		customKeysSet := make(map[string]bool)
		for _, k := range customKeys {
			customKeysSet[k] = true
		}

		var missingTaints []string
		for _, taint := range ng.Spec.NodeTemplate.Taints {
			if standardTaints[taint.Key] {
				continue
			}
			if !customKeysSet[taint.Key] {
				missingTaints = append(missingTaints, taint.Key)
			}
		}
		if len(missingTaints) > 0 {
			return admission.Denied(fmt.Sprintf(
				"it is forbidden to create a NodeGroup resource with taints not specified in ModuleConfig \"global\" "+
					"in the array .spec.settings.modules.placement.customTolerationKeys, add: %s to customTolerationKeys",
				strings.Join(missingTaints, ", ")))
		}
	}

	if ng.Spec.Disruptions != nil && ng.Spec.Disruptions.ApprovalMode == v1.DisruptionApprovalModeRollingUpdate {
		if ng.Spec.NodeType != v1.NodeTypeCloudEphemeral {
			return admission.Denied(
				"it is forbidden to set .spec.disruptions.approvalMode to \"RollingUpdate\" when spec.nodeType is not \"CloudEphemeral\"")
		}
	}

	if ng.Spec.NodeTemplate != nil && len(ng.Spec.NodeTemplate.Taints) > 0 {
		seen := make(map[string]bool)
		for _, taint := range ng.Spec.NodeTemplate.Taints {
			key := fmt.Sprintf("%s:%s", taint.Key, taint.Effect)
			if seen[key] {
				return admission.Denied(".spec.nodeTemplate.taints must contain only one taint with the same key and effect")
			}
			seen[key] = true
		}
	}

	if ng.Spec.Kubelet != nil && ng.Spec.Kubelet.TopologyManager != nil {
		if ng.Spec.Kubelet.TopologyManager.Enabled != nil && *ng.Spec.Kubelet.TopologyManager.Enabled {
			if ng.Spec.Kubelet.ResourceReservation == nil ||
				ng.Spec.Kubelet.ResourceReservation.Mode == "Off" {
				return admission.Denied(
					".spec.kubelet.resourceReservation must be enabled for .spec.kubelet.topologyManager.enabled to work")
			}

			if ng.Spec.Kubelet.ResourceReservation.Mode == "Static" {
				if ng.Spec.Kubelet.ResourceReservation.Static == nil ||
					ng.Spec.Kubelet.ResourceReservation.Static.CPU == nil {
					return admission.Denied(
						"for .spec.kubelet.topologyManager.enabled and .spec.kubelet.resourceReservation.mode == \"Static\", " +
							".spec.kubelet.resourceReservation.static.cpu must be specified")
				}
			}
		}
	}

	// Validation 18: CRI change blocked by nodes with custom containerd config
	if req.Operation == "UPDATE" && oldNG != nil {
		oldCRIType := getCRIType(oldNG, "")
		newCRIType := getCRIType(ng, "")
		if oldCRIType != newCRIType {
			customNodes := w.getNodesWithCustomContainerd(ctx, ng.Name)
			if len(customNodes) > 0 {
				return admission.Denied(fmt.Sprintf(
					"CRI cannot be changed because some nodes are using custom configuration: %s",
					strings.Join(customNodes, " ")))
			}
		}
	}

	// Validation 19: ContainerdV2 blocked by unsupported nodes
	if req.Operation == "UPDATE" {
		if ng.Spec.CRI != nil && ng.Spec.CRI.Type == v1.CRITypeContainerdV2 {
			unsupportedNodes := w.getNodesWithoutContainerdV2Support(ctx, ng.Name)
			if len(unsupportedNodes) > 0 {
				return admission.Denied(fmt.Sprintf(
					"It is forbidden for NodeGroup %q to use CRI ContainerdV2 because it contains nodes that do not support ContainerdV2. "+
						"You can list them with: kubectl get node -l node.deckhouse.io/containerd-v2-unsupported,node.deckhouse.io/group=%s",
					ng.Name, ng.Name))
			}
		}
	}

	// Validation 20: memorySwap requires cgroup v2
	if req.Operation == "UPDATE" {
		if ng.Spec.Kubelet != nil && ng.Spec.Kubelet.MemorySwap != nil {
			if ng.Spec.Kubelet.MemorySwap.SwapBehavior == "LimitedSwap" {
				unsupportedNodes := w.getNodesWithoutContainerdV2Support(ctx, ng.Name)
				if len(unsupportedNodes) > 0 {
					return admission.Denied(fmt.Sprintf(
						"memorySwap requires cgroup v2, but NodeGroup %q contains nodes where cgroup v2 is not supported",
						ng.Name))
				}
			}
		}
	}

	// Validation 21: Disruption windows format
	if ng.Spec.Disruptions != nil {
		if err := validateDisruptionWindows(ng.Spec.Disruptions); err != nil {
			return admission.Denied(err.Error())
		}
	}

	// All validations passed
	if len(warnings) > 0 {
		return admission.Allowed("").WithWarnings(warnings...)
	}
	return admission.Allowed("")
}

// Handle implements admission.Handler for defaulting.
func (d *NodeGroupDefaulter) Handle(ctx context.Context, req admission.Request) admission.Response {
	webhookLog.Info("defaulting nodegroup", "name", req.Name)

	ng := &v1.NodeGroup{}
	if err := d.decoder.Decode(req, ng); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Set defaults
	d.setDefaults(ng)

	// Marshal the mutated object
	marshaledNG, err := json.Marshal(ng)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledNG)
}

func (d *NodeGroupDefaulter) setDefaults(ng *v1.NodeGroup) {
	// Default disruption approval mode
	if ng.Spec.Disruptions != nil && ng.Spec.Disruptions.ApprovalMode == "" {
		ng.Spec.Disruptions.ApprovalMode = v1.DisruptionApprovalModeManual
	}

	// Default chaos mode
	if ng.Spec.Chaos != nil && ng.Spec.Chaos.Mode == "" {
		ng.Spec.Chaos.Mode = v1.ChaosModeDisabled
	}

	// Default CRI type
	if ng.Spec.CRI != nil && ng.Spec.CRI.Type == "" {
		ng.Spec.CRI.Type = v1.CRITypeContainerd
	}
}

// ClusterConfig holds relevant fields from d8-cluster-configuration Secret
type ClusterConfig struct {
	DefaultCRI              string
	ClusterPrefixLen        int
	ClusterType             string
	PodSubnetNodeCIDRPrefix int
}

// ProviderClusterConfig holds relevant fields from d8-provider-cluster-configuration Secret
type ProviderClusterConfig struct {
	Zones []string
}

var nodeGroupNameRegex = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

func validateNodeGroupName(name string) error {
	if len(name) > 42 {
		return fmt.Errorf("name must be no more than 42 characters, got %d", len(name))
	}
	if !nodeGroupNameRegex.MatchString(name) {
		return fmt.Errorf("name must match pattern ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$")
	}
	return nil
}

func validateDisruptionWindows(d *v1.DisruptionsSpec) error {
	timeRegex := regexp.MustCompile(`^(?:\d|[01]\d|2[0-3]):[0-5]\d$`)
	validDays := map[string]bool{
		"Mon": true, "Tue": true, "Wed": true, "Thu": true,
		"Fri": true, "Sat": true, "Sun": true,
	}

	validateWindows := func(windows []v1.DisruptionWindow, path string) error {
		for i, w := range windows {
			if !timeRegex.MatchString(w.From) {
				return fmt.Errorf("%s[%d].from: invalid time format %q, expected HH:MM", path, i, w.From)
			}
			if !timeRegex.MatchString(w.To) {
				return fmt.Errorf("%s[%d].to: invalid time format %q, expected HH:MM", path, i, w.To)
			}
			for _, day := range w.Days {
				if !validDays[day] {
					return fmt.Errorf("%s[%d].days: invalid day %q, expected one of Mon,Tue,Wed,Thu,Fri,Sat,Sun", path, i, day)
				}
			}
		}
		return nil
	}

	if d.Automatic != nil && d.Automatic.Windows != nil {
		if err := validateWindows(d.Automatic.Windows, ".spec.disruptions.automatic.windows"); err != nil {
			return err
		}
	}

	if d.RollingUpdate != nil && d.RollingUpdate.Windows != nil {
		if err := validateWindows(d.RollingUpdate.Windows, ".spec.disruptions.rollingUpdate.windows"); err != nil {
			return err
		}
	}

	return nil
}

func getCRIType(ng *v1.NodeGroup, defaultCRI string) string {
	if ng.Spec.CRI != nil && ng.Spec.CRI.Type != "" {
		return string(ng.Spec.CRI.Type)
	}
	if defaultCRI != "" {
		return defaultCRI
	}
	return "Containerd"
}

func (w *NodeGroupValidator) loadClusterConfig(ctx context.Context) *ClusterConfig {
	config := &ClusterConfig{PodSubnetNodeCIDRPrefix: 24}

	secret := &corev1.Secret{}
	err := w.Client.Get(ctx, types.NamespacedName{
		Namespace: "kube-system",
		Name:      "d8-cluster-configuration",
	}, secret)
	if err != nil {
		webhookLog.V(1).Info("failed to load cluster config", "error", err)
		return config
	}

	configYAML, ok := secret.Data["cluster-configuration.yaml"]
	if !ok {
		return config
	}

	// Parse defaultCRI
	if match := regexp.MustCompile(`defaultCRI:\s+(\S+)`).FindSubmatch(configYAML); match != nil {
		config.DefaultCRI = string(match[1])
	}

	// Parse clusterType
	if match := regexp.MustCompile(`clusterType:\s+(\S+)`).FindSubmatch(configYAML); match != nil {
		config.ClusterType = string(match[1])
	}

	// Parse prefix length
	if match := regexp.MustCompile(`prefix:\s+(\S+)`).FindSubmatch(configYAML); match != nil {
		config.ClusterPrefixLen = len(string(match[1]))
	}

	// Parse podSubnetNodeCIDRPrefix
	if match := regexp.MustCompile(`podSubnetNodeCIDRPrefix:\s*"?(\d+)"?`).FindSubmatch(configYAML); match != nil {
		fmt.Sscanf(string(match[1]), "%d", &config.PodSubnetNodeCIDRPrefix)
	}

	return config
}

func (w *NodeGroupValidator) loadProviderClusterConfig(ctx context.Context) *ProviderClusterConfig {
	config := &ProviderClusterConfig{}

	secret := &corev1.Secret{}
	err := w.Client.Get(ctx, types.NamespacedName{
		Namespace: "kube-system",
		Name:      "d8-provider-cluster-configuration",
	}, secret)
	if err != nil {
		webhookLog.V(1).Info("failed to load provider cluster config", "error", err)
		return config
	}

	discoveryData, ok := secret.Data["cloud-provider-discovery-data.json"]
	if !ok {
		return config
	}

	var data struct {
		Zones []string `json:"zones"`
	}
	if err := json.Unmarshal(discoveryData, &data); err != nil {
		webhookLog.V(1).Info("failed to parse discovery data", "error", err)
		return config
	}

	config.Zones = data.Zones
	return config
}

func (w *NodeGroupValidator) loadCustomTolerationKeys(ctx context.Context) []string {
	// TODO: Implement ModuleConfig reading when the CRD is available
	return nil
}

func (w *NodeGroupValidator) getKubernetesEndpointsCount(ctx context.Context) int {
	endpoints := &corev1.Endpoints{}
	err := w.Client.Get(ctx, types.NamespacedName{
		Namespace: "default",
		Name:      "kubernetes",
	}, endpoints)
	if err != nil {
		webhookLog.V(1).Info("failed to get kubernetes endpoints", "error", err)
		return 0
	}

	count := 0
	for _, subset := range endpoints.Subsets {
		count += len(subset.Addresses)
	}
	return count
}

func (w *NodeGroupValidator) getNodesWithCustomContainerd(ctx context.Context, nodeGroupName string) []string {
	nodeList := &corev1.NodeList{}
	err := w.Client.List(ctx, nodeList, client.MatchingLabels{
		"node.deckhouse.io/containerd-config": "custom",
		"node.deckhouse.io/group":             nodeGroupName,
	})
	if err != nil {
		webhookLog.V(1).Info("failed to list nodes with custom containerd", "error", err)
		return nil
	}

	var names []string
	for _, node := range nodeList.Items {
		names = append(names, node.Name)
	}
	return names
}

func (w *NodeGroupValidator) getNodesWithoutContainerdV2Support(ctx context.Context, nodeGroupName string) []string {
	nodeList := &corev1.NodeList{}
	err := w.Client.List(ctx, nodeList, client.MatchingLabels{
		"node.deckhouse.io/containerd-v2-unsupported": "",
		"node.deckhouse.io/group":                     nodeGroupName,
	})
	if err != nil {
		webhookLog.V(1).Info("failed to list nodes without containerd v2 support", "error", err)
		return nil
	}

	var names []string
	for _, node := range nodeList.Items {
		names = append(names, node.Name)
	}
	return names
}
