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
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	ggcrregistry "github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// pushModuleTag lists the module in the registry repository at host/path, as
// a module registry does: the module name is a tag of the repository.
func pushModuleTag(t *testing.T, host, path, module string) {
	t.Helper()

	img, err := random.Image(64, 1)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := name.ParseReference(host+"/"+path+":"+module, name.Insecure)
	if err != nil {
		t.Fatal(err)
	}
	if err := remote.Write(ref, img); err != nil {
		t.Fatal(err)
	}
}

func TestStaticSources(t *testing.T) {
	srv := httptest.NewServer(ggcrregistry.New())
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")

	pushModuleTag(t, host, "ee/modules", "echo")
	pushModuleTag(t, host, "ee/modules", "both")
	pushModuleTag(t, host, "fe/modules", "both")
	pushModuleTag(t, host, "fe/modules", "fe-only")

	ee, fe := "http://"+host+"/ee/modules", "http://"+host+"/fe/modules"
	s, err := NewStaticSources([]string{ee, fe, "http://" + host + "/missing"}, nil, NewClients(), log.NewNop())
	if err != nil {
		t.Fatal(err)
	}

	// The first request lists the registries itself, as a follower does.
	if srcs, err := s.ForModule(context.Background(), "echo"); err != nil || len(srcs) != 1 || srcs[0].Repo != host+"/ee/modules" {
		t.Fatalf("sources before listing = %v, %v; want the first registry", srcs, err)
	}

	// A registry that can not be listed is skipped.
	modules, err := s.Modules(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, m := range modules {
		if m.Enabled {
			t.Errorf("%s is enabled: the site has no cluster", m.Name)
		}
		names = append(names, m.Name)
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"both", "echo", "fe-only"}) {
		t.Fatalf("modules = %v", names)
	}

	// The first registry listing a module wins.
	for module, want := range map[string]string{"both": host + "/ee/modules", "echo": host + "/ee/modules", "fe-only": host + "/fe/modules"} {
		srcs, err := s.ForModule(context.Background(), module)
		if err != nil || len(srcs) != 1 || srcs[0].Repo != want || srcs[0].Scheme != "HTTP" {
			t.Errorf("ForModule(%s) = %+v, %v; want %s over HTTP", module, srcs, err, want)
		}
	}
	// A module no registry listed may be in the one that did not answer.
	if srcs, err := s.ForModule(context.Background(), "nope"); err != nil || len(srcs) != 1 || srcs[0].Repo != host+"/missing" {
		t.Errorf("ForModule(nope) = %v, %v; want the registry that did not answer", srcs, err)
	}

	// With every registry listed, an unknown module has no source.
	all, err := NewStaticSources([]string{ee, fe}, nil, NewClients(), log.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := all.ForModule(context.Background(), "nope"); !errors.Is(err, ErrNoSource) {
		t.Errorf("ForModule(nope) err = %v, want ErrNoSource", err)
	}
}

func TestStaticSourcesNoRegistryAnswers(t *testing.T) {
	srv := httptest.NewServer(ggcrregistry.New())
	defer srv.Close()

	s, err := NewStaticSources([]string{"http://" + strings.TrimPrefix(srv.URL, "http://") + "/missing"}, nil, NewClients(), log.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Modules(context.Background()); err == nil {
		t.Fatal("want an error when no registry can be listed")
	}
}

// A registry that does not answer keeps its modules where they were.
func TestStaticSourcesKeepModulesOfFailedRegistry(t *testing.T) {
	ee := httptest.NewServer(ggcrregistry.New())
	defer ee.Close()
	fe := httptest.NewServer(ggcrregistry.New())
	defer fe.Close()
	eeHost, feHost := strings.TrimPrefix(ee.URL, "http://"), strings.TrimPrefix(fe.URL, "http://")

	pushModuleTag(t, eeHost, "modules", "both")
	pushModuleTag(t, eeHost, "modules", "ee-only")
	pushModuleTag(t, feHost, "modules", "both")
	pushModuleTag(t, feHost, "modules", "fe-only")

	s, err := NewStaticSources([]string{"http://" + eeHost + "/modules", "http://" + feHost + "/modules"}, nil, NewClients(), log.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Modules(context.Background()); err != nil {
		t.Fatal(err)
	}

	ee.Close()
	modules, err := s.Modules(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, m := range modules {
		names = append(names, m.Name)
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"both", "ee-only", "fe-only"}) {
		t.Fatalf("modules = %v, want those of the failed registry kept", names)
	}
	if srcs, err := s.ForModule(context.Background(), "both"); err != nil || srcs[0].Repo != eeHost+"/modules" {
		t.Fatalf("ForModule(both) = %v, %v; want it kept on the first registry", srcs, err)
	}
}

// A list the flag splits by commas: spaces are dropped, and a repository
// that could never be listed fails the start instead of answering for every
// unknown module.
func TestNewStaticSourcesChecksRepositories(t *testing.T) {
	s, err := NewStaticSources([]string{" reg.example/ee/modules", "http://reg.example/fe/modules/ "}, nil, NewClients(), log.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	if got := []string{s.sources[0].Repo, s.sources[1].Repo}; !slices.Equal(got, []string{"reg.example/ee/modules", "reg.example/fe/modules/"}) {
		t.Fatalf("repositories = %q, want them without the spaces", got)
	}

	for name, repos := range map[string][]string{
		"trailing comma":   {"reg.example/modules", ""},
		"only spaces":      {" "},
		"upper case":       {"reg.example/Modules"},
		"space inside":     {"reg.example/mod ules"},
		"other scheme":     {"https://reg.example/modules"},
		"repeated":         {"reg.example/modules", "reg.example/modules/"},
		"repeated as http": {"reg.example/modules", "http://reg.example/modules"},
	} {
		if _, err := NewStaticSources(repos, nil, NewClients(), log.NewNop()); err == nil {
			t.Errorf("%s: %q is accepted", name, repos)
		}
	}
}
