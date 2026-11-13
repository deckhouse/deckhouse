// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package registry

import (
	"context"
	"encoding/base64"
	"errors"
	"reflect"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/deckhouse/deckhouse/pkg/log"
)

const storePassword = "dh-test-secret-0123456789abcdef"

// registryConfig is the RegistryConfig of the registry module; upstream is
// spec.primary.upstream, none when nil.
func registryConfig(mode string, cache bool, upstream map[string]any) *unstructured.Unstructured {
	spec := map[string]any{"mode": mode, "storage": map[string]any{"cache": cache}}
	if upstream != nil {
		spec["primary"] = map[string]any{"upstream": upstream}
	}

	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "RegistryConfig",
		"metadata":   map[string]any{"name": "registry"},
		"spec":       spec,
	}}
}

// upstreamOf is an upstream as the registry module renders it.
func upstreamOf(auth map[string]any) map[string]any {
	upstream := map[string]any{"scheme": "HTTPS", "host": "registry.example.io", "path": "/deckhouse/ee", "ca": "upstream CA"}
	if auth != nil {
		upstream["auth"] = auth
	}
	return upstream
}

// storeAccess is the Secret with the access to the store; replicas are its
// addresses.
func storeAccess(replicas string) *unstructured.Unstructured {
	data := map[string]any{}
	for key, value := range map[string]string{"ca.crt": "store CA", "username": "ro", "password": storePassword, "addresses": replicas} {
		data[key] = base64.StdEncoding.EncodeToString([]byte(value))
	}

	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata":   map[string]any{"name": "registry-storage-client", "namespace": "d8-system"},
		"data":       data,
	}}
}

// authFor is the docker config that gives host login:password.
func authFor(host, login, password string) string {
	auth := base64.StdEncoding.EncodeToString([]byte(login + ":" + password))
	return base64.StdEncoding.EncodeToString([]byte(`{"auths":{"` + host + `":{"auth":"` + auth + `"}}}`))
}

