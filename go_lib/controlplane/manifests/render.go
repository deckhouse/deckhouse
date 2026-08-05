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

// Package manifests renders the control-plane static pod manifests.
//
// The templates live here rather than in candi because go:embed neither follows symlinks nor
// reaches outside its module, and three binaries built from two repositories have to render
// exactly the same bytes: dhctl on the bootstrap path, the on-node agent, and — through the
// helm chart of module 040-control-plane-manager — control-plane-manager itself. The Secret
// those bytes end up in is what the manager checksums, so a difference of one byte between
// two renderers is a control-plane rollout.
//
// candi/control-plane stays as a symlink to this directory: the path is part of the module's
// published layout and outlives the move.
package manifests

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"maps"
	"path"
	"sort"
	"text/template"

	"github.com/Masterminds/sprig/v3"
)

//go:embed templates/*.yaml.tpl
var templatesFS embed.FS

// templates is parsed once at package load. A template that does not parse fails `go test` of
// this package rather than the bootstrap of a cluster.
var templates = template.Must(
	template.New("control-plane").Funcs(funcMap()).ParseFS(templatesFS, "templates/*.yaml.tpl"),
)

// templateDir is where the embedded templates live inside the package.
const templateDir = "templates"

// etcdManifest is rendered first: control-plane-manager writes the bundle in order, and etcd
// has to exist before anything that talks to it.
const etcdManifest = "etcd.yaml"

// funcMap is sprig minus the two functions that read the renderer's own environment. A
// manifest that depended on where it was rendered would differ between dhctl, the agent and
// the manager, which is the one thing this package exists to prevent.
//
// The control-plane templates use no helm-specific helpers — no toYaml, include, tpl or
// lookup — so the map deliberately stops at sprig instead of growing a second copy of
// dhctl's extras.
func funcMap() template.FuncMap {
	f := sprig.TxtFuncMap()
	delete(f, "env")
	delete(f, "expandenv")
	return f
}

// NodeInput is the per-node half of the render context. control-plane-manager passes the
// placeholders it expands on the node ("$NODE_NAME", "$MY_IP") so that one Secret serves
// every node; dhctl and the agent pass real values.
type NodeInput struct {
	NodeName string
	NodeIP   string
}

// Artifact is one rendered file. Name is both the file name on the node and the key of the
// control-plane-manager Secret.
type Artifact struct {
	Name    string
	Content []byte
}

// Bundle is every artifact of one render, in the order they must be written.
type Bundle []Artifact

// Render renders the control-plane manifests.
//
// data is the template context as the templates read it today — the version map merged with
// the cluster settings. It is a map rather than a type on purpose: the typed contract is a
// separate step, and freezing it here would freeze it for the agent's repository too.
func Render(_ context.Context, data map[string]any, node NodeInput) (Bundle, error) {
	entries, err := templatesFS.ReadDir(templateDir)
	if err != nil {
		return nil, fmt.Errorf("read embedded templates: %w", err)
	}

	ctxData := make(map[string]any, len(data)+2)
	maps.Copy(ctxData, data)
	ctxData["nodeName"] = node.NodeName
	ctxData["nodeIP"] = node.NodeIP

	bundle := make(Bundle, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		var buf bytes.Buffer
		if err := templates.ExecuteTemplate(&buf, name, ctxData); err != nil {
			return nil, fmt.Errorf("render %s: %w", name, err)
		}

		bundle = append(bundle, Artifact{
			Name:    manifestName(name),
			Content: buf.Bytes(),
		})
	}

	sortBundle(bundle)
	return bundle, nil
}

// manifestName is the template name without the .tpl suffix, which is what dhctl has always
// written to disk and what the Secret keys are named after.
func manifestName(templateName string) string {
	return templateName[:len(templateName)-len(path.Ext(templateName))]
}

// sortBundle puts etcd first and the rest in name order, so the same inputs always produce
// the same sequence of writes.
func sortBundle(b Bundle) {
	sort.Slice(b, func(i, j int) bool {
		switch {
		case b[i].Name == etcdManifest:
			return true
		case b[j].Name == etcdManifest:
			return false
		default:
			return b[i].Name < b[j].Name
		}
	})
}
