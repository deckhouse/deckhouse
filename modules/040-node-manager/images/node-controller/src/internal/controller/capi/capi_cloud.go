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
	"bytes"
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	// Separate instances on purpose: the provider spec patch is deep-merged into spec below,
	// so a patch touching template.metadata.labels would otherwise write through the shared map
	// into the MachineDeployment's own metadata.labels — the node-group label every List
	// selector, prune and cleanup depends on.
	commonLabels := func() map[string]interface{} {
		return map[string]interface{}{
			"heritage":   "deckhouse",
			"module":     "node-manager",
			"node-group": in.ng.Name,
		}
	}

	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "cluster.x-k8s.io/v1beta2",
		"kind":       "MachineDeployment",
		"metadata": map[string]interface{}{
			"name":        in.mdName,
			"namespace":   common.MachineNamespace,
			"labels":      commonLabels(),
			"annotations": annotations,
		},
		"spec": map[string]interface{}{
			"clusterName": in.clusterName,
			"replicas":    int64(in.desired),
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": commonLabels(),
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

// existingCAPIMachineDeployment carries the parts of an already-created MachineDeployment that
// node-controller must take into account instead of overwriting blindly.
type existingCAPIMachineDeployment struct {
	// replicas is spec.replicas; hasReplicas is false when the field is absent, which is
	// legitimate (it was the autoscaler's field to own, and pre-migration objects may lack it).
	replicas    int32
	hasReplicas bool
	// bootstrapSecretName is spec.template.spec.bootstrap.dataSecretName as stored today.
	bootstrapSecretName string
}

// readExistingCAPIMachineDeployment returns the MachineDeployment as it currently exists, or nil
// when there is none yet.
//
// The read is LIVE (APIReader), never the informer cache, because both values it feeds are
// read-modify-write of fields a foreign controller changes:
//   - spec.replicas is owned by the cluster autoscaler. A cached read can lag its write by the
//     informer propagation delay (seconds under load), and re-applying that stale value would
//     stomp a fresh scale-up/down until the autoscaler retries.
//   - dataSecretName decides whether nodes roll (see resolveBootstrapSecretName), so it must
//     reflect the object as it really is.
//
// One read serves both: the previous code fetched the same object twice per zone.
func (r *MachineDeploymentReconciler) readExistingCAPIMachineDeployment(ctx context.Context, mdName string) (*existingCAPIMachineDeployment, error) {
	obj := newUnstructured("cluster.x-k8s.io", "v1beta2", "MachineDeployment")
	if err := r.APIReader.Get(ctx, types.NamespacedName{Name: mdName, Namespace: common.MachineNamespace}, obj); err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get CAPI MachineDeployment %s: %w", mdName, err)
	}

	out := &existingCAPIMachineDeployment{}
	replicas, found, err := unstructured.NestedInt64(obj.Object, "spec", "replicas")
	if err != nil {
		return nil, fmt.Errorf("read spec.replicas of CAPI MachineDeployment %s: %w", mdName, err)
	}
	if found {
		out.replicas, out.hasReplicas = int32(replicas), true
	}
	out.bootstrapSecretName, _, _ = unstructured.NestedString(obj.Object,
		"spec", "template", "spec", "bootstrap", "dataSecretName")
	return out, nil
}

// desiredReplicas clamps the replica count that is already in the cluster into [min,max]. A
// MachineDeployment that does not exist yet starts at min; one that exists without spec.replicas
// counts as zero (scale-from-zero) rather than an error, otherwise adopting such an object would
// deadlock the NodeGroup — mcmDesiredReplicas treats it the same way.
func desiredReplicas(existing *existingCAPIMachineDeployment, minReplicas, maxReplicas int32) int32 {
	if existing == nil {
		return minReplicas
	}
	current := int32(0)
	if existing.hasReplicas {
		current = existing.replicas
	}
	return calculateReplicas(current, minReplicas, maxReplicas)
}

