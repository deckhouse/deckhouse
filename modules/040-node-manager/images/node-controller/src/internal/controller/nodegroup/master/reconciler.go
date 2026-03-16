package master

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/dynr"
	"github.com/deckhouse/node-controller/internal/rcname"
)

const masterNodeGroupName = "master"

func init() {
	dynr.RegisterReconciler(rcname.NodeGroupMaster, &deckhousev1.NodeGroup{}, &NodeGroupMasterReconciler{})
}

var _ dynr.Reconciler = (*NodeGroupMasterReconciler)(nil)

type NodeGroupMasterReconciler struct {
	dynr.Base
}

func (r *NodeGroupMasterReconciler) SetupWatches(_ dynr.Watcher) {}

func (r *NodeGroupMasterReconciler) SetupForPredicates() []predicate.Predicate {
	return []predicate.Predicate{predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetName() == masterNodeGroupName
	})}
}

func (r *NodeGroupMasterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	existing := &deckhousev1.NodeGroup{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: masterNodeGroupName}, existing)
	if err == nil {
		return ctrl.Result{}, nil
	}
	if !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("get master node group: %w", err)
	}

	nodeType := detectMasterNodeType(ctx, r.Client)

	ng := buildMasterNodeGroup(nodeType)
	if err := r.Client.Create(ctx, ng); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("create master node group: %w", err)
	}

	log.Info("created master node group", "nodeType", nodeType)
	return ctrl.Result{}, nil
}

func detectMasterNodeType(ctx context.Context, c client.Client) deckhousev1.NodeType {
	secret := &corev1.Secret{}
	err := c.Get(ctx, client.ObjectKey{Namespace: "kube-system", Name: "d8-cluster-configuration"}, secret)
	if err != nil {
		return deckhousev1.NodeTypeCloudPermanent
	}

	clusterType := string(secret.Data["clusterType"])
	if clusterType == "Static" {
		return deckhousev1.NodeTypeStatic
	}

	return deckhousev1.NodeTypeCloudPermanent
}

func buildMasterNodeGroup(nodeType deckhousev1.NodeType) *deckhousev1.NodeGroup {
	return &deckhousev1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: masterNodeGroupName,
		},
		Spec: deckhousev1.NodeGroupSpec{
			NodeType: nodeType,
			Disruptions: &deckhousev1.DisruptionsSpec{
				ApprovalMode: deckhousev1.DisruptionApprovalModeManual,
			},
			NodeTemplate: &deckhousev1.NodeTemplate{
				Labels: map[string]string{
					"node-role.kubernetes.io/control-plane": "",
					"node-role.kubernetes.io/master":        "",
				},
				Taints: []corev1.Taint{
					{
						Key:    "node-role.kubernetes.io/control-plane",
						Effect: corev1.TaintEffectNoSchedule,
					},
				},
			},
		},
	}
}
