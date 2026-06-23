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

package capi

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	sigsyaml "sigs.k8s.io/yaml"

	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/register"
)

const (
	engineCAPI = "CAPI"
	engineMCM  = "MCM"
)

func init() {
	register.RegisterController("capi-machine-deployment", &deckhousev1.NodeGroup{}, &MachineDeploymentReconciler{})
}

type MachineDeploymentReconciler struct {
	BaseWithReader
}

func (r *MachineDeploymentReconciler) SetupWatches(w register.Watcher) {
	mcmMD := &unstructured.Unstructured{}
	mcmMD.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "machine.sapcloud.io", Version: "v1alpha1", Kind: "MachineDeployment",
	})
	w.Watches(mcmMD, handler.EnqueueRequestsFromMapFunc(mdToNodeGroup))
	w.Watches(&capiv1beta2.MachineDeployment{}, handler.EnqueueRequestsFromMapFunc(mdToNodeGroup))
}

func mdToNodeGroup(_ context.Context, obj client.Object) []reconcile.Request {
	ng, ok := obj.GetLabels()["node-group"]
	if !ok || ng == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: ng}}}
}

func (r *MachineDeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	ng := &deckhousev1.NodeGroup{}
	if err := r.Client.Get(ctx, req.NamespacedName, ng); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get NodeGroup: %w", err)
	}

	switch ng.Spec.NodeType {
	case deckhousev1.NodeTypeCloudEphemeral:
		switch ng.Status.Engine {
		case engineCAPI:
			if err := r.reconcileCloudMDs(ctx, ng); err != nil {
				return ctrl.Result{}, err
			}
		case engineMCM:
			minReplicas, maxReplicas := getMinMax(ng)
			if err := r.reconcileMCMReplicas(ctx, logger, ng.Name, minReplicas, maxReplicas); err != nil {
				return ctrl.Result{}, err
			}
		default:
			logger.V(1).Info("skipping: engine not set or unsupported", "engine", ng.Status.Engine)
		}
	case deckhousev1.NodeTypeStatic, deckhousev1.NodeTypeCloudStatic:
		if ng.Spec.StaticInstances != nil {
			if err := r.reconcileStaticMD(ctx, ng); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *MachineDeploymentReconciler) reconcileCloudMDs(ctx context.Context, ng *deckhousev1.NodeGroup) error {
	logger := log.FromContext(ctx)

	if ng.Spec.CloudInstances == nil {
		logger.V(1).Info("skipping: no cloudInstances")
		return nil
	}

	cloudConfig, err := r.readCloudProviderConfig(ctx)
	if err != nil {
		return err
	}
	if cloudConfig.capiClusterName == "" {
		logger.V(1).Info("skipping: capiClusterName is empty")
		return nil
	}

	zones := ng.Spec.CloudInstances.Zones
	if len(zones) == 0 {
		zones = cloudConfig.zones
	}
	if len(zones) == 0 {
		logger.V(1).Info("skipping: no zones in NodeGroup or cloud provider secret")
		return nil
	}

	instanceClassChecksum, err := r.readInstanceClassChecksum(ctx, cloudConfig, ng.Name)
	if err != nil {
		return err
	}
	if instanceClassChecksum == "" {
		logger.V(1).Info("skipping: infrastructure template not found yet, waiting for helm")
		return nil
	}

	clusterUUID, err := r.readClusterUUID(ctx)
	if err != nil {
		return err
	}

	instancePrefix, err := r.readInstancePrefix(ctx)
	if err != nil {
		return err
	}

	minReplicas := ng.Spec.CloudInstances.MinPerZone
	maxReplicas := ng.Spec.CloudInstances.MaxPerZone
	maxSurge := intOrDefault(ng.Spec.CloudInstances.MaxSurgePerZone, 1)
	maxUnavailable := intOrDefault(ng.Spec.CloudInstances.MaxUnavailablePerZone, 0)

	drainTimeout := 600
	if ng.Spec.NodeDrainTimeoutSecond != nil {
		drainTimeout = *ng.Spec.NodeDrainTimeoutSecond
	}

	infraAPIGroup := cloudConfig.capiMachineTemplateAPIVersion
	if idx := strings.LastIndex(infraAPIGroup, "/"); idx >= 0 {
		infraAPIGroup = infraAPIGroup[:idx]
	}

	for _, zone := range zones {
		mdHash := sha256Hash(clusterUUID + zone)
		mdSuffix := fmt.Sprintf("%s-%s", ng.Name, mdHash)
		mdName := mdSuffix
		if instancePrefix != "" {
			mdName = fmt.Sprintf("%s-%s", instancePrefix, mdSuffix)
		}

		templateHash := sha256Hash(clusterUUID + zone + instanceClassChecksum)
		templateName := fmt.Sprintf("%s-%s", ng.Name, templateHash)
		bootstrapSecretName := templateName

		annotations := map[string]interface{}{
			"cluster.x-k8s.io/cluster-api-autoscaler-node-group-min-size": fmt.Sprintf("%d", minReplicas),
			"cluster.x-k8s.io/cluster-api-autoscaler-node-group-max-size": fmt.Sprintf("%d", maxReplicas),
		}

		serializedLabels := serializeNodeGroupLabels(ng)
		if serializedLabels != "" {
			annotations["capacity.cluster-autoscaler.kubernetes.io/labels"] = serializedLabels
		}
		serializedTaints := serializeNodeGroupTaints(ng)
		if serializedTaints != "" {
			annotations["capacity.cluster-autoscaler.kubernetes.io/taints"] = serializedTaints
		}

		commonLabels := map[string]interface{}{
			"heritage":   "deckhouse",
			"module":     "node-manager",
			"node-group": ng.Name,
		}

		var desired int32
		existing := &unstructured.Unstructured{}
		existing.SetGroupVersionKind(schema.GroupVersionKind{
			Group: "cluster.x-k8s.io", Version: "v1beta2", Kind: "MachineDeployment",
		})
		err := r.Client.Get(ctx, types.NamespacedName{Name: mdName, Namespace: common.MachineNamespace}, existing)
		if err != nil {
			if !errors.IsNotFound(err) {
				return fmt.Errorf("get MachineDeployment %s: %w", mdName, err)
			}
			desired = minReplicas
		} else {
			replicas, _, _ := unstructured.NestedInt64(existing.Object, "spec", "replicas")
			desired = calculateReplicas(int32(replicas), minReplicas, maxReplicas)
		}

		md := &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "cluster.x-k8s.io/v1beta2",
			"kind":       "MachineDeployment",
			"metadata": map[string]interface{}{
				"name":        mdName,
				"namespace":   common.MachineNamespace,
				"labels":      commonLabels,
				"annotations": annotations,
			},
			"spec": map[string]interface{}{
				"clusterName": cloudConfig.capiClusterName,
				"replicas":    int64(desired),
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": commonLabels,
					},
					"spec": map[string]interface{}{
						"clusterName": cloudConfig.capiClusterName,
						"bootstrap": map[string]interface{}{
							"dataSecretName": bootstrapSecretName,
						},
						"infrastructureRef": map[string]interface{}{
							"apiGroup": infraAPIGroup,
							"kind":     cloudConfig.capiMachineTemplateKind,
							"name":     templateName,
						},
						"deletion": map[string]interface{}{
							"nodeDrainTimeoutSeconds":        int64(drainTimeout),
							"nodeDeletionTimeoutSeconds":     int64(600),
							"nodeVolumeDetachTimeoutSeconds": int64(600),
						},
					},
				},
				"rollout": map[string]interface{}{
					"strategy": map[string]interface{}{
						"type": "RollingUpdate",
						"rollingUpdate": map[string]interface{}{
							"maxSurge":       int64(maxSurge),
							"maxUnavailable": int64(maxUnavailable),
						},
					},
				},
			},
		}}

		if err := r.Client.Patch(ctx, md, client.Apply, client.FieldOwner("node-controller"), client.ForceOwnership); err != nil {
			return fmt.Errorf("apply MachineDeployment %s: %w", mdName, err)
		}
		logger.Info("applied cloud MachineDeployment", "name", mdName, "zone", zone)
	}

	return nil
}

