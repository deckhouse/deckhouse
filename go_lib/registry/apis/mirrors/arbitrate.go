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

// Package mirrors arbitrates RegistryMirror objects: which of them take effect, and on which
// nodes.
//
// A node is served by one mirror at most. Two mirrors on one node would need an order between
// them that nobody declared, so the second is refused rather than merged, the same way two
// RegistryUpstream objects asking for one host are (go_lib/registry/apis/upstreams). And refused
// whole rather than on the shared nodes only: an object that applies to some of the nodes it
// selects says one thing in its spec and does another, and "accepted" on it would be true of a
// set of nodes its status could not name.
//
// A pure function of the objects and the nodes, so that its verdicts are exhaustively testable,
// and so that a later arbitrator for nodes without the registry agent can share it.
package mirrors

import (
	"fmt"
	"sort"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/registry/apis/upstreams"
)

// Node is what the arbitration needs to know about a node.
type Node struct {
	Name   string
	Labels map[string]string
}

// Verdict is the arbitration result for one RegistryMirror.
type Verdict struct {
	// Accepted reports whether the mirror was laid out for the nodes it selects.
	Accepted bool

	// Reason is the machine-readable reason, the Accepted condition's.
	Reason string

	// Message explains a rejection, and is empty when accepted.
	Message string

	// MatchedNodes is how many nodes the mirror is laid out for; zero when rejected.
	MatchedNodes int32
}

// Result is the outcome of arbitrating a whole set.
type Result struct {
	// Nodes maps a node name to the mirror laid out for it. A node no accepted mirror selects
	// is absent.
	Nodes map[string]registryv1alpha1.PrimaryMirror

	// Verdicts maps an object name to its verdict. Every input gets one.
	Verdicts map[string]Verdict
}

// Arbitrate decides which mirrors take effect, and where.
//
// Oldest first, with the object name as a tiebreak: a property of the objects rather than of
// the order they arrive in, so the winner never flips between passes. It also makes a new object
// the one refused when it overlaps an existing one, and keeps an accepted object accepted when
// a node is relabelled into the selector of a newer one. The reverse can happen — a node
// relabelled into an older object's selector while a newer one serves it refuses the newer
// object — and that is the rule doing what it says: the older object has the node.
func Arbitrate(mirrors []registryv1alpha1.RegistryMirror, nodes []Node) Result {
	result := Result{
		Nodes:    make(map[string]registryv1alpha1.PrimaryMirror),
		Verdicts: make(map[string]Verdict, len(mirrors)),
	}

	ordered := make([]*registryv1alpha1.RegistryMirror, len(mirrors))
	for i := range mirrors {
		ordered[i] = &mirrors[i]
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		left, right := ordered[i], ordered[j]
		if !left.CreationTimestamp.Equal(&right.CreationTimestamp) {
			return left.CreationTimestamp.Before(&right.CreationTimestamp)
		}
		return left.Name < right.Name
	})

	// Sorted, so the node a conflict names is the same on every pass.
	sortedNodes := make([]Node, len(nodes))
	copy(sortedNodes, nodes)
	sort.Slice(sortedNodes, func(i, j int) bool { return sortedNodes[i].Name < sortedNodes[j].Name })

	claimed := make(map[string]string)
	for _, mirror := range ordered {
		selector, verdict := validate(mirror)
		if selector == nil {
			result.Verdicts[mirror.Name] = verdict
			continue
		}

		var selected []string
		var contested []string
		for i := range sortedNodes {
			node := &sortedNodes[i]
			if !selector.Matches(labels.Set(node.Labels)) {
				continue
			}
			selected = append(selected, node.Name)
			if _, taken := claimed[node.Name]; taken {
				contested = append(contested, node.Name)
			}
		}

		if len(contested) > 0 {
			result.Verdicts[mirror.Name] = Verdict{
				Reason:  registryv1alpha1.ReasonNodeConflict,
				Message: conflictMessage(contested, claimed),
			}
			continue
		}

		for _, node := range selected {
			claimed[node] = mirror.Name
			result.Nodes[node] = registryv1alpha1.PrimaryMirror{
				Name:     mirror.Name,
				Upstream: *mirror.Spec.Source.DeepCopy(),
			}
		}
		result.Verdicts[mirror.Name] = Verdict{
			Accepted:     true,
			Reason:       registryv1alpha1.ReasonAccepted,
			MatchedNodes: int32(len(selected)),
		}
	}

	return result
}

