/*
Copyright 2023 Flant JSC

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

package checker

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"d8.io/upmeter/pkg/check"
)

func Test_checker_dsPodsReadinessChecker(t *testing.T) {
	const (
		creationTimeout = time.Minute
		deletionTimeout = 5 * time.Second
	)

	type state struct {
		pods  []v1.Pod
		nodes []*v1.Node

		dsErr, podsErr, nodesErr error
	}
	tests := []struct {
		name  string
		state state
		want  check.Status
	}{
		{
			name: "no daemonset is not fine",
			want: check.Down,
			state: state{
				dsErr: apierrors.NewNotFound(schema.GroupResource{}, ""),
			},
		},
		{
			name: "arbitrary error getting daemonset results in Unknown",
			want: check.Unknown,
			state: state{
				dsErr: fmt.Errorf("whatever"),
			},
		},
		{
			name: "arbitrary error getting nodes results in Unknown",
			want: check.Unknown,
			state: state{
				nodesErr: fmt.Errorf("whatever"),
			},
		},
		{
			name: "arbitrary error getting pods results in Unknown",
			want: check.Unknown,
			state: state{
				podsErr: fmt.Errorf("whatever"),
			},
		},
		{
			name:  "daemonset exists, but no nodes and no pods, resulting in success",
			want:  check.Up,
			state: state{},
		},
		{
			name: "everything in place, resulting in success",
			want: check.Up,
			state: state{
				pods:  []v1.Pod{scheduledPod("a"), scheduledPod("b"), scheduledPod("c")},
				nodes: []*v1.Node{healthyNode("a"), healthyNode("b"), healthyNode("c")},
			},
		},
		{
			name: "missing pod is not fine",
			want: check.Down,
			state: state{
				pods:  []v1.Pod{scheduledPod("a") /*  no pod "b"  */, scheduledPod("c")},
				nodes: []*v1.Node{healthyNode("a"), healthyNode("b"), healthyNode("c")},
			},
		},
		{
			name: "missing pod on a tainted node is fine",
			want: check.Up,
			state: state{
				pods:  []v1.Pod{scheduledPod("a") /*  no pod "b"  */, scheduledPod("c")},
				nodes: []*v1.Node{healthyNode("a"), taintedNode("b"), healthyNode("c")},
			},
		},
		{
			name: "missing node is fine",
			want: check.Up,
			state: state{
				pods:  []v1.Pod{scheduledPod("a"), scheduledPod("b"), scheduledPod("c")},
				nodes: []*v1.Node{healthyNode("a") /*  no node "b" */, healthyNode("c")},
			},
		},
		{
			name: "not-ready node is fine",
			want: check.Up,
			state: state{
				pods:  []v1.Pod{scheduledPod("a"), scheduledPod("b"), scheduledPod("c")},
				nodes: []*v1.Node{healthyNode("a"), notReadyNode("b"), healthyNode("c")},
			},
		},
		{
			name: "not-ready pending pod is not fine",
			want: check.Down,
			state: state{
				pods:  []v1.Pod{scheduledPod("a"), pendingPod("b"), scheduledPod("c")},
				nodes: []*v1.Node{healthyNode("a"), healthyNode("b"), healthyNode("c")},
			},
		},
		{
			name: "not-ready pending pod for not too long is fine",
			want: check.Up,
			state: state{
				pods:  []v1.Pod{scheduledPod("a"), agedPod(creationTimeout/2, pendingPod("b")), scheduledPod("c")},
				nodes: []*v1.Node{healthyNode("a"), healthyNode("b"), healthyNode("c")},
			},
		},
		{
			name: "not-ready running pod is not fine",
			want: check.Down,
			state: state{
				pods:  []v1.Pod{scheduledPod("a"), notReadyPod("b"), scheduledPod("c")},
				nodes: []*v1.Node{healthyNode("a"), healthyNode("b"), healthyNode("c")},
			},
		},
		{
			name: "not-ready running pod on very fresh node is fine",
			want: check.Up,
			state: state{
				pods:  []v1.Pod{scheduledPod("a"), notReadyPod("b"), scheduledPod("c")},
				nodes: []*v1.Node{healthyNode("a"), freshNode("b", creationTimeout/2), healthyNode("c")},
			},
		},
		{
			name: "not-ready running pod for too long is not fine",
			want: check.Down,
			state: state{
				pods:  []v1.Pod{scheduledPod("a"), agedPod(creationTimeout*2, notReadyPod("b")), scheduledPod("c")},
				nodes: []*v1.Node{healthyNode("a"), healthyNode("b"), healthyNode("c")},
			},
		},
		{
			name: "terminating pod for not too long is fine",
			want: check.Up,
			state: state{
				pods:  []v1.Pod{scheduledPod("a"), terminatingPod("b", deletionTimeout/2), scheduledPod("c")},
				nodes: []*v1.Node{healthyNode("a"), healthyNode("b"), healthyNode("c")},
			},
		},
		{
			name: "terminating pod for too long is not fine",
			want: check.Down,
			state: state{
				pods:  []v1.Pod{scheduledPod("a"), terminatingPod("b", deletionTimeout*2), scheduledPod("c")},
				nodes: []*v1.Node{healthyNode("a"), healthyNode("b"), healthyNode("c")},
			},
		},
		{
			name: "pod-node mismatch is not fine (=missing pod on a node)",
			want: check.Down,
			state: state{
				pods:  []v1.Pod{scheduledPod("a"), scheduledPod("y"), scheduledPod("c")},
				nodes: []*v1.Node{healthyNode("a"), healthyNode("x"), healthyNode("c")},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsRepo := &dsRepoMock{
				ds:    daemonSet("node-exporter"),
				dsErr: tt.state.dsErr,

				pods:    tt.state.pods,
				podsErr: tt.state.podsErr,
			}

			nodeLister := &nodeListerMock{
				nodes: tt.state.nodes,
				err:   tt.state.nodesErr,
			}

			checker := &dsPodsReadinessChecker{
				dsRepo:          dsRepo,
				nodeLister:      nodeLister,
				creationTimeout: creationTimeout,
				deletionTimeout: deletionTimeout,
			}

			err := checker.Check()
			assertCheckStatus(t, tt.want, err)
		})
	}
}

