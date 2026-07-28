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

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	sigsyaml "sigs.k8s.io/yaml"

	"github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/machineclass"
)

// applyCAPIMachineTemplate renders the provider CAPI machine-template.yaml (from the
// cloud-provider secret) for a single zone and server-side-applies it. The template is
// named by the instance-class checksum, so its content stays byte-identical to helm's
// former output and existing nodes never roll.
func (r *MachineDeploymentReconciler) applyCAPIMachineTemplate(
	ctx context.Context,
	templateContent []byte,
	cloudProvider, nodeGroupValues map[string]interface{},
	clusterUUID, podSubnet, zone, templateName, checksum string,
) error {
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
		"nodeGroup":             nodeGroupValues,
		"zoneName":              zone,
		"templateName":          templateName,
		"instanceClassChecksum": checksum,
	}

	rendered, err := machineclass.RenderMachineClass(templateContent, renderCtx)
	if err != nil {
		return fmt.Errorf("render CAPI MachineTemplate %s: %w", templateName, err)
	}
	obj := map[string]interface{}{}
	if err := sigsyaml.Unmarshal(rendered, &obj); err != nil {
		return fmt.Errorf("parse rendered CAPI MachineTemplate %s: %w", templateName, err)
	}
	mt := &unstructured.Unstructured{Object: obj}
	if err := r.Client.Patch(ctx, mt, client.Apply, client.FieldOwner("node-controller"), client.ForceOwnership); err != nil {
		return fmt.Errorf("apply CAPI MachineTemplate %s: %w", templateName, err)
	}
	return nil
}

// pruneStaleCAPI deletes CAPI MachineDeployments and infrastructure MachineTemplates that
// belong to the NodeGroup but are no longer desired (e.g. after a zone is removed or the
// instance-class checksum changed). The bootstrap Secret is still helm-owned and pruned by
// helm, so it is not touched here.
func (r *MachineDeploymentReconciler) pruneStaleCAPI(
	ctx context.Context,
	ngName string,
	cloudConfig *cloudProviderConfig,
	desiredMDs, desiredTemplates map[string]struct{},
) error {
	logger := log.FromContext(ctx)

	mdList := &unstructured.UnstructuredList{}
	mdList.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "cluster.x-k8s.io", Version: "v1beta2", Kind: "MachineDeploymentList",
	})
	if err := r.Client.List(ctx, mdList,
		client.InNamespace(common.MachineNamespace),
		client.MatchingLabels{"node-group": ngName},
	); err != nil {
		return fmt.Errorf("list CAPI MachineDeployments for NodeGroup %s: %w", ngName, err)
	}
	for i := range mdList.Items {
		md := &mdList.Items[i]
		if _, ok := desiredMDs[md.GetName()]; ok {
			continue
		}
		if !md.GetDeletionTimestamp().IsZero() {
			continue
		}
		if err := r.Client.Delete(ctx, md); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("delete stale CAPI MachineDeployment %s: %w", md.GetName(), err)
		}
		logger.Info("pruned stale CAPI MachineDeployment", "name", md.GetName(), "ng", ngName)
	}

	gv, err := schema.ParseGroupVersion(cloudConfig.capiMachineTemplateAPIVersion)
	if err != nil {
		return fmt.Errorf("parse capiMachineTemplateAPIVersion %q: %w", cloudConfig.capiMachineTemplateAPIVersion, err)
	}
	tmplList := &unstructured.UnstructuredList{}
	tmplList.SetGroupVersionKind(schema.GroupVersionKind{
		Group: gv.Group, Version: gv.Version, Kind: cloudConfig.capiMachineTemplateKind + "List",
	})
	// Read live, like templatesInUse below: the kind comes from the provider Secret and is not
	// in cache.Options.ByObject, so a cached list would lazily start a cluster-wide informer for
	// it. It cannot be added to ByObject either — controller-runtime resolves a REST mapping for
	// every entry when the manager is built, so naming a provider kind whose CRD is absent (any
	// other cloud) would stop node-controller from starting.
	if err := r.APIReader.List(ctx, tmplList,
		client.InNamespace(common.MachineNamespace),
		client.MatchingLabels{"node-group": ngName},
	); err != nil {
		return fmt.Errorf("list CAPI MachineTemplates for NodeGroup %s: %w", ngName, err)
	}
	inUse, err := r.templatesInUse(ctx, ngName)
	if err != nil {
		return err
	}

	for i := range tmplList.Items {
		tmpl := &tmplList.Items[i]
		if _, ok := desiredTemplates[tmpl.GetName()]; ok {
			continue
		}
		if !tmpl.GetDeletionTimestamp().IsZero() {
			continue
		}
		// A rollout keeps the previous MachineSet alive until its Machines are replaced, and
		// that MachineSet still points at the old template. Deleting it mid-rollout leaves the
		// old MachineSet unable to create replacement Machines, so the rollout stalls
		// (kubernetes-sigs/cluster-api#6588 — the helm templates carried resource-policy: keep
		// for the same reason).
		if _, ok := inUse[tmpl.GetName()]; ok {
			logger.V(1).Info("keeping CAPI MachineTemplate still referenced by a MachineSet", "name", tmpl.GetName(), "ng", ngName)
			continue
		}
		if err := r.Client.Delete(ctx, tmpl); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("delete stale CAPI MachineTemplate %s: %w", tmpl.GetName(), err)
		}
		logger.Info("pruned stale CAPI MachineTemplate", "name", tmpl.GetName(), "ng", ngName)
	}

	return nil
}

