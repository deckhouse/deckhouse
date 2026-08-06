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
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
)

//go:embed templates/*.yaml.tpl
var templatesFS embed.FS

//go:embed extra-files/*.yaml.tpl
var extraFilesFS embed.FS

// templates is parsed once at package load. A template that does not parse fails `go test` of
// this package rather than the bootstrap of a cluster.
var templates = template.Must(
	template.New("control-plane").Funcs(funcMap()).ParseFS(templatesFS, "templates/*.yaml.tpl"),
)

var extraFiles = template.Must(
	template.New("extra-files").Funcs(funcMap()).ParseFS(extraFilesFS, "extra-files/*.yaml.tpl"),
)

// templateDir is where the embedded templates live inside the package.
const templateDir = "templates"

// authenticationConfig is the one extra file kube-apiserver demands on every run, bootstrap
// included — the manifest passes --authentication-config unconditionally.
const authenticationConfig = "authentication-config.yaml"

const templateSuffix = ".tpl"

// RunTypeClusterBootstrap is the run that brings the first control plane up, before there is a
// cluster to read anything from. The manifests gate several flags on it.
const RunTypeClusterBootstrap = "ClusterBootstrap"

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

	ctxData := renderContext(data, node)

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

// RenderExtraFiles renders the files the manifests reference by path — the ones that live in
// /etc/kubernetes/deckhouse/extra-files rather than inside the pod spec.
//
// Only the bootstrap set is here. kube-apiserver passes --authentication-config on every run,
// so a node that starts the static pod without that file on disk gets an apiserver that never
// comes up and a bootstrap that hangs waiting for the API rather than failing. The rest of the
// keys are still produced by the module's helm templates for the day-2 path.
//
// An input that would make a manifest reference a file this package cannot produce yet is an
// error, not a silent omission: the alternative is exactly the hang above, discovered on a node
// instead of here.
func RenderExtraFiles(_ context.Context, data map[string]any, node NodeInput) (Bundle, error) {
	if missing := unsupportedExtraFiles(data); len(missing) > 0 {
		return nil, fmt.Errorf(
			"the manifests would reference %s, which this package does not render yet — "+
				"the control-plane module's helm templates still own those files",
			strings.Join(missing, ", "),
		)
	}

	ctxData := renderContext(data, node)

	var buf bytes.Buffer
	name := authenticationConfig + templateSuffix
	if err := extraFiles.ExecuteTemplate(&buf, name, ctxData); err != nil {
		return nil, fmt.Errorf("render %s: %w", authenticationConfig, err)
	}

	return Bundle{{Name: authenticationConfig, Content: buf.Bytes()}}, nil
}

// unsupportedExtraFiles names the files this input would make a manifest reference and that the
// package does not render. The conditions mirror the gates in the manifests one for one; if a
// gate there changes, this has to change with it.
func unsupportedExtraFiles(data map[string]any) []string {
	apiserver, _ := data["apiserver"].(map[string]any)

	var missing []string
	for _, dep := range []struct {
		file     string
		required bool
	}{
		{"audit-policy.yaml", isSet(apiserver["auditPolicy"])},
		{"authorization-config.yaml", isSet(apiserver["webhookURL"])},
		{"authn-webhook-config.yaml", isSet(apiserver["authnWebhookURL"])},
		{"audit-webhook-config.yaml", isSet(apiserver["auditWebhookURL"])},
		{"secret-encryption-config.yaml", isSet(apiserver["secretEncryptionKey"]) || isSet(apiserver["signature"])},
		{"admission-control-config.yaml", data["runType"] != RunTypeClusterBootstrap},
		{"scheduler-config.yaml", data["runType"] != RunTypeClusterBootstrap},
	} {
		if dep.required {
			missing = append(missing, dep.file)
		}
	}

	return missing
}

// isSet answers the question a template's {{ if }} asks: sprig truthiness, not presence.
func isSet(v any) bool {
	switch value := v.(type) {
	case nil:
		return false
	case string:
		return value != ""
	case bool:
		return value
	default:
		return true
	}
}

// renderContext is the caller's data with the node placed on top. The node wins: it is the one
// part of the context the caller cannot be trusted to have filled for the node being rendered.
func renderContext(data map[string]any, node NodeInput) map[string]any {
	ctxData := make(map[string]any, len(data)+2)
	maps.Copy(ctxData, data)
	ctxData["nodeName"] = node.NodeName
	ctxData["nodeIP"] = node.NodeIP
	return ctxData
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