func TestSourcesForModuleOnTheInClusterRegistry(t *testing.T) {
	const imageSet = "registry.d8-system.svc:5001/system/deckhouse/modules"
	upstream := upstreamOf(map[string]any{"username": "u", "password": "p"})
	fromStore := func(repo string) Source {
		return Source{Name: "deckhouse (store)", Repo: "registry.d8-system.svc:4281/" + repo, Scheme: "HTTPS", CA: "store CA",
			DockerCfg: authFor("registry.d8-system.svc:4281", "ro", storePassword)}
	}
	fromUpstream := func(repo string) Source {
		return Source{Name: "deckhouse (upstream)", Repo: "registry.example.io/" + repo, Scheme: "HTTPS", CA: "upstream CA",
			DockerCfg: authFor("registry.example.io", "u", "p")}
	}
	// The store, and the upstream where it fails.
	fromStoreThenUpstream := func(storeRepo, upstreamRepo string) Source {
		store, upstream := fromStore(storeRepo), fromUpstream(upstreamRepo)
		store.Fallback = &upstream
		return store
	}

	tests := []struct {
		name    string
		repo    string
		objects []runtime.Object
		want    []Source
		wantErr error
	}{
		{
			// Whatever serves the address serves it in the cluster.
			name: "the pull path is not managed: as it is",
			repo: imageSet,
			want: []Source{{Name: "deckhouse", Repo: imageSet, Scheme: "HTTPS", DockerCfg: "e30="}},
		},
		{
			name:    "Unmanaged: as it is",
			repo:    imageSet,
			objects: []runtime.Object{registryConfig("Unmanaged", false, upstream)},
			want:    []Source{{Name: "deckhouse", Repo: imageSet, Scheme: "HTTPS", DockerCfg: "e30="}},
		},
		{
			name:    "no cache: the upstream under its prefix",
			repo:    imageSet,
			objects: []runtime.Object{registryConfig("Managed", false, upstream), storeAccess("10.0.0.1:4281")},
			want:    []Source{fromUpstream("deckhouse/ee/modules")},
		},
		{
			name:    "a cache: the store first, the upstream behind it",
			repo:    imageSet,
			objects: []runtime.Object{registryConfig("Managed", true, upstream), storeAccess("10.0.0.1:4281,10.0.0.2:4281")},
			want:    []Source{fromStoreThenUpstream("system/deckhouse/modules", "deckhouse/ee/modules")},
		},
		{
			name:    "air-gap: the store",
			repo:    imageSet,
			objects: []runtime.Object{registryConfig("Managed", true, nil), storeAccess("10.0.0.1:4281")},
			want:    []Source{fromStore("system/deckhouse/modules")},
		},
		{
			name:    "the store has no replicas yet: the upstream",
			repo:    imageSet,
			objects: []runtime.Object{registryConfig("Managed", true, upstream), storeAccess("")},
			want:    []Source{fromUpstream("deckhouse/ee/modules")},
		},
		{
			// Only the store keeps the modules of the other sources; the
			// upstream answers 404 there.
			name:    "another source kept in the store: the store first",
			repo:    "registry.d8-system.svc:5001/system/deckhouse/module-sources/partner",
			objects: []runtime.Object{registryConfig("Managed", true, upstream), storeAccess("10.0.0.1:4281")},
			want:    []Source{fromStoreThenUpstream("system/deckhouse/module-sources/partner", "deckhouse/ee/module-sources/partner")},
		},
		{
			name:    "beside the image set: the upstream at the same path",
			repo:    "registry.d8-system.svc:5001/flant/modules",
			objects: []runtime.Object{registryConfig("Managed", true, upstream), storeAccess("10.0.0.1:4281")},
			want:    []Source{fromUpstream("flant/modules")},
		},
		{
			name:    "a path that only starts like the image set is beside it",
			repo:    "registry.d8-system.svc:5001/system/deckhouse-extra",
			objects: []runtime.Object{registryConfig("Managed", true, upstream), storeAccess("10.0.0.1:4281")},
			want:    []Source{fromUpstream("system/deckhouse-extra")},
		},
		{
			name:    "beside the image set in air-gap: nothing serves it",
			repo:    "registry.d8-system.svc:5001/flant/modules",
			objects: []runtime.Object{registryConfig("Managed", true, nil), storeAccess("10.0.0.1:4281")},
			wantErr: ErrNoSource,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := append([]runtime.Object{module("m", "deckhouse"), moduleSource("deckhouse", tt.repo)}, tt.objects...)
			client := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), objects...)

			got, err := NewSources(client, log.NewNop()).ForModule(context.Background(), "m")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("sources = %+v\nwant %+v", got, tt.want)
			}
			// The docker config is the one of the host the source is read
			// from. A source read as it is keeps its own.
			for _, src := range got {
				if src.Name == "deckhouse" {
					continue
				}
				for s := &src; s != nil; s = s.Fallback {
					if _, err := NewClient(Source{Repo: s.Repo, DockerCfg: s.DockerCfg}); err != nil {
						t.Fatalf("client of %s: %v", s.Name, err)
					}
				}
			}
		})
	}
}

func TestInClusterUpstreamCredentials(t *testing.T) {
	const encoded = "dXNlcm5hbWU6cGFzc3dvcmQ=" // username:password
	tests := []struct {
		name string
		auth map[string]any
		want string
	}{
		{"anonymous", nil, ""},
		{"pre-encoded", map[string]any{"auth": encoded}, authFor("registry.example.io", "username", "password")},
		// The module takes the pre-encoded credentials first.
		{"both", map[string]any{"auth": encoded, "username": "u", "password": "p"}, authFor("registry.example.io", "username", "password")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(),
				module("m", "deckhouse"), moduleSource("deckhouse", "registry.d8-system.svc:5001/system/deckhouse/modules"),
				registryConfig("Managed", false, upstreamOf(tt.auth)))

			got, err := NewSources(client, log.NewNop()).ForModule(context.Background(), "m")
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 1 || got[0].DockerCfg != tt.want {
				t.Fatalf("sources = %+v, want the docker config %q", got, tt.want)
			}
		})
	}
}

