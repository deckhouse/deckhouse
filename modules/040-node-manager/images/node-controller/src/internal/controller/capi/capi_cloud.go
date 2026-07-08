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
	"strings"

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

// capiInstanceClassChecksum computes the checksum/instance-class value the CAPI
// MachineTemplate carries, in the controller instead of helm: it reads the baked
// capi/<type>/instance-class.checksum template and renders it against the blob
// element (the resolved instanceClass). This replaces readInstanceClassChecksum's
// "waiting for helm" dependency on the helm-rendered MachineTemplate annotation.
//
// The value must stay byte-identical to the helm define — it is hashed into the
// MachineTemplate name (templateName), so any drift renames the template and rolls
// every CAPI node.
func capiInstanceClassChecksum(cloudType string, blob map[string]interface{}) (string, error) {
	checksumTemplate, err := machineclass.ReadChecksumTemplate(
		machineclass.DefaultTemplateBaseDirs, machineclass.FallbackTemplateBaseDir,
		cloudType, machineclass.CAPIChecksumSubPath)
	if err != nil {
		return "", err
	}
	checksum, err := machineclass.RenderChecksum(checksumTemplate, blob)
	if err != nil {
		return "", fmt.Errorf("render instance-class checksum: %w", err)
	}
	return checksum, nil
}

// capiMachineTemplateContext builds the render context for capi/<type>/machine-template.yaml,
// mirroring the helm capi_node_group_machine_template tpl context. All CAPI provider
// templates read only .Values.nodeManager.internal.cloudProvider.<provider>, .nodeGroup
// (the blob element with the resolved instanceClass), .zoneName, .templateName and
// .instanceClassChecksum — so no global.discovery/podSubnet is needed here.
func capiMachineTemplateContext(cloudProvider, blob map[string]interface{}, zone, templateName, checksum string) map[string]interface{} {
	return map[string]interface{}{
		"Chart": map[string]interface{}{"Name": "node-manager"},
		"Values": map[string]interface{}{
			"nodeManager": map[string]interface{}{
				"internal": map[string]interface{}{
					"cloudProvider": cloudProvider,
				},
			},
		},
		"nodeGroup":             blob,
		"zoneName":              zone,
		"templateName":          templateName,
		"instanceClassChecksum": checksum,
	}
}

// renderCAPIMachineTemplate reads the baked capi/<type>/machine-template.yaml and
// renders it (through the shared RenderMachineClass engine) into the infrastructure
// MachineTemplate object the controller applies — replacing the helm-rendered
// MachineTemplate. renderCtx is built by capiMachineTemplateContext.
func renderCAPIMachineTemplate(cloudType string, renderCtx map[string]interface{}) (*unstructured.Unstructured, error) {
	tmpl, err := machineclass.ReadChecksumTemplate(
		machineclass.DefaultTemplateBaseDirs, machineclass.FallbackTemplateBaseDir,
		cloudType, machineclass.CAPIMachineTemplateSubPath)
	if err != nil {
		return nil, err
	}
	mtBytes, err := machineclass.RenderMachineClass(tmpl, renderCtx)
	if err != nil {
		return nil, fmt.Errorf("render MachineTemplate for cloud type %q: %w", cloudType, err)
	}
	obj := map[string]interface{}{}
	if err := sigsyaml.Unmarshal(mtBytes, &obj); err != nil {
		return nil, fmt.Errorf("parse rendered MachineTemplate for cloud type %q: %w", cloudType, err)
	}
	return &unstructured.Unstructured{Object: obj}, nil
}

// capiMDInput is the resolved, side-effect-free input for a single zone's CAPI
// MachineDeployment, extracted so both the live reconcileCloudMDs and the
// controller-rendered reconcileCloudMDsRendered build byte-identical objects.
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

// buildCAPIMachineDeployment renders the cluster.x-k8s.io/v1beta2 MachineDeployment
// for one zone. It mirrors the helm _machine_deployment CAPI shape: autoscaler
// min/max-size annotations, capacity labels/taints annotations, module labels and
// the RollingUpdate strategy.
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

// capiDesiredReplicas reads the current replicas of an existing CAPI MachineDeployment
// and clamps to [min,max] (preserving an in-range autoscaler value), or seeds a new
// deployment at min. Mirrors mcmDesiredReplicas for the CAPI GVK.
func (r *MachineDeploymentReconciler) capiDesiredReplicas(ctx context.Context, mdName string, minReplicas, maxReplicas int32) int32 {
	existing := newUnstructured("cluster.x-k8s.io", "v1beta2", "MachineDeployment")
	if err := r.Client.Get(ctx, types.NamespacedName{Name: mdName, Namespace: common.MachineNamespace}, existing); err != nil {
		return minReplicas
	}
	replicas, _, _ := unstructured.NestedInt64(existing.Object, "spec", "replicas")
	return calculateReplicas(int32(replicas), minReplicas, maxReplicas)
}

