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
	cloudProvider, blob map[string]interface{},
	clusterUUID, podSubnet, zone, templateName, checksum string,
) error {
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
		"nodeGroup":             blob,
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
	if err := r.Client.List(ctx, tmplList,
		client.InNamespace(common.MachineNamespace),
		client.MatchingLabels{"node-group": ngName},
	); err != nil {
		return fmt.Errorf("list CAPI MachineTemplates for NodeGroup %s: %w", ngName, err)
	}
	for i := range tmplList.Items {
		tmpl := &tmplList.Items[i]
		if _, ok := desiredTemplates[tmpl.GetName()]; ok {
			continue
		}
		if !tmpl.GetDeletionTimestamp().IsZero() {
			continue
		}
		if err := r.Client.Delete(ctx, tmpl); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("delete stale CAPI MachineTemplate %s: %w", tmpl.GetName(), err)
		}
		logger.Info("pruned stale CAPI MachineTemplate", "name", tmpl.GetName(), "ng", ngName)
	}

	return nil
}
