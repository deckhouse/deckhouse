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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	sigsyaml "sigs.k8s.io/yaml"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/bootstrap"
	"github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/controller/bootstrapsecrets"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/bashiblecontext"
	ngcommon "github.com/deckhouse/node-controller/internal/controller/nodegroup/common"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/derived_status"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/machineclass"
)

func (r *MachineDeploymentReconciler) reconcileCloudMCMs(
	ctx context.Context, ng *deckhousev1.NodeGroup, resolved derived_status.ResolvedNodeGroup, validationErr string,
) error {
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
	checksum, err := machineclass.RenderChecksum(checksumTemplate, nodeGroupValues, cloudProvider)
	if err != nil {
		return fmt.Errorf("render checksum for NodeGroup %s: %w", ng.Name, err)
	}

	// Rendered once for the whole group: bootstrap.Input carries no zone, so every zone
	// would get the same bytes. Before the loop, so a cluster read that fails cannot leave
	// part of the zones applied.
	userData, err := r.machineClassUserData(ctx, resolved)
	if err != nil {
		return err
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

		// Before the MachineClass: machine-controller-manager resolves the credentials and
		// the cloud-init through secretRef, so a class applied first is a class it cannot act on.
		if err := r.applyMachineClassSecret(ctx, ng.Name, cloudType, machineClassName, userData, renderCtx); err != nil {
			return err
		}
		if err := r.Client.Patch(ctx, machineClassObj, client.Apply, client.FieldOwner("node-controller"), client.ForceOwnership); err != nil {
			return fmt.Errorf("apply MachineClass %s: %w", machineClassName, err)
		}
		if err := r.Client.Patch(ctx, md, client.Apply, client.FieldOwner("node-controller"), client.ForceOwnership); err != nil {
			return fmt.Errorf("apply MCM MachineDeployment %s: %w", mdName, err)
		}
		logger.Info("applied MCM secret + MachineClass + MachineDeployment", "name", mdName, "zone", zone)
	}

	if _, err := r.pruneStaleMCMs(ctx, r.Client, ng.Name, machineClassKind, desiredMDNames, desiredClassNames); err != nil {
		return err
	}

	return nil
}

// mcmConfigTemplateKey is the provider's own half of the machine-class Secret: the cloud
// credentials machine-controller-manager authenticates with, as key/base64 pairs.
const mcmConfigTemplateKey = "config-for-machine-controller-manager.yaml"

// machineClassUserData renders the cloud-init every machine of the NodeGroup boots from —
// the userData of its machine-class Secret.
func (r *MachineDeploymentReconciler) machineClassUserData(ctx context.Context, resolved derived_status.ResolvedNodeGroup) ([]byte, error) {
	// machine-controller-manager replaces this literal with a token it orders per machine
	// (pkg/util/provider/machinecontroller/userdata.go:29), so no token is minted here.
	in, err := bootstrapsecrets.BuildInput(ctx,
		&bashiblecontext.Service{Client: r.Client, Reader: r.APIReader}, resolved, "<<BOOTSTRAP_TOKEN>>")
	if err != nil {
		return nil, err
	}
	userData, err := bootstrap.RenderCloudConfig(in)
	if err != nil {
		return nil, fmt.Errorf("render cloud-config for NodeGroup %s: %w", resolved.Name, err)
	}
	return userData, nil
}

