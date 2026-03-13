/*
Copyright 2025 Flant JSC

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

package updateapproval

import (
	"context"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	ua "github.com/deckhouse/node-controller/internal/controller/updateapproval/common"
	"github.com/deckhouse/node-controller/internal/controller/updateapproval/engine"
	"github.com/deckhouse/node-controller/internal/controller/updateapproval/kubeclient"
	uametrics "github.com/deckhouse/node-controller/internal/controller/updateapproval/metrics"
	"github.com/deckhouse/node-controller/internal/register"
	"github.com/deckhouse/node-controller/internal/register/dynctrl"
)

func init() {
	uametrics.Register()
	register.RegisterController(register.NodeGroupUpdateApproval, &v1.NodeGroup{}, New())
}

type Reconciler struct {
	dynctrl.Base
	deckhouseNodeName string
}

func New() *Reconciler {
	return &Reconciler{
		deckhouseNodeName: os.Getenv("DECKHOUSE_NODE_NAME"),
	}
}

func (r *Reconciler) SetupWatches(w dynctrl.Watcher) {
	nodeHasGroupLabel := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		_, exists := obj.GetLabels()[ua.NodeGroupLabel]
		return exists
	})

	w.Watches(
		&corev1.Node{},
		handler.EnqueueRequestsFromMapFunc(r.nodeToNodeGroup),
		builder.WithPredicates(nodeHasGroupLabel),
	)
	w.Watches(
		&corev1.Secret{},
		handler.EnqueueRequestsFromMapFunc(r.secretToAllNodeGroups),
		builder.WithPredicates(predicate.NewPredicateFuncs(func(obj client.Object) bool {
			return obj.GetNamespace() == ua.MachineNamespace && obj.GetName() == ua.ConfigurationChecksumsSecretName
		})),
	)
}

func (r *Reconciler) nodeToNodeGroup(_ context.Context, obj client.Object) []reconcile.Request {
	node, ok := obj.(*corev1.Node)
	if !ok {
		return nil
	}

	ngName, exists := node.Labels[ua.NodeGroupLabel]
	if !exists {
		return nil
	}

	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: ngName}}}
}

func (r *Reconciler) secretToAllNodeGroups(ctx context.Context, _ client.Object) []reconcile.Request {
	ngList := &v1.NodeGroupList{}
	if err := r.Client.List(ctx, ngList); err != nil {
		log.FromContext(ctx).Error(err, "failed to list nodegroups for secret event")
		return nil
	}

	requests := make([]reconcile.Request, 0, len(ngList.Items))
	for _, ng := range ngList.Items {
		requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: ng.Name}})
	}

	return requests
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.V(1).Info("reconciling update approval", "nodegroup", req.Name)

	ng := &v1.NodeGroup{}
	if err := r.Client.Get(ctx, req.NamespacedName, ng); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get nodegroup %s: %w", req.Name, err)
	}

	kubeSvc := kubeclient.Client{Client: r.Client}
	engineSvc := engine.Processor{
		Kube:              kubeSvc,
		Recorder:          r.Recorder,
		DeckhouseNodeName: r.deckhouseNodeName,
	}

	checksums, err := kubeSvc.GetConfigurationChecksums(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}
	if len(checksums) == 0 {
		logger.V(1).Info("no configuration checksums secret found, skipping")
		return ctrl.Result{}, nil
	}
	ngChecksum := checksums[ng.Name]

	nodes, err := kubeSvc.GetNodesForNodeGroup(ctx, ng.Name)
	if err != nil {
		return ctrl.Result{}, err
	}

	nodeInfos := make([]ua.NodeInfo, 0, len(nodes))
	for _, node := range nodes {
		nodeInfos = append(nodeInfos, ua.BuildNodeInfo(&node))
	}

	for _, node := range nodeInfos {
		uametrics.SetNodeMetrics(node, ng, ngChecksum)
	}

	finished, err := engineSvc.ProcessUpdatedNodes(ctx, ng, nodeInfos, ngChecksum)
	if err != nil {
		return ctrl.Result{}, err
	}
	if finished {
		return ctrl.Result{}, nil
	}

	finished, err = engineSvc.ApproveDisruptions(ctx, ng, nodeInfos)
	if err != nil {
		return ctrl.Result{}, err
	}
	if finished {
		return ctrl.Result{}, nil
	}

	if _, err := engineSvc.ApproveUpdates(ctx, ng, nodeInfos); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}
