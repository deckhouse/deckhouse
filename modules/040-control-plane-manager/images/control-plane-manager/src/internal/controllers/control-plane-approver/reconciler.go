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

package controlplaneapprover

import (
	"context"
	"fmt"
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/approver"
	"control-plane-manager/internal/constants"
)

// recheckInterval is how often Reconcile re-checks operations waiting for kube-apiserver versions,
// for their images or for missingOperationTimeout to run out. Only a Pod starting to run an image
// triggers a reconcile (ControlPlanePodPredicate); the config Secret is not watched.
const recheckInterval = 15 * time.Second

type reconciler struct {
	client client.Client
	// apiReader reads past the cache: ControlPlaneOperations, so that a decision sees the approvals
	// of the previous reconcile, and leader election Leases without a cluster-wide Lease informer.
	apiReader client.Reader
	approver  *approver.Approver
	// etcdLeader returns the etcd leader member's name, which is the name of its node.
	etcdLeader func(context.Context) (string, error)
}

func newReconciler(cl client.Client, apiReader client.Reader) *reconciler {
	return &reconciler{
		client:     cl,
		apiReader:  apiReader,
		approver:   approver.NewApprover(approver.StaticPodPipeline),
		etcdLeader: localEtcdLeader,
	}
}

func (r *reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	masterList := &corev1.NodeList{}
	if err := r.client.List(ctx, masterList, client.MatchingLabels{constants.ControlPlaneNodeLabelKey: ""}); err != nil {
		return reconcile.Result{}, err
	}

	arbiterList := &corev1.NodeList{}
	if err := r.client.List(ctx, arbiterList, client.MatchingLabels{constants.EtcdArbiterNodeLabelKey: ""}); err != nil {
		return reconcile.Result{}, err
	}

	nodes, masterNames, nodeNames := r.getReadyNodes(masterList, arbiterList)
	if nodes.IsZero() {
		log.FromContext(ctx).V(1).Info("no ready control plane nodes found")
		return reconcile.Result{}, nil
	}

	// A live list: the cache may still show an operation approved by the previous reconcile as
	// pending, and a decision on it could approve a second etcd restart.
	operations := &controlplanev1alpha1.ControlPlaneOperationList{}
	if err := r.apiReader.List(
		ctx, operations,
		client.InNamespace(constants.KubeSystemNamespace),
		operationsMatchingNodes(slices.Concat(nodeNamesOf(masterList), nodeNamesOf(arbiterList))),
	); err != nil {
		return reconcile.Result{}, err
	}
	// An approved operation on a NotReady node still holds its slot: its node may be restarting
	// the component right now. Only pending operations wait for their node to be Ready.
	ops := slices.DeleteFunc(operations.Items, func(op controlplanev1alpha1.ControlPlaneOperation) bool {
		return !op.Spec.Approved && !slices.Contains(nodeNames, op.Spec.NodeName)
	})
	if len(ops) == 0 {
		log.FromContext(ctx).V(1).Info("no control plane operations found")
		return reconcile.Result{}, nil
	}

	candidates, recheckAPIServers, err := r.holdBackVersionSkew(ctx, ops, masterNames)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to check kube-apiserver versions: %w", err)
	}

	cpnList := &controlplanev1alpha1.ControlPlaneNodeList{}
	if err := r.client.List(ctx, cpnList, client.InNamespace(constants.KubeSystemNamespace)); err != nil {
		return reconcile.Result{}, err
	}
	cp := controlPlane{
		readyMasters: masterNames,
		readyEtcd:    nodeNames,
		cpns:         cpnList.Items,
	}

	nodes.Leaders = r.leaderNodes(ctx, candidates)
	recheckLeaders := markWaitingLeaders(ctx, candidates, nodes.Leaders, cp, time.Now())

	approvable, recheckImages, err := r.holdBackUnpulledImages(ctx, r.approver.SelectApprovable(candidates, nodes))
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to check control plane images: %w", err)
	}
	for _, op := range approvable {
		original := op.DeepCopy()
		op.Spec.Approved = true

		if err := r.client.Patch(ctx, &op, client.MergeFrom(original)); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to approve ControlPlaneOperation %q: %w", op.Name, err)
		}
	}

	// recheckAPIServers: kube-controller-manager/kube-scheduler wait for updated kube-apiservers;
	// recheckLeaders: a leader waits for updated followers, so leadership moves only once;
	// recheckImages: an operation waits for its node to hold its images.
	if recheckAPIServers || recheckLeaders || recheckImages {
		return reconcile.Result{RequeueAfter: recheckInterval}, nil
	}

	return reconcile.Result{}, nil
}

// getReadyNodes returns the ready node counts, the ready master names and all ready node names.
func (r *reconciler) getReadyNodes(masterList *corev1.NodeList, arbiterList *corev1.NodeList) (approver.Nodes, []string, []string) {
	nodes := approver.Nodes{}
	var masterNames, arbiterNames []string

	for _, node := range masterList.Items {
		if isNodeReady(node) {
			nodes.Masters++
			masterNames = append(masterNames, node.Name)
		}
	}

	for _, node := range arbiterList.Items {
		if isNodeReady(node) {
			nodes.Arbiters++
			arbiterNames = append(arbiterNames, node.Name)
		}
	}

	return nodes, masterNames, slices.Concat(masterNames, arbiterNames)
}

func nodeNamesOf(list *corev1.NodeList) []string {
	names := make([]string, 0, len(list.Items))
	for _, node := range list.Items {
		names = append(names, node.Name)
	}
	return names
}

func operationsMatchingNodes(nodeNames []string) client.ListOption {
	requirement, err := labels.NewRequirement(constants.ControlPlaneNodeNameLabelKey, selection.In, nodeNames)
	if err != nil {
		return client.MatchingLabels{}
	}

	return client.MatchingLabelsSelector{Selector: labels.NewSelector().Add(*requirement)}
}

func isNodeReady(node corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			return condition.Status == corev1.ConditionTrue
		}
	}

	return false
}
