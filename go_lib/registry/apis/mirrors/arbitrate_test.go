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

package mirrors

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/registry/apis/upstreams"
)

var epoch = time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)

const groupLabel = "node.deckhouse.io/group"

func mirrorObj(name string, ageMinutes int, group, host string) registryv1alpha1.RegistryMirror {
	selector := metav1.LabelSelector{}
	if group != "" {
		selector.MatchLabels = map[string]string{groupLabel: group}
	}
	return registryv1alpha1.RegistryMirror{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			CreationTimestamp: metav1.NewTime(epoch.Add(time.Duration(ageMinutes) * time.Minute)),
		},
		Spec: registryv1alpha1.RegistryMirrorSpec{
			NodeSelector: selector,
			Source: registryv1alpha1.Upstream{
				Endpoint: registryv1alpha1.Endpoint{
					Scheme: registryv1alpha1.SchemeHTTPS,
					Host:   host,
					Path:   "/d8-proxy/deckhouse/ee",
				},
			},
		},
	}
}

func node(name, group string) Node {
	return Node{Name: name, Labels: map[string]string{groupLabel: group}}
}

func TestArbitrateLaysOutForSelectedNodesOnly(t *testing.T) {
	result := Arbitrate(
		[]registryv1alpha1.RegistryMirror{mirrorObj("eu", 0, "gpu-eu", "harbor.eu.example.com")},
		[]Node{node("master-0", "master"), node("gpu-eu-0", "gpu-eu"), node("gpu-eu-1", "gpu-eu")},
	)

	require.Equal(t, Verdict{Accepted: true, Reason: registryv1alpha1.ReasonAccepted, MatchedNodes: 2},
		result.Verdicts["eu"])
	require.Len(t, result.Nodes, 2)
	assert.NotContains(t, result.Nodes, "master-0")
	assert.Equal(t, "eu", result.Nodes["gpu-eu-0"].Name)
	assert.Equal(t, "harbor.eu.example.com", result.Nodes["gpu-eu-1"].Host)
	assert.Equal(t, "/d8-proxy/deckhouse/ee", result.Nodes["gpu-eu-1"].Path)
}

func TestArbitrateEmptySelectorPicksNoNode(t *testing.T) {
	result := Arbitrate(
		[]registryv1alpha1.RegistryMirror{mirrorObj("empty", 0, "", "harbor.example.com")},
		[]Node{node("a", "x"), node("b", "y")},
	)

	assert.Equal(t, Verdict{Accepted: true, Reason: registryv1alpha1.ReasonAccepted}, result.Verdicts["empty"])
	assert.Empty(t, result.Nodes)
}

func TestArbitrateAnEmptySelectorConflictsWithNothing(t *testing.T) {
	// It holds no node, so a mirror created after it takes the nodes it selects.
	result := Arbitrate(
		[]registryv1alpha1.RegistryMirror{
			mirrorObj("empty", 0, "", "harbor.example.com"),
			mirrorObj("eu", 5, "gpu-eu", "harbor.eu.example.com"),
		},
		[]Node{node("gpu-eu-0", "gpu-eu")},
	)

	assert.True(t, result.Verdicts["empty"].Accepted)
	assert.True(t, result.Verdicts["eu"].Accepted)
	assert.Equal(t, "eu", result.Nodes["gpu-eu-0"].Name)
}

func TestArbitrateAcceptsAMirrorSelectingNoNode(t *testing.T) {
	// Nodes of a group that is scaled to zero still have a mirror waiting for them.
	result := Arbitrate(
		[]registryv1alpha1.RegistryMirror{mirrorObj("eu", 0, "gpu-eu", "harbor.eu.example.com")},
		[]Node{node("master-0", "master")},
	)

	assert.Equal(t, Verdict{Accepted: true, Reason: registryv1alpha1.ReasonAccepted}, result.Verdicts["eu"])
	assert.Empty(t, result.Nodes)
}

func TestArbitrateRefusesTheNewerOfTwoMirrorsOnOneNodeWhole(t *testing.T) {
	nodes := []Node{
		node("gpu-eu-0", "gpu-eu"),
		node("gpu-eu-1", "gpu-eu"),
		node("gpu-us-0", "gpu-us"),
	}
	older := mirrorObj("eu", 0, "gpu-eu", "harbor.eu.example.com")
	// Selects every node of any group: the EU ones are contested, the US one is not, and it is
	// refused whole.
	newer := mirrorObj("everywhere", 5, "", "harbor.example.com")
	newer.Spec.NodeSelector.MatchExpressions = []metav1.LabelSelectorRequirement{{
		Key: groupLabel, Operator: metav1.LabelSelectorOpExists,
	}}

	// Listed newest first, to show the order is the objects' own and not the list's.
	result := Arbitrate([]registryv1alpha1.RegistryMirror{newer, older}, nodes)

	assert.True(t, result.Verdicts["eu"].Accepted)
	refused := result.Verdicts["everywhere"]
	assert.False(t, refused.Accepted)
	assert.Equal(t, registryv1alpha1.ReasonNodeConflict, refused.Reason)
	assert.Zero(t, refused.MatchedNodes)
	assert.Contains(t, refused.Message, `node "gpu-eu-0" is already served by RegistryMirror/eu`)
	assert.Contains(t, refused.Message, "1 more node(s)")

	assert.Equal(t, "eu", result.Nodes["gpu-eu-0"].Name)
	assert.Equal(t, "eu", result.Nodes["gpu-eu-1"].Name)
	assert.NotContains(t, result.Nodes, "gpu-us-0", "a refused mirror is not laid out on its uncontested nodes either")
}

