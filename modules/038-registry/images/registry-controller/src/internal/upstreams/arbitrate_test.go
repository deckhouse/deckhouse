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

package upstreams

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

var epoch = time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

func upstreamObj(name, match, host string, ageMinutes int) registryv1alpha1.RegistryUpstream {
	return registryv1alpha1.RegistryUpstream{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			CreationTimestamp: metav1.NewTime(epoch.Add(time.Duration(ageMinutes) * time.Minute)),
		},
		Spec: registryv1alpha1.RegistryUpstreamSpec{
			Match: match,
			Upstream: registryv1alpha1.Upstream{
				Endpoint: registryv1alpha1.Endpoint{
					Scheme: registryv1alpha1.SchemeHTTPS,
					Host:   host,
					Path:   "/virt",
				},
			},
		},
	}
}

func primaryUpstream() *registryv1alpha1.Upstream {
	return &registryv1alpha1.Upstream{
		Endpoint: registryv1alpha1.Endpoint{
			Scheme: registryv1alpha1.SchemeHTTPS,
			Host:   "registry.deckhouse.io",
			Path:   "/deckhouse/ee",
		},
		Mirrors: []registryv1alpha1.Endpoint{
			{Scheme: registryv1alpha1.SchemeHTTPS, Host: "registry-mirror.example.com"},
		},
	}
}

func TestArbitrateAcceptsDistinctMatches(t *testing.T) {
	got := Arbitrate(primaryUpstream(), []registryv1alpha1.RegistryUpstream{
		upstreamObj("virtualization-images", "images.virtualization.example.com", "registry-vendor.example.com", 0),
		upstreamObj("ghcr-proxy", "ghcr.io", "proxy.example.com", 5),
	})

	require.Len(t, got.Routes, 2)
	assert.True(t, got.Verdicts["virtualization-images"].Accepted)
	assert.True(t, got.Verdicts["ghcr-proxy"].Accepted)
	assert.Empty(t, got.Verdicts["ghcr-proxy"].Conflict)

	// Ordered oldest first, so the compiled layout is stable between reconciles.
	assert.Equal(t, "images.virtualization.example.com", got.Routes[0].Match)
	assert.Equal(t, "ghcr.io", got.Routes[1].Match)
	assert.Equal(t, "registry-vendor.example.com/virt", got.Routes[0].Address())
}

// TestArbitrateOldestWinsAMatchCollision pins the tiebreak down. Whoever got
// there first keeps the match, and the loser is told who holds it.
func TestArbitrateOldestWinsAMatchCollision(t *testing.T) {
	got := Arbitrate(primaryUpstream(), []registryv1alpha1.RegistryUpstream{
		upstreamObj("newcomer", "ghcr.io", "newcomer.example.com", 10),
		upstreamObj("incumbent", "ghcr.io", "incumbent.example.com", 0),
	})

	require.Len(t, got.Routes, 1)
	assert.Equal(t, "incumbent.example.com/virt", got.Routes[0].Address())

	assert.True(t, got.Verdicts["incumbent"].Accepted)

	loser := got.Verdicts["newcomer"]
	assert.False(t, loser.Accepted)
	assert.Contains(t, loser.Conflict, "already claimed by RegistryUpstream/incumbent")
	assert.Equal(t, registryv1alpha1.ReasonMatchConflict, loser.Reason)
}

// TestArbitrateIsStableRegardlessOfInputOrder is the point of the whole rule: the
// winner must be a property of the objects, not of the order the API returned
// them in. Otherwise the layout of every node would be rewritten on a whim.
func TestArbitrateIsStableRegardlessOfInputOrder(t *testing.T) {
	older := upstreamObj("beta", "ghcr.io", "beta.example.com", 0)
	newer := upstreamObj("alpha", "ghcr.io", "alpha.example.com", 10)

	forward := Arbitrate(primaryUpstream(), []registryv1alpha1.RegistryUpstream{older, newer})
	backward := Arbitrate(primaryUpstream(), []registryv1alpha1.RegistryUpstream{newer, older})

	assert.Equal(t, forward.Routes, backward.Routes)
	assert.True(t, forward.Verdicts["beta"].Accepted)
	assert.True(t, backward.Verdicts["beta"].Accepted)
}

func TestArbitrateBreaksTiesByName(t *testing.T) {
	// Two objects created in the same instant: the timestamp has second precision,
	// so this is not a corner case but the normal outcome of applying a directory.
	got := Arbitrate(primaryUpstream(), []registryv1alpha1.RegistryUpstream{
		upstreamObj("zulu", "ghcr.io", "zulu.example.com", 0),
		upstreamObj("alpha", "ghcr.io", "alpha.example.com", 0),
	})

	require.Len(t, got.Routes, 1)
	assert.Equal(t, "alpha.example.com/virt", got.Routes[0].Address())
	assert.True(t, got.Verdicts["alpha"].Accepted)
	assert.False(t, got.Verdicts["zulu"].Accepted)
}

