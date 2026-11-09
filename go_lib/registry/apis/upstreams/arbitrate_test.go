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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
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
			match:       InClusterHost,
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
		upstreamObj("hijack", InClusterHost, "attacker.example.com", 0),
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

// testCertificate is a self-signed certificate made for the test run, PEM-encoded.
func testCertificate(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "registry-vendor.example.com"},
		NotBefore:    epoch,
		NotAfter:     epoch.Add(24 * time.Hour),
		IsCA:         true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

// TestArbitrateJudgesEveryFieldTheSameForEveryReader is the one place the shape of an
// object is decided. Each of these was accepted before and reached the nodes, where
// nodelet, bashible or containerd refused it one node at a time — or, in the case of a
// bad CA, took it and failed every pull of the match.
func TestArbitrateJudgesEveryFieldTheSameForEveryReader(t *testing.T) {
	ca := testCertificate(t)
	long := func(n int) string { return strings.Repeat("a", n) }

	tests := []struct {
		name    string
		mutate  func(*registryv1alpha1.RegistryUpstream)
		wantErr string
	}{
		{name: "a match outside the pattern", mutate: func(u *registryv1alpha1.RegistryUpstream) { u.Spec.Match = "ghcr.io/library" }, wantErr: "spec.match"},
		{name: "a match over 255 characters", mutate: func(u *registryv1alpha1.RegistryUpstream) { u.Spec.Match = long(256) }, wantErr: "longer than 255"},
		{name: "an unknown scheme", mutate: func(u *registryv1alpha1.RegistryUpstream) { u.Spec.Upstream.Scheme = "https" }, wantErr: "neither HTTP nor HTTPS"},
		{name: "a host that is no host", mutate: func(u *registryv1alpha1.RegistryUpstream) { u.Spec.Upstream.Host = "vendor.example.com/v2" }, wantErr: "is not a registry host"},
		{name: "a host over 255 characters", mutate: func(u *registryv1alpha1.RegistryUpstream) { u.Spec.Upstream.Host = long(256) }, wantErr: "longer than 255"},
		{name: "a match port over 65535", mutate: func(u *registryv1alpha1.RegistryUpstream) { u.Spec.Match = "a.example.com:99999" }, wantErr: "port 99999"},
		{name: "a match port of 0", mutate: func(u *registryv1alpha1.RegistryUpstream) { u.Spec.Match = "a.example.com:0" }, wantErr: "out of range"},
		{name: "a host port of 0", mutate: func(u *registryv1alpha1.RegistryUpstream) { u.Spec.Upstream.Host = "up.example.com:00000" }, wantErr: "out of range"},
		{name: "an IPv6 address without a port", mutate: func(u *registryv1alpha1.RegistryUpstream) { u.Spec.Upstream.Host = "[2001:db8::2]" }, wantErr: "has no port"},
		{name: "brackets around IPv4", mutate: func(u *registryv1alpha1.RegistryUpstream) { u.Spec.Upstream.Host = "[1.2.3.4]:5000" }, wantErr: "not an IPv6 address"},
		{name: "auth.auth with a line break", mutate: func(u *registryv1alpha1.RegistryUpstream) {
			u.Spec.Upstream.Auth = &registryv1alpha1.Auth{Auth: "dXNlcm5hbWU6cGFzc3dvcmQ=\n"}
		}, wantErr: "not base64"},
		{name: "a path outside the reference grammar", mutate: func(u *registryv1alpha1.RegistryUpstream) { u.Spec.Upstream.Path = "/Virt" }, wantErr: "is not a repository path"},
		{name: "a path over 1024 characters", mutate: func(u *registryv1alpha1.RegistryUpstream) { u.Spec.Upstream.Path = long(1025) }, wantErr: "longer than 1024"},
		{name: "a CA that is not PEM", mutate: func(u *registryv1alpha1.RegistryUpstream) { u.Spec.Upstream.CA = "VENDOR-CA" }, wantErr: "not a PEM bundle"},
		{name: "a CA with a key in it", mutate: func(u *registryv1alpha1.RegistryUpstream) {
			u.Spec.Upstream.CA = ca + "-----BEGIN PRIVATE KEY-----\nZGVja2hvdXNlLXRlc3QtcHJpdmF0ZS1rZXktbm90LWEtcmVhbC1zZWNyZXQtMDEyMzQ1Njc4OWFiY2RlZg==\n-----END PRIVATE KEY-----\n"
		}, wantErr: `type "PRIVATE KEY"`},
		{name: "a CA whose certificate does not parse", mutate: func(u *registryv1alpha1.RegistryUpstream) {
			u.Spec.Upstream.CA = "-----BEGIN CERTIFICATE-----\ndGVzdC12YWx1ZQ==\n-----END CERTIFICATE-----\n"
		}, wantErr: "parse certificate"},
		{name: "a CA of a mirror", mutate: func(u *registryv1alpha1.RegistryUpstream) {
			u.Spec.Upstream.Mirrors = []registryv1alpha1.Endpoint{{Host: "mirror.example.com", CA: "VENDOR-CA"}}
		}, wantErr: "spec.upstream.mirrors[0]: ca"},
		{name: "a CA over 65536 characters", mutate: func(u *registryv1alpha1.RegistryUpstream) { u.Spec.Upstream.CA = long(65537) }, wantErr: "longer than 65536"},
		{name: "auth.auth that is not base64", mutate: func(u *registryv1alpha1.RegistryUpstream) {
			u.Spec.Upstream.Auth = &registryv1alpha1.Auth{Auth: "username:password"}
		}, wantErr: "not base64"},
		{name: "auth.auth that is not a pair", mutate: func(u *registryv1alpha1.RegistryUpstream) {
			u.Spec.Upstream.Auth = &registryv1alpha1.Auth{Auth: "dGVzdC12YWx1ZQ=="}
		}, wantErr: `"username:password"`},
		{name: "two credentials in one", mutate: func(u *registryv1alpha1.RegistryUpstream) {
			u.Spec.Upstream.Auth = &registryv1alpha1.Auth{Username: "robot", Password: "dh-test-secret-0123456789abcdef", Auth: "dXNlcm5hbWU6cGFzc3dvcmQ="}
		}, wantErr: "name different credentials"},
		{name: "a secret reference", mutate: func(u *registryv1alpha1.RegistryUpstream) {
			u.Spec.Upstream.Auth = &registryv1alpha1.Auth{SecretRef: &registryv1alpha1.AuthSecretRef{Name: "creds", Key: "vendor"}}
		}, wantErr: "auth.secretRef"},
		{name: "a username over 1024 characters", mutate: func(u *registryv1alpha1.RegistryUpstream) {
			u.Spec.Upstream.Auth = &registryv1alpha1.Auth{Username: long(1025)}
		}, wantErr: "longer than 1024"},
		{name: "a password over 4096 characters", mutate: func(u *registryv1alpha1.RegistryUpstream) {
			u.Spec.Upstream.Auth = &registryv1alpha1.Auth{Password: long(4097)}
		}, wantErr: "longer than 4096"},
		{name: "auth.auth over 4096 characters", mutate: func(u *registryv1alpha1.RegistryUpstream) {
			u.Spec.Upstream.Auth = &registryv1alpha1.Auth{Auth: long(4100)}
		}, wantErr: "longer than 4096"},
		{name: "more than 8 mirrors", mutate: func(u *registryv1alpha1.RegistryUpstream) {
			for i := range 9 {
				u.Spec.Upstream.Mirrors = append(u.Spec.Upstream.Mirrors, registryv1alpha1.Endpoint{Host: fmt.Sprintf("mirror-%d.example.com", i)})
			}
		}, wantErr: "more than 8"},
		{name: "a mirror repeating the registry", mutate: func(u *registryv1alpha1.RegistryUpstream) {
			u.Spec.Upstream.Mirrors = []registryv1alpha1.Endpoint{{Host: "Registry-Vendor.example.com", Path: "/other"}}
		}, wantErr: "https://registry-vendor.example.com is named by spec.upstream already"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := upstreamObj("vendor", "images.example.com", "registry-vendor.example.com", 0)
			tt.mutate(&obj)

			got := Arbitrate(primaryUpstream(), []registryv1alpha1.RegistryUpstream{obj})

			assert.Empty(t, got.Routes)
			verdict := got.Verdicts["vendor"]
			assert.False(t, verdict.Accepted)
			assert.Equal(t, registryv1alpha1.ReasonInvalidSpec, verdict.Reason)
			assert.Contains(t, verdict.Conflict, tt.wantErr)
		})
	}
}