// reconcileCloudMDsRendered is the controller-rendered CAPI branch that replaces
// the helm CAPI generation (node-group.yaml capi_node_group_machine_template +
// instance-class checksum + bootstrap Secret naming). Per zone it:
//   - computes the instance-class checksum from the blob (no "waiting for helm"),
//   - renders and applies the infrastructure MachineTemplate,
//   - applies the MachineDeployment referencing that template.
//
// The bootstrap Secret keeps living in helm but under the checksum-independent name
// {ng.name}-{sha(clusterUUID+zone)} (path 2b), so the MachineDeployment's
// dataSecretName points at that stable Secret.
//
// ⚠ Must not run while the helm CAPI define still renders the same MachineTemplate
// (dual-writer / SSA ownership conflict). Wiring this into Reconcile and removing
// the helm CAPI resources + renaming the Secret must ship together (cutover brick).
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
	ds := &derived_status.Service{Client: r.Client}
	blob, validationErr, err := ds.BuildElement(ctx, ng, rawSpec)
	if err != nil {
		return fmt.Errorf("build blob element for NodeGroup %s: %w", ng.Name, err)
	}
	if validationErr != "" {
		logger.V(1).Info("skipping CAPI: NodeGroup failed validation", "nodeGroup", ng.Name, "error", validationErr)
		return nil
	}

	zones := blobZones(blob)
	if len(zones) == 0 {
		zones = cloudConfig.zones
	}
	if len(zones) == 0 {
		logger.V(1).Info("skipping CAPI: no zones")
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

	checksum, err := capiInstanceClassChecksum(cloudType, blob)
	if err != nil {
		return fmt.Errorf("compute instance-class checksum for NodeGroup %s: %w", ng.Name, err)
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
		mdSuffix := fmt.Sprintf("%s-%s", ng.Name, sha256Hash(clusterUUID+zone))
		mdName := mdSuffix
		if instancePrefix != "" {
			mdName = fmt.Sprintf("%s-%s", instancePrefix, mdSuffix)
		}

		templateName := fmt.Sprintf("%s-%s", ng.Name, sha256Hash(clusterUUID+zone+checksum))
		// Path 2b: the bootstrap Secret name no longer embeds the instance-class
		// checksum (helm renders it on the thin blob under this stable name).
		bootstrapSecretName := fmt.Sprintf("%s-%s", ng.Name, sha256Hash(clusterUUID+zone))

		mtCtx := capiMachineTemplateContext(cloudProvider, blob, zone, templateName, checksum)
		mt, err := renderCAPIMachineTemplate(cloudType, mtCtx)
		if err != nil {
			return fmt.Errorf("render MachineTemplate for NodeGroup %s zone %s: %w", ng.Name, zone, err)
		}

		md := buildCAPIMachineDeployment(capiMDInput{
			ng:                  ng,
			mdName:              mdName,
			templateName:        templateName,
			bootstrapSecretName: bootstrapSecretName,
			clusterName:         cloudConfig.capiClusterName,
			infraAPIGroup:       infraAPIGroup,
			infraKind:           cloudConfig.capiMachineTemplateKind,
			desired:             r.capiDesiredReplicas(ctx, mdName, minReplicas, maxReplicas),
			minReplicas:         minReplicas,
			maxReplicas:         maxReplicas,
			maxSurge:            int32(maxSurge),
			maxUnavailable:      int32(maxUnavailable),
			drainTimeout:        drainTimeout,
		})

		// Apply the MachineTemplate first: the MachineDeployment's infrastructureRef
		// points at templateName, so the template must exist before the deployment
		// references it (mirrors the helm hook ordering the checksum enforced).
		if err := r.Client.Patch(ctx, mt, client.Apply, client.FieldOwner("node-controller"), client.ForceOwnership); err != nil {
			return fmt.Errorf("apply MachineTemplate %s: %w", templateName, err)
		}
		if err := r.Client.Patch(ctx, md, client.Apply, client.FieldOwner("node-controller"), client.ForceOwnership); err != nil {
			return fmt.Errorf("apply CAPI MachineDeployment %s: %w", mdName, err)
		}
		logger.V(1).Info("applied CAPI MachineTemplate + MachineDeployment", "name", mdName, "zone", zone)
	}

	return nil
}

// buildStaticMachineTemplate renders the infrastructure.cluster.x-k8s.io/v1alpha1
// StaticMachineTemplate for a NodeGroup, mirroring the helm
// node_group_static_or_hybrid_machine_template define: name = ng.Name, the
// two-arg module labels (heritage/module/node-group) on both metadata and
// spec.template.metadata, and spec.template.spec.labelSelector copied from
// staticInstances.labelSelector when set (else an empty spec).
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

// reconcileStaticMDRendered is the controller-rendered static branch that replaces
// the helm node_group_static_or_hybrid_machine_template. It applies the
// StaticMachineTemplate (the MachineDeployment's infrastructureRef points at it, so
// it must exist first) then the static MachineDeployment via the shared buildStaticMD.
//
// ⚠ Must not run while the helm static define still renders the same
// StaticMachineTemplate (dual-writer / SSA ownership conflict). Wiring this into
// Reconcile and removing the helm static machine_template must ship together.
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
	logger.V(1).Info("applied static MachineTemplate + MachineDeployment", "name", ng.Name)
	return nil
}