func (r *MachineDeploymentReconciler) reconcileStaticMD(ctx context.Context, ng *deckhousev1.NodeGroup) error {
	logger := log.FromContext(ctx)

	mdName := ng.Name
	var replicas int32
	if ng.Spec.StaticInstances.Count != nil {
		replicas = *ng.Spec.StaticInstances.Count
	}

	commonLabels := map[string]interface{}{
		"heritage":   "deckhouse",
		"module":     "node-manager",
		"node-group": ng.Name,
		"app":        "caps-controller",
	}

	md := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "cluster.x-k8s.io/v1beta2",
		"kind":       "MachineDeployment",
		"metadata": map[string]interface{}{
			"name":      mdName,
			"namespace": common.MachineNamespace,
			"labels":    commonLabels,
		},
		"spec": map[string]interface{}{
			"clusterName": "static",
			"replicas":    int64(replicas),
			"rollout": map[string]interface{}{
				"strategy": map[string]interface{}{
					"type": "RollingUpdate",
					"rollingUpdate": map[string]interface{}{
						"maxSurge":       int64(1),
						"maxUnavailable": int64(0),
					},
				},
			},
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{
						"cluster.x-k8s.io/cluster-name":    "static",
						"cluster.x-k8s.io/deployment-name": ng.Name,
					},
				},
				"spec": map[string]interface{}{
					"clusterName": "static",
					"bootstrap": map[string]interface{}{
						"dataSecretName": fmt.Sprintf("manual-bootstrap-for-%s", ng.Name),
					},
					"infrastructureRef": map[string]interface{}{
						"apiGroup": "infrastructure.cluster.x-k8s.io",
						"kind":     "StaticMachineTemplate",
						"name":     ng.Name,
					},
				},
			},
			"selector": map[string]interface{}{
				"matchLabels": map[string]interface{}{
					"cluster.x-k8s.io/cluster-name":    "static",
					"cluster.x-k8s.io/deployment-name": ng.Name,
				},
			},
		},
	}}

	if err := r.Client.Patch(ctx, md, client.Apply, client.FieldOwner("node-controller"), client.ForceOwnership); err != nil {
		return fmt.Errorf("apply static MachineDeployment %s: %w", mdName, err)
	}
	logger.Info("applied static MachineDeployment", "name", mdName)
	return nil
}

