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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	sigsyaml "sigs.k8s.io/yaml"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/derived_status"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/machineclass"
)

func (r *MachineDeploymentReconciler) reconcileCloudMCMs(ctx context.Context, ng *deckhousev1.NodeGroup) error {
	logger := log.FromContext(ctx)

	if ng.Spec.CloudInstances == nil {
		logger.Info("skipping MCM: no cloudInstances", "nodeGroup", ng.Name)
		return nil
	}

	cloudProvider, err := r.readCloudProviderTree(ctx)
	if err != nil {
		return err
	}
	machineClassKind, _ := cloudProvider["machineClassKind"].(string)
	if machineClassKind == "" {
		logger.Info("skipping MCM: machineClassKind not set (not an MCM cloud)", "nodeGroup", ng.Name)
		return nil
	}
	cloudType, _ := cloudProvider["type"].(string)
	region, _ := cloudProvider["region"].(string)

	rawSpec, err := r.readNodeGroupRawSpec(ctx, ng.Name)
	if err != nil {
		return err
	}
	ds := &derived_status.Service{Client: r.Client, Reader: r.APIReader}
	blob, validationErr, err := ds.BuildElement(ctx, ng, rawSpec)
	if err != nil {
		return fmt.Errorf("build blob element for NodeGroup %s: %w", ng.Name, err)
	}
	zones := blobZones(blob)
	logger.Info("MCM reconcile decision", "nodeGroup", ng.Name, "validationErr", validationErr, "zones", zones, "machineClassKind", machineClassKind)
	if validationErr != "" {
		logger.Info("skipping MCM: NodeGroup failed validation", "nodeGroup", ng.Name, "error", validationErr)
		return nil
	}

	if len(zones) == 0 {
		logger.Info("skipping MCM: no zones", "nodeGroup", ng.Name)
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

	desiredMDNames := make(map[string]struct{}, len(zones))

	for _, zone := range zones {
		hash := sha256Hash(clusterUUID + zone)
		machineClassName := fmt.Sprintf("%s-%s", ng.Name, hash)
		mdName := machineClassName
		if instancePrefix != "" {
			mdName = fmt.Sprintf("%s-%s", instancePrefix, machineClassName)
		}
		desiredMDNames[mdName] = struct{}{}

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

		replicas, err := r.mcmDesiredReplicas(ctx, mdName, minReplicas, maxReplicas)
		if err != nil {
			return err
		}

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

		if err := r.Client.Patch(ctx, machineClassObj, client.Apply, client.FieldOwner("node-controller"), client.ForceOwnership); err != nil {
			return fmt.Errorf("apply MachineClass %s: %w", machineClassName, err)
		}
		if err := r.Client.Patch(ctx, md, client.Apply, client.FieldOwner("node-controller"), client.ForceOwnership); err != nil {
			return fmt.Errorf("apply MCM MachineDeployment %s: %w", mdName, err)
		}
		logger.Info("applied MCM MachineClass + MachineDeployment", "name", mdName, "zone", zone)
	}

	if err := r.pruneStaleMCMs(ctx, ng.Name, desiredMDNames); err != nil {
		return err
	}

	return nil
}

// pruneStaleMCMs deletes MCM MachineDeployments (and their referenced MachineClasses)
// that belong to the NodeGroup but are no longer desired, e.g. after a zone is removed.
// MachineDeployments are the reliable anchor: both helm (pre-migration) and node-controller
// stamp them with the node-group label, and each one references its MachineClass by name.
func (r *MachineDeploymentReconciler) pruneStaleMCMs(ctx context.Context, ngName string, desired map[string]struct{}) error {
	logger := log.FromContext(ctx)

	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "machine.sapcloud.io", Version: "v1alpha1", Kind: "MachineDeploymentList",
	})
	if err := r.Client.List(ctx, list,
		client.InNamespace(common.MachineNamespace),
		client.MatchingLabels{"node-group": ngName},
	); err != nil {
		return fmt.Errorf("list MCM MachineDeployments for NodeGroup %s: %w", ngName, err)
	}

	for i := range list.Items {
		md := &list.Items[i]
		if _, ok := desired[md.GetName()]; ok {
			continue
		}
		if !md.GetDeletionTimestamp().IsZero() {
			continue
		}
		if err := r.deleteReferencedMachineClass(ctx, md); err != nil {
			return err
		}
		if err := r.Client.Delete(ctx, md); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("delete stale MCM MachineDeployment %s: %w", md.GetName(), err)
		}
		logger.Info("pruned stale MCM MachineDeployment", "name", md.GetName(), "ng", ngName)
	}

	return nil
}

// deleteReferencedMachineClass deletes the MCM MachineClass referenced by the given
// MachineDeployment via spec.template.spec.class. A missing MachineClass is not an error.
func (r *MachineDeploymentReconciler) deleteReferencedMachineClass(ctx context.Context, md *unstructured.Unstructured) error {
	kind, _, _ := unstructured.NestedString(md.Object, "spec", "template", "spec", "class", "kind")
	name, _, _ := unstructured.NestedString(md.Object, "spec", "template", "spec", "class", "name")
	if kind == "" || name == "" {
		return nil
	}
	mc := newUnstructured("machine.sapcloud.io", "v1alpha1", kind)
	mc.SetName(name)
	mc.SetNamespace(common.MachineNamespace)
	if err := r.Client.Delete(ctx, mc); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("delete MachineClass %s: %w", name, err)
	}
	return nil
}

func (r *MachineDeploymentReconciler) mcmDesiredReplicas(ctx context.Context, mdName string, minReplicas, maxReplicas int32) (int64, error) {
	existing := newUnstructured("machine.sapcloud.io", "v1alpha1", "MachineDeployment")
	if err := r.Client.Get(ctx, types.NamespacedName{Name: mdName, Namespace: common.MachineNamespace}, existing); err != nil {
		if errors.IsNotFound(err) {
			return int64(minReplicas), nil
		}
		return 0, fmt.Errorf("get MCM MachineDeployment %s: %w", mdName, err)
	}
	current, found, err := unstructured.NestedInt64(existing.Object, "spec", "replicas")
	if err != nil {
		return 0, fmt.Errorf("read spec.replicas of MCM MachineDeployment %s: %w", mdName, err)
	}
	if !found {
		return 0, fmt.Errorf("MCM MachineDeployment %s has no spec.replicas", mdName)
	}
	return int64(calculateReplicas(int32(current), minReplicas, maxReplicas)), nil
}

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
	switch raw := ci["zones"].(type) {
	case []string:
		return raw
	case []interface{}:
		zones := make([]string, 0, len(raw))
		for _, z := range raw {
			if s, ok := z.(string); ok {
				zones = append(zones, s)
			}
		}
		return zones
	default:
		return nil
	}
}

func blobInstanceClassSpot(blob map[string]interface{}) bool {
	ic := blobMap(blob, "instanceClass")
	spot, _ := ic["spot"].(bool)
	return spot
}
