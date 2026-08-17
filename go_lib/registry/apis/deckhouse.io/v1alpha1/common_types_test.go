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

package v1alpha1

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestSchemeHelpers(t *testing.T) {
	assert.True(t, SchemeHTTPS.IsSecure())
	assert.False(t, SchemeHTTP.IsSecure())
	// An unset scheme must not read as insecure: the zero value has to fail safe,
	// because it reaches the containerd drop-in unchanged.
	assert.True(t, Scheme("").IsSecure())

	assert.Equal(t, "https", SchemeHTTPS.Lower())
	assert.Equal(t, "http", SchemeHTTP.Lower())
}

func TestAuthEncoded(t *testing.T) {
	// A credential that arrives already in the wire form. Encoded here rather than
	// written out as a literal, so that nothing in this file reads as a real secret to
	// a scanner — and so the pair it stands for is stated in the code instead of in a
	// comment beside an opaque string.
	preEncoded := base64.StdEncoding.EncodeToString([]byte("pre:encoded"))

	tests := []struct {
		name  string
		auth  *Auth
		empty bool
		want  string
	}{
		{name: "nil", auth: nil, empty: true, want: ""},
		{name: "zero", auth: &Auth{}, empty: true, want: ""},
		{
			name: "username and password",
			auth: &Auth{Username: "user", Password: "pass"},
			want: "dXNlcjpwYXNz", // base64("user:pass")
		},
		{
			name: "pre-encoded wins over username and password",
			auth: &Auth{Username: "user", Password: "pass", Auth: preEncoded},
			want: preEncoded,
		},
		{
			name: "username only",
			auth: &Auth{Username: "user"},
			want: "dXNlcjo=", // base64("user:")
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.empty, tt.auth.IsEmpty())
			assert.Equal(t, tt.want, tt.auth.Encoded())
		})
	}
}

func TestEndpointAddressAndURL(t *testing.T) {
	tests := []struct {
		name        string
		endpoint    *Endpoint
		wantAddress string
		wantURL     string
	}{
		{name: "nil", endpoint: nil, wantAddress: "", wantURL: ""},
		{
			name:        "host only",
			endpoint:    &Endpoint{Scheme: SchemeHTTPS, Host: "registry.deckhouse.io"},
			wantAddress: "registry.deckhouse.io",
			wantURL:     "https://registry.deckhouse.io",
		},
		{
			name:        "host with path",
			endpoint:    &Endpoint{Scheme: SchemeHTTPS, Host: "registry.deckhouse.io", Path: "/deckhouse/ee"},
			wantAddress: "registry.deckhouse.io/deckhouse/ee",
			wantURL:     "https://registry.deckhouse.io/deckhouse/ee",
		},
		{
			name:        "path without leading slash and with trailing slash",
			endpoint:    &Endpoint{Scheme: SchemeHTTP, Host: "registry.local:5000", Path: "deckhouse/ce/"},
			wantAddress: "registry.local:5000/deckhouse/ce",
			wantURL:     "http://registry.local:5000/deckhouse/ce",
		},
		{
			name:        "empty scheme defaults to https",
			endpoint:    &Endpoint{Host: "registry.d8-system.svc:5001"},
			wantAddress: "registry.d8-system.svc:5001",
			wantURL:     "https://registry.d8-system.svc:5001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantAddress, tt.endpoint.Address())
			assert.Equal(t, tt.wantURL, tt.endpoint.URL())
		})
	}
}

func TestEndpointUniqueKeyIgnoresCredentials(t *testing.T) {
	a := &Endpoint{Scheme: SchemeHTTPS, Host: "registry.example.com", Path: "/ee", Auth: &Auth{Username: "one"}}
	b := &Endpoint{Scheme: SchemeHTTPS, Host: "registry.example.com", Path: "/ee", Auth: &Auth{Username: "two"}}
	assert.Equal(t, a.UniqueKey(), b.UniqueKey(),
		"endpoints differing only by credentials point at the same content and must de-duplicate")

	// An unset scheme must key the same as an explicit HTTPS, otherwise the same
	// endpoint would be counted twice depending on how it was spelled.
	assert.Equal(t,
		(&Endpoint{Host: "registry.example.com"}).UniqueKey(),
		(&Endpoint{Scheme: SchemeHTTPS, Host: "registry.example.com"}).UniqueKey())

	assert.NotEqual(t, a.UniqueKey(), (&Endpoint{Scheme: SchemeHTTP, Host: "registry.example.com", Path: "/ee"}).UniqueKey())
}