// resolveBootstrapSecretName decides which bootstrap Secret the MachineDeployment must point at.
//
// Background: the Secret used to be named after the infrastructure MachineTemplate, i.e. with the
// instance-class checksum in it. That coupling was incidental — the template needs a
// checksum-derived name because CAPI infrastructure templates are immutable and a new name is
// what drives a rollout, while the Secret's content (cloud-init) does not depend on the instance
// class at all. It is now named per zone, so an instance-class change no longer churns it.
//
// The catch is existing clusters: dataSecretName lives in spec.template, so changing it makes
// CAPI create a new MachineSet and replace every node of the group. Rolling nodes on an upgrade
// that changes nothing the user asked for is not acceptable, so an existing MachineDeployment
// keeps the name it already carries, forever. Only MachineDeployments created from now on (new
// clusters, new zones) get the zone-based name; a migrated cluster therefore ends up with mixed
// names, which is the deliberate price of not rolling.
func (r *MachineDeploymentReconciler) resolveBootstrapSecretName(
	ctx context.Context,
	existing *existingCAPIMachineDeployment,
	rendered string,
) (string, error) {
	if existing == nil || existing.bootstrapSecretName == "" || existing.bootstrapSecretName == rendered {
		return rendered, nil
	}

	if err := r.mirrorBootstrapSecret(ctx, rendered, existing.bootstrapSecretName); err != nil {
		return "", err
	}
	return existing.bootstrapSecretName, nil
}

// mirrorBootstrapSecret copies the Secret helm renders under the current name onto the legacy
// name an adopted MachineDeployment still references.
//
// It is required, not cosmetic: the cloud-init inside the Secret embeds a bootstrap token that is
// rotated roughly every 4 hours (hooks/order_bootstrap_token.go), and helm only renders the
// current name. Without mirroring, the adopted Secret would keep a token that expires within
// hours and the next scale-up would create a Machine that cannot join the cluster.
//
// Both reads are live: the bootstrap Secret carries no app label, so it is outside the
// namespace-scoped Secret informer and a cached read would never find it.
func (r *MachineDeploymentReconciler) mirrorBootstrapSecret(ctx context.Context, from, to string) error {
	src := &corev1.Secret{}
	if err := r.APIReader.Get(ctx, types.NamespacedName{Name: from, Namespace: common.MachineNamespace}, src); err != nil {
		if errors.IsNotFound(err) {
			// helm has not rendered it yet; the adopted Secret still holds a usable token.
			return nil
		}
		return fmt.Errorf("get bootstrap secret %s: %w", from, err)
	}

	dst := &corev1.Secret{}
	err := r.APIReader.Get(ctx, types.NamespacedName{Name: to, Namespace: common.MachineNamespace}, dst)
	if errors.IsNotFound(err) {
		dst = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: to, Namespace: common.MachineNamespace, Labels: src.Labels},
			Type:       src.Type,
			Data:       src.Data,
		}
		if err := r.Client.Create(ctx, dst); err != nil && !errors.IsAlreadyExists(err) {
			return fmt.Errorf("create adopted bootstrap secret %s: %w", to, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("get adopted bootstrap secret %s: %w", to, err)
	}
	// Write only on a real difference: the token rotation is the only expected change, and an
	// unconditional update would bump resourceVersion on every reconcile of every zone.
	if secretDataEqual(dst.Data, src.Data) {
		return nil
	}
	dst.Data = src.Data
	dst.Type = src.Type
	if err := r.Client.Update(ctx, dst); err != nil {
		return fmt.Errorf("refresh adopted bootstrap secret %s: %w", to, err)
	}
	return nil
}

func secretDataEqual(a, b map[string][]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for k, av := range a {
		bv, ok := b[k]
		if !ok || !bytes.Equal(av, bv) {
			return false
		}
	}
	return true
}

