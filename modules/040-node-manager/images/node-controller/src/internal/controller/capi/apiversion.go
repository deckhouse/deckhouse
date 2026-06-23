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

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	"github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/register"
)

const capiInfraAPIGroup = "infrastructure.cluster.x-k8s.io"

var machineTemplateAPIGroups = map[string]string{
	"DeckhouseMachineTemplate":   capiInfraAPIGroup,
	"DynamixMachineTemplate":     capiInfraAPIGroup,
	"HuaweiCloudMachineTemplate": capiInfraAPIGroup,
	"StaticMachineTemplate":      capiInfraAPIGroup,
	"VCDMachineTemplate":         capiInfraAPIGroup,
	"ZvirtMachineTemplate":       capiInfraAPIGroup,
}

var machineAPIGroups = map[string]string{
	"DeckhouseMachine":   capiInfraAPIGroup,
	"DynamixMachine":     capiInfraAPIGroup,
	"HuaweiCloudMachine": capiInfraAPIGroup,
	"StaticMachine":      capiInfraAPIGroup,
	"VCDMachine":         capiInfraAPIGroup,
	"ZvirtMachine":       capiInfraAPIGroup,
}

var clusterInfraAPIGroups = map[string]string{
	"DeckhouseCluster":   capiInfraAPIGroup,
	"DynamixCluster":     capiInfraAPIGroup,
	"HuaweiCloudCluster": capiInfraAPIGroup,
	"StaticCluster":      capiInfraAPIGroup,
	"VCDCluster":         capiInfraAPIGroup,
	"ZvirtCluster":       capiInfraAPIGroup,
}

var controlPlaneAPIGroups = map[string]string{
	"DeckhouseControlPlane": capiInfraAPIGroup,
}

func init() {
	register.RegisterController("capi-api-version", &capiv1beta2.MachineDeployment{}, &APIVersionReconciler{})
}

// APIVersionReconciler patches empty apiGroup on infra refs of CAPI resources.
type APIVersionReconciler struct {
	register.Base
}

func (r *APIVersionReconciler) SetupWatches(w register.Watcher) {
	w.Watches(&capiv1beta2.Machine{}, handler.EnqueueRequestsFromMapFunc(r.machineToMD))

	clusterObj := newUnstructured("cluster.x-k8s.io", "v1beta2", "Cluster")
	w.Watches(clusterObj, handler.EnqueueRequestsFromMapFunc(
		func(_ context.Context, obj client.Object) []reconcile.Request {
			return []reconcile.Request{{NamespacedName: types.NamespacedName{
				Name:      "cluster:" + obj.GetName(),
				Namespace: obj.GetNamespace(),
			}}}
		},
	))
}

func (r *APIVersionReconciler) machineToMD(_ context.Context, obj client.Object) []reconcile.Request {
	ng, ok := obj.GetLabels()["node-group"]
	if !ok || ng == "" {
		return nil
	}
	mdList := &capiv1beta2.MachineDeploymentList{}
	if err := r.Client.List(context.Background(), mdList,
		client.InNamespace(common.MachineNamespace),
		client.MatchingLabels{"node-group": ng},
	); err != nil {
		return nil
	}
	var reqs []reconcile.Request
	for i := range mdList.Items {
		reqs = append(reqs, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(&mdList.Items[i])})
	}
	return reqs
}

func (r *APIVersionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if isClusterRequest(req.Name) {
		return r.reconcileCluster(ctx, req)
	}

	md := &capiv1beta2.MachineDeployment{}
	if err := r.Client.Get(ctx, req.NamespacedName, md); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get MachineDeployment: %w", err)
	}

	ref := &md.Spec.Template.Spec.InfrastructureRef
	if ref.Kind != "" && ref.APIGroup == "" {
		expected, ok := machineTemplateAPIGroups[ref.Kind]
		if ok {
			patch := client.MergeFrom(md.DeepCopy())
			ref.APIGroup = expected
			if err := r.Client.Patch(ctx, md, patch); err != nil {
				return ctrl.Result{}, fmt.Errorf("patch MachineDeployment infrastructureRef.apiGroup: %w", err)
			}
			logger.Info("patched MachineDeployment infrastructureRef.apiGroup", "name", md.Name, "apiGroup", expected)
		} else {
			logger.Info("unknown infra template kind", "name", md.Name, "kind", ref.Kind)
		}
	}

	machineList := &capiv1beta2.MachineList{}
	if err := r.Client.List(ctx, machineList,
		client.InNamespace(req.Namespace),
		client.MatchingLabels{"node-group": md.Labels["node-group"]},
	); err != nil {
		return ctrl.Result{}, fmt.Errorf("list Machines: %w", err)
	}
	for i := range machineList.Items {
		m := &machineList.Items[i]
		mRef := &m.Spec.InfrastructureRef
		if mRef.Kind != "" && mRef.APIGroup == "" {
			expected, ok := machineAPIGroups[mRef.Kind]
			if !ok {
				logger.Info("unknown infra machine kind", "machine", m.Name, "kind", mRef.Kind)
				continue
			}
			patch := client.MergeFrom(m.DeepCopy())
			mRef.APIGroup = expected
			if err := r.Client.Patch(ctx, m, patch); err != nil {
				return ctrl.Result{}, fmt.Errorf("patch Machine %s infrastructureRef.apiGroup: %w", m.Name, err)
			}
			logger.Info("patched Machine infrastructureRef.apiGroup", "name", m.Name, "apiGroup", expected)
		}
	}

	return ctrl.Result{}, nil
}

func isClusterRequest(name string) bool {
	return len(name) > 8 && name[:8] == "cluster:"
}

func (r *APIVersionReconciler) reconcileCluster(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	clusterName := req.Name[8:]

	cluster := newUnstructured("cluster.x-k8s.io", "v1beta2", "Cluster")
	if err := r.Client.Get(ctx, types.NamespacedName{Name: clusterName, Namespace: req.Namespace}, cluster); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get Cluster: %w", err)
	}

	patched := false
	original := cluster.DeepCopy()

	infraKind, _, _ := unstructured.NestedString(cluster.Object, "spec", "infrastructureRef", "kind")
	infraAPIGroup, _, _ := unstructured.NestedString(cluster.Object, "spec", "infrastructureRef", "apiGroup")
	if infraKind != "" && infraAPIGroup == "" {
		if expected, ok := clusterInfraAPIGroups[infraKind]; ok {
			_ = unstructured.SetNestedField(cluster.Object, expected, "spec", "infrastructureRef", "apiGroup")
			patched = true
			logger.Info("setting Cluster infrastructureRef.apiGroup", "cluster", clusterName, "apiGroup", expected)
		}
	}

	cpKind, _, _ := unstructured.NestedString(cluster.Object, "spec", "controlPlaneRef", "kind")
	cpAPIGroup, _, _ := unstructured.NestedString(cluster.Object, "spec", "controlPlaneRef", "apiGroup")
	if cpKind != "" && cpAPIGroup == "" {
		if expected, ok := controlPlaneAPIGroups[cpKind]; ok {
			_ = unstructured.SetNestedField(cluster.Object, expected, "spec", "controlPlaneRef", "apiGroup")
			patched = true
			logger.Info("setting Cluster controlPlaneRef.apiGroup", "cluster", clusterName, "apiGroup", expected)
		}
	}

	if patched {
		if err := r.Client.Patch(ctx, cluster, client.MergeFrom(original)); err != nil {
			return ctrl.Result{}, fmt.Errorf("patch Cluster refs: %w", err)
		}
	}

	return ctrl.Result{}, nil
}