func assertCheckStatus(t *testing.T, want check.Status, err check.Error) {
	if want == check.Up {
		assert.NoError(t, err, "Expected no err")
	} else {
		var got check.Status
		if err == nil {
			got = check.Up
		} else {
			got = err.Status()
		}
		assert.Equal(t, want.String(), got.String())
	}
}

func daemonSet(name string) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "DaemonSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec:   appsv1.DaemonSetSpec{},
		Status: appsv1.DaemonSetStatus{},
	}
}

func agedPod(age time.Duration, pod v1.Pod) v1.Pod {
	ts := metav1.NewTime(time.Now().Add(-age))
	pod.CreationTimestamp = ts
	pod.Status.Conditions[0].LastTransitionTime = ts
	return pod
}

func terminatingPod(name string, termAge time.Duration) v1.Pod {
	when := metav1.NewTime(time.Now().Add(-termAge))

	pod := scheduledPod(name)
	pod.DeletionTimestamp = &when
	pod.Status.Conditions[0].Status = v1.ConditionFalse
	return pod
}

func pendingPod(name string) v1.Pod {
	pod := scheduledPod(name)
	pod.Status.Phase = v1.PodPending
	pod.Status.Conditions[0].Status = v1.ConditionFalse
	return pod
}

func notReadyPod(name string) v1.Pod {
	pod := scheduledPod(name)
	pod.Status.Conditions[0].Status = v1.ConditionFalse
	return pod
}

func scheduledPod(name string) v1.Pod {
	hourAgo := metav1.NewTime(time.Now().Add(-time.Hour))

	return v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{},

			CreationTimestamp: hourAgo,
		},
		Spec: v1.PodSpec{
			NodeName: name,
			Affinity: &v1.Affinity{
				NodeAffinity: &v1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
						NodeSelectorTerms: []v1.NodeSelectorTerm{
							{
								MatchExpressions: []v1.NodeSelectorRequirement{
									{
										Key:      "kubernetes.io/hostname",
										Operator: v1.NodeSelectorOpIn,
										Values:   []string{name},
									},
								},
							},
						},
					},
				},
			},
		}, Status: v1.PodStatus{
			Phase: v1.PodRunning,
			Conditions: []v1.PodCondition{
				{
					Type:               v1.PodReady,
					Status:             v1.ConditionTrue,
					LastTransitionTime: hourAgo,
				},
			},
		},
	}
}

func taintedNode(name string) *v1.Node {
	node := healthyNode(name)
	node.Spec.Taints = []v1.Taint{{
		Key:    "key1",
		Value:  "value1",
		Effect: v1.TaintEffectNoExecute,
	}}
	return node
}

func notReadyNode(name string) *v1.Node {
	node := healthyNode(name)
	node.Status.Conditions[0].Status = v1.ConditionFalse
	return node
}

func freshNode(name string, age time.Duration) *v1.Node {
	when := metav1.NewTime(time.Now().Add(-age))

	node := healthyNode(name)
	node.Status.Conditions[0].LastTransitionTime = when
	node.ObjectMeta.CreationTimestamp = when

	return node
}

func healthyNode(name string) *v1.Node {
	hourAgo := metav1.NewTime(time.Now().Add(-time.Hour))

	return &v1.Node{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Node",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			CreationTimestamp: hourAgo,
		},
		Spec: v1.NodeSpec{
			Taints:       nil,
			ConfigSource: nil,
		},
		Status: v1.NodeStatus{
			Capacity:    nil,
			Allocatable: nil,
			Phase:       "",
			Conditions: []v1.NodeCondition{
				{
					Type:               v1.NodeReady,
					Status:             v1.ConditionTrue,
					LastTransitionTime: hourAgo,
				},
			},
		},
	}
}

type dsRepoMock struct {
	dsErr   error
	podsErr error
	ds      *appsv1.DaemonSet
	pods    []v1.Pod
}

func (m *dsRepoMock) Get() (*appsv1.DaemonSet, error) {
	if m.dsErr != nil {
		return nil, m.dsErr
	}
	return m.ds, nil
}

func (m *dsRepoMock) Pods() ([]v1.Pod, error) {
	if m.podsErr != nil {
		return nil, m.podsErr
	}
	return m.pods, nil
}

type nodeListerMock struct {
	err   error
	nodes []*v1.Node
}

func (m *nodeListerMock) List() ([]*v1.Node, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.nodes, nil
}