// applyMachineClassSecret writes the Secret every MCM MachineClass points its secretRef at:
// the given cloud-init in userData plus the cloud credentials the provider renders from
// config-for-machine-controller-manager.yaml. Port of the helm define
// "node_group_machine_class_secret" (templates/node-group/_machine_class_secret.tpl).
//
// The name is the contract with the provider templates, which compute it themselves:
// <ng>-<sha256(clusterUUID+zone)[:8]>, the same name the MachineClass carries.
//
// A Secret whose zone was removed is left behind, the way the CAPI bootstrap Secret is
// (pruneStaleCAPI): CollectOrphanedSecrets only takes Secrets whose NodeGroup is gone.
// Inert — the name is deterministic, so re-adding the zone overwrites it.
func (r *MachineDeploymentReconciler) applyMachineClassSecret(
	ctx context.Context, ngName, cloudType, secretName string, userData []byte, renderCtx map[string]interface{},
) error {
	configTemplate, err := r.readProviderTemplate(ctx, cloudType, engineMCMTemplates, mcmConfigTemplateKey)
	if err != nil {
		return fmt.Errorf("read %s for NodeGroup %s: %w", mcmConfigTemplateKey, ngName, err)
	}
	config, err := machineclass.RenderMachineClass(configTemplate, renderCtx)
	if err != nil {
		return fmt.Errorf("render %s for NodeGroup %s: %w", mcmConfigTemplateKey, ngName, err)
	}
	// The fragment is what helm nindented under `data:`, so its values are base64 and the
	// apiserver decoded them. Unmarshalling into []byte undoes that encoding, which the
	// client then redoes — a raw copy would hand the provider a double-encoded credential.
	data := map[string][]byte{}
	if err := sigsyaml.Unmarshal(config, &data); err != nil {
		return fmt.Errorf("parse rendered %s for NodeGroup %s: %w", mcmConfigTemplateKey, ngName, err)
	}
	data["userData"] = userData

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
		ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: common.MachineNamespace, Labels: map[string]string{
			"heritage": "deckhouse",
			"module":   "node-manager",
			// helm never set it. It is what a prune of the Secret left by a removed zone
			// would have to select on, the way pruneStaleMCMs selects MachineClasses.
			ngcommon.MachineDeploymentNodeGroupLabel: ngName,
		}},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}
	if err := r.Client.Patch(ctx, secret, client.Apply, client.FieldOwner("node-controller"), client.ForceOwnership); err != nil {
		return fmt.Errorf("apply MachineClass secret %s: %w", secretName, err)
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
		client.MatchingLabels{ngcommon.MachineDeploymentNodeGroupLabel: ngName},
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
	if err := r.deleteOrphanSecrets(ctx, ngName, desiredClasses, inUse); err != nil {
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
		client.MatchingLabels{ngcommon.MachineDeploymentNodeGroupLabel: ngName},
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

// deleteOrphanSecrets deletes the Secrets this module wrote for the NodeGroup that are no
// longer wanted: the machine-class Secret of a removed zone, and on the teardown pass, where
// nothing is desired, every one of them. The desired/in-use filter is the MachineClasses',
// because machine-controller-manager resolves the credentials through secretRef while it drains.
//
// The teardown pass runs from the first reconcile that sees a deletionTimestamp
// (machinedeployment.go:301), so a Static group's manual-bootstrap Secret goes here before
// CollectOrphanedSecrets would take it. Nothing reads it by then: CAPS reads bootstrap.sh only
// to bootstrap a node, and tears one down by running /var/lib/bashible/cleanup_static_node.sh
// on the node itself (caps-controller-manager/src/internal/client/cleanup.go:124).
//
// The selection is stricter than deleteOrphanMachineClasses', which lists by node-group alone:
// heritage and module are required here, and that is what keeps a hand-made Secret of the same
// name safe. The cost is the mirror image of adoptMachineClass — a helm-era machine-class Secret
// of a zone removed before the upgrade carries no node-group label, no MachineDeployment points
// at it, and nothing can adopt it. It leaks permanently and must be removed by hand.
func (r *MachineDeploymentReconciler) deleteOrphanSecrets(ctx context.Context, ngName string, desired, inUse map[string]struct{}) error {
	logger := log.FromContext(ctx)

	// Metadata-only: this runs on every pass of every MCM NodeGroup, and a machine-class
	// Secret body is the rendered cloud-init plus the cloud credentials.
	list := &metav1.PartialObjectMetadataList{}
	list.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("SecretList"))
	// APIReader: these Secrets carry no app label, so the namespace-scoped Secret informer
	// holds none of them and a cached list would come back empty.
	//
	// Both module labels: a Secret an operator wrote by hand carries neither, and is not
	// this controller's to delete.
	if err := r.APIReader.List(ctx, list,
		client.InNamespace(common.MachineNamespace),
		client.MatchingLabels{
			"heritage":                               "deckhouse",
			"module":                                 "node-manager",
			ngcommon.MachineDeploymentNodeGroupLabel: ngName,
		},
	); err != nil {
		return fmt.Errorf("list secrets of NodeGroup %s: %w", ngName, err)
	}

	for i := range list.Items {
		secret := &list.Items[i]
		if _, ok := desired[secret.Name]; ok {
			continue
		}
		if _, ok := inUse[secret.Name]; ok {
			continue
		}
		gone := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secret.Name, Namespace: secret.Namespace}}
		if err := r.Client.Delete(ctx, gone); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("delete orphan secret %s: %w", secret.Name, err)
		}
		logger.Info("deleted orphan secret of NodeGroup", "name", secret.Name, "ng", ngName)
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
	patch := fmt.Appendf(nil, `{"metadata":{"labels":{%q:%q}}}`, ngcommon.MachineDeploymentNodeGroupLabel, ngName)
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
	labels[ngcommon.MachineDeploymentNodeGroupLabel] = ngName
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

// readCloudProviderRegistration reads the same Secret as readCloudProviderTree, but typed. The
// tree stays for the template render context, which needs the provider's own subtree verbatim.
func (r *MachineDeploymentReconciler) readCloudProviderRegistration(ctx context.Context) (derived_status.CloudProviderRegistration, error) {
	secret := &corev1.Secret{}
	if err := r.Client.Get(ctx, types.NamespacedName{
		Name: cloudProviderSecretName, Namespace: cloudProviderSecretNamespace,
	}, secret); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return derived_status.CloudProviderRegistration{}, nil
		}
		return derived_status.CloudProviderRegistration{}, fmt.Errorf("get cloud-provider secret: %w", err)
	}
	return derived_status.DecodeRegistration(secret.Data), nil
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
	instanceClass := resolved.InstanceClass
	spot, _ := instanceClass["spot"].(bool)
	return spot
}
