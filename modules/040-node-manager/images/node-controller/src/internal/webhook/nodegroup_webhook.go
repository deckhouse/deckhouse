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
// This is a reimplementation of the bash validation hook:
// - modules/040-node-manager/hooks/node_group
//
// NOTE: This webhook only validates things that CRD OpenAPI schema cannot validate:
// - Cross-field logic (minPerZone vs maxPerZone)
// - Cluster state dependent checks (zones, endpoints, nodes)
// - Immutability on UPDATE
// - Cross-resource validation (ModuleConfig)
//
// The following validations are handled by CRD and NOT duplicated here:
// - Name format (pattern), name length (maxLength)
// - nodeType enum values
// - CloudEphemeral requires cloudInstances (oneOf)
// - Static must not have cloudInstances (oneOf)
// - cloudInstances requires classReference (required)
// - CRI type enum values
package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

// SetupWithManager registers the webhooks with the manager.
func SetupWithManager(mgr ctrl.Manager) error {
	hookServer := mgr.GetWebhookServer()
	decoder := admission.NewDecoder(mgr.GetScheme())

	// Validating webhook
	hookServer.Register("/validate-deckhouse-io-v1-nodegroup", &webhook.Admission{
		Handler: &NodeGroupValidator{
			Client:  mgr.GetClient(),
			decoder: decoder,
		},
	})

	// Conversion webhook with cluster state access
	hookServer.Register("/convert", &NodeGroupConversionHandler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
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

	clusterConfig := w.loadClusterConfig(ctx)
	providerConfig := w.loadProviderClusterConfig(ctx)
	var warnings []string

	if req.Operation == "CREATE" && clusterConfig.ClusterType == "Cloud" {
		// Dynamic node name is <clusterPrefix>-<nodeGroupName>-<hashes>
		// Label value must be <= 63 characters, hashes = 21 chars
		maxAllowed := 63 - clusterConfig.ClusterPrefixLen - 1 - 21
		if len(ng.Name) > maxAllowed {
			return admission.Denied(fmt.Sprintf(
				"it is forbidden for this cluster to set (cluster prefix + node group name) longer than 42 symbols; "+
					"max nodegroup name length for this cluster is %d", maxAllowed))
		}
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

	if ng.Spec.Kubelet != nil && ng.Spec.Kubelet.MaxPods != nil {
		maxPods := *ng.Spec.Kubelet.MaxPods
		prefix := clusterConfig.PodSubnetNodeCIDRPrefix
		if prefix == 0 {
			prefix = 24
		}
		availableIPs := (1 << (32 - prefix)) - 3
		// Every pod can use two IPs (one in terminating + one in starting phase)
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

	if req.Operation == "UPDATE" && ng.Name == "master" && oldNG != nil {
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

	if req.Operation == "UPDATE" && oldNG != nil {
		if ng.Spec.NodeType == v1.NodeTypeStatic || ng.Spec.NodeType == v1.NodeTypeCloudStatic {
			if err := validateLabelSelectorImmutability(oldNG, ng); err != nil {
				return admission.Denied(err.Error())
			}
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
		if ng.Spec.Kubelet.TopologyManager.Policy != "" {
			if ng.Spec.Kubelet.ResourceReservation == nil ||
				ng.Spec.Kubelet.ResourceReservation.Mode == "Off" {
				return admission.Denied(
					".spec.kubelet.resourceReservation must be enabled for .spec.kubelet.topologyManager to work")
			}

			if ng.Spec.Kubelet.ResourceReservation.Mode == "Static" {
				if ng.Spec.Kubelet.ResourceReservation.Static == nil ||
					ng.Spec.Kubelet.ResourceReservation.Static.CPU == nil {
					return admission.Denied(
						"for .spec.kubelet.topologyManager and .spec.kubelet.resourceReservation.mode == \"Static\", " +
							".spec.kubelet.resourceReservation.static.cpu must be specified")
				}
			}
		}
	}

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

	if req.Operation == "UPDATE" {
		if ng.Spec.Kubelet != nil && ng.Spec.Kubelet.MemorySwap != nil {
			if ng.Spec.Kubelet.MemorySwap.Behavior == "LimitedSwap" {
				unsupportedNodes := w.getNodesWithoutContainerdV2Support(ctx, ng.Name)
				if len(unsupportedNodes) > 0 {
					return admission.Denied(fmt.Sprintf(
						"memorySwap requires cgroup v2, but NodeGroup %q contains nodes where cgroup v2 is not supported",
						ng.Name))
				}
			}
		}
	}

	if ng.Spec.Disruptions != nil {
		if err := validateDisruptionWindows(ng.Spec.Disruptions); err != nil {
			return admission.Denied(err.Error())
		}
	}

	// Return with warnings if any
	if len(warnings) > 0 {
		return admission.Allowed("").WithWarnings(warnings...)
	}
	return admission.Allowed("")
}

// validateLabelSelectorImmutability checks that staticInstances.labelSelector
// cannot be modified or removed once set (but can be added).
func validateLabelSelectorImmutability(oldNG, newNG *v1.NodeGroup) error {
	// Check if old staticInstances exists
	if oldNG.Spec.StaticInstances == nil {
		return nil // Can add new staticInstances
	}

	// Check if old labelSelector exists
	if oldNG.Spec.StaticInstances.LabelSelector == nil {
		return nil // Can add new labelSelector
	}

	oldLS := oldNG.Spec.StaticInstances.LabelSelector

	// Check if old labelSelector is empty
	oldIsEmpty := (len(oldLS.MatchLabels) == 0) && (len(oldLS.MatchExpressions) == 0)
	if oldIsEmpty {
		return nil // Empty labelSelector can be changed
	}

	// Old labelSelector is not empty - check if it was changed

	// Check if new staticInstances or labelSelector was removed
	if newNG.Spec.StaticInstances == nil || newNG.Spec.StaticInstances.LabelSelector == nil {
		return fmt.Errorf(".spec.staticInstances.labelSelector can be added but cannot be modified or removed once set. To change it, create a new NodeGroup")
	}

	newLS := newNG.Spec.StaticInstances.LabelSelector

	// Check if new labelSelector is empty
	newIsEmpty := (len(newLS.MatchLabels) == 0) && (len(newLS.MatchExpressions) == 0)
	if newIsEmpty {
		return fmt.Errorf(".spec.staticInstances.labelSelector can be added but cannot be modified or removed once set. To change it, create a new NodeGroup")
	}

	// Compare old and new labelSelector
	if !reflect.DeepEqual(oldLS, newLS) {
		return fmt.Errorf(".spec.staticInstances.labelSelector can be added but cannot be modified once set. To change it, create a new NodeGroup")
	}

	return nil
}

// validateDisruptionWindows validates the format of disruption windows.
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

	if match := regexp.MustCompile(`defaultCRI:\s+(\S+)`).FindSubmatch(configYAML); match != nil {
		config.DefaultCRI = string(match[1])
	}

	if match := regexp.MustCompile(`clusterType:\s+(\S+)`).FindSubmatch(configYAML); match != nil {
		config.ClusterType = string(match[1])
	}

	if match := regexp.MustCompile(`prefix:\s+(\S+)`).FindSubmatch(configYAML); match != nil {
		config.ClusterPrefixLen = len(string(match[1]))
	}

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

// loadCustomTolerationKeys reads customTolerationKeys from ModuleConfig "global"
func (w *NodeGroupValidator) loadCustomTolerationKeys(ctx context.Context) []string {
	// ModuleConfig is deckhouse.io/v1alpha1
	mc := &unstructured.Unstructured{}
	mc.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "deckhouse.io",
		Version: "v1alpha1",
		Kind:    "ModuleConfig",
	})

	err := w.Client.Get(ctx, types.NamespacedName{Name: "global"}, mc)
	if err != nil {
		webhookLog.V(1).Info("failed to load ModuleConfig global", "error", err)
		return nil
	}

	// Path: .spec.settings.modules.placement.customTolerationKeys
	settings, found, _ := unstructured.NestedMap(mc.Object, "spec", "settings")
	if !found {
		return nil
	}

	modules, found, _ := unstructured.NestedMap(settings, "modules")
	if !found {
		return nil
	}

	placement, found, _ := unstructured.NestedMap(modules, "placement")
	if !found {
		return nil
	}

	keys, found, _ := unstructured.NestedStringSlice(placement, "customTolerationKeys")
	if !found {
		return nil
	}

	return keys
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
