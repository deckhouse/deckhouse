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
	"fmt"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	sigsyaml "sigs.k8s.io/yaml"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/derived_status"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/machineclass"
)

type capiMDInput struct {
	ng                  *deckhousev1.NodeGroup
	mdName              string
	templateName        string
	bootstrapSecretName string
	clusterName         string
	infraAPIGroup       string
	infraKind           string
	desired             int32
	minReplicas         int32
	maxReplicas         int32
	maxSurge            int32
	maxUnavailable      int32
	drainTimeout        int
}

func buildCAPIMachineDeployment(in capiMDInput) *unstructured.Unstructured {
	annotations := map[string]interface{}{
		"cluster.x-k8s.io/cluster-api-autoscaler-node-group-min-size": fmt.Sprintf("%d", in.minReplicas),
		"cluster.x-k8s.io/cluster-api-autoscaler-node-group-max-size": fmt.Sprintf("%d", in.maxReplicas),
	}
	if s := serializeNodeGroupLabels(in.ng); s != "" {
		annotations["capacity.cluster-autoscaler.kubernetes.io/labels"] = s
	}
	if s := serializeNodeGroupTaints(in.ng); s != "" {
		annotations["capacity.cluster-autoscaler.kubernetes.io/taints"] = s
	}

	commonLabels := map[string]interface{}{
		"heritage":   "deckhouse",
		"module":     "node-manager",
		"node-group": in.ng.Name,
	}

	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "cluster.x-k8s.io/v1beta2",
		"kind":       "MachineDeployment",
		"metadata": map[string]interface{}{
			"name":        in.mdName,
			"namespace":   common.MachineNamespace,
			"labels":      commonLabels,
			"annotations": annotations,
		},
		"spec": map[string]interface{}{
			"clusterName": in.clusterName,
			"replicas":    int64(in.desired),
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": commonLabels,
				},
				"spec": map[string]interface{}{
					"clusterName": in.clusterName,
					"bootstrap": map[string]interface{}{
						"dataSecretName": in.bootstrapSecretName,
					},
					"infrastructureRef": map[string]interface{}{
						"apiGroup": in.infraAPIGroup,
						"kind":     in.infraKind,
						"name":     in.templateName,
					},
					"deletion": map[string]interface{}{
						"nodeDrainTimeoutSeconds":        int64(in.drainTimeout),
						"nodeDeletionTimeoutSeconds":     int64(600),
						"nodeVolumeDetachTimeoutSeconds": int64(600),
					},
				},
			},
			"rollout": map[string]interface{}{
				"strategy": map[string]interface{}{
					"type": "RollingUpdate",
					"rollingUpdate": map[string]interface{}{
						"maxSurge":       int64(in.maxSurge),
						"maxUnavailable": int64(in.maxUnavailable),
					},
				},
			},
		},
	}}
}

func (r *MachineDeploymentReconciler) capiDesiredReplicas(ctx context.Context, mdName string, minReplicas, maxReplicas int32) (int32, error) {
	existing := newUnstructured("cluster.x-k8s.io", "v1beta2", "MachineDeployment")
	if err := r.Client.Get(ctx, types.NamespacedName{Name: mdName, Namespace: common.MachineNamespace}, existing); err != nil {
		if errors.IsNotFound(err) {
			return minReplicas, nil
		}
		return 0, fmt.Errorf("get CAPI MachineDeployment %s: %w", mdName, err)
	}
	replicas, found, err := unstructured.NestedInt64(existing.Object, "spec", "replicas")
	if err != nil {
		return 0, fmt.Errorf("read spec.replicas of CAPI MachineDeployment %s: %w", mdName, err)
	}
	if !found {
		return 0, fmt.Errorf("CAPI MachineDeployment %s has no spec.replicas", mdName)
	}
	return calculateReplicas(int32(replicas), minReplicas, maxReplicas), nil
}