// deleteInfraMachineTemplates removes every infrastructure MachineTemplate of a NodeGroup that is
// going away. Nothing else does: pruneStaleCAPI only runs while the NodeGroup still exists, and
// once it is gone no reconcile is left to clean up after it. Before the migration helm rendered
// these templates and pruned them when the NodeGroup left its values — set_keep_policy_on_capi_resources
// does not stamp them, so the prune worked — and taking the rendering over without taking the
// deletion over leaked one template per zone on every NodeGroup removal.
//
// Deleting them right away (rather than waiting for the Machines, as MCM MachineClasses must)
// matches what helm did: an infrastructure template is read when a Machine is created, never
// during its deletion.
func (r *MachineDeploymentReconciler) deleteInfraMachineTemplates(ctx context.Context, ngName string) error {
	logger := log.FromContext(ctx)

	cloudConfig, err := r.readCloudProviderConfig(ctx)
	if err != nil {
		return err
	}
	if cloudConfig.capiMachineTemplateKind == "" || cloudConfig.capiMachineTemplateAPIVersion == "" {
		return nil
	}
	gv, err := schema.ParseGroupVersion(cloudConfig.capiMachineTemplateAPIVersion)
	if err != nil {
		return fmt.Errorf("parse capiMachineTemplateAPIVersion %q: %w", cloudConfig.capiMachineTemplateAPIVersion, err)
	}

	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{
		Group: gv.Group, Version: gv.Version, Kind: cloudConfig.capiMachineTemplateKind + "List",
	})
	// Live read for the same reason as in pruneStaleCAPI: the kind comes from the provider Secret
	// and is not in cache.Options.ByObject.
	if err := r.APIReader.List(ctx, list,
		client.InNamespace(common.MachineNamespace),
		client.MatchingLabels{"node-group": ngName},
	); err != nil {
		if meta.IsNoMatchError(err) {
			return nil
		}
		return fmt.Errorf("list %s for NodeGroup %s: %w", cloudConfig.capiMachineTemplateKind, ngName, err)
	}

	for i := range list.Items {
		tmpl := &list.Items[i]
		if err := r.Client.Delete(ctx, tmpl); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("delete CAPI MachineTemplate %s: %w", tmpl.GetName(), err)
		}
		logger.V(1).Info("deleted CAPI MachineTemplate for removed NodeGroup", "name", tmpl.GetName(), "ng", ngName)
	}

	return nil
}

// templatesInUse returns the infrastructure MachineTemplate names the NodeGroup's MachineSets
// still reference. Read live: this runs only on the prune path, and a cached read would need a
// cluster-wide MachineSet informer for a rarely used check.
func (r *MachineDeploymentReconciler) templatesInUse(ctx context.Context, ngName string) (map[string]struct{}, error) {
	msList := &unstructured.UnstructuredList{}
	msList.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "cluster.x-k8s.io", Version: "v1beta2", Kind: "MachineSetList",
	})
	if err := r.APIReader.List(ctx, msList,
		client.InNamespace(common.MachineNamespace),
		client.MatchingLabels{"node-group": ngName},
	); err != nil {
		return nil, fmt.Errorf("list CAPI MachineSets for NodeGroup %s: %w", ngName, err)
	}

	inUse := make(map[string]struct{}, len(msList.Items))
	for i := range msList.Items {
		name, _, _ := unstructured.NestedString(msList.Items[i].Object,
			"spec", "template", "spec", "infrastructureRef", "name")
		if name != "" {
			inUse[name] = struct{}{}
		}
	}
	return inUse, nil
}
