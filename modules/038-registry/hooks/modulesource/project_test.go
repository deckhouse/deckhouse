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

package modulesource

import (
	"testing"
)

func TestProjectEntries_FromAnnotation(t *testing.T) {
	// already-rewritten MS: real upstream comes from the annotation.
	snaps := []MSSnap{{
		Name:         "nexus",
		RepoInSpec:   "registry.d8-system.svc:5001/nexus.example.com/modules/a",
		UpstreamJSON: `{"scheme":"HTTPS","repo":"nexus.example.com/modules/a","ca":"REAL-CA","dockerCfg":"REAL-DCFG"}`,
	}}
	entries, toPatch := projectEntries(snaps)
	if len(toPatch) != 0 {
		t.Fatalf("already-local MS must not need a backstop patch, got %d", len(toPatch))
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Host != "nexus.example.com/modules/a" {
		t.Errorf("host (path prefix): got %q", e.Host)
	}
	if e.Upstream.Host != "nexus.example.com" || e.Upstream.Path != "modules/a" {
		t.Errorf("upstream split: got host=%q path=%q", e.Upstream.Host, e.Upstream.Path)
	}
	if e.Upstream.CA != "REAL-CA" || e.Upstream.Credentials.DockerCfg != "REAL-DCFG" {
		t.Errorf("upstream creds/ca must be REAL: %+v", e.Upstream)
	}
}

func TestProjectEntries_BackstopNotYetRewritten(t *testing.T) {
	// pre-existing MS: spec still real, no annotation -> needs backstop + project from spec.
	snaps := []MSSnap{{
		Name:       "pre",
		RepoInSpec: "nexus.example.com/modules/b",
		SpecScheme: "HTTPS", SpecCA: "RCA", SpecDockerCfg: "RD",
	}}
	entries, toPatch := projectEntries(snaps)
	if len(toPatch) != 1 || toPatch[0] != "pre" {
		t.Fatalf("pre-existing MS must be queued for backstop rewrite, got %v", toPatch)
	}
	if len(entries) != 1 || entries[0].Host != "nexus.example.com/modules/b" || entries[0].Upstream.Path != "modules/b" {
		t.Fatalf("must project from spec when not yet rewritten, got %+v", entries)
	}
}