func (r *MachineDeploymentReconciler) reconcileCloudMDsRendered(ctx context.Context, ng *deckhousev1.NodeGroup) error {
	logger := log.FromContext(ctx)

	if ng.Spec.CloudInstances == nil {
		logger.V(1).Info("skipping CAPI: no cloudInstances")
		return nil
	}

	cloudConfig, err := r.readCloudProviderConfig(ctx)
	if err != nil {
		return err
	}
	if cloudConfig.capiClusterName == "" {
		logger.V(1).Info("skipping CAPI: capiClusterName is empty")
		return nil
	}

	cloudProvider, err := r.readCloudProviderTree(ctx)
	if err != nil {
		return err
	}
	cloudType, _ := cloudProvider["type"].(string)

	rawSpec, err := r.readNodeGroupRawSpec(ctx, ng.Name)
	if err != nil {
		return err
	}
	ds := &derived_status.Service{Client: r.Client, Reader: r.APIReader}
	blob, validationErr, err := ds.BuildElement(ctx, ng, rawSpec)
	if err != nil {
		return fmt.Errorf("build blob element for NodeGroup %s: %w", ng.Name, err)
	}
	if validationErr != "" {
		logger.Info("skipping CAPI: NodeGroup failed validation", "nodeGroup", ng.Name, "error", validationErr)
		return nil
	}
	zones := blobZones(blob)
	if len(zones) == 0 {
		logger.V(1).Info("skipping CAPI: no zones")
		return nil
	}

	// node-controller renders the infrastructure MachineTemplate and its instance-class
	// checksum from the cloud-provider CAPI template secret (published at the 030 step),
	// so it no longer waits for helm. The checksum must stay byte-identical to helm's
	// former output, otherwise the template name changes and existing nodes roll.
	machineTemplateTpl, err := r.readProviderTemplate(ctx, cloudType, engineCAPITemplates, "machine-template.yaml")
	if err != nil {
		return err
	}
	checksumTpl, err := r.readProviderTemplate(ctx, cloudType, engineCAPITemplates, "instance-class.checksum")
	if err != nil {
		return err
	}
	checksum, err := machineclass.RenderChecksum(checksumTpl, blob)
	if err != nil {
		return fmt.Errorf("render CAPI instance-class checksum for NodeGroup %s: %w", ng.Name, err)
	}

	clusterUUID, err := r.readClusterUUID(ctx)
	if err != nil {
		return err
	}
	podSubnet, err := r.readPodSubnet(ctx)
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

	desiredMDNames := make(map[string]struct{}, len(zones))
	desiredTemplateNames := make(map[string]struct{}, len(zones))

	for _, zone := range zones {
		mdSuffix := fmt.Sprintf("%s-%s", ng.Name, sha256Hash(clusterUUID+zone))
		mdName := mdSuffix
		if instancePrefix != "" {
			mdName = fmt.Sprintf("%s-%s", instancePrefix, mdSuffix)
		}
		desiredMDNames[mdName] = struct{}{}

		templateName := fmt.Sprintf("%s-%s", ng.Name, sha256Hash(clusterUUID+zone+checksum))
		desiredTemplateNames[templateName] = struct{}{}
		// The bootstrap Secret is still rendered by helm (its cloud-init needs candi) and
		// keeps mirroring the MachineTemplate name, so the dataSecretName stays stable.
		bootstrapSecretName := templateName

		if err := r.applyCAPIMachineTemplate(ctx, machineTemplateTpl, cloudProvider, blob, clusterUUID, podSubnet, zone, templateName, checksum); err != nil {
			return err
		}

		desired, err := r.capiDesiredReplicas(ctx, mdName, minReplicas, maxReplicas)
		if err != nil {
			return err
		}

		md := buildCAPIMachineDeployment(capiMDInput{
			ng:                  ng,
			mdName:              mdName,
			templateName:        templateName,
			bootstrapSecretName: bootstrapSecretName,
			clusterName:         cloudConfig.capiClusterName,
			infraAPIGroup:       infraAPIGroup,
			infraKind:           cloudConfig.capiMachineTemplateKind,
			desired:             desired,
			minReplicas:         minReplicas,
			maxReplicas:         maxReplicas,
			maxSurge:            int32(maxSurge),
			maxUnavailable:      int32(maxUnavailable),
			drainTimeout:        drainTimeout,
		})

		if err := applyMachineDeploymentSpecPatch(
			md.Object["spec"].(map[string]interface{}),
			cloudConfig.capiMachineDeploymentSpecPatch,
			map[string]string{
				"bootstrapSecretName": bootstrapSecretName,
				"clusterName":         cloudConfig.capiClusterName,
				"mdName":              mdName,
				"nodeGroupName":       ng.Name,
				"templateName":        templateName,
				"zone":                zone,
			},
		); err != nil {
			return fmt.Errorf("apply provider MachineDeployment spec patch for %s: %w", mdName, err)
		}

		if err := r.Client.Patch(ctx, md, client.Apply, client.FieldOwner("node-controller"), client.ForceOwnership); err != nil {
			return fmt.Errorf("apply CAPI MachineDeployment %s: %w", mdName, err)
		}
		logger.Info("applied CAPI MachineTemplate + MachineDeployment", "name", mdName, "zone", zone)
	}

	if err := r.pruneStaleCAPI(ctx, ng.Name, cloudConfig, desiredMDNames, desiredTemplateNames); err != nil {
		return err
	}

	return nil
}

