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

// Package upstreams arbitrates the additional registries declared by modules,
// administrators and users, and compiles the accepted ones into per-node routes.
//
// Two arbitrators use it, never at once: registry-controller while the registry agent
// owns the nodes' registry.d, node-controller otherwise. Both call Arbitrate with the
// objects alone, and every rule that does not depend on a node — the shape of each
// field, the reserved hosts, the contest for a match and the size budget — is decided
// here and nowhere else. That is what makes an object's verdict the same whichever of
// the two arbitrates it. What a particular node can or cannot apply is the node's
// business and never turns a verdict.
//
// This is the multi-writer half of the contract. Anyone may create a
// RegistryUpstream, which means two writers can ask for the same intercepted
// host, or ask to intercept a host that Deckhouse itself pulls from. Neither can
// be merged silently: the first would make the winner depend on reconcile order,
// and the second would quietly redirect Deckhouse component images to somebody
// else's registry.
package upstreams

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

// InClusterHost is the in-cluster registry endpoint, go_lib/registry/const.Host. Spelled out
// because this module keeps no dependency on go_lib/registry; registry-controller, which
// imports both, asserts that they agree.
const InClusterHost = "registry.d8-system.svc:5001"

// The limits of the accepted set as a whole. Every accepted object is copied into every
// node's configuration object (NodeConfig spec.registry.upstreams, RegistryNode
// spec.additionalRoutes), and the API server refuses such an object whole once it is too
// large: one object too many would cost every node its configuration.
const (
	// MaxRoutes is how many routes a node takes, the maxItems of
	// NodeConfig spec.registry.upstreams.
	MaxRoutes = 256
	// MaxTotalSpecBytes is how large the accepted specs may be together, each measured
	// by SpecSize.
	MaxTotalSpecBytes = 256 << 10
)

// Verdict is the arbitration result for one RegistryUpstream.
type Verdict struct {
	// Accepted reports whether the object was compiled into the node layouts.
	Accepted bool

	// Conflict explains a rejection, and is empty when accepted.
	Conflict string

	// Reason is the machine-readable counterpart of Conflict.
	Reason string
}

// Result is the outcome of arbitrating a whole set.
type Result struct {
	// Routes are the accepted upstreams, ready to be compiled into every node
	// layout, in arbitration order: oldest first, then by name.
	Routes []registryv1alpha1.Route

	// Accepted names the object each of Routes comes from, index for index.
	Accepted []string

	// Verdicts maps an object name to its verdict. Every input gets one.
	Verdicts map[string]Verdict

	// SpecBytes is the size of the accepted specs together, as SpecSize measures
	// them. A caller adding routes of its own counts them on top of it.
	SpecBytes int
}

// SpecSize is what one object costs against MaxTotalSpecBytes: its spec, JSON-encoded.
func SpecSize(spec *registryv1alpha1.RegistryUpstreamSpec) int {
	encoded, err := json.Marshal(spec)
	if err != nil {
		// Plain strings and slices only; cannot fail. Counted as over any budget
		// rather than as free, should it ever.
		return MaxTotalSpecBytes + 1
	}
	return len(encoded)
}

// Arbitrate decides which additional upstreams take effect.
//
// Conflicts are resolved oldest-first, with the object name as a tiebreak. The
// rule has to be a property of the objects rather than of the order they arrive
// in: anything else would make the winner flip between reconciliations, and every
// flip rewrites the layout of every node. The same order decides who fits into the
// budget: an object is accepted only while the accepted set stays within MaxRoutes
// and MaxTotalSpecBytes, so a new object never pushes out an older one.
func Arbitrate(primary *registryv1alpha1.Upstream, upstreams []registryv1alpha1.RegistryUpstream) Result {
	result := Result{Verdicts: make(map[string]Verdict, len(upstreams))}

	ordered := make([]*registryv1alpha1.RegistryUpstream, len(upstreams))
	for i := range upstreams {
		ordered[i] = &upstreams[i]
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		left, right := ordered[i], ordered[j]
		if !left.CreationTimestamp.Equal(&right.CreationTimestamp) {
			return left.CreationTimestamp.Before(&right.CreationTimestamp)
		}
		return left.Name < right.Name
	})

	reserved := ReservedHosts(primary)
	claimed := make(map[string]string, len(ordered))

	for _, upstream := range ordered {
		verdict := judge(upstream, reserved, claimed, &result)
		result.Verdicts[upstream.Name] = verdict
		if !verdict.Accepted {
			continue
		}
		claimed[NormalizeHost(upstream.Spec.Match)] = upstream.Name
		result.Routes = append(result.Routes, compile(upstream))
		result.Accepted = append(result.Accepted, upstream.Name)
		result.SpecBytes += SpecSize(&upstream.Spec)
	}

	return result
}

