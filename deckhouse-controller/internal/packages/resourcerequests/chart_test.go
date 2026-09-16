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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

// writeSourceChart lays out a chart to overlay, returning its path.
func writeSourceChart(t *testing.T, metadata string) string {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, templatesDir), 0o700))

	if metadata != "" {
		require.NoError(t, os.WriteFile(filepath.Join(dir, chartFile), []byte(metadata), 0o600))
	}

	return dir
}

// readChartMetadata parses the Chart.yaml the overlay wrote.
func readChartMetadata(t *testing.T, dir string) map[string]any {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(dir, chartFile))
	require.NoError(t, err)

	metadata := make(map[string]any)
	require.NoError(t, yaml.Unmarshal(raw, &metadata))

	return metadata
}

// templateFiles lists the overlay's templates, sorted by name.
func templateFiles(t *testing.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(filepath.Join(dir, templatesDir))
	require.NoError(t, err)

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}

	return names
}

func TestWriteChartCarriesSourceMetadataWithoutDependencies(t *testing.T) {
	src := writeSourceChart(t, `apiVersion: v2
name: console
version: 1.4.0
appVersion: "2.1"
dependencies:
  - name: redis
    version: 1.0.0
    repository: https://example.invalid
`)

	dir := filepath.Join(t.TempDir(), "overlay")
	require.NoError(t, WriteChart(dir, src, nil))

	metadata := readChartMetadata(t, dir)
	require.Equal(t, "console", metadata["name"])
	require.Equal(t, "1.4.0", metadata["version"])
	require.Equal(t, "2.1", metadata["appVersion"])

	// The overlay carries no charts/ directory, and Helm refuses a chart that
	// declares a dependency it cannot find.
	require.NotContains(t, metadata, "dependencies")
}

func TestWriteChartFallsBackWhenSourceHasNoMetadata(t *testing.T) {
	src := writeSourceChart(t, "")

	dir := filepath.Join(t.TempDir(), "overlay")
	require.NoError(t, WriteChart(dir, src, nil))

	metadata := readChartMetadata(t, dir)
	require.Equal(t, "v2", metadata["apiVersion"])
	require.Equal(t, filepath.Base(src), metadata["name"])
	require.Equal(t, "0.0.0", metadata["version"])
}

func TestWriteChartWritesOneTemplatePerResource(t *testing.T) {
	src := writeSourceChart(t, "apiVersion: v2\nname: console\nversion: 1.0.0\n")

	resources := []*unstructured.Unstructured{
		parse(t, "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: controller\n"),
		parse(t, "apiVersion: v1\nkind: Service\nmetadata:\n  name: Controller-API\n"),
	}

	dir := filepath.Join(t.TempDir(), "overlay")
	require.NoError(t, WriteChart(dir, src, resources))

	require.Equal(t, []string{
		"0000-deployment-controller.yaml",
		"0001-service-controller-api.yaml",
	}, templateFiles(t, dir))

	raw, err := os.ReadFile(filepath.Join(dir, templatesDir, "0000-deployment-controller.yaml"))
	require.NoError(t, err)
	require.Contains(t, string(raw), "kind: Deployment")
	require.Contains(t, string(raw), "name: controller")
}

// The overlay is rendered a second time by nelm, so a manifest that still holds
// template actions — a ConfigMap carrying a Grafana dashboard, say — has to come
// back out of that render byte for byte.
func TestWriteChartTemplatesRenderBackToTheSameManifest(t *testing.T) {
	src := writeSourceChart(t, "apiVersion: v2\nname: console\nversion: 1.0.0\n")

	dashboard := parse(t, `apiVersion: v1
kind: ConfigMap
metadata:
  name: dashboards
data:
  panel: '{{ $labels.pod }} uses {{ printf "%d" .Value }}'
`)

	dir := filepath.Join(t.TempDir(), "overlay")
	require.NoError(t, WriteChart(dir, src, []*unstructured.Unstructured{dashboard}))

	raw, err := os.ReadFile(filepath.Join(dir, templatesDir, "0000-configmap-dashboards.yaml"))
	require.NoError(t, err)
	require.NotContains(t, string(raw), "{{ $labels.pod }}")

	rendered := &strings.Builder{}
	tmpl, err := template.New("overlay").Parse(string(raw))
	require.NoError(t, err)
	require.NoError(t, tmpl.Execute(rendered, nil))

	roundTripped := make(map[string]any)
	require.NoError(t, yaml.Unmarshal([]byte(rendered.String()), &roundTripped))
	require.Equal(t, dashboard.Object, roundTripped)
}

func TestSanitize(t *testing.T) {
	require.Equal(t, "controller-api", sanitize("Controller-API"))
	require.Equal(t, "d8.io-agent", sanitize("d8.io/agent"))
	require.Equal(t, "resource", sanitize(""))
}
