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

package cluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// makeNode creates a Node with the given kubelet version.
func makeNode(name, kubeletVersion string) corev1.Node {
	return corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status: corev1.NodeStatus{
			NodeInfo: corev1.NodeSystemInfo{KubeletVersion: kubeletVersion},
		},
	}
}

// makePod creates a healthy Running control-plane pod with the given version annotation.
func makePod(name, nodeName, component, kubeVersion string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "kube-system",
			Labels:    map[string]string{componentLabelKey: component},
			Annotations: map[string]string{
				kubeVersionAnnotation: kubeVersion,
			},
		},
		Spec: corev1.PodSpec{NodeName: nodeName},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Ready: true,
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{},
					},
				},
			},
		},
	}
}

func masterPods(nodeName string, apiserverVer, schedulerVer, controllerManagerVer string) *corev1.PodList {
	return &corev1.PodList{
		Items: []corev1.Pod{
			makePod("kube-apiserver-"+nodeName, nodeName, "kube-apiserver", apiserverVer),
			makePod("kube-scheduler-"+nodeName, nodeName, "kube-scheduler", schedulerVer),
			makePod("kube-controller-manager-"+nodeName, nodeName, "kube-controller-manager", controllerManagerVer),
		},
	}
}

var defaultVersionSettings = VersionSettings{
	Supported: []string{"1.31", "1.32", "1.33", "1.34", "1.35"},
	Automatic: "1.33",
}

var defaultCfg = &Configuration{
	DesiredVersion: "1.33",
	UpdateMode:     UpdateModeAutomatic,
}

// buildState is a helper that constructs a cluster State from raw node/pod data.
func buildState(t *testing.T, cfg *Configuration, nodes []corev1.Node, pods *corev1.PodList, sourceVersion string) *State {
	t.Helper()

	nodesState, err := GetNodesState(nodes, cfg.DesiredVersion, sourceVersion)
	require.NoError(t, err)

	cpState, err := GetControlPlaneState(pods, cfg.DesiredVersion, sourceVersion)
	require.NoError(t, err)

	return GetState(cfg, nodesState, cpState, defaultVersionSettings, sourceVersion, sourceVersion, false)
}

func TestProgress_UpgradeOneVersionAtATime(t *testing.T) {
	// Upgrading from 1.31 to 1.33: 2 hops
	// 1 master + 1 worker = 5 trackable components (3 CP pods + 2 node kubelets)
	// totalSteps = 2 * 5 = 10

	t.Run("nothing updated yet — 0%", func(t *testing.T) {
		nodes := []corev1.Node{
			makeNode("master-0", "v1.31.0"),
			makeNode("worker-0", "v1.31.0"),
		}
		pods := masterPods("master-0", "v1.31.0", "v1.31.0", "v1.31.0")

		state := buildState(t, defaultCfg, nodes, pods, "1.31")

		assert.Equal(t, "0%", state.Progress)
		assert.Equal(t, Phase(ClusterControlPlaneUpdating), state.Phase)
	})

	t.Run("one CP component at 1.32 — 10%", func(t *testing.T) {
		// completedSteps: apiserver(1) + scheduler(0) + CM(0) + master-node(0) + worker(0) = 1
		nodes := []corev1.Node{
			makeNode("master-0", "v1.31.0"),
			makeNode("worker-0", "v1.31.0"),
		}
		pods := masterPods("master-0", "v1.32.0", "v1.31.0", "v1.31.0")

		state := buildState(t, defaultCfg, nodes, pods, "1.31")

		assert.Equal(t, "10%", state.Progress)
	})

	t.Run("all CP components at 1.32 — 30%", func(t *testing.T) {
		// completedSteps: 3 CP(1 each) + 2 nodes(0 each) = 3
		nodes := []corev1.Node{
			makeNode("master-0", "v1.31.0"),
			makeNode("worker-0", "v1.31.0"),
		}
		pods := masterPods("master-0", "v1.32.0", "v1.32.0", "v1.32.0")

		state := buildState(t, defaultCfg, nodes, pods, "1.31")

		assert.Equal(t, "30%", state.Progress)
	})

	t.Run("CP at 1.33, nodes at 1.32 — 80%", func(t *testing.T) {
		// completedSteps: 3 CP(2 each) + 2 nodes(1 each) = 6+2 = 8
		nodes := []corev1.Node{
			makeNode("master-0", "v1.32.0"),
			makeNode("worker-0", "v1.32.0"),
		}
		pods := masterPods("master-0", "v1.33.0", "v1.33.0", "v1.33.0")

		state := buildState(t, defaultCfg, nodes, pods, "1.31")

		assert.Equal(t, "80%", state.Progress)
		assert.Equal(t, Phase(ClusterNodesUpdating), state.Phase)
	})

	t.Run("everything at 1.33 — 100%", func(t *testing.T) {
		// completedSteps: 3 CP(2 each) + 2 nodes(2 each) = 6+4 = 10
		nodes := []corev1.Node{
			makeNode("master-0", "v1.33.0"),
			makeNode("worker-0", "v1.33.0"),
		}
		pods := masterPods("master-0", "v1.33.0", "v1.33.0", "v1.33.0")

		state := buildState(t, defaultCfg, nodes, pods, "1.31")

		assert.Equal(t, "100%", state.Progress)
		assert.Equal(t, Phase(ClusterUpToDate), state.Phase)
	})
}

func TestProgress_IdleNoUpgrade(t *testing.T) {
	// source == desired -> falls back to upToDateCount/totalCount logic
	t.Run("all components at desired version — 100%", func(t *testing.T) {
		nodes := []corev1.Node{
			makeNode("master-0", "v1.33.0"),
			makeNode("worker-0", "v1.33.0"),
		}
		pods := masterPods("master-0", "v1.33.0", "v1.33.0", "v1.33.0")

		state := buildState(t, defaultCfg, nodes, pods, "1.33")

		assert.Equal(t, "100%", state.Progress)
		assert.Equal(t, Phase(ClusterUpToDate), state.Phase)
	})
}
