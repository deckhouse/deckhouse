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

// Package testutil holds helpers shared by the tests of several packages. Production code must not
// import it.
package testutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	helmengine "helm.sh/helm/v3/pkg/engine"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// shippedTemplate is the module's templates/cluster-objects-controller/grantable-resources.yaml, found by
// walking up from the test's working directory (its package directory), so the result does not depend
// on where the calling test lives, nor on build flags such as -trimpath.
func shippedTemplate(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		path := filepath.Join(dir, "templates", "cluster-objects-controller", "grantable-resources.yaml")
		if _, err := os.Stat(path); err == nil {
			return path
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("templates/cluster-objects-controller/grantable-resources.yaml not found above the working directory")
		}
		dir = parent
	}
}

// RenderShipped renders the grant objects the module ships with the real Helm engine -- a stub for the
// lib-helm labels helper and cert-manager enabled, so the conditional objects render too -- and
// returns the documents of the given kind, decoded strictly, so a misspelt field fails instead of
// decoding to its zero value. The module's own chart is the source of truth instead of a transcript
// of it.
func RenderShipped[T any](t *testing.T, kind string) []*T {
	t.Helper()
	tpl, err := os.ReadFile(shippedTemplate(t))
	if err != nil {
		t.Fatal(err)
	}
	ch := &chart.Chart{
		Metadata: &chart.Metadata{Name: "multitenancy-manager", Version: "0.0.0", APIVersion: chart.APIVersionV2},
		Templates: []*chart.File{
			{Name: "templates/_helm_lib_stub.tpl", Data: []byte(`{{- define "helm_lib_module_labels" }}labels: {module: multitenancy-manager}{{- end }}`)},
			{Name: "templates/grantable-resources.yaml", Data: tpl},
		},
	}
	values := chartutil.Values{"Values": map[string]any{"global": map[string]any{"enabledModules": []any{"cert-manager"}}}}
	rendered, err := helmengine.Render(ch, values)
	if err != nil {
		t.Fatal(err)
	}
	out, ok := rendered["multitenancy-manager/templates/grantable-resources.yaml"]
	if !ok {
		t.Fatalf("rendered files: %v", rendered)
	}
	var objs []*T
	for _, doc := range strings.Split(out, "\n---") {
		var meta metav1.TypeMeta
		if err := yaml.Unmarshal([]byte(doc), &meta); err != nil {
			t.Fatalf("document %s: %v", doc, err)
		}
		if meta.Kind != kind {
			continue
		}
		obj := new(T)
		if err := yaml.UnmarshalStrict([]byte(doc), obj); err != nil {
			t.Fatalf("document %s: %v", doc, err)
		}
		objs = append(objs, obj)
	}
	return objs
}