func TestUpstreamEndpointsOrder(t *testing.T) {
	var nilUpstream *Upstream
	assert.Nil(t, nilUpstream.Endpoints())

	u := &Upstream{
		Endpoint: Endpoint{Scheme: SchemeHTTPS, Host: "primary.example.com"},
		Mirrors: []Endpoint{
			{Scheme: SchemeHTTPS, Host: "mirror-1.example.com"},
			{Scheme: SchemeHTTPS, Host: "mirror-2.example.com"},
		},
	}

	got := u.Endpoints()
	require.Len(t, got, 3)
	assert.Equal(t, "primary.example.com", got[0].Host, "the primary endpoint must be tried first")
	assert.Equal(t, "mirror-1.example.com", got[1].Host)
	assert.Equal(t, "mirror-2.example.com", got[2].Host)
}

func TestRegistryConfigSpecIsAirGap(t *testing.T) {
	upstream := &Upstream{Endpoint: Endpoint{Scheme: SchemeHTTPS, Host: "registry.deckhouse.io"}}

	tests := []struct {
		name string
		spec RegistryConfigSpec
		want bool
	}{
		{
			name: "cache with upstream is a pass-through cache",
			spec: RegistryConfigSpec{
				Primary: PrimarySource{Upstream: upstream},
				Storage: StorageConfig{Cache: true},
			},
			want: false,
		},
		{
			name: "cache without upstream is air-gap",
			spec: RegistryConfigSpec{Storage: StorageConfig{Cache: true}},
			want: true,
		},
		{
			name: "no cache and no upstream is not air-gap: there is nothing to serve from",
			spec: RegistryConfigSpec{},
			want: false,
		},
		{
			name: "no cache with upstream forwards directly",
			spec: RegistryConfigSpec{Primary: PrimarySource{Upstream: upstream}},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.spec.IsAirGap())
		})
	}
}

func TestRegistryStorageStatusLeaderReplica(t *testing.T) {
	var status RegistryStorageStatus
	assert.Nil(t, status.LeaderReplica(), "no replicas means no leader")

	status.Replicas = []StorageReplicaStatus{
		{Node: "master-1", Role: ReplicaRoleFollower, VerifiedDigests: 210},
		{Node: "master-0", Role: ReplicaRoleLeader, Full: true, VerifiedDigests: 459},
	}

	leader := status.LeaderReplica()
	require.NotNil(t, leader)
	assert.Equal(t, "master-0", leader.Node)
	assert.True(t, leader.Full)

	status.Replicas = []StorageReplicaStatus{{Node: "master-1", Role: ReplicaRoleFollower}}
	assert.Nil(t, status.LeaderReplica(), "a followers-only list has no leader")
}

func TestRegistryNodeSpecBackend(t *testing.T) {
	spec := RegistryNodeSpec{
		Cache: true,
		Backends: []Backend{
			{Name: BackendStorage, Endpoint: Endpoint{Host: "registry.d8-system.svc:5001"}},
			{Name: BackendUpstream, Endpoint: Endpoint{Host: "registry.deckhouse.io", Path: "/deckhouse/ee"}},
		},
	}

	storage := spec.Backend(BackendStorage)
	require.NotNil(t, storage)
	assert.Equal(t, "registry.d8-system.svc:5001", storage.Host)

	upstream := spec.Backend(BackendUpstream)
	require.NotNil(t, upstream)
	assert.Equal(t, "registry.deckhouse.io/deckhouse/ee", upstream.Address())

	// After going air-gap the upstream backend is gone from the layout.
	spec.Backends = spec.Backends[:1]
	assert.Nil(t, spec.Backend(BackendUpstream))
}

// TestAddToScheme guards the registration list: a kind added to the package but
// forgotten in addKnownTypes would fail at runtime in every consumer.
func TestAddToScheme(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, AddToScheme(scheme))

	for _, kind := range []string{
		RegistryConfigKind, RegistryStorageKind, RegistryUpstreamKind, RegistryNodeKind,
	} {
		gvk := GroupVersion.WithKind(kind)
		assert.Truef(t, scheme.Recognizes(gvk), "%s is not registered", kind)
		assert.Truef(t, scheme.Recognizes(GroupVersion.WithKind(kind+"List")), "%sList is not registered", kind)
	}
}

func TestResource(t *testing.T) {
	assert.Equal(t, "registryconfigs.deckhouse.io", Resource("registryconfigs").String())
}
