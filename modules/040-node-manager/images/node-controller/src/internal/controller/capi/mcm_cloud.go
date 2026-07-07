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
	"encoding/base64"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	sigsyaml "sigs.k8s.io/yaml"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/derived_status"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/machineclass"
)

// reconcileCloudMCMs ports the helm MCM branch of node-group.yaml
// (node_group_machine_class + node_group_machine_deployment) into the controller:
// per zone it renders the provider MachineClass CR and builds the MachineDeployment
// from the internal.nodeGroups blob element — the same source helm reads as $ng.*.
// The MachineClass Secret (config-for-machine-controller-manager) stays in helm;
// this only generates the CR (whose secretRef points at that helm-rendered Secret)
// and the MachineDeployment.
//
// ⚠ It must not run while the helm define still renders the same MCM objects
// (dual-writer / SSA ownership conflict). Wiring this into Reconcile and removing
// the helm MCM resources must ship together (brick 4e).
func (r *MachineDeploymentReconciler) reconcileCloudMCMs(ctx context.Context, ng *deckhousev1.NodeGroup) error {
	logger := log.FromContext(ctx)

	if ng.Spec.CloudInstances == nil {
		logger.V(1).Info("skipping MCM: no cloudInstances")
		return nil
	}

	cloudProvider, err := r.readCloudProviderTree(ctx)
	if err != nil {
		return err
	}
	machineClassKind, _ := cloudProvider["machineClassKind"].(string)
	if machineClassKind == "" {
		logger.V(1).Info("skipping MCM: machineClassKind not set (not an MCM cloud)")
		return nil
	}
	cloudType, _ := cloudProvider["type"].(string)
	region, _ := cloudProvider["region"].(string)

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
		logger.V(1).Info("skipping MCM: NodeGroup failed validation", "nodeGroup", ng.Name, "error", validationErr)
		return nil
	}

	zones := blobZones(blob)
	if len(zones) == 0 {
		logger.V(1).Info("skipping MCM: no zones")
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
	podSubnet, err := r.readPodSubnet(ctx)
	if err != nil {
		return err
	}

	machineClassTemplate, err := machineclass.ReadChecksumTemplate(
		machineclass.DefaultTemplateBaseDirs, machineclass.FallbackTemplateBaseDir,
		cloudType, machineclass.MCMMachineClassSubPath)
	if err != nil {
		return err
	}
	checksumTemplate, err := machineclass.ReadChecksumTemplate(
		machineclass.DefaultTemplateBaseDirs, machineclass.FallbackTemplateBaseDir,
		cloudType, machineclass.MCMChecksumSubPath)
	if err != nil {
		return err
	}

	checksum, err := machineclass.RenderChecksum(checksumTemplate, blob)
	if err != nil {
		return fmt.Errorf("render checksum for NodeGroup %s: %w", ng.Name, err)
	}

	minReplicas, maxReplicas := getMinMax(ng)
	awsSpot := cloudType == "aws" && blobInstanceClassSpot(blob)

	for _, zone := range zones {
		hash := sha256Hash(clusterUUID + zone)
		machineClassName := fmt.Sprintf("%s-%s", ng.Name, hash)
		mdName := machineClassName
		if instancePrefix != "" {
			mdName = fmt.Sprintf("%s-%s", instancePrefix, machineClassName)
		}

		renderCtx := map[string]interface{}{
			"Chart": map[string]interface{}{"Name": "node-manager"},
			"Values": map[string]interface{}{
				"global": map[string]interface{}{
					"discovery": map[string]interface{}{
						"clusterUUID": clusterUUID,
						"podSubnet":   podSubnet,
					},
				},
				"nodeManager": map[string]interface{}{
					"internal": map[string]interface{}{
						"cloudProvider": cloudProvider,
					},
				},
			},
			"nodeGroup": blob,
			"zoneName":  zone,
		}

		mcBytes, err := machineclass.RenderMachineClass(machineClassTemplate, renderCtx)
		if err != nil {
			return fmt.Errorf("render MachineClass for NodeGroup %s zone %s: %w", ng.Name, zone, err)
		}
		mcObject := map[string]interface{}{}
		if err := sigsyaml.Unmarshal(mcBytes, &mcObject); err != nil {
			return fmt.Errorf("parse rendered MachineClass for NodeGroup %s zone %s: %w", ng.Name, zone, err)
		}
		machineClassObj := &unstructured.Unstructured{Object: mcObject}

		replicas := r.mcmDesiredReplicas(ctx, mdName, minReplicas, maxReplicas)

		md := buildMCMMachineDeployment(mcmMachineDeploymentInput{
			blob:             blob,
			ngName:           ng.Name,
			zone:             zone,
			mdName:           mdName,
			machineClassName: machineClassName,
			machineClassKind: machineClassKind,
			region:           region,
			checksum:         checksum,
			replicas:         replicas,
			awsSpot:          awsSpot,
		})

		// Apply the MachineClass first: its checksum is baked into the
		// MachineDeployment annotation below, so the class must be current before
		// the deployment references that checksum (mirrors the helm ordering that
		// the machineclass_checksum hooks enforced).
		if err := r.Client.Patch(ctx, machineClassObj, client.Apply, client.FieldOwner("node-controller"), client.ForceOwnership); err != nil {
			return fmt.Errorf("apply MachineClass %s: %w", machineClassName, err)
		}
		if err := r.Client.Patch(ctx, md, client.Apply, client.FieldOwner("node-controller"), client.ForceOwnership); err != nil {
			return fmt.Errorf("apply MCM MachineDeployment %s: %w", mdName, err)
		}
		logger.V(1).Info("applied MCM MachineClass + MachineDeployment", "name", mdName, "zone", zone)
	}

	return nil
}

