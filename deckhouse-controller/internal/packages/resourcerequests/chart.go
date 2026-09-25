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

package resourcerequests

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

const (
	chartFile    = "Chart.yaml"
	templatesDir = "templates"

	// dirPerm and filePerm are the modes of the chart the overlay writes. It
	// lives in the process temp dir for the length of one install, so it is
	// readable by its owner only.
	dirPerm  = 0o700
	filePerm = 0o600
)

// WriteChart materialises already-rendered resources as a Helm chart under dir:
// one template per resource, holding the manifest verbatim.
//
// Rendering that chart again is a no-op, which is the point — it is how the
// overlay reaches the install, since nelm renders the chart it is given rather
// than taking manifests. Two consequences are worth knowing:
//
//   - Anything a chart carries beyond the manifests it renders is dropped:
//     NOTES.txt, values schemas, subcharts. Hooks survive, because nelm
//     reclassifies them from the annotations on the manifests themselves.
//   - Template actions that survived the first render (a ConfigMap holding a
//     Grafana dashboard, say) are escaped so the second render hands them back
//     unchanged.
//
// srcChart is the chart the resources were rendered from; its Chart.yaml is
// carried over so the release keeps its name, version and appVersion.
func WriteChart(dir, srcChart string, resources []*unstructured.Unstructured) error {
	if err := os.MkdirAll(filepath.Join(dir, templatesDir), dirPerm); err != nil {
		return fmt.Errorf("create chart dir: %w", err)
	}

	metadata, err := chartMetadata(srcChart)
	if err != nil {
		return fmt.Errorf("build chart metadata: %w", err)
	}

	if err = os.WriteFile(filepath.Join(dir, chartFile), metadata, filePerm); err != nil {
		return fmt.Errorf("write %s: %w", chartFile, err)
	}

	for i, resource := range resources {
		if resource == nil {
			continue
		}

		manifest, err := yaml.Marshal(resource)
		if err != nil {
			return fmt.Errorf("marshal resource: %w", err)
		}

		name := filepath.Join(dir, templatesDir, templateName(i, resource))
		if err = os.WriteFile(name, []byte(escapeTemplating(string(manifest))), filePerm); err != nil {
			return fmt.Errorf("write template: %w", err)
		}
	}

	return nil
}

// chartMetadata returns the Chart.yaml for the overlay chart, built from the
// source chart's own so the release keeps its identity.
//
// dependencies are dropped: the overlay carries no charts/ directory, and Helm
// refuses a chart that declares a dependency it cannot find. Nothing is lost,
// since every subchart resource is already rendered into the templates.
//
// A package may legitimately have no Chart.yaml at all — isHelmChart accepts a
// bare templates/ directory — in which case the minimal metadata below stands in,
// exactly as nelm's own DefaultChart* options would.
func chartMetadata(srcChart string) ([]byte, error) {
	raw, err := os.ReadFile(filepath.Join(srcChart, chartFile))
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read source %s: %w", chartFile, err)
		}

		return yaml.Marshal(map[string]any{
			"apiVersion": "v2",
			"name":       filepath.Base(srcChart),
			"version":    "0.0.0",
		})
	}

	metadata := make(map[string]any)
	if err = yaml.Unmarshal(raw, &metadata); err != nil {
		return nil, fmt.Errorf("parse source %s: %w", chartFile, err)
	}

	delete(metadata, "dependencies")

	return yaml.Marshal(metadata)
}

// templateName builds a stable, unique and greppable file name for a resource.
// The index prefix is what makes it unique — two resources of the same kind and
// name can legitimately live in different namespaces.
func templateName(index int, resource *unstructured.Unstructured) string {
	return fmt.Sprintf("%04d-%s-%s.yaml", index, sanitize(resource.GetKind()), sanitize(resource.GetName()))
}

// sanitize reduces a name to lowercase alphanumerics, dashes and dots, so it is
// safe as a file name whatever the manifest holds.
func sanitize(s string) string {
	mapped := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '.':
			return r
		case r >= 'A' && r <= 'Z':
			return r - 'A' + 'a'
		default:
			return '-'
		}
	}, s)

	if mapped == "" {
		return "resource"
	}

	return mapped
}

// escapeTemplating neutralises Go template actions in already-rendered YAML, so
// the second render returns the manifest unchanged.
//
// Only the opening delimiter needs escaping: a lone "}}" is literal text to the
// template engine, and the replacement itself renders back to "{{".
func escapeTemplating(manifest string) string {
	return strings.ReplaceAll(manifest, "{{", `{{"{{"}}`)
}
