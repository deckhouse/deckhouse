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
	CurrentVersion string       `json:"currentVersion" yaml:"currentVersion"`
	Phase          string       `json:"phase" yaml:"phase"`
	ControlPlane   ControlPlane `json:"controlPlane" yaml:"controlPlane"`
	Nodes          Nodes        `json:"nodes" yaml:"nodes"`
}

type ControlPlane struct {
	DesiredCount  int                    `json:"desiredCount" yaml:"desiredCount"`
	UpToDateCount int                    `json:"upToDateCount" yaml:"upToDateCount"`
	Progress      string                 `json:"progress" yaml:"progress"`
	Phase         string                 `json:"phase" yaml:"phase"`
	Nodes         map[string]*MasterNode `json:",inline" yaml:",inline"`
}

type MasterNode struct {
	Phase      string            `json:"phase" yaml:"phase"`
	Components map[string]string `json:"components" yaml:"components"`
}

type Nodes struct {
	DesiredCount  int `json:"desiredCount" yaml:"desiredCount"`
	UpToDateCount int `json:"upToDateCount" yaml:"upToDateCount"`
}

func renderConfigMapData(clusterState *cluster.State) ConfigMapData {
	if clusterState == nil {
		return ConfigMapData{
			Status: &Status{
				Phase: string(cluster.ClusterUnknown),
			},
		}
	}

	buildMasterNodes := func(m map[string]*cluster.MasterNodeState) map[string]*MasterNode {
		res := make(map[string]*MasterNode, len(m))
		for nodeName, nodeState := range m {
			res[nodeName] = &MasterNode{
				Phase:      string(nodeState.Phase),
				Components: make(map[string]string),
			}

			for component, componentState := range nodeState.ComponentsState {
				res[nodeName].Components[component] = componentState.Version
			}
		}

		return res
	}

	return ConfigMapData{
		Spec: &Spec{
			DesiredVersion: clusterState.Spec.DesiredVersion,
			UpdateMode:     string(clusterState.Spec.UpdateMode),
		},
		Status: &Status{
			CurrentVersion: clusterState.Status.CurrentVersion,
			Phase:          string(clusterState.Status.Phase),
			ControlPlane: ControlPlane{
				DesiredCount:  clusterState.Status.ControlPlaneState.DesiredCount,
				UpToDateCount: clusterState.Status.ControlPlaneState.UpToDateCount,
				Progress:      clusterState.Status.ControlPlaneState.Progress,
				Phase:         string(clusterState.Status.ControlPlaneState.Phase),
				Nodes:         buildMasterNodes(clusterState.Status.ControlPlaneState.NodesState),
			},
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
