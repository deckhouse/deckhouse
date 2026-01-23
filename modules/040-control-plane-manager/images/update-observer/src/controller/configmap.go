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

	"go.yaml.in/yaml/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ConfigMapData struct {
	*Spec
	*Status
}

type Spec struct {
	DesiredVersion string `yaml:"desiredVersion"`
	UpdateMode     string `yaml:"updateMode"`
}

type Status struct {
	CurrentVersion string             `yaml:"currentVersion"`
	Phase          string             `yaml:"phase"`
	Progress       string             `yaml:"progress"`
	ControlPlane   []ControlPlaneNode `yaml:"controlPlane"`
	Nodes          Nodes              `yaml:"nodes"`
}

type ControlPlaneNode struct {
	Name       string            `yaml:"name"`
	Phase      string            `yaml:"phase"`
	Components map[string]string `yaml:"components"`
}

type Nodes struct {
	DesiredCount  int `yaml:"desiredCount"`
	UpToDateCount int `yaml:"upToDateCount"`
}

func renderConfigMapData(clusterState *cluster.State) ConfigMapData {
	if clusterState == nil {
		return ConfigMapData{
			Status: &Status{
				Phase: string(cluster.ClusterUnknown),
			},
		}
	}

	renderControlPlanes := func(m map[string]*cluster.MasterNodeState) []ControlPlaneNode {
		controlPlanes := make([]ControlPlaneNode, 0, len(m))
		for name, nodeState := range m {
			controlPlaneNode := ControlPlaneNode{
				Name:       name,
				Phase:      string(nodeState.Phase),
				Components: make(map[string]string, len(nodeState.ComponentsState)),
			}

			for component, componentState := range nodeState.ComponentsState {
				controlPlaneNode.Components[component] = componentState.Version
			}

			controlPlanes = append(controlPlanes, controlPlaneNode)
		}

		return controlPlanes
	}

	return ConfigMapData{
		Spec: &Spec{
			DesiredVersion: clusterState.Spec.DesiredVersion,
			UpdateMode:     string(clusterState.Spec.UpdateMode),
		},
		Status: &Status{
			CurrentVersion: clusterState.Status.CurrentVersion,
			Phase:          string(clusterState.Status.Phase),
			Progress:       clusterState.Progress,
			ControlPlane:   renderControlPlanes(clusterState.ControlPlaneState.NodesState),
			Nodes: Nodes{
				DesiredCount:  clusterState.NodesState.DesiredCount,
				UpToDateCount: clusterState.NodesState.UpToDateCount,
			},
		},
	}
}

func (r *reconciler) getConfigMap(ctx context.Context) (*corev1.ConfigMap, error) {
	cm := &corev1.ConfigMap{}
	err := r.client.Get(ctx, client.ObjectKey{
		Name:      common.ConfigMapName,
		Namespace: common.KubeSystemNamespace,
	}, cm)

	if client.IgnoreNotFound(err) != nil {
		return nil, err
	}

	if err != nil {
		cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.ConfigMapName,
				Namespace: common.KubeSystemNamespace,
				Labels: map[string]string{
					common.HeritageLabelKey: common.DeckhouseLabel,
				},
			},
			Data: map[string]string{},
		}
	}

	return cm, nil
}

func (r *reconciler) touchConfigMap(ctx context.Context, configMap *corev1.ConfigMap, configMapData ConfigMapData) error {
	if configMap.Data == nil {
		configMap.Data = map[string]string{}
	}

	if configMapData.Spec != nil {
		specBytes, err := yaml.Marshal(configMapData.Spec)
		if err != nil {
			return fmt.Errorf("failed to marshal Spec: %w", err)
		}
		configMap.Data["spec"] = string(specBytes)
	}

	if configMapData.Status != nil {
		statusBytes, err := yaml.Marshal(configMapData.Status)
		if err != nil {
			return fmt.Errorf("failed to marshal Status: %w", err)
		}
		configMap.Data["status"] = string(statusBytes)
	}

	if configMap.ResourceVersion == "" {
		if err := r.client.Create(ctx, configMap); err != nil {
			return fmt.Errorf("failed to create configMap: %w", err)
		}
	} else {
		if err := r.client.Update(ctx, configMap); err != nil {
			return fmt.Errorf("failed to update configMap: %w", err)
		}
	}

	return nil
}
