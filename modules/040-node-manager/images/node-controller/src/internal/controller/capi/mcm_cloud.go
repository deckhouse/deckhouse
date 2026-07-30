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
	"k8s.io/apimachinery/pkg/api/meta"
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

func (r *MachineDeploymentReconciler) reconcileCloudMCMs(ctx context.Context, ng *deckhousev1.NodeGroup, rawSpec map[string]interface{}) error {
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

	ds := &derived_status.Service{Client: r.Client, Reader: r.APIReader}
	resolved, validationErr, err := ds.ResolveNodeGroup(ctx, ng, rawSpec)
	if err != nil {
		return fmt.Errorf("resolve NodeGroup %s: %w", ng.Name, err)
	}
	zones := resolved.Zones
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

	machineClassTemplate, err := r.readProviderTemplate(ctx, cloudType, engineMCMTemplates, "machine-class.yaml")
	if err != nil {
		return err
	}
	checksumTemplate, err := r.readProviderTemplate(ctx, cloudType, engineMCMTemplates, "machine-class.checksum")
	if err != nil {
		return err
	}

	// The templates read .nodeGroup.<field>: text/template resolves a lowercase name on a map
	// only, so the resolved NodeGroup is serialized here and nowhere else.
	nodeGroupValues := resolved.ToMap()
	checksum, err := machineclass.RenderChecksumForInstanceClass(checksumTemplate, resolved.InstanceClass, resolved.ManualRolloutID, cloudProvider)
	if err != nil {
		return fmt.Errorf("render checksum for NodeGroup %s: %w", ng.Name, err)
	}

	minReplicas, maxReplicas := getMinMax(ng)
	awsSpot := cloudType == "aws" && instanceClassSpot(resolved)

	desiredMDNames := make(map[string]struct{}, len(zones))
	desiredClassNames := make(map[string]struct{}, len(zones))

	for _, zone := range zones {
		hash := sha256Hash(clusterUUID + zone)
		machineClassName := fmt.Sprintf("%s-%s", ng.Name, hash)
		mdName := machineClassName
		if instancePrefix != "" {
			mdName = fmt.Sprintf("%s-%s", instancePrefix, machineClassName)
		}
		desiredMDNames[mdName] = struct{}{}
		desiredClassNames[machineClassName] = struct{}{}

		renderCtx := map[string]interface{}{
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
			"nodeGroup": nodeGroupValues,
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
		// The node-group label is what makes a MachineClass findable after its
		// MachineDeployment is gone — pruning and NodeGroup cleanup list by it.
		setNodeGroupLabel(machineClassObj, ng.Name)

		replicas, err := r.mcmDesiredReplicas(ctx, mdName, minReplicas, maxReplicas)
		if err != nil {
			return err
		}

		md := buildMCMMachineDeployment(mcmMachineDeploymentInput{
			resolved:         resolved,
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

	if _, err := r.pruneStaleMCMs(ctx, r.Client, ng.Name, machineClassKind, desiredMDNames, desiredClassNames); err != nil {
		return err
	}

	return nil
}

// pruneStaleMCMs deletes the MCM MachineDeployments of the NodeGroup that are no longer desired
// (a removed zone, or every one of them when desiredMDs is empty) and then the MachineClasses
// left without a deployment. It returns how many undesired MachineDeployments are still present:
// machine-controller-manager keeps them alive on its finalizer until the Machines are deleted,
// and a caller tearing the NodeGroup down must wait for that count to reach zero.
//
// Deletion order is the whole point. machine-controller-manager resolves the cloud credentials
// through MachineClass.spec.secretRef while it drains and deletes the Machines, so a class
// removed first orphans the cloud VMs (still billed) and wedges the Machines on their
// finalizers. A stale MachineClass is inert; an early deletion is not.
//
// reader picks where the MachineDeployment list comes from: the informer cache is fine while the
// NodeGroup lives, but the teardown path passes APIReader — dropping the finalizer on a stale
// "no deployments left" read is exactly the mistake this ordering exists to prevent.
func (r *MachineDeploymentReconciler) pruneStaleMCMs(ctx context.Context, reader client.Reader, ngName, machineClassKind string, desiredMDs, desiredClasses map[string]struct{}) (int, error) {
	logger := log.FromContext(ctx)

	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "machine.sapcloud.io", Version: "v1alpha1", Kind: "MachineDeploymentList",
	})
	if err := reader.List(ctx, list,
		client.InNamespace(common.MachineNamespace),
		client.MatchingLabels{"node-group": ngName},
	); err != nil {
		if meta.IsNoMatchError(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("list MCM MachineDeployments for NodeGroup %s: %w", ngName, err)
	}

	stale := 0
	inUse := make(map[string]struct{}, len(list.Items))
	for i := range list.Items {
		md := &list.Items[i]
		if _, ok := desiredMDs[md.GetName()]; ok {
			continue
		}
		stale++

		classKind, _, _ := unstructured.NestedString(md.Object, "spec", "template", "spec", "class", "kind")
		className, _, _ := unstructured.NestedString(md.Object, "spec", "template", "spec", "class", "name")
		if className != "" {
			inUse[className] = struct{}{}
			// MachineClasses rendered by helm before the migration carry no node-group label,
			// and this reference is the only way back to them once the MachineDeployment is
			// gone. Stamp the label while it is still readable.
			if err := r.adoptMachineClass(ctx, classKind, className, ngName); err != nil {
				return 0, err
			}
		}

		if !md.GetDeletionTimestamp().IsZero() {
			continue
		}
		if err := r.Client.Delete(ctx, md); err != nil && !errors.IsNotFound(err) {
			return 0, fmt.Errorf("delete MCM MachineDeployment %s: %w", md.GetName(), err)
		}
		logger.Info("deleted stale MCM MachineDeployment", "name", md.GetName(), "ng", ngName)
	}

	if machineClassKind == "" {
		return stale, nil
	}
	if err := r.deleteOrphanMachineClasses(ctx, ngName, machineClassKind, desiredClasses, inUse); err != nil {
		return 0, err
	}
	return stale, nil
}

// deleteOrphanMachineClasses removes the MachineClasses labelled with this NodeGroup that are
// neither wanted nor still referenced by a (possibly terminating) MachineDeployment.
func (r *MachineDeploymentReconciler) deleteOrphanMachineClasses(ctx context.Context, ngName, machineClassKind string, desired, inUse map[string]struct{}) error {
	logger := log.FromContext(ctx)

	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "machine.sapcloud.io", Version: "v1alpha1", Kind: machineClassKind + "List",
	})
	// APIReader: a cached list would start a cluster-wide informer for a provider-specific kind
	// nothing else reads.
	if err := r.APIReader.List(ctx, list,
		client.InNamespace(common.MachineNamespace),
		client.MatchingLabels{"node-group": ngName},
	); err != nil {
		if meta.IsNoMatchError(err) {
			return nil
		}
		return fmt.Errorf("list %s for NodeGroup %s: %w", machineClassKind, ngName, err)
	}

	for i := range list.Items {
		mc := &list.Items[i]
		if _, ok := desired[mc.GetName()]; ok {
			continue
		}
		if _, ok := inUse[mc.GetName()]; ok {
			continue
		}
		if err := r.Client.Delete(ctx, mc); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("delete MachineClass %s: %w", mc.GetName(), err)
		}
		logger.Info("deleted orphan MCM MachineClass", "name", mc.GetName(), "ng", ngName)
	}

	return nil
}

