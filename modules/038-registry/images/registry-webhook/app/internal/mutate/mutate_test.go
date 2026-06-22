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

package mutate

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMutate_RewritesRealUpstream(t *testing.T) {
	obj := ModuleSource{
		Metadata: Metadata{Name: "nexus", Annotations: nil},
		Spec:     Spec{Registry: Registry{Scheme: "HTTPS", Repo: "nexus.example.com/modules/a", CA: "REAL-CA", DockerCFG: "REAL-DOCKERCFG"}},
	}
	ops, err := Mutate(obj, Local{ModuleCA: "MODULE-CA", DockerCfg: "LOCAL-DOCKERCFG"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) == 0 {
		t.Fatal("expected patch ops")
	}
	byPath := map[string]PatchOp{}
	for _, o := range ops {
		byPath[o.Path] = o
	}
	// annotations map was nil -> a single add of the whole map.
	anno, ok := byPath["/metadata/annotations"]
	if !ok {
		t.Fatalf("expected add /metadata/annotations, got paths %v", keys(byPath))
	}
	m := anno.Value.(map[string]string)
	var captured Registry
	if err := json.Unmarshal([]byte(m["registry.deckhouse.io/upstream"]), &captured); err != nil {
		t.Fatalf("annotation not the captured registry JSON: %v", err)
	}
	if captured.Repo != "nexus.example.com/modules/a" || captured.CA != "REAL-CA" || captured.DockerCFG != "REAL-DOCKERCFG" {
		t.Fatalf("annotation must capture the REAL upstream, got %+v", captured)
	}
	if byPath["/spec/registry/repo"].Value != "registry.d8-system.svc:5001/nexus.example.com/modules/a" {
		t.Errorf("repo rewrite: got %v", byPath["/spec/registry/repo"].Value)
	}
	if byPath["/spec/registry/scheme"].Value != "HTTPS" {
		t.Errorf("scheme: got %v", byPath["/spec/registry/scheme"].Value)
	}
	if byPath["/spec/registry/ca"].Value != "MODULE-CA" {
		t.Errorf("ca must be the module CA, got %v", byPath["/spec/registry/ca"].Value)
	}
	if byPath["/spec/registry/dockerCfg"].Value != "LOCAL-DOCKERCFG" {
		t.Errorf("dockerCfg must be local creds, got %v", byPath["/spec/registry/dockerCfg"].Value)
	}
}

func TestMutate_IdempotentWhenAlreadyLocal(t *testing.T) {
	obj := ModuleSource{
		Metadata: Metadata{Annotations: map[string]string{"registry.deckhouse.io/upstream": "{}"}},
		Spec:     Spec{Registry: Registry{Repo: "registry.d8-system.svc:5001/nexus.example.com/modules/a"}},
	}
	ops, err := Mutate(obj, Local{ModuleCA: "MODULE-CA", DockerCfg: "LOCAL"})
	if err != nil {
		t.Fatal(err)
	}
	if ops != nil {
		t.Fatalf("already-local registry must be a no-op, got %d ops", len(ops))
	}
}

func TestMutate_ExistingAnnotationsMap(t *testing.T) {
	obj := ModuleSource{
		Metadata: Metadata{Annotations: map[string]string{"other": "x"}},
		Spec:     Spec{Registry: Registry{Repo: "reg.io", Scheme: "HTTP"}},
	}
	ops, _ := Mutate(obj, Local{ModuleCA: "CA", DockerCfg: "L"})
	for _, o := range ops {
		if o.Path == "/metadata/annotations" {
			t.Fatal("must NOT replace the whole annotations map when it already exists")
		}
	}
	found := false
	for _, o := range ops {
		if o.Path == "/metadata/annotations/registry.deckhouse.io~1upstream" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected escaped-pointer add for the upstream annotation key")
	}
}

func keys(m map[string]PatchOp) []string {
	out := []string{}
	for k := range m {
		out = append(out, k)
	}
	return out
}

var _ = strings.TrimSpace