func (r *MachineDeploymentReconciler) reconcileCloudMDsRendered(ctx context.Context, ng *deckhousev1.NodeGroup, rawSpec map[string]interface{}) error {
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

	ds := &derived_status.Service{Client: r.Client, Reader: r.APIReader}
	resolved, validationErr, err := ds.ResolveNodeGroup(ctx, ng, rawSpec)
	if err != nil {
		return fmt.Errorf("resolve NodeGroup %s: %w", ng.Name, err)
	}
	if validationErr != "" {
		logger.Info("skipping CAPI: NodeGroup failed validation", "nodeGroup", ng.Name, "error", validationErr)
		return nil
	}
	zones := resolved.Zones
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
	// The templates read .nodeGroup.<field>: text/template resolves a lowercase name on a map
	// only, so the resolved NodeGroup is serialized here and nowhere else.
	nodeGroupValues := resolved.ToMap()
	checksum, err := machineclass.RenderChecksumForInstanceClass(checksumTpl, resolved.InstanceClass, resolved.ManualRolloutID, cloudProvider)
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

	// Same falsy-zero rule as intOrDefault above: helm branched on
	// `{{- if $ng.nodeDrainTimeoutSecond }}`, so an explicit 0 kept the default.
	drainTimeout := 600
	if ng.Spec.NodeDrainTimeoutSecond != nil && *ng.Spec.NodeDrainTimeoutSecond > 0 {
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
		// helm renders the bootstrap Secret under a stable per-zone name, which mdSuffix
		// reproduces ($ng-$zone_hash). resolveBootstrapSecretName may override it to keep an
		// already-created MachineDeployment on its legacy name.
		existingMD, err := r.readExistingCAPIMachineDeployment(ctx, mdName)
		if err != nil {
			return err
		}
		bootstrapSecretName, err := r.resolveBootstrapSecretName(ctx, existingMD, mdSuffix)
		if err != nil {
			return err
		}

		if err := r.applyCAPIMachineTemplate(ctx, machineTemplateTpl, cloudProvider, nodeGroupValues, clusterUUID, podSubnet, zone, templateName, checksum); err != nil {
			return err
		}

		desired := desiredReplicas(existingMD, minReplicas, maxReplicas)

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
	// One map instance per placement — see buildCAPIMachineDeployment: a shared map lets any
	// later merge into spec write through into the object's own metadata.labels.
	labels := func() map[string]interface{} {
		return map[string]interface{}{
			"heritage":   "deckhouse",
			"module":     "node-manager",
			"node-group": ng.Name,
		}
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
			"labels":    labels(),
		},
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": labels(),
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

// applyMachineDeploymentSpecPatch merges the provider spec-patch into the MachineDeployment
// spec. ${var} placeholders are substituted AFTER the YAML parse, inside string values only:
// substituting into the raw text would let a zone name containing ":" or a leading "!" change
// the YAML structure, and the mis-typed value would land in spec.template and roll the group.
func applyMachineDeploymentSpecPatch(spec map[string]interface{}, rawPatch string, vars map[string]string) error {
	if strings.TrimSpace(rawPatch) == "" {
		return nil
	}

	patch := map[string]interface{}{}
	if err := sigsyaml.Unmarshal([]byte(rawPatch), &patch); err != nil {
		return fmt.Errorf("unmarshal spec patch: %w", err)
	}

	substituteInStrings(patch, vars)
	deepMergeMaps(spec, patch)
	return nil
}

// substituteInStrings replaces ${key} inside every string value of a parsed YAML tree.
// Keys are not substituted — a placeholder only makes sense as (part of) a value.
func substituteInStrings(node interface{}, vars map[string]string) {
	if len(vars) == 0 {
		return
	}
	switch v := node.(type) {
	case map[string]interface{}:
		for k, child := range v {
			if s, ok := child.(string); ok {
				v[k] = substituteVars(s, vars)
				continue
			}
			substituteInStrings(child, vars)
		}
	case []interface{}:
		for i, child := range v {
			if s, ok := child.(string); ok {
				v[i] = substituteVars(s, vars)
				continue
			}
			substituteInStrings(child, vars)
		}
	}
}

func substituteVars(s string, vars map[string]string) string {
	for k, val := range vars {
		s = strings.ReplaceAll(s, "${"+k+"}", val)
	}
	return s
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