func (r *MachineDeploymentReconciler) reconcileMCMReplicas(ctx context.Context, logger interface{ Info(string, ...any) }, ngName string, minReplicas, maxReplicas int32) error {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "machine.sapcloud.io", Version: "v1alpha1", Kind: "MachineDeploymentList",
	})

	if err := r.Client.List(ctx, list,
		client.InNamespace(common.MachineNamespace),
		client.MatchingLabels{"node-group": ngName},
	); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return nil
		}
		return fmt.Errorf("list MCM MachineDeployments: %w", err)
	}

	for i := range list.Items {
		md := &list.Items[i]
		replicas, _, _ := unstructured.NestedInt64(md.Object, "spec", "replicas")
		current := int32(replicas)

		desired := calculateReplicas(current, minReplicas, maxReplicas)
		if desired == current {
			continue
		}

		patch := &unstructured.Unstructured{}
		patch.SetGroupVersionKind(md.GroupVersionKind())
		patch.SetName(md.GetName())
		patch.SetNamespace(md.GetNamespace())
		if err := unstructured.SetNestedField(patch.Object, int64(desired), "spec", "replicas"); err != nil {
			return fmt.Errorf("set replicas field: %w", err)
		}

		if err := r.Client.Patch(ctx, patch, client.Apply, client.FieldOwner("capi-set-replicas"), client.ForceOwnership); err != nil {
			return fmt.Errorf("patch MCM MachineDeployment %s replicas: %w", md.GetName(), err)
		}
		logger.Info("patched MCM replicas", "name", md.GetName(), "from", current, "to", desired)
	}
	return nil
}

type cloudProviderConfig struct {
	capiClusterName               string
	capiMachineTemplateKind       string
	capiMachineTemplateAPIVersion string
	zones                         []string
}

func (r *MachineDeploymentReconciler) readCloudProviderConfig(ctx context.Context) (*cloudProviderConfig, error) {
	secret := &corev1.Secret{}
	if err := r.APIReader.Get(ctx, types.NamespacedName{
		Name: cloudProviderSecretName, Namespace: cloudProviderSecretNamespace,
	}, secret); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return &cloudProviderConfig{}, nil
		}
		return nil, fmt.Errorf("get cloud-provider secret: %w", err)
	}

	cfg := &cloudProviderConfig{
		capiClusterName:               string(secret.Data["capiClusterName"]),
		capiMachineTemplateKind:       string(secret.Data["capiMachineTemplateKind"]),
		capiMachineTemplateAPIVersion: string(secret.Data["capiMachineTemplateAPIVersion"]),
	}
	if cfg.capiMachineTemplateAPIVersion == "" {
		cfg.capiMachineTemplateAPIVersion = "infrastructure.cluster.x-k8s.io/v1alpha1"
	}
	if raw := secret.Data["zones"]; len(raw) > 0 {
		_ = json.Unmarshal(raw, &cfg.zones)
	}
	return cfg, nil
}

