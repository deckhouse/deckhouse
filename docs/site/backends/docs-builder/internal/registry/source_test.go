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
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/deckhouse/deckhouse/pkg/log"
)

func module(name, source string, available ...string) *unstructured.Unstructured {
	props := map[string]any{}
	if source != "" {
		props["source"] = source
	}
	if len(available) > 0 {
		list := make([]any, 0, len(available))
		for _, a := range available {
			list = append(list, a)
		}
		props["availableSources"] = list
	}

	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "Module",
		"metadata":   map[string]any{"name": name},
		"properties": props,
	}}
}

func moduleSource(name, repo string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "ModuleSource",
		"metadata":   map[string]any{"name": name},
		"spec": map[string]any{"registry": map[string]any{
			"repo":      repo,
			"scheme":    "HTTPS",
			"dockerCfg": "e30=",
		}},
	}}
}

func TestSourcesForModule(t *testing.T) {
	sources := []runtime.Object{
		moduleSource("deckhouse", "registry.example.io/deckhouse/modules"),
		moduleSource("prod", "registry.example.io/prod/modules"),
		moduleSource("broken", ""),
	}

	tests := []struct {
		name    string
		module  *unstructured.Unstructured
		want    []string
		wantErr error
	}{
		{"installed source first, then the others", module("m", "prod", "deckhouse", "prod"), []string{"prod", "deckhouse"}, nil},
		{"not installed: available sources in order", module("m", "", "deckhouse", "prod"), []string{"deckhouse", "prod"}, nil},
		// Deckhouse runs the embedded copy: the versions of the sources can
		// not be installed.
		{"embedded with sources too", module("m", embeddedSource, "deckhouse"), nil, ErrNoSource},
		{"embedded only", module("m", embeddedSource), nil, ErrNoSource},
		{"a removed source is skipped", module("m", "prod", "removed", "deckhouse"), []string{"prod", "deckhouse"}, nil},
		{"a source without repo is skipped", module("m", "broken", "deckhouse"), []string{"deckhouse"}, nil},
		{"every source is broken", module("m", "removed", "broken"), nil, ErrNoSource},
		{"no sources at all", module("m", ""), nil, ErrNoSource},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), append([]runtime.Object{tt.module}, sources...)...)

			got, err := NewSources(client, log.NewNop()).ForModule(context.Background(), "m")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}

			var names []string
			for _, s := range got {
				names = append(names, s.Name)
			}
			if !reflect.DeepEqual(names, tt.want) {
				t.Fatalf("sources = %v, want %v", names, tt.want)
			}
		})
	}
}

func TestSourcesForModuleFields(t *testing.T) {
	client := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(),
		module("m", "deckhouse"), moduleSource("deckhouse", "registry.example.io/deckhouse/modules"))

	got, err := NewSources(client, log.NewNop()).ForModule(context.Background(), "m")
	if err != nil {
		t.Fatal(err)
	}

	want := []Source{{Name: "deckhouse", Repo: "registry.example.io/deckhouse/modules", Scheme: "HTTPS", DockerCfg: "e30="}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestSourcesForModuleErrors(t *testing.T) {
	tests := []struct {
		name    string
		objects []runtime.Object
	}{
		{"module not found", nil},
		{"module source not found", []runtime.Object{module("m", "missing")}},
		{"module source without repo", []runtime.Object{module("m", "broken"), moduleSource("broken", "")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), tt.objects...)

			if _, err := NewSources(client, log.NewNop()).ForModule(context.Background(), "m"); err == nil {
				t.Fatal("want an error")
			}
		})
	}
}

