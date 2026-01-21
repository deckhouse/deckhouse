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

package controller

import (
	"context"
	"fmt"
	"update-observer/cluster"
	"update-observer/common"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *reconciler) getClusterState(ctx context.Context) (*cluster.State, error) {
	cfg, err := r.getClusterConfiguration(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster configuration: %w", err)
	}

	nodesState, err := r.getNodesState(ctx, cfg.DesiredVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes state: %w", err)
	}

	controlPlaneState, err := r.getControlPlaneState(ctx, cfg.DesiredVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to get control plane state: %w", err)
	}

	return combine(cfg, nodesState, controlPlaneState), nil
}

func (r *reconciler) getClusterConfiguration(ctx context.Context) (*cluster.Configuration, error) {
	secret := &corev1.Secret{}
	err := r.client.Get(ctx, client.ObjectKey{
		Name:      common.SecretName,
		Namespace: common.KubeSystemNamespace,
	}, secret)
	if err != nil {
		return nil, common.WrapIntoReconcileTolerantError(err, "failed to get secret")
	}

	return cluster.GetConfiguration(secret)
}

func (r *reconciler) getNodesState(ctx context.Context, desiredVersion string) (*cluster.NodesState, error) {
	var continueToken string

	var nodes []corev1.Node
	for {
		list := &corev1.NodeList{}
		err := r.client.List(ctx, list, &client.ListOptions{
			Limit:    nodeListPageSize,
			Continue: continueToken,
		})
		if err != nil {
			return nil, common.WrapIntoReconcileTolerantError(err, "failed to list nodes")
		}

		if list.Continue == "" {
			break
		}
		continueToken = list.Continue
		nodes = append(nodes, list.Items...)
	}

	return cluster.GetNodesState(nodes, desiredVersion)
}

func (r *reconciler) getControlPlaneState(ctx context.Context, desiredVersion string) (*cluster.ControlPlaneState, error) {
	pods, err := r.getControlPlanePods(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get control plane pods: %w", err)
	}

	return cluster.GetControlPlaneState(pods, desiredVersion)
}

func (r *reconciler) getControlPlanePods(ctx context.Context) (*corev1.PodList, error) {
	res := &corev1.PodList{}

	labelSelector, err := labels.Parse(fmt.Sprintf(
		"component in (%s,%s,%s)",
		cluster.KubeApiServer,
		cluster.KubeScheduler,
		cluster.KubeControllerManager))
	if err != nil {
		return nil, fmt.Errorf("failed to parse components selector: %w", err)
	}

	err = r.client.List(ctx, res, &client.ListOptions{
		LabelSelector: labelSelector,
		Namespace:     common.KubeSystemNamespace,
	})
	if err != nil {
		return nil, common.WrapIntoReconcileTolerantError(err, "failed to fetch pod list")
	}

	return res, nil
}

func combine(cfg *cluster.Configuration, nodes *cluster.NodesState, controlPlane *cluster.ControlPlaneState) *cluster.State {
	state := &cluster.State{
		Spec: cluster.Spec{
			DesiredVersion: cfg.DesiredVersion,
			UpdateMode:     cfg.UpdateMode,
		},
		Status: cluster.Status{
			ControlPlaneState: *controlPlane,
			NodesState:        *nodes,
		},
	}

	determineStatePhase(state)

	return state
}

func determineStatePhase(s *cluster.State) {
	var phase cluster.Phase
	switch s.ControlPlaneState.Phase {
	case cluster.ControlPlaneUpdating:
		phase = cluster.ClusterControlPlaneUpdating
	case cluster.ControlPlaneVersionDrift:
		phase = cluster.ClusterControlPlaneVersionDrift
	case cluster.ControlPlaneInconsistent:
		phase = cluster.ClusterControlPlaneInconsistent
	case cluster.ControlPlaneUpToDate:
		if s.Spec.UpdateMode == cluster.UpdateModeAutomatic && s.NodesState.CurrentVersion > s.Spec.DesiredVersion {
			phase = cluster.ClusterVersionDrift
			break
		}
		if s.NodesState.UpToDateCount < s.NodesState.DesiredCount {
			phase = cluster.ClusterNodesUpdating
			break
		}
		if s.NodesState.UpToDateCount == s.NodesState.DesiredCount {
			phase = cluster.ClusterUpToDate
			break
		}
		if s.NodesState.UpToDateCount > s.NodesState.DesiredCount {
			phase = cluster.ClusterInconsistent
			break
		}
	}

	s.Phase = phase
}
