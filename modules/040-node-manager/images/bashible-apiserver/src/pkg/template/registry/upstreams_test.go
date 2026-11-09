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

package registry

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/registry/models/bashible"
)

// testCertificate is generated at runtime: the CA is parsed, so a placeholder would not do.
func testCertificate(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "registry upstream test CA"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func deckhouseRegistrySecretFor(address string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: deckhouseRegistrySecretName, Namespace: "d8-system"},
		Data: map[string][]byte{
			"address":           []byte(address),
			"path":              []byte("/deckhouse/ce"),
			"scheme":            []byte("https"),
			".dockerconfigjson": []byte(`{"auths":{"` + address + `":{"auth":"dXNlcm5hbWU6cGFzc3dvcmQ="}}}`),
		},
	}
}

// upstreamObject is an object node-controller has accepted for its current generation.
func upstreamObject(name, match string, created time.Time, upstream map[string]any) *unstructured.Unstructured {
	obj := newUpstreamObject()
	obj.SetName(name)
	obj.SetCreationTimestamp(metav1.NewTime(created))
	obj.SetGeneration(1)
	obj.Object["spec"] = map[string]any{"match": match, "upstream": upstream}
	obj.Object["status"] = map[string]any{"accepted": true, "observedGeneration": int64(1)}
	return obj
}

// upstream is an object node-controller has accepted for its current generation.
func upstream(name, match string, created time.Time, endpoint upstreamEndpoint) registryUpstream {
	u := registryUpstream{ObjectMeta: metav1.ObjectMeta{Name: name, CreationTimestamp: metav1.NewTime(created), Generation: 1}}
	u.Spec.Match = match
	u.Spec.Upstream = endpoint
	u.Status.Accepted = true
	u.Status.ObservedGeneration = 1
	return u
}

// User story: As a cluster operator with the registry module off and nodes on containerd v1,
// I want my RegistryUpstream objects on those nodes too, so that pulls of the intercepted host
// reach the upstream there as well. That runtime reads inline mirrors, so they go to their own
// key, which only the inline configuration reads.
func TestUpstreamsReachTheInlineConfigurationWithoutAnAgent(t *testing.T) {
	ca := testCertificate(t)
	data, err := reconcileWith(t,
		deckhouseRegistrySecretFor("registry.example.com"),
		upstreamObject("virtualization", "Images.Example.com", time.Now(), map[string]any{
			"host": "vendor.example.com", "ca": ca,
			"auth":    map[string]any{"username": "username", "password": "password"},
			"mirrors": []any{map[string]any{"scheme": "HTTP", "host": "mirror.example.com:5000"}},
		}),
	)
	require.NoError(t, err)

	hosts := data.Data["hosts"].(map[string]any)
	require.Equal(t, []string{"registry.example.com"}, keys(hosts), "registry.d belongs to nodelet: the hosts stay the primary alone")

	inline := data.Data["inlineUpstreams"].(map[string]any)
	entry, ok := inline["images.example.com"].(map[string]any)
	require.True(t, ok, "the upstream must be an inline entry of its own, keyed by its match: %v", inline)

	mirrors := entry["mirrors"].([]any)
	require.Len(t, mirrors, 2, "the upstream first, its mirror after it")
	vendor := mirrors[0].(map[string]any)
	require.Equal(t, "vendor.example.com", vendor["host"])
	require.Equal(t, "https", vendor["scheme"])
	require.Equal(t, map[string]any{"username": "username", "password": "password", "auth": ""}, vendor["auth"])
	require.NotEmpty(t, vendor["ca"])
	require.Equal(t, "http", mirrors[1].(map[string]any)["scheme"])
}

// The agent applies RegistryUpstream itself behind its `_default`.
func TestUpstreamsStayOutOfTheContextUnderTheAgent(t *testing.T) {
	data, err := reconcileWith(t,
		bashibleAgentLayoutSecret(),
		upstreamObject("virtualization", "images.example.com", time.Now(), map[string]any{"host": "vendor.example.com"}),
	)
	require.NoError(t, err)
	require.NotContains(t, data.Data, "inlineUpstreams")
	require.NotContains(t, data.Data["hosts"], "images.example.com")
}