func TestSourcesModules(t *testing.T) {
	withState := func(m *unstructured.Unstructured, version string, enabled bool) *unstructured.Unstructured {
		if version != "" {
			m.Object["properties"].(map[string]any)["version"] = version
		}
		status := "False"
		if enabled {
			status = "True"
		}
		m.Object["status"] = map[string]any{"conditions": []any{
			map[string]any{"type": "IsReady", "status": "True"},
			map[string]any{"type": enabledCondition, "status": status},
		}}
		return m
	}
	override := func(name, tag, digest string) *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "deckhouse.io/v1alpha2",
			"kind":       "ModulePullOverride",
			"metadata":   map[string]any{"name": name},
			"spec":       map[string]any{"imageTag": tag},
			"status":     map[string]any{"imageDigest": digest},
		}}
	}

	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{moduleGVR: "ModuleList", overrideGVR: "ModulePullOverrideList", modulePackageGVR: "ModulePackageList"},
		withState(module("console", "deckhouse", "deckhouse"), "main", true),
		// The v2 module manager: modules of package repositories.
		modulePackage("csi-nfs", "main-repo", "v0.3.1", "main-repo", "extra"),
		modulePackage("storage", "", "", "extra"),
		modulePackage("orphan", "", ""),
		// A module of a ModuleSource stays as the Module describes it.
		modulePackage("virtualization", "extra", "v9.9.9", "extra"),
		withState(module("prometheus", "deckhouse", "deckhouse"), "v1.0.29", true),
		withState(module("virtualization", "", "deckhouse", "prod"), "v1.12.0", false),
		withState(module("echo", "", "deckhouse"), "", false),
		withState(module("cni-cilium", embeddedSource), "", true),
		// An embedded module a source offers too (moving out of Deckhouse),
		// also as a package: Deckhouse runs the embedded copy.
		withState(module("cloud-provider-dvp", embeddedSource, "deckhouse", "deckhouse-prod"), "v1.78.0", true),
		modulePackage("cloud-provider-dvp", "", "", "extra"),
		override("console", "main", "sha256:37463b63"),
		override("echo", "pr-1", "sha256:aaaa"),
	)

	got, err := NewSources(client, log.NewNop()).Modules(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	byName := map[string]Module{}
	for _, m := range got {
		byName[m.Name] = m
	}

	want := map[string]Module{
		"console":        {Name: "console", Enabled: true, Version: "main", Override: &Override{Tag: "main", Digest: "sha256:37463b63"}},
		"prometheus":     {Name: "prometheus", Enabled: true, Version: "v1.0.29"},
		"virtualization": {Name: "virtualization", Version: "v1.12.0"},
		// A pull override of a disabled module does not change what it shows.
		"echo":    {Name: "echo"},
		"csi-nfs": {Name: "csi-nfs", Enabled: true, Version: "v0.3.1"},
		"storage": {Name: "storage"},
	}
	if !reflect.DeepEqual(byName, want) {
		t.Fatalf("modules = %+v, want %+v", byName, want)
	}
	// The tag a reader may open before the search index pins it; the
	// override of a disabled module counts too.
	sources := NewSources(client, log.NewNop())
	lists := len(client.Actions())
	for module, want := range map[string]string{"console": "main", "echo": "pr-1", "prometheus": ""} {
		tag, err := sources.OverrideTag(context.Background(), module)
		if err != nil || tag != want {
			t.Fatalf("OverrideTag(%s) = %q, %v, want %q", module, tag, err, want)
		}
	}
	// One list answers all three.
	if n := len(client.Actions()) - lists; n != 1 {
		t.Fatalf("apiserver requests = %d, want 1", n)
	}
}

// The answer of the API server is kept for a minute, a failure too; the
// error of a caller that gave up (a leader that lost its Lease) is not: the
// next caller would take every version for a branch and keep it out of S3.
func TestSourcesOverrideTagKeepsNoGivenUpRequest(t *testing.T) {
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{overrideGVR: "ModulePullOverrideList"},
		&unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "deckhouse.io/v1alpha2",
			"kind":       "ModulePullOverride",
			"metadata":   map[string]any{"name": "echo"},
			"spec":       map[string]any{"imageTag": "pr-1"},
		}},
	)
	var fail error
	client.PrependReactor("list", "modulepulloverrides", func(k8stesting.Action) (bool, runtime.Object, error) {
		return fail != nil, nil, fail
	})
	sources := NewSources(client, log.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fail = context.Canceled
	if _, err := sources.OverrideTag(ctx, "echo"); err == nil {
		t.Fatal("no error for a request given up")
	}
	fail = nil
	if tag, err := sources.OverrideTag(context.Background(), "echo"); err != nil || tag != "pr-1" {
		t.Fatalf("OverrideTag after a request given up = %q, %v, want pr-1", tag, err)
	}

	// A failure of the API server is kept: one request answers the minute.
	sources = NewSources(client, log.NewNop())
	fail = errors.New("apiserver is down")
	if _, err := sources.OverrideTag(context.Background(), "echo"); err == nil {
		t.Fatal("no error with the API server down")
	}
	fail = nil
	if _, err := sources.OverrideTag(context.Background(), "echo"); err == nil {
		t.Fatal("the failure of the API server is not kept")
	}
}