// What the rules above must not refuse: every field at its best, both credential forms
// naming one pair, a CA bundle on the registry and a mirror, one host over both schemes, the
// highest port and an IPv6 address.
func TestArbitrateAcceptsAFullSpec(t *testing.T) {
	ca := testCertificate(t)
	obj := upstreamObj("vendor", "Images.Example.com:65535", "registry-vendor.example.com", 0)
	obj.Spec.Upstream.CA = ca + ca
	obj.Spec.Upstream.Path = "/virt/images__v1"
	obj.Spec.Upstream.Auth = &registryv1alpha1.Auth{Username: "username", Password: "password", Auth: "dXNlcm5hbWU6cGFzc3dvcmQ="}
	obj.Spec.Upstream.Mirrors = []registryv1alpha1.Endpoint{
		{Scheme: registryv1alpha1.SchemeHTTP, Host: "registry-vendor.example.com"},
		{Host: "[2001:db8::1]:5000", CA: ca, Auth: &registryv1alpha1.Auth{Auth: "dXNlcm5hbWU6cGFzc3dvcmQ="}},
	}

	got := Arbitrate(primaryUpstream(), []registryv1alpha1.RegistryUpstream{obj})

	assert.Equal(t, Verdict{Accepted: true, Reason: registryv1alpha1.ReasonAccepted}, got.Verdicts["vendor"])
	assert.Equal(t, []string{"vendor"}, got.Accepted)
	assert.Equal(t, SpecSize(&obj.Spec), got.SpecBytes)
}