// mcmDesiredReplicas returns the replica count to write for the zone's MCM
// MachineDeployment: it reads the current replicas of an existing deployment and
// clamps to [min,max] via calculateReplicas (preserving an in-range autoscaler
// value), or seeds a new deployment at min.
func (r *MachineDeploymentReconciler) mcmDesiredReplicas(ctx context.Context, mdName string, minReplicas, maxReplicas int32) int64 {
	existing := newUnstructured("machine.sapcloud.io", "v1alpha1", "MachineDeployment")
	err := r.Client.Get(ctx, types.NamespacedName{Name: mdName, Namespace: common.MachineNamespace}, existing)
	if err != nil {
		return int64(minReplicas)
	}
	current, _, _ := unstructured.NestedInt64(existing.Object, "spec", "replicas")
	return int64(calculateReplicas(int32(current), minReplicas, maxReplicas))
}

// readCloudProviderTree decodes the whole d8-node-manager-cloud-provider Secret
// into the internal.cloudProvider value tree (.type, .region, .machineClassKind,
// .<provider>, ...), mirroring derived_status.readCloudProviderData so the tree
// fed to RenderMachineClass matches what helm reads from internal.cloudProvider.
func (r *MachineDeploymentReconciler) readCloudProviderTree(ctx context.Context) (map[string]interface{}, error) {
	secret := &corev1.Secret{}
	if err := r.APIReader.Get(ctx, types.NamespacedName{
		Name: cloudProviderSecretName, Namespace: cloudProviderSecretNamespace,
	}, secret); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return map[string]interface{}{}, nil
		}
		return nil, fmt.Errorf("get cloud-provider secret: %w", err)
	}
	return decodeCloudProviderSecret(secret.Data), nil
}

// decodeCloudProviderSecret JSON-decodes each Secret value (already base64-decoded
// by the client), falling back to the raw string for non-JSON values — identical
// to derived_status.decodeSecretData.
func decodeCloudProviderSecret(data map[string][]byte) map[string]interface{} {
	res := make(map[string]interface{}, len(data))
	for k, v := range data {
		var val interface{}
		if err := json.Unmarshal(v, &val); err != nil {
			res[k] = string(v)
			continue
		}
		res[k] = val
	}
	return res
}

// readNodeGroupRawSpec fetches the NodeGroup as unstructured and returns its raw
// .spec (CRD-shaped, apiserver-pruned) — the input BuildElement needs to preserve
// byte-parity, since the hand-rolled typed spec's JSON shape diverges from the CRD.
func (r *MachineDeploymentReconciler) readNodeGroupRawSpec(ctx context.Context, name string) (map[string]interface{}, error) {
	obj := newUnstructured(deckhousev1.GroupVersion.Group, deckhousev1.GroupVersion.Version, "NodeGroup")
	if err := r.Client.Get(ctx, types.NamespacedName{Name: name}, obj); err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get NodeGroup %s: %w", name, err)
	}
	spec, _ := obj.Object["spec"].(map[string]interface{})
	return spec, nil
}

// readPodSubnet returns global.discovery.podSubnet — the podSubnetCIDR of the
// d8-cluster-configuration Secret — which the openstack MachineClass template reads.
// The value is base64-decoded with a raw fallback, matching readInstancePrefix.
func (r *MachineDeploymentReconciler) readPodSubnet(ctx context.Context) (string, error) {
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
	if decoded, decErr := base64.StdEncoding.DecodeString(string(raw)); decErr == nil {
		raw = decoded
	}
	var cfg struct {
		PodSubnetCIDR string `json:"podSubnetCIDR"`
	}
	if err := sigsyaml.Unmarshal(raw, &cfg); err != nil {
		return "", fmt.Errorf("unmarshal cluster configuration: %w", err)
	}
	return cfg.PodSubnetCIDR, nil
}

// blobZones extracts cloudInstances.zones from the blob element as a string slice.
func blobZones(blob map[string]interface{}) []string {
	ci := blobMap(blob, "cloudInstances")
	raw, ok := ci["zones"].([]interface{})
	if !ok {
		return nil
	}
	zones := make([]string, 0, len(raw))
	for _, z := range raw {
		if s, ok := z.(string); ok {
			zones = append(zones, s)
		}
	}
	return zones
}

// blobInstanceClassSpot reports whether the resolved instanceClass has spot: true,
// gating the aws creationTimeout the helm node_group_spot_creation_timeout define adds.
func blobInstanceClassSpot(blob map[string]interface{}) bool {
	ic := blobMap(blob, "instanceClass")
	spot, _ := ic["spot"].(bool)
	return spot
}
