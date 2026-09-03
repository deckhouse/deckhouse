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

package bootstrap

import (
	"context"
	"fmt"
	"path"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

// TemplatesConfigMapName holds the candi/bashible templates the bootstrap
// render needs. Helm fills it from the chart, so the templates and this binary
// always come from the same release — a copy baked into the image could not
// promise that across a rollback.
const TemplatesConfigMapName = "bashible-bootstrap-templates"

// Files answers the .Files.Get calls the bashible templates make. Keys are
// basenames: the templates read each other by full path, and both an absolute
// and a repo-relative spelling of the same file must resolve alike.
type Files struct {
	text   map[string]string
	binary map[string][]byte
}

func LoadFiles(ctx context.Context, r client.Reader) (*Files, error) {
	cm := &corev1.ConfigMap{}
	key := types.NamespacedName{Namespace: nodecommon.MachineNamespace, Name: TemplatesConfigMapName}
	if err := r.Get(ctx, key, cm); err != nil {
		return nil, fmt.Errorf("read bootstrap templates %s: %w", key, err)
	}
	if cm.Data["lib.sh.tpl"] == "" {
		return nil, fmt.Errorf("bootstrap templates %s carry no lib.sh.tpl", key)
	}
	return &Files{text: cm.Data, binary: cm.BinaryData}, nil
}

// Get returns the file by its basename, or an empty string when it is absent —
// the templates rely on `.Files.Get A | default (.Files.Get B)`.
func (f *Files) Get(p string) string {
	return f.text[templateKey(p)]
}

func (f *Files) Binary(name string) []byte {
	return f.binary[name]
}

// templateKey maps a candi path to the flat ConfigMap key. A provider network
// script keeps its provider in the key, because all of them share a basename.
func templateKey(p string) string {
	base := path.Base(p)
	if base != "bootstrap-networks.sh.tpl" {
		return base
	}
	parts := strings.Split(path.Clean(p), "/")
	for i, part := range parts {
		if part == "cloud-providers" && i+1 < len(parts) {
			return "bootstrap-networks-" + parts[i+1] + ".sh.tpl"
		}
	}
	return base
}