func buildStaticMachineTemplate(ng *deckhousev1.NodeGroup) (*unstructured.Unstructured, error) {
	labels := map[string]interface{}{
		"heritage":   "deckhouse",
		"module":     "node-manager",
		"node-group": ng.Name,
	}

	templateSpec := map[string]interface{}{}
	if ls := ng.Spec.StaticInstances.LabelSelector; ls != nil {
		m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(ls)
		if err != nil {
			return nil, fmt.Errorf("convert staticInstances.labelSelector for NodeGroup %s: %w", ng.Name, err)
		}
		templateSpec["labelSelector"] = m
	}

	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "infrastructure.cluster.x-k8s.io/v1alpha1",
		"kind":       "StaticMachineTemplate",
		"metadata": map[string]interface{}{
			"name":      ng.Name,
			"namespace": common.MachineNamespace,
			"labels":    labels,
		},
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": labels,
				},
				"spec": templateSpec,
			},
		},
	}}, nil
}

func (r *MachineDeploymentReconciler) reconcileStaticMDRendered(ctx context.Context, ng *deckhousev1.NodeGroup) error {
	logger := log.FromContext(ctx)

	smt, err := buildStaticMachineTemplate(ng)
	if err != nil {
		return err
	}
	if err := r.Client.Patch(ctx, smt, client.Apply, client.FieldOwner("node-controller"), client.ForceOwnership); err != nil {
		return fmt.Errorf("apply StaticMachineTemplate %s: %w", ng.Name, err)
	}

	md := buildStaticMD(ng)
	if err := r.Client.Patch(ctx, md, client.Apply, client.FieldOwner("node-controller"), client.ForceOwnership); err != nil {
		return fmt.Errorf("apply static MachineDeployment %s: %w", ng.Name, err)
	}
	logger.Info("applied static MachineTemplate + MachineDeployment", "name", ng.Name)
	return nil
}

func applyMachineDeploymentSpecPatch(spec map[string]interface{}, rawPatch string, vars map[string]string) error {
	if strings.TrimSpace(rawPatch) == "" {
		return nil
	}

	patch := map[string]interface{}{}
	if err := sigsyaml.Unmarshal([]byte(substitutePatchVariables(rawPatch, vars)), &patch); err != nil {
		return fmt.Errorf("unmarshal spec patch: %w", err)
	}

	deepMergeMaps(spec, patch)
	return nil
}

func substitutePatchVariables(raw string, vars map[string]string) string {
	if len(vars) == 0 {
		return raw
	}

	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	replacements := make([]string, 0, len(keys)*2)
	for _, k := range keys {
		replacements = append(replacements, "${"+k+"}", vars[k])
	}

	return strings.NewReplacer(replacements...).Replace(raw)
}

func deepMergeMaps(dst, src map[string]interface{}) {
	for k, v := range src {
		srcMap, srcIsMap := v.(map[string]interface{})
		if !srcIsMap {
			dst[k] = v
			continue
		}

		dstMap, dstIsMap := dst[k].(map[string]interface{})
		if !dstIsMap {
			dst[k] = srcMap
			continue
		}

		deepMergeMaps(dstMap, srcMap)
	}
}