func (r *MachineDeploymentReconciler) readClusterUUID(ctx context.Context) (string, error) {
	cm := &corev1.ConfigMap{}
	if err := r.APIReader.Get(ctx, types.NamespacedName{
		Name: clusterUUIDConfigMapName, Namespace: clusterUUIDConfigMapNS,
	}, cm); err != nil {
		return "", fmt.Errorf("get cluster-uuid configmap: %w", err)
	}
	return cm.Data["cluster-uuid"], nil
}

type mdClusterConfiguration struct {
	Cloud struct {
		Prefix string `json:"prefix"`
	} `json:"cloud"`
}

func (r *MachineDeploymentReconciler) readInstancePrefix(ctx context.Context) (string, error) {
	secret := &corev1.Secret{}
	if err := r.APIReader.Get(ctx, types.NamespacedName{
		Name: clusterConfigSecretName, Namespace: clusterConfigSecretNamespace,
	}, secret); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return "", nil
		}
		return "", fmt.Errorf("get cluster-configuration secret: %w", err)
	}

	raw, ok := secret.Data["cluster-configuration.yaml"]
	if !ok {
		return "", nil
	}

	decoded, err := base64.StdEncoding.DecodeString(string(raw))
	if err != nil {
		decoded = raw
	}

	cfg := &mdClusterConfiguration{}
	if err := sigsyaml.Unmarshal(decoded, cfg); err != nil {
		return "", fmt.Errorf("unmarshal cluster configuration: %w", err)
	}
	return cfg.Cloud.Prefix, nil
}

func (r *MachineDeploymentReconciler) readInstanceClassChecksum(ctx context.Context, cloudConfig *cloudProviderConfig, ngName string) (string, error) {
	templateList := &unstructured.UnstructuredList{}
	templateList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   strings.Split(cloudConfig.capiMachineTemplateAPIVersion, "/")[0],
		Version: strings.Split(cloudConfig.capiMachineTemplateAPIVersion, "/")[1],
		Kind:    cloudConfig.capiMachineTemplateKind + "List",
	})

	if err := r.APIReader.List(ctx, templateList,
		client.InNamespace(common.MachineNamespace),
		client.MatchingLabels{"node-group": ngName},
	); err != nil {
		return "", fmt.Errorf("list infrastructure templates for %s: %w", ngName, err)
	}

	for i := range templateList.Items {
		annotations := templateList.Items[i].GetAnnotations()
		if v, ok := annotations["checksum/instance-class"]; ok && v != "" {
			return v, nil
		}
	}
	return "", nil
}

func getMinMax(ng *deckhousev1.NodeGroup) (min, max int32) {
	if ng.Spec.StaticInstances != nil && ng.Spec.StaticInstances.Count != nil {
		count := *ng.Spec.StaticInstances.Count
		return count, count
	}
	if ng.Spec.CloudInstances != nil {
		min = ng.Spec.CloudInstances.MinPerZone
		max = ng.Spec.CloudInstances.MaxPerZone
	}
	return min, max
}

func calculateReplicas(current, min, max int32) int32 {
	switch {
	case min >= max:
		return max
	case current == 0:
		return min
	case current <= min:
		return min
	case current > max:
		return max
	default:
		return current
	}
}

func sha256Hash(input string) string {
	h := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", h)[:8]
}

func intOrDefault(ptr *int32, def int) int {
	if ptr != nil {
		return int(*ptr)
	}
	return def
}

func serializeNodeGroupLabels(ng *deckhousev1.NodeGroup) string {
	merged := make(map[string]string)
	if ng.Spec.NodeTemplate != nil {
		for k, v := range ng.Spec.NodeTemplate.Labels {
			merged[k] = v
		}
	}
	merged["node.deckhouse.io/group"] = ng.Name
	merged["node.deckhouse.io/type"] = string(ng.Spec.NodeType)
	merged["node-role.kubernetes.io/"+ng.Name] = ""
	return labels.FormatLabels(merged)
}

func serializeNodeGroupTaints(ng *deckhousev1.NodeGroup) string {
	if ng.Spec.NodeTemplate == nil || len(ng.Spec.NodeTemplate.Taints) == 0 {
		return ""
	}
	res := make([]string, 0, len(ng.Spec.NodeTemplate.Taints))
	for _, taint := range ng.Spec.NodeTemplate.Taints {
		res = append(res, taint.ToString())
	}
	sort.Strings(res)
	return strings.Join(res, ",")
}
