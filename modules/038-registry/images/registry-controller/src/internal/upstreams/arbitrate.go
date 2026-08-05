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
// This is the multi-writer half of the contract. Anyone may create a
// RegistryUpstream, which means two writers can ask for the same intercepted
// host, or ask to intercept a host that Deckhouse itself pulls from. Neither can
// be merged silently: the first would make the winner depend on reconcile order,
// and the second would quietly redirect Deckhouse component images to somebody
// else's registry.
package upstreams

import (
	"fmt"
	"sort"
	"strings"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
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
	// layout. Ordered deterministically.
	Routes []registryv1alpha1.Route

	// Verdicts maps an object name to its verdict. Every input gets one.
	Verdicts map[string]Verdict
}

// Arbitrate decides which additional upstreams take effect.
//
// Conflicts are resolved oldest-first, with the object name as a tiebreak. The
// rule has to be a property of the objects rather than of the order they arrive
// in: anything else would make the winner flip between reconciliations, and every
// flip rewrites the layout of every node.
func Arbitrate(
	primary *registryv1alpha1.Upstream, upstreams []registryv1alpha1.RegistryUpstream,
) Result {
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

	reserved := reservedHosts(primary)
	claimed := make(map[string]string, len(ordered))

	for _, upstream := range ordered {
		match := normalizeHost(upstream.Spec.Match)

		if match == "" {
			result.Verdicts[upstream.Name] = Verdict{
				Conflict: "spec.match is empty, so there is no host to intercept",
				Reason:   registryv1alpha1.ReasonInvalidSpec,
			}
			continue
		}

		if what, taken := reserved[match]; taken {
			// Intercepting a host Deckhouse pulls from would redirect platform images
			// to a registry nobody vetted.
			result.Verdicts[upstream.Name] = Verdict{
				Conflict: fmt.Sprintf("spec.match %q shadows %s", upstream.Spec.Match, what),
				Reason:   registryv1alpha1.ReasonShadowsPrimary,
			}
			continue
		}

		if owner, taken := claimed[match]; taken {
			result.Verdicts[upstream.Name] = Verdict{
				Conflict: fmt.Sprintf("spec.match %q is already claimed by RegistryUpstream/%s",
					upstream.Spec.Match, owner),
				Reason: registryv1alpha1.ReasonMatchConflict,
			}
			continue
		}

		if upstream.Spec.Upstream.Host == "" {
			result.Verdicts[upstream.Name] = Verdict{
				Conflict: "spec.upstream.host is empty, so intercepted pulls would have nowhere to go",
				Reason:   registryv1alpha1.ReasonInvalidSpec,
			}
			continue
		}

		claimed[match] = upstream.Name
		result.Verdicts[upstream.Name] = Verdict{Accepted: true, Reason: registryv1alpha1.ReasonAccepted}
		result.Routes = append(result.Routes, compile(upstream))
	}

	return result
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

// reservedHosts are the hosts an additional upstream may not intercept, mapped to
// a description used in the rejection message.
func reservedHosts(primary *registryv1alpha1.Upstream) map[string]string {
	reserved := map[string]string{
		// The in-cluster endpoint every node and every image reference points at.
		// Intercepting it would break the whole pull path, not merely one registry.
		normalizeHost(constant.Host): "the in-cluster registry endpoint",
	}

	if primary == nil {
		return reserved
	}

	reserved[normalizeHost(primary.Host)] = "the primary upstream of Deckhouse components"
	for i := range primary.Mirrors {
		host := normalizeHost(primary.Mirrors[i].Host)
		if _, exists := reserved[host]; !exists {
			reserved[host] = "a mirror of the primary upstream"
		}
	}

	return reserved
}

// normalizeHost makes host comparison case-insensitive, since DNS names are and
// a writer could otherwise claim a reserved host just by capitalising it.
func normalizeHost(host string) string {
	return strings.ToLower(strings.TrimSpace(host))
}