// judge gives one object its verdict against what was accepted before it.
func judge(
	upstream *registryv1alpha1.RegistryUpstream, reserved, claimed map[string]string, accepted *Result,
) Verdict {
	invalid := func(format string, args ...any) Verdict {
		return Verdict{Conflict: fmt.Sprintf(format, args...), Reason: registryv1alpha1.ReasonInvalidSpec}
	}

	if upstream.Spec.Match == "" {
		return invalid("spec.match is empty, so there is no host to intercept")
	}
	if err := ValidateMatch(upstream.Spec.Match); err != nil {
		return invalid("spec.match: %v", err)
	}

	match := NormalizeHost(upstream.Spec.Match)
	if what, taken := reserved[match]; taken {
		// Intercepting a host Deckhouse pulls from would redirect platform images
		// to a registry nobody vetted.
		return Verdict{
			Conflict: fmt.Sprintf("spec.match %q shadows %s", upstream.Spec.Match, what),
			Reason:   registryv1alpha1.ReasonShadowsPrimary,
		}
	}
	if owner, taken := claimed[match]; taken {
		return Verdict{
			Conflict: fmt.Sprintf("spec.match %q is already claimed by RegistryUpstream/%s", upstream.Spec.Match, owner),
			Reason:   registryv1alpha1.ReasonMatchConflict,
		}
	}

	if upstream.Spec.Upstream.Host == "" {
		return invalid("spec.upstream.host is empty, so intercepted pulls would have nowhere to go")
	}
	if err := ValidateUpstream(&upstream.Spec.Upstream); err != nil {
		return invalid("%v", err)
	}

	if len(accepted.Routes) >= MaxRoutes {
		return Verdict{
			Conflict: fmt.Sprintf("%d RegistryUpstream objects are accepted already, the most a node takes; "+
				"objects are accepted oldest first", MaxRoutes),
			Reason: registryv1alpha1.ReasonLimitExceeded,
		}
	}
	if size := SpecSize(&upstream.Spec); accepted.SpecBytes+size > MaxTotalSpecBytes {
		return Verdict{
			Conflict: fmt.Sprintf("the spec takes %d bytes, and the accepted RegistryUpstream objects take %d of the %d bytes "+
				"(256 KiB) a node takes for all of them together; objects are accepted oldest first",
				size, accepted.SpecBytes, MaxTotalSpecBytes),
			Reason: registryv1alpha1.ReasonLimitExceeded,
		}
	}

	return Verdict{Accepted: true, Reason: registryv1alpha1.ReasonAccepted}
}

// compile turns an accepted upstream into the flat route the agent applies.
func compile(upstream *registryv1alpha1.RegistryUpstream) registryv1alpha1.Route {
	spec := upstream.Spec.Upstream.DeepCopy()

	return registryv1alpha1.Route{
		Match:    upstream.Spec.Match,
		Endpoint: spec.Endpoint,
		Mirrors:  spec.Mirrors,
	}
}

// ReservedHosts are the hosts an additional upstream may not intercept, normalized and
// mapped to a description used in the rejection message.
func ReservedHosts(primary *registryv1alpha1.Upstream) map[string]string {
	reserved := map[string]string{
		// The in-cluster endpoint every node and every image reference points at.
		// Intercepting it would break the whole pull path, not merely one registry.
		NormalizeHost(InClusterHost): "the in-cluster registry endpoint",
	}

	if primary == nil {
		return reserved
	}

	if host := NormalizeHost(primary.Host); host != "" {
		reserved[host] = "the primary upstream of Deckhouse components"
	}
	for i := range primary.Mirrors {
		host := NormalizeHost(primary.Mirrors[i].Host)
		if _, exists := reserved[host]; !exists && host != "" {
			reserved[host] = "a mirror of the primary upstream"
		}
	}

	return reserved
}

// NormalizeHost makes host comparison case-insensitive, since DNS names are and
// a writer could otherwise claim a reserved host just by capitalising it.
func NormalizeHost(host string) string {
	return strings.ToLower(strings.TrimSpace(host))
}