// Adding an object changes the hash, and the hash is what makes bashible run again.
func TestAnUpstreamChangesTheHash(t *testing.T) {
	without, err := reconcileWith(t, deckhouseRegistrySecretFor("registry.example.com"))
	require.NoError(t, err)
	with, err := reconcileWith(t, deckhouseRegistrySecretFor("registry.example.com"),
		upstreamObject("virtualization", "images.example.com", time.Now(), map[string]any{"host": "vendor.example.com"}))
	require.NoError(t, err)
	require.NotEqual(t, without.HashSum, with.HashSum)
}

// node-controller arbitrates; this server renders only what it accepted, for the generation
// the object has now. Every value reaches a root shell script, so anything that is not
// plainly what it must be is refused — and refused alone, without taking the rest of the
// registry context down.
func TestAddUpstreams(t *testing.T) {
	early := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	late := early.Add(time.Hour)
	ok := upstreamEndpoint{Host: "vendor.example.com"}
	ca := testCertificate(t)

	notAccepted := upstream("refused", "c.example.com", early, ok)
	notAccepted.Status.Accepted = false
	stale := upstream("stale", "d.example.com", early, ok)
	stale.Generation = 2

	tests := []struct {
		name         string
		upstreams    []registryUpstream
		wantAccepted []string
		wantRejected []string
	}{
		{
			name:         "only what node-controller accepted, for the current generation",
			upstreams:    []registryUpstream{notAccepted, stale, upstream("good", "b.example.com", late, ok)},
			wantAccepted: []string{"b.example.com"},
		},
		{
			name:         "a registry the cluster pulls from is never intercepted",
			upstreams:    []registryUpstream{upstream("primary", "registry.example.com", early, ok), upstream("in-cluster", "registry.d8-system.svc:5001", early, ok)},
			wantRejected: []string{"primary", "in-cluster"},
		},
		{
			name: "a path cannot be applied inline",
			upstreams: []registryUpstream{
				upstream("path", "a.example.com", early, upstreamEndpoint{Host: "vendor.example.com", Path: "/virt"}),
				upstream("mirror-path", "b.example.com", early, upstreamEndpoint{Host: "vendor.example.com", Mirrors: []upstreamEndpoint{{Host: "mirror.example.com", Path: "/virt"}}}),
			},
			wantRejected: []string{"path", "mirror-path"},
		},
		{
			name: "a registry that config.toml configures already",
			upstreams: []registryUpstream{
				upstream("to-primary", "a.example.com", early, upstreamEndpoint{Host: "registry.example.com"}),
				upstream("first", "b.example.com", early, ok),
				upstream("second", "c.example.com", late, ok),
			},
			wantAccepted: []string{"b.example.com"},
			wantRejected: []string{"to-primary", "second"},
		},
		{
			name: "one registry over both schemes in one object",
			upstreams: []registryUpstream{upstream("both", "a.example.com", early, upstreamEndpoint{
				Host: "twice.example.com", Mirrors: []upstreamEndpoint{{Scheme: "HTTP", Host: "Twice.Example.com"}},
			})},
			wantRejected: []string{"both"},
		},
		{
			name:         "a match that is not a host",
			upstreams:    []registryUpstream{upstream("subst", "$(id).example.com", early, ok), upstream("dots", "../etc", early, ok)},
			wantRejected: []string{"subst", "dots"},
		},
		{
			name: "a path outside the repository alphabet",
			upstreams: []registryUpstream{upstream("dollar", "a.example.com", early,
				upstreamEndpoint{Host: "vendor.example.com", Path: "/virt$(id)"})},
			wantRejected: []string{"dollar"},
		},
		{
			name: "a pre-encoded pair that is not base64",
			upstreams: []registryUpstream{upstream("auth", "a.example.com", early, upstreamEndpoint{Host: "vendor.example.com",
				Auth: &struct {
					Username string `json:"username,omitempty"`
					Password string `json:"password,omitempty"`
					Auth     string `json:"auth,omitempty"`
				}{Auth: `"$(id)"`}})},
			wantRejected: []string{"auth"},
		},
		{
			name: "a CA that would close the heredoc it is written into",
			upstreams: []registryUpstream{upstream("heredoc", "a.example.com", early,
				upstreamEndpoint{Host: "vendor.example.com", CA: ca + "EOF\nid\n"})},
			wantRejected: []string{"heredoc"},
		},
		{
			name: "a mirror is checked like the upstream",
			upstreams: []registryUpstream{upstream("mirror", "a.example.com", early,
				upstreamEndpoint{Host: "vendor.example.com", Mirrors: []upstreamEndpoint{{Host: "`id`"}}})},
			wantRejected: []string{"mirror"},
		},
		{
			name:         "a refused object leaves the others in",
			upstreams:    []registryUpstream{upstream("bad", "$(id)", early, ok), upstream("good", "b.example.com", late, ok)},
			wantAccepted: []string{"b.example.com"},
			wantRejected: []string{"bad"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rd := RegistryData{Mode: "unmanaged", Version: "unknown", ImagesBase: "registry.example.com/deckhouse/ce", Hosts: map[string]bashible.ContextHosts{
				"registry.example.com": {Mirrors: []bashible.ContextMirrorHost{{Host: "registry.example.com", Scheme: "https"}}},
			}}
			rejected := rd.addUpstreams(tt.upstreams)

			require.Len(t, rd.Hosts, 1, "the hosts are the primary alone")
			var accepted []string
			for host := range rd.InlineUpstreams {
				accepted = append(accepted, host)
			}
			require.ElementsMatch(t, tt.wantAccepted, accepted)
			var rejectedNames []string
			for name := range rejected {
				rejectedNames = append(rejectedNames, name)
			}
			require.ElementsMatch(t, tt.wantRejected, rejectedNames)
			require.NoError(t, bashible.Context(rd).Validate(), "what is accepted must pass the context validation")
		})
	}
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// The CRD gives a pre-encoded pair precedence; the template prefers the username.
// The context carries only the one the object means.
func TestAPreEncodedPairWins(t *testing.T) {
	mirror, err := upstreamMirror(upstreamEndpoint{Host: "vendor.example.com", Auth: &struct {
		Username string `json:"username,omitempty"`
		Password string `json:"password,omitempty"`
		Auth     string `json:"auth,omitempty"`
	}{Username: "ignored", Password: "ignored", Auth: "dXNlcm5hbWU6cGFzc3dvcmQ="}})
	require.NoError(t, err)
	require.Equal(t, bashible.ContextAuth{Auth: "dXNlcm5hbWU6cGFzc3dvcmQ="}, mirror.Auth)
}

func bashibleAgentLayoutSecret() *corev1.Secret {
	secret := bashibleLayoutSecret()
	secret.Data["config"] = append(secret.Data["config"], []byte(`
agent:
  endpoint: 127.0.0.1:5001
  dropInFile: /etc/containerd/registry.d/_default/hosts.toml
  layout: e30=
`)...)
	return secret
}

// While the module configures the nodes, containerd v1 reads registry.d like every runtime
// (step 032 sets config_path), and nodelet routes the objects there. Rendering them inline as well would
// configure one registry twice; and the path and host rules of the inline form would warn
// about objects that are applied after all.
func TestUpstreamsStayOutOfTheContextWithTheModuleOn(t *testing.T) {
	data, err := reconcileWith(t,
		bashibleLayoutSecret(),
		upstreamObject("virtualization", "images.example.com", time.Now(), map[string]any{"host": "vendor.example.com", "path": "/virt"}),
	)
	require.NoError(t, err)
	require.NotContains(t, data.Data, "inlineUpstreams")
	require.Equal(t, true, data.Data["registryModuleEnable"])
}
