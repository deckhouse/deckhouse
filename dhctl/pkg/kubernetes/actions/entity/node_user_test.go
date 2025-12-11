// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package entity

import (
	"context"
	"testing"
	"time"

	"github.com/name212/govalue"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/deckhouse/deckhouse/dhctl/pkg/apis/v1"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

func TestWaitNodeUserPresentOnNode(t *testing.T) {
	convergerNodeUserProvider := testConvergerNodeUserProvider(t)
	nodeUserOnAllNodesProvider := testNodeUserOnNodeGroups(t, nil)
	nodeUserOnNodeGroupsListProvider := testNodeUserOnNodeGroups(t, []string{
		"master",
		"worker",
	})

	tests := []testNodeUserWaiterParams{
		// tests converger user
		{
			name:     "without nodes",
			nodes:    make([]testNode, 0),
			nodeUser: convergerNodeUserProvider(),
			hasErr:   true,
		},
		{
			name: "one control plane node without user",
			nodes: []testNode{
				testCreateTestControlPlaneNode("master-0", nil),
			},
			nodeUser: convergerNodeUserProvider(),
			hasErr:   true,
		},
		{
			name: "many control plane nodes without user",
			nodes: []testNode{
				testCreateTestControlPlaneNode("master-0", nil),
				testCreateTestControlPlaneNode("master-1", map[string]string{}),
				testCreateTestControlPlaneNode("master-2", map[string]string{
					"foo": "bar",
				}),
			},
			nodeUser: convergerNodeUserProvider(),
			hasErr:   true,
		},
		{
			name: "one of control plane node with user another not",
			nodes: []testNode{
				testCreateTestControlPlaneNode("master-0", testCreateAnnotationsWithConvergerUser()),
				testCreateTestControlPlaneNode("master-1", map[string]string{}),
				testCreateTestControlPlaneNode("master-2", map[string]string{
					"foo": "bar",
				}),
			},
			nodeUser: convergerNodeUserProvider(),
			hasErr:   true,
		},
		{
			name: "two of control plane node with user one not",
			nodes: []testNode{
				testCreateTestControlPlaneNode("master-0", testCreateAnnotationsWithConvergerUser()),
				testCreateTestControlPlaneNode("master-1", map[string]string{
					"foo": "bar",
				}),
				testCreateTestControlPlaneNode("master-2", testCreateAnnotationsWithConvergerUser()),
			},
			nodeUser: convergerNodeUserProvider(),
			hasErr:   true,
		},
		{
			name: "two of control plane node with user one not and with user on workers",
			nodes: []testNode{
				testCreateTestControlPlaneNode("master-0", testCreateAnnotationsWithConvergerUser()),
				testCreateTestControlPlaneNode("master-1", map[string]string{
					"foo": "bar",
				}),
				testCreateTestControlPlaneNode("master-2", testCreateAnnotationsWithConvergerUser()),
				testCreateTestWorkerNode("worker-0", testCreateAnnotationsWithConvergerUser()),
				testCreateTestWorkerNode("worker-1", testCreateAnnotationsWithConvergerUser()),
			},
			nodeUser: convergerNodeUserProvider(),
			hasErr:   true,
		},
		{
			name: "all of control plane node with user",
			nodes: []testNode{
				testCreateTestControlPlaneNode("master-0", testCreateAnnotationsWithConvergerUser()),
				testCreateTestControlPlaneNode("master-1", testCreateAnnotationsWithConvergerUser()),
				testCreateTestControlPlaneNode("master-2", testCreateAnnotationsWithConvergerUser()),
			},
			nodeUser: convergerNodeUserProvider(),
			hasErr:   false,
		},
		{
			name: "all of control plane node with user but not on workers",
			nodes: []testNode{
				testCreateTestControlPlaneNode("master-0", testCreateAnnotationsWithConvergerUser()),
				testCreateTestControlPlaneNode("master-1", testCreateAnnotationsWithConvergerUser()),
				testCreateTestControlPlaneNode("master-2", testCreateAnnotationsWithConvergerUser()),
				testCreateTestWorkerNode("worker-0", nil),
				testCreateTestWorkerNode("worker-1", make(map[string]string)),
				testCreateTestWorkerNode("worker-2", map[string]string{
					"foo": "bar",
				}),
			},
			nodeUser: convergerNodeUserProvider(),
			hasErr:   false,
		},
		{
			name: "one of control plane node with user",
			nodes: []testNode{
				testCreateTestControlPlaneNode("master-0", testCreateAnnotationsWithConvergerUser()),
			},
			nodeUser: convergerNodeUserProvider(),
			hasErr:   false,
		},

		{
			name: "one of control plane node with user and worker",
			nodes: []testNode{
				testCreateTestControlPlaneNode("master-0", testCreateAnnotationsWithConvergerUser()),
				testCreateTestWorkerNode("worker-0", testCreateAnnotationsWithConvergerUser()),
			},
			nodeUser: convergerNodeUserProvider(),
			hasErr:   false,
		},
		{
			name: "one of control plane node with user but not on workers",
			nodes: []testNode{
				testCreateTestControlPlaneNode("master-0", testCreateAnnotationsWithConvergerUser()),
				testCreateTestWorkerNode("worker-0", nil),
				testCreateTestWorkerNode("worker-1", make(map[string]string)),
				testCreateTestWorkerNode("worker-2", map[string]string{
					"foo": "bar",
				}),
			},
			nodeUser: convergerNodeUserProvider(),
			hasErr:   false,
		},
		// another users tests
		{
			name:     "all nodes: without nodes",
			nodes:    make([]testNode, 0),
			nodeUser: nodeUserOnAllNodesProvider(),
			hasErr:   true,
		},
		{
			name: "all nodes: no one node with user",
			nodes: []testNode{
				testCreateTestControlPlaneNodeWithAdditionalLabels("master-0", nil),
				testCreateTestWorkerNodeWithAdditionalLabels("worker-0", map[string]string{
					"foo": "bar",
				}),
			},
			nodeUser: nodeUserOnAllNodesProvider(),
			hasErr:   true,
		},
		{
			name: "all nodes: one node with user",
			nodes: []testNode{
				testCreateTestControlPlaneNodeWithAdditionalLabels("master-0", map[string]string{}),
				testCreateTestWorkerNodeWithAdditionalLabels("worker-0", testCreateLabelsWithAdditionalUser()),
			},
			nodeUser: nodeUserOnAllNodesProvider(),
			hasErr:   true,
		},
		{
			name: "all nodes: one another node with user",
			nodes: []testNode{
				testCreateTestControlPlaneNodeWithAdditionalLabels("master-0", testCreateLabelsWithAdditionalUser()),
				testCreateTestWorkerNodeWithAdditionalLabels("worker-0", map[string]string{
					"foo": "bar",
				}),
			},
			nodeUser: nodeUserOnAllNodesProvider(),
			hasErr:   true,
		},
		{
			name: "all nodes: all nodes with user",
			nodes: []testNode{
				testCreateTestControlPlaneNodeWithAdditionalLabels("master-0", testCreateLabelsWithAdditionalUser()),
				testCreateTestWorkerNodeWithAdditionalLabels("worker-0", testCreateLabelsWithAdditionalUser()),
			},
			nodeUser: nodeUserOnAllNodesProvider(),
			hasErr:   false,
		},
		{
			name:     "node groups list: no nodes",
			nodes:    make([]testNode, 0),
			nodeUser: nodeUserOnNodeGroupsListProvider(),
			hasErr:   true,
		},
		{
			name: "node groups list: no one node with user",
			nodes: []testNode{
				testCreateTestControlPlaneNodeWithAdditionalLabels("master-0", nil),
				testCreateTestWorkerNodeWithAdditionalLabels("worker-0", map[string]string{
					"foo": "bar",
				}),
			},
			nodeUser: nodeUserOnNodeGroupsListProvider(),
			hasErr:   true,
		},
		{
			name: "node groups list: one node with user",
			nodes: []testNode{
				testCreateTestControlPlaneNodeWithAdditionalLabels("master-0", map[string]string{}),
				testCreateTestWorkerNodeWithAdditionalLabels("worker-0", testCreateLabelsWithAdditionalUser()),
			},
			nodeUser: nodeUserOnNodeGroupsListProvider(),
			hasErr:   true,
		},
		{
			name: "node groups list: one another node with user",
			nodes: []testNode{
				testCreateTestControlPlaneNodeWithAdditionalLabels("master-0", testCreateLabelsWithAdditionalUser()),
				testCreateTestWorkerNodeWithAdditionalLabels("worker-0", map[string]string{
					"foo": "bar",
				}),
			},
			nodeUser: nodeUserOnNodeGroupsListProvider(),
			hasErr:   true,
		},
		{
			name: "node groups list: all nodes with user",
			nodes: []testNode{
				testCreateTestControlPlaneNodeWithAdditionalLabels("master-0", testCreateLabelsWithAdditionalUser()),
				testCreateTestWorkerNodeWithAdditionalLabels("worker-0", testCreateLabelsWithAdditionalUser()),
			},
			nodeUser: nodeUserOnNodeGroupsListProvider(),
			hasErr:   false,
		},
	}

	for _, tstParams := range tests {
		t.Run(tstParams.name, func(t *testing.T) {
			tst := testCreateWaiterTest(t, tstParams)

			err := tst.waiter.WaitPresentOnNodes(context.TODO(), tst.params.nodeUser.nodeUser)

			if tstParams.hasErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

type testNode struct {
	name        string
	annotations map[string]string
	labels      map[string]string
}

type testNodeUserWithChecker struct {
	checker  NodeUserPresentsChecker
	nodeUser *v1.NodeUserCredentials
}

type testNodeUserWaiterParams struct {
	name string

	nodes    []testNode
	nodeUser testNodeUserWithChecker

	hasErr bool
}

type testNodeUserWaiterTest struct {
	params testNodeUserWaiterParams

	waiter *NodeUserPresentsWaiter
}

func testCreateWaiterTest(t *testing.T, test testNodeUserWaiterParams) testNodeUserWaiterTest {
	require.NotEmpty(t, test.name)
	require.False(t, govalue.IsNil(test.nodeUser.nodeUser))
	require.False(t, govalue.IsNil(test.nodeUser.checker))

	kubeCl := client.NewFakeKubernetesClient()
	ctx := context.TODO()

	for _, node := range test.nodes {
		obj := corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:        node.name,
				Labels:      node.labels,
				Annotations: node.annotations,
			},
		}

		_, err := kubeCl.CoreV1().Nodes().Create(ctx, &obj, metav1.CreateOptions{})
		require.NoError(t, err, test.name, node.name)
	}

	nodes, err := kubeCl.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, nodes.Items, len(test.nodes))

	kubeProvider := kubernetes.NewSimpleKubeClientGetter(kubeCl)

	waiter := NewNodeUserExistsWaiter(test.nodeUser.checker, kubeProvider)
	waiter.attempts = 1
	waiter.sleep = 1 * time.Second

	return testNodeUserWaiterTest{
		params: test,
		waiter: waiter,
	}
}

func testConvergerNodeUserProvider(t *testing.T) func() testNodeUserWithChecker {
	_, convergerCreds, err := v1.GenerateNodeUser(v1.ConvergerNodeUser())
	require.NoError(t, err)
	require.NotNil(t, convergerCreds)

	return func() testNodeUserWithChecker {
		return testNodeUserWithChecker{
			nodeUser: convergerCreds,
			checker:  v1.ConvergerNodeUserExistsChecker,
		}
	}
}

func testCreateTestControlPlaneNode(name string, annotations map[string]string) testNode {
	return testNode{
		name:        name,
		annotations: annotations,
		labels: map[string]string{
			"node.deckhouse.io/group": "master",
		},
	}
}

func testCreateAnnotationsWithConvergerUser() map[string]string {
	return map[string]string{
		"another":                          "true",
		global.ConvergerNodeUserAnnotation: "true",
		"foo":                              "bar",
	}
}

func testCreateTestWorkerNode(name string, annotations map[string]string) testNode {
	return testNode{
		name:        name,
		annotations: annotations,
		labels: map[string]string{
			"node.deckhouse.io/group": "worker",
		},
	}
}

func testNodeUserExistsOnLabel(node corev1.Node) bool {
	labels := node.GetLabels()
	if len(labels) == 0 {
		return false
	}

	val, ok := labels["user-exists"]
	if !ok || val != "true" {
		return false
	}

	_, ok = labels["some-label"]
	return ok

}

func testNodeUserOnNodeGroups(t *testing.T, nodeGroups []string) func() testNodeUserWithChecker {
	_, convergerCreds, err := v1.GenerateNodeUser(v1.NodeUserParams{
		Name:       "some-user",
		UUID:       11111,
		NodeGroups: nodeGroups,
	})
	require.NoError(t, err)
	require.NotNil(t, convergerCreds)

	return func() testNodeUserWithChecker {
		return testNodeUserWithChecker{
			nodeUser: convergerCreds,
			checker:  testNodeUserExistsOnLabel,
		}
	}
}

func testCreateTestControlPlaneNodeWithAdditionalLabels(name string, labels map[string]string) testNode {
	n := testCreateTestControlPlaneNode(name, map[string]string{
		"some-annotation": "test",
	})

	for k, v := range labels {
		n.labels[k] = v
	}

	return n
}

func testCreateTestWorkerNodeWithAdditionalLabels(name string, labels map[string]string) testNode {
	n := testCreateTestWorkerNode(name, map[string]string{
		"some-annotation-worker": "test",
	})

	for k, v := range labels {
		n.labels[k] = v
	}

	return n
}

func testCreateLabelsWithAdditionalUser() map[string]string {
	return map[string]string{
		"user-exists": "true",
		"some-label":  "label",
		"another":     "true",
	}
}
