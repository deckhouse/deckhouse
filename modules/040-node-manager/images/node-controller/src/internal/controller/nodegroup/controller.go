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

package nodegroup

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	controllerregistry "github.com/deckhouse/node-controller/internal/controller"
	cloudstatus "github.com/deckhouse/node-controller/internal/controller/nodegroup/cloud_status"
	ngcommon "github.com/deckhouse/node-controller/internal/controller/nodegroup/common"
	ngconditions "github.com/deckhouse/node-controller/internal/controller/nodegroup/conditions"
	calcconditions "github.com/deckhouse/node-controller/internal/controller/nodegroup/conditionscalc"
	nodestatus "github.com/deckhouse/node-controller/internal/controller/nodegroup/node_status"
	processedstatus "github.com/deckhouse/node-controller/internal/controller/nodegroup/processed_status"
)

func init() {
	controllerregistry.Register("NodeGroup", "NodeGroupStatus", SetupNodeGroupStatus)
}

type NodeGroupStatusReconciler struct {
	Client   client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

func SetupNodeGroupStatus(mgr ctrl.Manager) error {
	return (&NodeGroupStatusReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("node-controller"),
	}).SetupWithManager(mgr)
}

func (r *NodeGroupStatusReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.NodeGroup{}).
		Watches(&corev1.Node{}, handler.EnqueueRequestsFromMapFunc(ngcommon.NodeToNodeGroup), builder.WithPredicates(ngcommon.NodeHasGroupLabelPredicate())).
		Watches(ngcommon.NewUnstructured(ngcommon.MCMMachineGVK), handler.EnqueueRequestsFromMapFunc(ngcommon.MachineToNodeGroup)).
		Watches(ngcommon.NewUnstructured(ngcommon.MCMMachineDeploymentGVK), handler.EnqueueRequestsFromMapFunc(ngcommon.MachineDeploymentToNodeGroup)).
		Watches(ngcommon.NewUnstructured(ngcommon.CAPIMachineGVK), handler.EnqueueRequestsFromMapFunc(ngcommon.MachineToNodeGroup)).
		Watches(ngcommon.NewUnstructured(ngcommon.CAPIMachineDeploymentGVK), handler.EnqueueRequestsFromMapFunc(ngcommon.MachineDeploymentToNodeGroup)).
		Named("nodegroup-status").
		Complete(r)
}

func (r *NodeGroupStatusReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.V(1).Info("reconciling nodegroup status", "name", req.Name)

	ng := &v1.NodeGroup{}
	if err := r.Client.Get(ctx, req.NamespacedName, ng); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	nodeService := nodestatus.Service{Client: r.Client}
	nodeResult, err := nodeService.Compute(ctx, ng.Name)
	if err != nil {
		logger.Error(err, "failed to get nodes")
		return ctrl.Result{}, err
	}

	cloudService := cloudstatus.Service{Client: r.Client}
	cloudResult := cloudService.Compute(ctx, ng)

	var conditionErrors []string
	if ng.Status.Error != "" {
		conditionErrors = append(conditionErrors, ng.Status.Error)
	}
	if cloudResult.LatestError != "" {
		conditionErrors = append(conditionErrors, cloudResult.LatestError)
	}

	eventMsg := fmt.Sprintf("%s %s", ng.Status.Error, cloudResult.LatestError)
	eventMsg = strings.TrimSpace(eventMsg)
	if len(eventMsg) > 1024 {
		eventMsg = eventMsg[:1024]
	}

	conditionService := ngconditions.Service{Recorder: r.Recorder}
	var statusMsg string
	if eventMsg != "" {
		conditionService.CreateEventIfChanged(ng, eventMsg)
		statusMsg = "Machine creation failed. Check events for details."
	}

	ngForConditions := calcconditions.NodeGroup{
		Type:                       ng.Spec.NodeType,
		Desired:                    cloudResult.Desired,
		Instances:                  cloudResult.Instances,
		HasFrozenMachineDeployment: cloudResult.IsFrozen,
	}
	existingConditions := ngcommon.ConvertToCalcConditions(ng.Status.Conditions)
	calculated := calcconditions.CalculateNodeGroupConditions(
		ngForConditions,
		nodeResult.NodesForConditions,
		existingConditions,
		conditionErrors,
		int(cloudResult.Min),
	)
	newConditions := ngcommon.ConvertFromCalcConditions(calculated)
	conditionSummary := ngconditions.CalculateConditionSummary(newConditions, statusMsg)

	patch := client.MergeFrom(ng.DeepCopy())
	ng.Status.Nodes = nodeResult.NodesCount
	ng.Status.Ready = nodeResult.ReadyCount
	ng.Status.UpToDate = nodeResult.UpToDateCount
	ng.Status.Conditions = newConditions
	ng.Status.ConditionSummary = conditionSummary

	if ng.Spec.NodeType == v1.NodeTypeCloudEphemeral {
		ng.Status.Desired = cloudResult.Desired
		ng.Status.Min = cloudResult.Min
		ng.Status.Max = cloudResult.Max
		ng.Status.Instances = cloudResult.Instances
		ng.Status.LastMachineFailures = ngcommon.EnsureNonNilMachineFailures(
			cloudstatus.ConvertMachineFailures(cloudResult.Failures),
		)
	} else {
		ng.Status.Desired = 0
		ng.Status.Min = 0
		ng.Status.Max = 0
		ng.Status.Instances = 0
		ng.Status.LastMachineFailures = nil
	}

	if err := r.Client.Status().Patch(ctx, ng, patch); err != nil {
		logger.Error(err, "failed to patch nodegroup status")
		return ctrl.Result{}, err
	}

	processedService := processedstatus.Service{Client: r.Client}
	if err := processedService.PatchProcessedStatus(ctx, ng.Name); err != nil {
		logger.Error(err, "failed to patch nodegroup processed status", "name", ng.Name)
	}

	logger.V(1).Info("updated nodegroup status", "name", ng.Name, "nodes", nodeResult.NodesCount, "ready", nodeResult.ReadyCount, "upToDate", nodeResult.UpToDateCount)
	return ctrl.Result{}, nil
}