func TestArbitrateBreaksATieByName(t *testing.T) {
	result := Arbitrate(
		[]registryv1alpha1.RegistryMirror{
			mirrorObj("b", 0, "gpu-eu", "harbor-b.example.com"),
			mirrorObj("a", 0, "gpu-eu", "harbor-a.example.com"),
		},
		[]Node{node("gpu-eu-0", "gpu-eu")},
	)

	assert.True(t, result.Verdicts["a"].Accepted)
	assert.False(t, result.Verdicts["b"].Accepted)
}

func TestArbitrateDisjointMirrorsAreBothAccepted(t *testing.T) {
	result := Arbitrate(
		[]registryv1alpha1.RegistryMirror{
			mirrorObj("eu", 0, "gpu-eu", "harbor.eu.example.com"),
			mirrorObj("us", 1, "gpu-us", "harbor.us.example.com"),
		},
		[]Node{node("gpu-eu-0", "gpu-eu"), node("gpu-us-0", "gpu-us")},
	)

	assert.True(t, result.Verdicts["eu"].Accepted)
	assert.True(t, result.Verdicts["us"].Accepted)
	assert.Equal(t, "harbor.us.example.com", result.Nodes["gpu-us-0"].Host)
}

func TestArbitrateRefusesAnInvalidSpec(t *testing.T) {
	badSelector := mirrorObj("bad-selector", 0, "", "harbor.example.com")
	badSelector.Spec.NodeSelector.MatchExpressions = []metav1.LabelSelectorRequirement{{
		Key: groupLabel, Operator: "Near", Values: []string{"gpu"},
	}}

	noHost := mirrorObj("no-host", 0, "", "")

	badMirror := mirrorObj("bad-mirror", 0, "", "harbor.example.com")
	badMirror.Spec.Source.Mirrors = []registryv1alpha1.Endpoint{{Host: "not a host"}}

	loop := mirrorObj("loop", 0, "", upstreams.InClusterHost)

	self := mirrorObj("self", 0, "", "harbor.example.com")
	self.Spec.Source.Mirrors = []registryv1alpha1.Endpoint{{Host: "127.0.0.1:5001", Path: "/d8"}}

	noPath := mirrorObj("no-path", 0, "", "harbor.example.com")
	noPath.Spec.Source.Path = "/"

	noMirrorPath := mirrorObj("no-mirror-path", 0, "", "harbor.example.com")
	noMirrorPath.Spec.Source.Mirrors = []registryv1alpha1.Endpoint{{Host: "harbor-2.example.com"}}

	result := Arbitrate(
		[]registryv1alpha1.RegistryMirror{badSelector, noHost, badMirror, loop, self, noPath, noMirrorPath},
		[]Node{node("a", "gpu")},
	)

	for name, field := range map[string]string{
		"bad-selector":   "spec.nodeSelector",
		"no-host":        "spec.source.host",
		"bad-mirror":     "spec.source.mirrors[0]",
		"loop":           "in-cluster registry endpoint",
		"self":           "spec.source.mirrors[0].host: 127.0.0.1:5001 is the registry agent's own listener",
		"no-path":        "spec.source.path is empty",
		"no-mirror-path": "spec.source.mirrors[0].path is empty",
	} {
		verdict := result.Verdicts[name]
		assert.False(t, verdict.Accepted, name)
		assert.Equal(t, registryv1alpha1.ReasonInvalidSpec, verdict.Reason, name)
		assert.Contains(t, verdict.Message, field, name)
	}
	assert.Empty(t, result.Nodes, "an invalid mirror claims no node")
}

func TestArbitrateAnInvalidOlderMirrorDoesNotBlockANewerOne(t *testing.T) {
	invalid := mirrorObj("invalid", 0, "gpu-eu", "")
	valid := mirrorObj("valid", 5, "gpu-eu", "harbor.eu.example.com")

	result := Arbitrate([]registryv1alpha1.RegistryMirror{invalid, valid}, []Node{node("gpu-eu-0", "gpu-eu")})

	assert.True(t, result.Verdicts["valid"].Accepted)
	assert.Equal(t, "valid", result.Nodes["gpu-eu-0"].Name)
}

func TestArbitrateLayoutsDoNotShareTheObjectsMemory(t *testing.T) {
	mirror := mirrorObj("eu", 0, "gpu-eu", "harbor.eu.example.com")
	mirror.Spec.Source.Mirrors = []registryv1alpha1.Endpoint{{Host: "harbor-2.eu.example.com", Path: "/d8"}}
	objects := []registryv1alpha1.RegistryMirror{mirror}

	result := Arbitrate(objects, []Node{node("gpu-eu-0", "gpu-eu"), node("gpu-eu-1", "gpu-eu")})

	laidOut := result.Nodes["gpu-eu-0"]
	laidOut.Mirrors[0].Host = "changed"
	assert.Equal(t, "harbor-2.eu.example.com", objects[0].Spec.Source.Mirrors[0].Host)
	assert.Equal(t, "harbor-2.eu.example.com", result.Nodes["gpu-eu-1"].Mirrors[0].Host)
}