func TestSourcesForPackageModuleOnTheInClusterRegistry(t *testing.T) {
	client := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(),
		modulePackage("csi-nfs", "main", "v0.3.1", "main"),
		packageRepository("main", map[string]any{"repo": "registry.d8-system.svc:5001/system/deckhouse/packages"}),
		registryConfig("Managed", false, upstreamOf(nil)))

	got, err := NewSources(client, log.NewNop()).ForModule(context.Background(), "csi-nfs")
	if err != nil {
		t.Fatal(err)
	}
	want := []Source{{Name: "main (upstream)", Repo: "registry.example.io/deckhouse/ee/packages", Scheme: "HTTPS", CA: "upstream CA"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sources = %+v, want %+v", got, want)
	}
}

func TestInClusterRegistryUnanswered(t *testing.T) {
	incomplete := storeAccess("10.0.0.1:4281")
	unstructured.RemoveNestedField(incomplete.Object, "data", "password")

	tests := []struct {
		name    string
		objects []runtime.Object
		fail    string // the resource whose get fails
	}{
		{"the registry config can not be read", []runtime.Object{registryConfig("Managed", true, upstreamOf(nil))}, "registryconfigs"},
		{"the store access can not be read", []runtime.Object{registryConfig("Managed", true, upstreamOf(nil)), storeAccess("10.0.0.1:4281")}, "secrets"},
		{"the store access is incomplete", []runtime.Object{registryConfig("Managed", true, nil), incomplete}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := append([]runtime.Object{module("m", "deckhouse"), moduleSource("deckhouse", "registry.d8-system.svc:5001/system/deckhouse/modules")}, tt.objects...)
			client := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), objects...)
			if tt.fail != "" {
				client.PrependReactor("get", tt.fail, func(k8stesting.Action) (bool, runtime.Object, error) {
					return true, nil, apierrors.NewServiceUnavailable("etcd is slow")
				})
			}

			// Not a module without a source: the next pass asks again.
			_, err := NewSources(client, log.NewNop()).ForModule(context.Background(), "m")
			if err == nil || errors.Is(err, ErrNoSource) {
				t.Fatalf("err = %v, want an error other than ErrNoSource", err)
			}
		})
	}
}

func TestInClusterRegistryReadOnceAMinute(t *testing.T) {
	client := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(),
		module("a", "deckhouse"), module("b", "other"),
		moduleSource("deckhouse", "registry.d8-system.svc:5001/system/deckhouse/modules"),
		moduleSource("other", "registry.example.io/other/modules"),
		registryConfig("Managed", false, upstreamOf(nil)))
	var fail error
	client.PrependReactor("get", "registryconfigs", func(k8stesting.Action) (bool, runtime.Object, error) {
		return fail != nil, nil, fail
	})
	reads := func() int {
		n := 0
		for _, a := range client.Actions() {
			if a.Matches("get", "registryconfigs") {
				n++
			}
		}
		return n
	}
	sources := NewSources(client, log.NewNop())

	// Another registry is no business of the registry module.
	if _, err := sources.ForModule(context.Background(), "b"); err != nil || reads() != 0 {
		t.Fatalf("another registry: err %v, %d reads of the registry config, want none", err, reads())
	}

	// A caller that gave up leaves nothing behind.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fail = context.Canceled
	if _, err := sources.ForModule(ctx, "a"); err == nil {
		t.Fatal("no error for a request given up")
	}
	fail = nil
	for range 3 {
		if got, err := sources.ForModule(context.Background(), "a"); err != nil || len(got) != 1 {
			t.Fatalf("sources = %+v, %v", got, err)
		}
	}
	if reads() != 2 {
		t.Fatalf("%d reads of the registry config, want 2: the request given up and one for the minute", reads())
	}
}
