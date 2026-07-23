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

package nodegroup

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/machine.sapcloud.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	cloudstatus "github.com/deckhouse/node-controller/internal/controller/nodegroup/cloud_status"
	ngcommon "github.com/deckhouse/node-controller/internal/controller/nodegroup/common"
	ngconditions "github.com/deckhouse/node-controller/internal/controller/nodegroup/conditions"
	calcconditions "github.com/deckhouse/node-controller/internal/controller/nodegroup/conditionscalc"
	derivedstatus "github.com/deckhouse/node-controller/internal/controller/nodegroup/derived_status"
	nodestatus "github.com/deckhouse/node-controller/internal/controller/nodegroup/node_status"
	processedstatus "github.com/deckhouse/node-controller/internal/controller/nodegroup/processed_status"
	"github.com/deckhouse/node-controller/internal/register"
)

func init() {
	register.RegisterController("nodegroup-status", &v1.NodeGroup{}, &Status{})
}

type Status struct {
	register.Base
	apiReader        client.Reader
	conditionService ngconditions.Service
}

func (r *Status) Setup(mgr ctrl.Manager) error {
	r.apiReader = mgr.GetAPIReader()
	return nil
}

// ForPredicates: the status is derived from the NodeGroup spec plus Node/Machine/MD state
// (watched separately below); the controller's own status writes must not re-enqueue the
// NodeGroup — during a burst that echo multiplied reconciles ~40x per NodeGroup.
func (r *Status) ForPredicates() []predicate.Predicate {
	return []predicate.Predicate{predicate.Or(
		predicate.GenerationChangedPredicate{},
		predicate.AnnotationChangedPredicate{},
	)}
}

func (r *Status) SetupWatches(w register.Watcher) {
	w.Watches(&corev1.Node{}, handler.EnqueueRequestsFromMapFunc(ngcommon.NodeToNodeGroup), builder.WithPredicates(ngcommon.NodeHasGroupLabelPredicate()))
	w.Watches(&mcmv1alpha1.Machine{}, handler.EnqueueRequestsFromMapFunc(ngcommon.MachineToNodeGroup))
	w.Watches(ngcommon.NewUnstructured(ngcommon.MCMMachineDeploymentGVK), handler.EnqueueRequestsFromMapFunc(ngcommon.MachineDeploymentToNodeGroup))
	w.Watches(&capiv1beta2.Machine{}, handler.EnqueueRequestsFromMapFunc(ngcommon.MachineToNodeGroup))
	w.Watches(ngcommon.NewUnstructured(ngcommon.CAPIMachineDeploymentGVK), handler.EnqueueRequestsFromMapFunc(ngcommon.MachineDeploymentToNodeGroup))
	w.Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(r.secretToAllNodeGroups), builder.WithPredicates(nodecommon.ChecksumSecretPredicate()))
	w.Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(r.secretToAllNodeGroups), builder.WithPredicates(predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetNamespace() == "kube-system" && obj.GetName() == ngcommon.CloudProviderSecretName
	})))
}

func (r *Status) secretToAllNodeGroups(ctx context.Context, _ client.Object) []reconcile.Request {
	return nodecommon.SecretToAllNodeGroups(ctx, r.Client)
}

func (r *Status) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.V(1).Info("reconciling nodegroup status", "name", req.Name)

	ng, err := nodecommon.GetNodeGroup(ctx, r.Client, req.Name)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.V(1).Info("NodeGroup not found, skipping", "name", req.Name)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	logger.V(1).Info("computing node status", "nodeGroup", ng.Name, "nodeType", ng.Spec.NodeType)
	nodeService := nodestatus.Service{Client: r.Client}
	nodeResult, err := nodeService.Compute(ctx, ng.Name)
	if err != nil {
		logger.Error(err, "failed to compute node status", "nodeGroup", ng.Name)
		return ctrl.Result{}, err
	}

	cloudService := cloudstatus.Service{Client: r.Client}
	cloudResult := cloudService.Compute(ctx, ng)
	logger.V(1).Info("status computed",
		"nodeGroup", ng.Name,
		"nodes", nodeResult.NodesCount,
		"ready", nodeResult.ReadyCount,
		"upToDate", nodeResult.UpToDateCount,
		"desired", cloudResult.Desired,
		"instances", cloudResult.Instances,
	)

	ds := derivedstatus.Service{Client: r.Client, Reader: r.apiReader}
	derivedResult, validationResult, err := ds.ComputeWithCloudChecks(ctx, ng)
	if err != nil {
		logger.Error(err, "failed to compute derived nodegroup status", "nodeGroup", ng.Name)
		return ctrl.Result{}, err
	}
	validationError := validationResult.Error

	var conditionErrors []string
	if validationError != "" {
		conditionErrors = append(conditionErrors, validationError)
	}
	if cloudResult.LatestError != "" {
		conditionErrors = append(conditionErrors, cloudResult.LatestError)
	}

	eventMsg := fmt.Sprintf("%s %s", validationError, cloudResult.LatestError)
	eventMsg = strings.TrimSpace(eventMsg)
	if len(eventMsg) > 1024 {
		eventMsg = eventMsg[:1024]
	}

	r.conditionService.Recorder = r.Recorder
	var statusMsg string
	if eventMsg != "" {
		r.conditionService.CreateEventIfChanged(ng, eventMsg)
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
	statusBefore := ng.Status
	ng.Status.Nodes = nodeResult.NodesCount
	ng.Status.Ready = nodeResult.ReadyCount
	ng.Status.UpToDate = nodeResult.UpToDateCount
	ng.Status.Error = validationError
	ng.Status.KubernetesVersion = derivedResult.KubernetesVersion
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

	// Persist status.engine, which get_crds used to write and which the
	// MachineDeployment reconciler gates on. Only a definitive engine is
	// written: "None" (provider info absent yet, or genuinely engineless) is
	// left empty so a later reconcile — re-triggered by the cloud-provider
	// secret watch — can fill it, instead of getting stuck on a sticky "None".
	if ng.Status.Engine == "" {
		if engine := derivedResult.Engine; engine != "" && engine != "None" {
			ng.Status.Engine = engine
		}
	}

	if !apiequality.Semantic.DeepEqual(statusBefore, ng.Status) {
		if err := r.Client.Status().Patch(ctx, ng, patch); err != nil {
			if errors.IsConflict(err) {
				logger.Info("nodegroup status patch conflict, likely concurrent update", "name", ng.Name)
			}
			logger.Error(err, "failed to patch nodegroup status")
			return ctrl.Result{}, err
		}
		logger.V(1).Info("patched nodegroup status", "name", ng.Name)
	} else {
		logger.V(1).Info("nodegroup status unchanged, patch skipped", "name", ng.Name)
	}

	processedService := processedstatus.Service{Client: r.Client}
	if err := processedService.PatchProcessedStatus(ctx, ng.Name); err != nil {
		logger.Error(err, "failed to patch nodegroup processed status", "name", ng.Name)
	}

	logger.V(1).Info("updated nodegroup status", "name", ng.Name, "nodes", nodeResult.NodesCount, "ready", nodeResult.ReadyCount, "upToDate", nodeResult.UpToDateCount)
	return ctrl.Result{}, nil
}