// validate checks what does not depend on the nodes, and returns the selector when the object
// passes; a nil selector comes with the verdict that refuses it.
func validate(mirror *registryv1alpha1.RegistryMirror) (labels.Selector, Verdict) {
	invalid := func(format string, args ...any) (labels.Selector, Verdict) {
		return nil, Verdict{Reason: registryv1alpha1.ReasonInvalidSpec, Message: fmt.Sprintf(format, args...)}
	}

	selector, err := metav1.LabelSelectorAsSelector(&mirror.Spec.NodeSelector)
	if err != nil {
		return invalid("spec.nodeSelector: %v", err)
	}
	// An empty selector picks no node, where the label selector convention would pick every one.
	// See RegistryMirrorSpec.NodeSelector.
	if len(mirror.Spec.NodeSelector.MatchLabels) == 0 && len(mirror.Spec.NodeSelector.MatchExpressions) == 0 {
		selector = labels.Nothing()
	}

	source := &mirror.Spec.Source
	if source.Host == "" {
		return invalid("spec.source.host is empty, so the nodes would have no mirror to ask")
	}
	if err := upstreams.ValidateUpstreamAt(source, "spec.source"); err != nil {
		return invalid("%v", err)
	}

	for i, endpoint := range source.Endpoints() {
		at := "spec.source"
		if i > 0 {
			at = fmt.Sprintf("spec.source.mirrors[%d]", i-1)
		}

		// The agent's own addresses: the in-cluster name every image reference is built from, and
		// the loopback listener the runtime actually dials. A mirror at either would be the agent
		// asking itself, which then asks the mirror again.
		if what, own := agentAddresses[upstreams.NormalizeHost(endpoint.Host)]; own {
			return invalid("%s.host: %s is %s, which is not a mirror of anything", at, endpoint.Host, what)
		}

		// The prefix replaces the image set's own, and the image of an embedded module names that
		// prefix as its whole repository (`<prefix>@<digest>`). Without one it would be asked for
		// `/v2//manifests/<digest>`, a repository with no name, which no registry serves.
		if strings.Trim(endpoint.Path, "/") == "" {
			return invalid("%s.path is empty: the image set is served under a prefix, and without one the "+
				"images of embedded modules would be asked for a repository with no name", at)
		}
	}

	return selector, Verdict{}
}

// agentAddresses are the addresses the registry agent answers on, normalized, mapped to what each is.
// Spelled out for the reason InClusterHost is: this module keeps no dependency on go_lib/registry,
// whose AgentPort is 5001.
var agentAddresses = map[string]string{
	upstreams.NormalizeHost(upstreams.InClusterHost): "the in-cluster registry endpoint",
	"127.0.0.1:5001": "the registry agent's own listener",
	"localhost:5001": "the registry agent's own listener",
	"[::1]:5001":     "the registry agent's own listener",
}

// conflictMessage names the first contested node and who has it, and how many more there are.
func conflictMessage(contested []string, claimed map[string]string) string {
	first := contested[0]
	message := fmt.Sprintf("node %q is already served by RegistryMirror/%s", first, claimed[first])
	if more := len(contested) - 1; more > 0 {
		message += fmt.Sprintf(", and %d more node(s) selected here are served by older mirrors", more)
	}
	return message + "; a node is served by one mirror at most, and the older object keeps it"
}