func TestSourcesPendingReleases(t *testing.T) {
	release := func(name, module, version, phase string) *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "deckhouse.io/v1alpha1",
			"kind":       "ModuleRelease",
			"metadata":   map[string]any{"name": name},
			"spec":       map[string]any{"moduleName": module, "version": version},
			"status":     map[string]any{"phase": phase},
		}}
	}

	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{releaseGVR: "ModuleReleaseList"},
		release("console-v1.70.0", "console", "1.70.0", "Deployed"),
		release("console-v1.71.0", "console", "1.71.0", "Pending"),
		release("echo-v0.2.0", "echo", "v0.2.0", "Pending"),
		release("echo-v0.1.0", "echo", "0.1.0", "Superseded"),
		release("broken", "", "1.0.0", "Pending"),
	)

	got, err := NewSources(client, log.NewNop()).PendingReleases(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	slices.SortFunc(got, func(a, b Release) int { return strings.Compare(a.Module, b.Module) })

	want := []Release{{Module: "console", Version: "v1.71.0"}, {Module: "echo", Version: "v0.2.0"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("pending releases = %+v, want %+v", got, want)
	}
}

func TestSourcesForModuleTellsAGoneSourceFromAnUnanswered(t *testing.T) {
	for _, tt := range []struct {
		name     string
		objects  []runtime.Object
		failGet  bool
		noSource bool
	}{
		{"module source removed", []runtime.Object{module("m", "missing")}, false, true},
		{"module source without repo", []runtime.Object{module("m", "broken"), moduleSource("broken", "")}, false, true},
		{"API server does not answer", []runtime.Object{module("m", "src"), moduleSource("src", "r.example/modules")}, true, false},
		{"one source removed, the API server does not answer for the other", []runtime.Object{module("m", "missing", "src"), moduleSource("src", "r.example/modules")}, true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), tt.objects...)
			if tt.failGet {
				client.PrependReactor("get", "modulesources", func(a k8stesting.Action) (bool, runtime.Object, error) {
					if a.(k8stesting.GetAction).GetName() != "src" {
						return false, nil, nil
					}
					return true, nil, apierrors.NewServiceUnavailable("etcd is slow")
				})
			}

			_, err := NewSources(client, log.NewNop()).ForModule(context.Background(), "m")
			if err == nil || errors.Is(err, ErrNoSource) != tt.noSource || apierrors.IsNotFound(err) && !tt.noSource {
				t.Fatalf("err = %v; want ErrNoSource %v", err, tt.noSource)
			}
		})
	}
}

func modulePackage(name, usedBy, version string, available ...string) *unstructured.Unstructured {
	status := map[string]any{}
	if usedBy != "" {
		status["usedBy"] = map[string]any{"repositoryName": usedBy, "version": version}
	}
	if len(available) > 0 {
		list := make([]any, 0, len(available))
		for _, a := range available {
			list = append(list, a)
		}
		status["availableRepositories"] = list
	}

	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "ModulePackage",
		"metadata":   map[string]any{"name": name},
		"status":     status,
	}}
}

func packageRepository(name string, registry map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "PackageRepository",
		"metadata":   map[string]any{"name": name},
		"spec":       map[string]any{"registry": registry},
	}}
}

func TestSourcesForPackageModule(t *testing.T) {
	client := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(),
		modulePackage("csi-nfs", "main-repo", "v0.3.1", "gone", "main-repo", "extra"),
		packageRepository("main-repo", map[string]any{"repo": "registry.example.com/packages", "login": "u", "password": "p"}),
		packageRepository("extra", map[string]any{"repo": "registry.example.com/extra", "dockerCfg": "e30="}),
		// A Module of the v2 manager has no source: the package is used.
		module("storage", ""),
		modulePackage("storage", "", "", "extra"),
		// Deckhouse runs the embedded copy of a module a package offers too.
		module("cloud-provider-dvp", embeddedSource),
		modulePackage("cloud-provider-dvp", "", "", "extra"),
	)
	sources := NewSources(client, log.NewNop())

	got, err := sources.ForModule(context.Background(), "csi-nfs")
	if err != nil {
		t.Fatal(err)
	}
	want := []Source{
		// The repository the installed module came from first; a removed one is skipped.
		{Name: "main-repo", Repo: "registry.example.com/packages", Login: "u", Password: "p"},
		{Name: "extra", Repo: "registry.example.com/extra", DockerCfg: "e30="},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sources = %+v, want %+v", got, want)
	}

	if got, err := sources.ForModule(context.Background(), "storage"); err != nil || len(got) != 1 || got[0].Name != "extra" {
		t.Fatalf("module without a source: %+v, %v", got, err)
	}
	if got, err := sources.ForModule(context.Background(), "cloud-provider-dvp"); !errors.Is(err, ErrNoSource) {
		t.Fatalf("embedded module: %+v, %v, want ErrNoSource", got, err)
	}
	if _, err := sources.ForModule(context.Background(), "nothing"); !errors.Is(err, ErrNoSource) {
		t.Fatalf("unknown module: err %v, want ErrNoSource", err)
	}
}

func TestPackageSourcesTellAGoneRepositoryFromAnUnanswered(t *testing.T) {
	for _, tt := range []struct {
		name     string
		failGet  bool
		noSource bool
	}{
		{"package repository removed", false, true},
		{"API server does not answer", true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			client := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), modulePackage("m", "repo", "v1.0.0", "repo"))
			if tt.failGet {
				client.PrependReactor("get", "packagerepositories", func(k8stesting.Action) (bool, runtime.Object, error) {
					return true, nil, apierrors.NewServiceUnavailable("etcd is slow")
				})
			}

			// The module has no Module with a source: its package is used.
			_, err := NewSources(client, log.NewNop()).ForModule(context.Background(), "m")
			if err == nil || errors.Is(err, ErrNoSource) != tt.noSource {
				t.Fatalf("err = %v; want ErrNoSource %v", err, tt.noSource)
			}
		})
	}
}
