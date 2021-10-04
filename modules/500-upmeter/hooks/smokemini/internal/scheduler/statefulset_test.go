/*
Copyright 2021 Flant JSC

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

package scheduler

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"

	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/modules/500-upmeter/hooks/smokemini/internal/snapshot"
)

func assertOk(xs ...string) func(*testing.T, string, error) {
	expected := set.New(xs...)
	return func(t *testing.T, x string, err error) {
		if assert.NoError(t, err, "should return no error") {
			assert.Contains(t, expected.Slice(), x)
		}
	}
}

var assertAny = assertOk("a", "b", "c", "d", "e")

func assertNone(t *testing.T, x string, err error) {
	if assert.ErrorIs(t, err, errNext, "should return errNext") {
		assert.Equal(t, "", x, "should be not selected")
	}
}

func assertAbortion(t *testing.T, x string, err error) {
	if assert.ErrorIs(t, err, ErrAbort, "should return aborting error") {
		assert.Equal(t, "", x, "should be not selected")
	}
}

func Test_stsSelectorByNode_Select(t *testing.T) {
	const (
		// for indexing convenience
		a = iota
		b
		c
		d
		e
	)

	tests := []struct {
		name   string
		input  func() (State, []snapshot.Node)
		assert func(*testing.T, string, error)
	}{
		{
			name: "empty state and nodes, selects any to move from not existing node",
			input: func() (State, []snapshot.Node) {
				return newState(), nil
			},
			assert: assertAny,
		},
		{
			name: "filled state but no nodes, selects any to move from not existing node",
			input: func() (State, []snapshot.Node) {
				return fakeState(), nil
			},
			assert: assertAny,
		},
		{
			name: "nodes and state are fine, no selection, because no node problem",
			input: func() (State, []snapshot.Node) {
				state := fakeState()
				nodes := fakeNodes(5)
				return state, nodes
			},
			assert: assertNone,
		},
		{
			name: "one lacking node matches the selection",
			input: func() (State, []snapshot.Node) {
				state := fakeState()
				nodes := []snapshot.Node{
					fakeNode(a), fakeNode(b),
					fakeNode(d), fakeNode(e),
				}
				return state, nodes // no c (2) node
			},
			assert: assertOk("c"),
		},
		{
			name: "one lacking node matches the selection",
			input: func() (State, []snapshot.Node) {
				state := fakeState()
				nodes := []snapshot.Node{
					fakeNode(a), fakeNode(c),
					fakeNode(d), fakeNode(e),
				}
				return state, nodes // no b (1) node
			},
			assert: assertOk("b"),
		},
		{
			name: "one unavailable node matches the selection",
			input: func() (State, []snapshot.Node) {
				state := fakeState()
				nodes := fakeNodes(5)
				nodes[d].Schedulable = false
				return state, nodes
			},
			assert: assertOk("d"),
		},
		{
			name: "one unscheduled StatefulSet matches the selection",
			input: func() (State, []snapshot.Node) {
				state := fakeState()
				nodes := fakeNodes(5)
				state["b"].Node = ""
				return state, nodes
			},
			assert: assertOk("b"),
		},
		{
			name: "absent node is more important than unschedulable one",
			input: func() (State, []snapshot.Node) {
				state := fakeState()
				nodes := fakeNodes(5)
				state["b"].Node = ""
				nodes[a].Schedulable = false
				return state, nodes
			},
			assert: assertOk("b"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, nodes := tt.input()
			s := &selectByNode{
				nodes: nodes,
			}

			x, err := s.Select(state)

			tt.assert(t, x, err)
		})
	}
}

func Test_stsSelectorByPod_Select(t *testing.T) {
	const (
		// for indexing convenience
		a = iota
		b
		c
		d
		e
	)

	tests := []struct {
		name   string
		input  func() (State, []snapshot.Pod, bool)
		assert func(*testing.T, string, error)
	}{
		{
			name: "filled state but no pods, disruption forbidden; selects any to deploy",
			input: func() (State, []snapshot.Pod, bool) {
				return fakeState(), nil, false
			},
			assert: assertAny,
		},
		{
			name: "filled state but no pods, disruption allowed; selects any to deploy",
			input: func() (State, []snapshot.Pod, bool) {
				return fakeState(), nil, true
			},
			assert: assertAny,
		},
		{
			name: "pods and state are fine, disruption allowed; no selection, because no pods problem",
			input: func() (State, []snapshot.Pod, bool) {
				state := fakeState()
				pods := fakePods(5)
				return state, pods, true
			},
			assert: assertNone,
		},
		{
			name: "pods created more than 4 min ago is selected",
			input: func() (State, []snapshot.Pod, bool) {
				state := fakeState()
				pods := fakePods(5)
				pods[d].Created = time.Now().Add(-4*time.Minute - time.Millisecond)
				return state, pods, true
			},
			assert: assertOk("d"),
		},
		{
			name: "one lacking pod matches the selection",
			input: func() (State, []snapshot.Pod, bool) {
				state := fakeState()
				pods := []snapshot.Pod{
					fakePod(a), fakePod(b),
					fakePod(d), fakePod(e),
				}
				return state, pods, false // no c (2) node
			},
			assert: assertOk("c"),
		},
		{
			name: "one lacking pod matches the selection",
			input: func() (State, []snapshot.Pod, bool) {
				state := fakeState()
				pods := []snapshot.Pod{
					fakePod(a), fakePod(c),
					fakePod(d), fakePod(e),
				}
				return state, pods, false // no b (1) node
			},
			assert: assertOk("b"),
		},
		{
			name: "forbidden disruption aborts selection",
			input: func() (State, []snapshot.Pod, bool) {
				state := fakeState()
				pods := fakePods(5)
				return state, pods, false
			},
			assert: assertAbortion,
		},
		{
			name: "forbidden disruption precedes outdated pods decision",
			input: func() (State, []snapshot.Pod, bool) {
				state := fakeState()
				pods := fakePods(5)
				pods[c].Phase = v1.PodPending
				pods[d].Created = time.Now().Add(-4*time.Minute - time.Millisecond)
				return state, pods, false
			},
			assert: assertAbortion,
		},
		{
			name: "pending for more than 1 minutes precedes disruption control",
			input: func() (State, []snapshot.Pod, bool) {
				state := fakeState()
				pods := fakePods(5)
				pods[c].Phase = v1.PodPending
				pods[c].Created = time.Now().Add(-time.Minute - time.Millisecond)
				return state, pods, false
			},
			assert: assertOk("c"),
		},
		{
			name: "when all fine, oldest pod is prioritized",
			input: func() (State, []snapshot.Pod, bool) {
				state := fakeState()
				pods := fakePods(5)
				pods[b].Created = time.Now().Add(-5 * time.Minute)
				pods[c].Created = time.Now().Add(-7 * time.Minute)
				pods[d].Created = time.Now().Add(-6 * time.Minute)
				return state, pods, true
			},
			assert: assertOk("c"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, pods, disruptionAllowed := tt.input()
			s := &selectByPod{
				pods:              pods,
				disruptionAllowed: disruptionAllowed,
			}

			x, err := s.Select(state)

			tt.assert(t, x, err)
		})
	}
}

func Test_stsSelectorByStorageClass_Select(t *testing.T) {
	defaultStorageClass := "default"

	tests := []struct {
		name   string
		input  func() (State, string)
		assert func(*testing.T, string, error)
	}{
		{
			name: "filled state and used current storage class; none to change",
			input: func() (State, string) {
				return fakeState(), defaultStorageClass
			},
			assert: assertNone,
		},
		{
			name: "filled state and unused default storage class; selects any to deploy",
			input: func() (State, string) {
				return fakeState(), "newer"
			},
			assert: assertAny,
		},
		{
			name: "selects index with deviating storageclass",
			input: func() (State, string) {
				state := fakeState()
				state["d"].StorageClass = "outdated"
				return state, defaultStorageClass
			},
			assert: assertOk("d"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, sc := tt.input()
			s := &selectByStorageClass{
				storageClass: sc,
			}

			x, err := s.Select(state)

			tt.assert(t, x, err)
		})
	}
}

func indexed(prefix string) []string {
	xs := []string{"a", "b", "c", "d", "e"}
	ret := make([]string, len(xs))
	for i := range xs {
		ret[i] = named(prefix, i)
	}
	return ret
}

func named(prefix string, i int) string {
	xs := []string{"a", "b", "c", "d", "e"}

	switch strings.ToLower(prefix) {
	case "pod":
		return snapshot.Index(xs[i]).PodName()
	case "statefulset":
		return snapshot.Index(xs[i]).StatefulSetName()
	case "pvc":
		fallthrough
	case "persistencevolumeclaim":
		return snapshot.Index(xs[i]).PersistenceVolumeClaimName()
	default:
		return fmt.Sprintf("%s-%d", prefix, i)
	}
}

func newState() State {
	return map[string]*XState{"a": {}, "b": {}, "c": {}, "d": {}, "e": {}}
}

func fakeStateInSingleZone(zone string) State {
	state := newState()

	index := []string{"a", "b", "c", "d", "e"}
	nodes := indexed("node")

	for i := range nodes {
		x := index[i]
		state[x] = &XState{
			Image:        "smoke-mini",
			Node:         nodes[i],
			Zone:         zone,
			StorageClass: "default",
		}
	}
	return state
}

func fakeState() State {
	state := newState()

	index := []string{"a", "b", "c", "d", "e"}
	nodes := indexed("node")
	zones := indexed("zone")

	for i := range nodes {
		x := index[i]
		state[x] = &XState{
			Image:        "smoke-mini",
			Node:         nodes[i],
			Zone:         zones[i],
			StorageClass: "default",
		}
	}
	return state
}

func fakePods(n int) []snapshot.Pod {
	index := []string{"a", "b", "c", "d", "e"}
	pods := make([]snapshot.Pod, n)
	nodes := indexed("node")

	for i := 0; i < n; i++ {
		pods[i] = snapshot.Pod{
			Index:   index[i],
			Node:    nodes[i],
			Phase:   v1.PodRunning,
			Created: time.Now(),
		}
	}

	return pods
}

func fakePod(i int) snapshot.Pod {
	index := []string{"a", "b", "c", "d", "e"}
	return snapshot.Pod{
		Index:   index[i],
		Node:    named("node", i),
		Phase:   v1.PodRunning,
		Created: time.Now(),
	}
}

func fakeNodes(n int) []snapshot.Node {
	nodes := make([]snapshot.Node, n)
	nodeNames := indexed("node")
	zones := indexed("zone")

	for i := 0; i < n; i++ {
		// use fakeNode
		nodes[i] = snapshot.Node{
			Name:        nodeNames[i],
			Zone:        zones[i],
			Schedulable: true,
		}
	}

	return nodes
}

func fakeNode(i int, zz ...string) snapshot.Node {
	name := named("node", i)
	zone := named("zone", i)
	if len(zz) == 1 {
		zone = zz[0]
	}
	return snapshot.Node{
		Name:        name,
		Zone:        zone,
		Schedulable: true,
	}
}