// adoptMachineClass stamps the node-group label on a MachineClass this NodeGroup owns but did
// not render itself (helm did, before the migration). A missing class is not an error.
func (r *MachineDeploymentReconciler) adoptMachineClass(ctx context.Context, machineClassKind, name, ngName string) error {
	if machineClassKind == "" {
		return nil
	}
	mc := newUnstructured("machine.sapcloud.io", "v1alpha1", machineClassKind)
	mc.SetName(name)
	mc.SetNamespace(common.MachineNamespace)
	patch := fmt.Appendf(nil, `{"metadata":{"labels":{"node-group":%q}}}`, ngName)
	err := r.Client.Patch(ctx, mc, client.RawPatch(types.MergePatchType, patch))
	if err != nil && !errors.IsNotFound(err) && !meta.IsNoMatchError(err) {
		return fmt.Errorf("label MachineClass %s: %w", name, err)
	}
	return nil
}

func setNodeGroupLabel(obj *unstructured.Unstructured, ngName string) {
	labels := obj.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels["node-group"] = ngName
	obj.SetLabels(labels)
}

func (r *MachineDeploymentReconciler) mcmDesiredReplicas(ctx context.Context, mdName string, minReplicas, maxReplicas int32) (int64, error) {
	existing := newUnstructured("machine.sapcloud.io", "v1alpha1", "MachineDeployment")
	// Read spec.replicas LIVE (APIReader), not from the informer cache — see the identical
	// reasoning in readExistingCAPIMachineDeployment. MCM is the more dangerous of the two engines here:
	// it has no mutating webhook to re-default the field, so a stale cached read that stomps
	// a fresh autoscaler value has no safety net. A live GET keeps the read-modify-write
	// window at microseconds.
	if err := r.APIReader.Get(ctx, types.NamespacedName{Name: mdName, Namespace: common.MachineNamespace}, existing); err != nil {
		if errors.IsNotFound(err) {
			return int64(minReplicas), nil
		}
		return 0, fmt.Errorf("get MCM MachineDeployment %s: %w", mdName, err)
	}
	current, found, err := unstructured.NestedInt64(existing.Object, "spec", "replicas")
	if err != nil {
		return 0, fmt.Errorf("read spec.replicas of MCM MachineDeployment %s: %w", mdName, err)
	}
	// Helm never rendered spec.replicas (the autoscaler owned it), so a pre-migration
	// MachineDeployment can exist without the field. Treat that as scale-from-zero
	// instead of erroring, otherwise the NodeGroup reconcile deadlocks on adoption.
	if !found {
		current = 0
	}
	return int64(calculateReplicas(int32(current), minReplicas, maxReplicas)), nil
}

func (r *MachineDeploymentReconciler) readCloudProviderTree(ctx context.Context) (map[string]interface{}, error) {
	secret := &corev1.Secret{}
	if err := r.Client.Get(ctx, types.NamespacedName{
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

func (r *MachineDeploymentReconciler) readPodSubnet(ctx context.Context) (string, error) {
	secret := &corev1.Secret{}
	if err := r.Client.Get(ctx, types.NamespacedName{
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

// instanceClassSpot reports the provider spot flag; only aws acts on it.
func instanceClassSpot(resolved derived_status.ResolvedNodeGroup) bool {
	instanceClass, _ := resolved.InstanceClass.(map[string]interface{})
	spot, _ := instanceClass["spot"].(bool)
	return spot
}
