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

// Package mutate contains the pure logic for rewriting a ModuleSource registry
// to the local in-cluster registry service.
package mutate

import (
	"encoding/json"
	"strings"
)

// PrimarySvc is the in-cluster registry service that intercepts module-source
// traffic; the agent demuxes by the repository path prefix.
const PrimarySvc = "registry.d8-system.svc:5001"

// UpstreamAnnotation holds the JSON of the original (real) spec.registry.
const UpstreamAnnotation = "registry.deckhouse.io/upstream"

type Registry struct {
	Scheme    string `json:"scheme,omitempty"`
	Repo      string `json:"repo"`
	DockerCFG string `json:"dockerCfg"`
	CA        string `json:"ca"`
}
type Spec struct {
	Registry Registry `json:"registry"`
}
type Metadata struct {
	Name        string            `json:"name"`
	Annotations map[string]string `json:"annotations,omitempty"`
}
type ModuleSource struct {
	Metadata Metadata `json:"metadata"`
	Spec     Spec     `json:"spec"`
}

// Local is the local rewrite material injected into spec.registry.
type Local struct {
	ModuleCA  string // module CA PEM (so consumers trust the agent's serving cert)
	DockerCfg string // base64 docker config for the local ReadOnly user @ PrimarySvc
}

type PatchOp struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value,omitempty"`
}

// Mutate returns the JSON patch capturing the real spec.registry into the
// upstream annotation and rewriting spec.registry to the local svc. It returns
// nil (no-op) when the registry is already local (idempotent / GitOps-safe: a
// real upstream — incl. a re-applied original — is always re-captured).
func Mutate(obj ModuleSource, local Local) ([]PatchOp, error) {
	reg := obj.Spec.Registry
	if strings.HasPrefix(reg.Repo, PrimarySvc+"/") {
		return nil, nil // already rewritten to the local svc
	}
	captured, err := json.Marshal(reg)
	if err != nil {
		return nil, err
	}

	ops := make([]PatchOp, 0, 5)
	if obj.Metadata.Annotations == nil {
		ops = append(ops, PatchOp{Op: "add", Path: "/metadata/annotations", Value: map[string]string{UpstreamAnnotation: string(captured)}})
	} else {
		ops = append(ops, PatchOp{Op: "add", Path: "/metadata/annotations/" + escapePtr(UpstreamAnnotation), Value: string(captured)})
	}
	ops = append(ops,
		PatchOp{Op: "replace", Path: "/spec/registry/repo", Value: PrimarySvc + "/" + reg.Repo},
		PatchOp{Op: "replace", Path: "/spec/registry/scheme", Value: "HTTPS"},
		PatchOp{Op: "replace", Path: "/spec/registry/ca", Value: local.ModuleCA},
		PatchOp{Op: "replace", Path: "/spec/registry/dockerCfg", Value: local.DockerCfg},
	)
	return ops, nil
}

func escapePtr(s string) string {
	s = strings.ReplaceAll(s, "~", "~0")
	return strings.ReplaceAll(s, "/", "~1")
}
