package instanceclass

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/dynr"
	"github.com/deckhouse/node-controller/internal/rcname"
)

func init() {
	dynr.RegisterReconciler(rcname.NodeGroupInstanceClass, &deckhousev1.NodeGroup{}, &NodeGroupInstanceClassReconciler{})
}

var _ dynr.Reconciler = (*NodeGroupInstanceClassReconciler)(nil)

var kindToAPIVersion = map[string]string{
	"vcdinstanceclass":         "deckhouse.io/v1",
	"zvirtinstanceclass":       "deckhouse.io/v1",
	"dynamixinstanceclass":     "deckhouse.io/v1",
	"huaweicloudinstanceclass": "deckhouse.io/v1",
	"dvpinstanceclass":         "deckhouse.io/v1alpha1",
}

type NodeGroupInstanceClassReconciler struct {
	dynr.Base
}

func (r *NodeGroupInstanceClassReconciler) SetupWatches(_ dynr.Watcher) {}

func (r *NodeGroupInstanceClassReconciler) SetupForPredicates() []predicate.Predicate {
	return []predicate.Predicate{predicate.NewPredicateFuncs(func(obj client.Object) bool {
		ng, ok := obj.(*deckhousev1.NodeGroup)
		if !ok {
			return false
		}
		return ng.Spec.NodeType == deckhousev1.NodeTypeCloudEphemeral
	})}
}

func (r *NodeGroupInstanceClassReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	ng := &deckhousev1.NodeGroup{}
	if err := r.Client.Get(ctx, req.NamespacedName, ng); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if ng.Spec.CloudInstances == nil {
		return ctrl.Result{}, nil
	}

	ref := ng.Spec.CloudInstances.ClassReference
	if ref.Kind == "" || ref.Name == "" {
		return ctrl.Result{}, nil
	}

	ngList := &deckhousev1.NodeGroupList{}
	if err := r.Client.List(ctx, ngList); err != nil {
		return ctrl.Result{}, fmt.Errorf("list node groups: %w", err)
	}

	consumers := collectConsumers(ngList.Items, ref)

	if err := r.patchInstanceClassStatus(ctx, ref, consumers); err != nil {
		return ctrl.Result{}, fmt.Errorf("patch instance class %s/%s status: %w", ref.Kind, ref.Name, err)
	}

	log.V(1).Info("reconciled instance class usage", "kind", ref.Kind, "name", ref.Name, "consumers", consumers)
	return ctrl.Result{}, nil
}

func collectConsumers(nodeGroups []deckhousev1.NodeGroup, ref deckhousev1.ClassReference) []string {
	var consumers []string
	for i := range nodeGroups {
		ng := &nodeGroups[i]
		if ng.Spec.NodeType != deckhousev1.NodeTypeCloudEphemeral || ng.Spec.CloudInstances == nil {
			continue
		}
		ngRef := ng.Spec.CloudInstances.ClassReference
		if ngRef.Kind == ref.Kind && ngRef.Name == ref.Name {
			consumers = append(consumers, ng.Name)
		}
	}
	return consumers
}

func (r *NodeGroupInstanceClassReconciler) patchInstanceClassStatus(ctx context.Context, ref deckhousev1.ClassReference, consumers []string) error {
	apiVersion := resolveAPIVersion(ref.Kind)
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return fmt.Errorf("parse group version %s: %w", apiVersion, err)
	}

	ic := &unstructured.Unstructured{}
	ic.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   gv.Group,
		Version: gv.Version,
		Kind:    ref.Kind,
	})

	if err := r.Client.Get(ctx, client.ObjectKey{Name: ref.Name}, ic); err != nil {
		return client.IgnoreNotFound(err)
	}

	patch := &unstructured.Unstructured{}
	patch.SetGroupVersionKind(ic.GroupVersionKind())
	patch.SetName(ic.GetName())
	if err := unstructured.SetNestedStringSlice(patch.Object, consumers, "status", "nodeGroupConsumers"); err != nil {
		return fmt.Errorf("set nested string slice: %w", err)
	}

	return r.Client.Status().Patch(ctx, ic, client.Apply, client.FieldOwner("node-controller"), client.ForceOwnership)
}

func resolveAPIVersion(kind string) string {
	if v, ok := kindToAPIVersion[strings.ToLower(kind)]; ok {
		return v
	}
	return "deckhouse.io/v1"
}
