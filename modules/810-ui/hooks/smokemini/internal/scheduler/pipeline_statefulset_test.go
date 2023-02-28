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

package scheduler

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

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
	if assert.ErrorIs(t, err, ErrSkip, "should return skipping error") {
		assert.Equal(t, "", x, "should be not selected")
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

// nolint: unparam
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

func withDefaultStorageClass(s State) State {
	for _, xs := range s {
		xs.StorageClass = snapshot.DefaultStorageClass
	}
	return s
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
			Ready:   true,
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
		Ready:   true,
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
