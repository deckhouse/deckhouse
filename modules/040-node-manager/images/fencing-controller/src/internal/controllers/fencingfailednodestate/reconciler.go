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

package fencingfailednodestate

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-controller/internal/common"
)

// Reconciler observes FencingFailedNodeState without changing cluster objects.
type Reconciler struct {
	client client.Client
}

func New(c client.Client) *Reconciler {
	return &Reconciler{client: c}
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named(common.ControllerName).
		For(&v1alpha1.FencingFailedNodeState{}).
		Complete(r)
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	var state v1alpha1.FencingFailedNodeState

	if err := r.client.Get(ctx, req.NamespacedName, &state); err != nil {
		if apierrors.IsNotFound(err) {
			// A missing CR means the Node has no active fencing signal.
			logger.Info("fencingfailednodestate is gone, node has no active fencing signal", "node", req.Name)

			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("get fencingfailednodestate %q: %w", req.Name, err)
	}

	logger.Info("observed fencingfailednodestate", observedFields(&state)...)

	return ctrl.Result{}, nil
}

func observedFields(state *v1alpha1.FencingFailedNodeState) []any {
	fields := []any{
		"node", state.Name,
		"node_group", state.Spec.NodeGroup,
		"profile", string(state.Spec.ProfileRef.Name),
		"phase", string(state.Status.Phase),
		"generation", state.Generation,
		"observed_generation", state.Status.ObservedGeneration,
		"resource_version", state.ResourceVersion,
	}

	if failed := state.Status.Failed; failed != nil {
		fields = append(fields,
			"failed_detected_at", formatTime(&failed.DetectedAt),
			"failed_detected_by", failed.DetectedBy,
			"failed_reason", string(failed.Reason),
			"failed_alive_count", failed.AliveCount,
			"failed_quorum_size", failed.QuorumSize,
		)
	}

	if fallback := state.Status.Fallback; fallback != nil {
		fields = append(fields,
			"fallback_active", fallback.Active,
			"fallback_last_heartbeat_at", formatTime(fallback.LastHeartbeatAt),
			"fallback_quorum_lost_at", formatTime(fallback.QuorumLostAt),
			"fallback_heartbeat_interval_seconds", fallback.HeartbeatIntervalSeconds,
		)
	}

	if deletedAt := state.DeletionTimestamp; deletedAt != nil {
		fields = append(fields, "deletion_timestamp", formatTime(deletedAt))
	}

	return fields
}

func formatTime(t *metav1.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}

	return t.UTC().Format(time.RFC3339)
}