// bulky is an object whose spec takes about 120 KiB: two of them fit into the budget,
// a third does not.
func bulky(t *testing.T, name, match string, ageMinutes int, ca string) registryv1alpha1.RegistryUpstream {
	t.Helper()
	bundle := strings.Repeat(ca, 60*1024/len(ca))
	obj := upstreamObj(name, match, "registry-vendor.example.com", ageMinutes)
	obj.Spec.Upstream.CA = bundle
	obj.Spec.Upstream.Mirrors = []registryv1alpha1.Endpoint{{Host: "mirror.example.com", CA: bundle}}
	return obj
}

// The budget is spent oldest first, so an object that fits keeps fitting whatever is
// created after it, and the verdict is the same whichever controller arbitrates.
func TestArbitrateKeepsTheAcceptedSetWithinTheBudget(t *testing.T) {
	ca := testCertificate(t)
	first := bulky(t, "first", "a.example.com", 0, ca)
	second := bulky(t, "second", "b.example.com", 1, ca)
	third := bulky(t, "third", "c.example.com", 2, ca)
	small := upstreamObj("small", "d.example.com", "registry-vendor.example.com", 3)
	require.Greater(t, SpecSize(&first.Spec)*3, MaxTotalSpecBytes)
	require.LessOrEqual(t, SpecSize(&first.Spec)*2+SpecSize(&small.Spec), MaxTotalSpecBytes)

	for _, input := range [][]registryv1alpha1.RegistryUpstream{
		{first, second, third, small},
		{small, third, second, first},
	} {
		got := Arbitrate(primaryUpstream(), input)

		assert.Equal(t, []string{"first", "second", "small"}, got.Accepted)
		refused := got.Verdicts["third"]
		assert.False(t, refused.Accepted)
		assert.Equal(t, registryv1alpha1.ReasonLimitExceeded, refused.Reason)
		assert.Contains(t, refused.Conflict, "262144 bytes (256 KiB)")
		assert.LessOrEqual(t, got.SpecBytes, MaxTotalSpecBytes)
	}
}

// One object alone may be too large for the budget: it is refused, not truncated.
func TestArbitrateRefusesAnObjectLargerThanTheBudget(t *testing.T) {
	ca := testCertificate(t)
	bundle := strings.Repeat(ca, 60*1024/len(ca))
	obj := upstreamObj("huge", "images.example.com", "registry-vendor.example.com", 0)
	obj.Spec.Upstream.CA = bundle
	for i := range 4 {
		obj.Spec.Upstream.Mirrors = append(obj.Spec.Upstream.Mirrors, registryv1alpha1.Endpoint{Host: fmt.Sprintf("mirror-%d.example.com", i), CA: bundle})
	}

	got := Arbitrate(primaryUpstream(), []registryv1alpha1.RegistryUpstream{obj})

	assert.Empty(t, got.Routes)
	assert.Equal(t, registryv1alpha1.ReasonLimitExceeded, got.Verdicts["huge"].Reason)
}

// A node takes MaxRoutes routes: the next object is refused however small it is.
func TestArbitrateAcceptsAtMostMaxRoutes(t *testing.T) {
	input := make([]registryv1alpha1.RegistryUpstream, 0, MaxRoutes+1)
	for i := range MaxRoutes + 1 {
		input = append(input, upstreamObj(fmt.Sprintf("u%03d", i), fmt.Sprintf("r%03d.example.com", i), "vendor.example.com", i))
	}

	got := Arbitrate(primaryUpstream(), input)

	assert.Len(t, got.Routes, MaxRoutes)
	last := got.Verdicts[fmt.Sprintf("u%03d", MaxRoutes)]
	assert.False(t, last.Accepted)
	assert.Equal(t, registryv1alpha1.ReasonLimitExceeded, last.Reason)
}

// A rejected object does not hold the match: the next object asking for it is judged on
// its own, which is what both arbitrators do.
func TestArbitrateAnInvalidObjectClaimsNothing(t *testing.T) {
	broken := upstreamObj("broken", "ghcr.io", "registry-vendor.example.com", 0)
	broken.Spec.Upstream.CA = "VENDOR-CA"
	fine := upstreamObj("fine", "GHCR.io", "proxy.example.com", 1)

	got := Arbitrate(primaryUpstream(), []registryv1alpha1.RegistryUpstream{broken, fine})

	assert.Equal(t, []string{"fine"}, got.Accepted)
	assert.Equal(t, registryv1alpha1.ReasonInvalidSpec, got.Verdicts["broken"].Reason)
}