func TestArbitrateRejectsShadowingTheReservedHosts(t *testing.T) {
	tests := []struct {
		name        string
		match       string
		wantMessage string
	}{
		{
			name:        "the primary upstream",
			match:       "registry.deckhouse.io",
			wantMessage: "the primary upstream of Deckhouse components",
		},
		{
			name:        "a mirror of the primary upstream",
			match:       "registry-mirror.example.com",
			wantMessage: "a mirror of the primary upstream",
		},
		{
			name:        "the in-cluster endpoint",
			match:       constant.Host,
			wantMessage: "the in-cluster registry endpoint",
		},
		{
			name:        "the primary upstream, capitalised",
			match:       "Registry.Deckhouse.IO",
			wantMessage: "the primary upstream of Deckhouse components",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Arbitrate(primaryUpstream(), []registryv1alpha1.RegistryUpstream{
				upstreamObj("hijack", tt.match, "attacker.example.com", 0),
			})

			assert.Empty(t, got.Routes)

			verdict := got.Verdicts["hijack"]
			assert.False(t, verdict.Accepted)
			assert.Contains(t, verdict.Conflict, tt.wantMessage)
			assert.Equal(t, registryv1alpha1.ReasonShadowsPrimary, verdict.Reason)
		})
	}
}

// TestArbitrateProtectsTheInClusterEndpointWithoutAPrimary covers the air-gapped
// cluster: there is no primary upstream to shadow, but the endpoint every image
// reference points at still must not be hijackable.
func TestArbitrateProtectsTheInClusterEndpointWithoutAPrimary(t *testing.T) {
	got := Arbitrate(nil, []registryv1alpha1.RegistryUpstream{
		upstreamObj("hijack", constant.Host, "attacker.example.com", 0),
		upstreamObj("legitimate", "ghcr.io", "proxy.example.com", 1),
	})

	require.Len(t, got.Routes, 1)
	assert.Equal(t, "ghcr.io", got.Routes[0].Match)
	assert.False(t, got.Verdicts["hijack"].Accepted)
}

func TestArbitrateRejectsIncompleteSpecs(t *testing.T) {
	empty := upstreamObj("no-match", "", "vendor.example.com", 0)
	nowhere := upstreamObj("no-upstream", "ghcr.io", "", 1)

	got := Arbitrate(primaryUpstream(), []registryv1alpha1.RegistryUpstream{empty, nowhere})

	assert.Empty(t, got.Routes)
	assert.Equal(t, registryv1alpha1.ReasonInvalidSpec, got.Verdicts["no-match"].Reason)
	assert.Contains(t, got.Verdicts["no-upstream"].Conflict, "nowhere to go")
}

// TestArbitrateRejectionDoesNotFreeTheMatch checks that a rejected claim does not
// let a third object take the host: the incumbent keeps it.
func TestArbitrateRejectionDoesNotFreeTheMatch(t *testing.T) {
	got := Arbitrate(primaryUpstream(), []registryv1alpha1.RegistryUpstream{
		upstreamObj("incumbent", "ghcr.io", "incumbent.example.com", 0),
		upstreamObj("second", "ghcr.io", "second.example.com", 1),
		upstreamObj("third", "ghcr.io", "third.example.com", 2),
	})

	require.Len(t, got.Routes, 1)
	assert.Equal(t, "incumbent.example.com/virt", got.Routes[0].Address())
	assert.False(t, got.Verdicts["second"].Accepted)
	assert.False(t, got.Verdicts["third"].Accepted)
	assert.Contains(t, got.Verdicts["third"].Conflict, "RegistryUpstream/incumbent")
}

func TestArbitrateEveryInputGetsAVerdict(t *testing.T) {
	got := Arbitrate(primaryUpstream(), []registryv1alpha1.RegistryUpstream{
		upstreamObj("accepted", "ghcr.io", "proxy.example.com", 0),
		upstreamObj("rejected", "registry.deckhouse.io", "attacker.example.com", 1),
		upstreamObj("invalid", "", "", 2),
	})

	// A silent omission would leave an object with a stale status forever, so the
	// caller needs a verdict for every one of them.
	assert.Len(t, got.Verdicts, 3)
	for _, name := range []string{"accepted", "rejected", "invalid"} {
		assert.Contains(t, got.Verdicts, name)
	}
}

func TestArbitrateDoesNotMutateItsInputs(t *testing.T) {
	// The objects come out of the informer cache, which is shared process-wide.
	input := []registryv1alpha1.RegistryUpstream{
		upstreamObj("virtualization", "images.example.com", "vendor.example.com", 0),
	}
	input[0].Spec.Upstream.Mirrors = []registryv1alpha1.Endpoint{{Host: "mirror.example.com"}}
	before := input[0].DeepCopy()

	got := Arbitrate(primaryUpstream(), input)

	require.Len(t, got.Routes, 1)
	got.Routes[0].Host = "mutated"
	got.Routes[0].Mirrors[0].Host = "mutated"

	assert.Equal(t, before.Spec, input[0].Spec)
}

func TestArbitrateWithoutUpstreams(t *testing.T) {
	got := Arbitrate(primaryUpstream(), nil)
	assert.Empty(t, got.Routes)
	assert.Empty(t, got.Verdicts)
}
