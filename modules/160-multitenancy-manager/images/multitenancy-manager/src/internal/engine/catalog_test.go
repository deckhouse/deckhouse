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

package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"controller/api/v1alpha1"
	"controller/internal/jsonpath"
	"controller/internal/testutil"
)

// projected runs ProjectCatalogFields and renders the result as name -> raw JSON for comparison.
func projected(fields []v1alpha1.CatalogField, obj map[string]any) map[string]string {
	got, _ := ProjectCatalogFields(factory(), fields, obj)
	if got == nil {
		return nil
	}
	out := make(map[string]string, len(got))
	for k, v := range got {
		out[k] = string(v.Raw)
	}
	return out
}

func field(name, path string) v1alpha1.CatalogField {
	return v1alpha1.CatalogField{Name: name, Path: path}
}

func assertProjected(t *testing.T, got, want map[string]string) {
	t.Helper()
	// DeepEqual, not a printed comparison: "no fields" is nil by contract, and an empty map is not.
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("projected %v, want %v", got, want)
	}
}

func TestProjectCatalogFields_Values(t *testing.T) {
	obj := map[string]any{
		"provisioner":          "csi.example.com",
		"allowVolumeExpansion": true,
		"replicas":             int64(3),
		"ratio":                1.5,
		"spec":                 map[string]any{"zone": "a", "tags": []any{"x", "y"}},
		"empty":                nil,
		"exact":                strings.Repeat("a", MaxCatalogFieldValueBytes-2), // 512 bytes with the quotes
		"over":                 strings.Repeat("a", MaxCatalogFieldValueBytes-1), // 513 bytes
	}
	got := projected([]v1alpha1.CatalogField{
		field("provisioner", "$.provisioner"),
		field("expand", "$.allowVolumeExpansion"),
		field("replicas", "$.replicas"),
		field("ratio", "$.ratio"),
		field("spec", "$.spec"),
		field("firstTag", "$.spec.tags[0]"),
		field("missing", "$.spec.nope"),
		field("empty", "$.empty"),
		field("exact", "$.exact"),
		field("over", "$.over"),
	}, obj)
	assertProjected(t, got, map[string]string{
		"provisioner": `"csi.example.com"`,
		"expand":      `true`,
		"replicas":    `3`,
		"ratio":       `1.5`,
		"spec":        `{"tags":["x","y"],"zone":"a"}`,
		"firstTag":    `"x"`,
		"exact":       `"` + strings.Repeat("a", MaxCatalogFieldValueBytes-2) + `"`,
	})
}

func TestProjectCatalogFields_NothingProjectedIsNil(t *testing.T) {
	if got, _ := ProjectCatalogFields(factory(), []v1alpha1.CatalogField{field("a", "$.a")}, map[string]any{}); got != nil {
		t.Fatalf("want nil, got %v", got)
	}
}

func TestCompileCatalogPath_Refused(t *testing.T) {
	for _, c := range []struct{ path, reason string }{
		// Does not compile.
		{"$.a[", "not a valid"},
		{"spec.a", "not a valid"},
		// Not singular.
		{"$.a[*]", "not a singular"},
		{"$..a", "not a singular"},
		{"$.a[?@.b == 1]", "not a singular"},
		{"$.a[0:2]", "not a singular"},
		{"$['a','b']", "not a singular"},
		// Forbidden, in any spelling, and their parents.
		{"$.metadata.managedFields", "overlaps"},
		{"$.metadata.managedFields[0]", "overlaps"},
		{"$.metadata.managedFields[0].manager", "overlaps"},
		{`$["metadata"]["managedFields"]`, "overlaps"},
		{`$.metadata["managedFields"]`, "overlaps"},
		{"$.metadata.annotations['kubectl.kubernetes.io/last-applied-configuration']", "overlaps"},
		{`$.metadata.annotations["kubectl.kubernetes.io/last-applied-configuration"]`, "overlaps"},
		{"$.metadata.annotations", "overlaps"},
		{"$.metadata", "overlaps"},
		{"$", "overlaps"},
	} {
		path := c.path
		t.Run(path, func(t *testing.T) {
			if _, err := CompileCatalogPath(factory(), path); err == nil || !strings.Contains(err.Error(), c.reason) {
				t.Fatalf("path %q must be refused with %q, err=%v", path, c.reason, err)
			}
			obj := map[string]any{
				"a":        []any{map[string]any{"b": 1}},
				"metadata": map[string]any{"managedFields": []any{map[string]any{"manager": "m"}}, "annotations": map[string]any{"kubectl.kubernetes.io/last-applied-configuration": "{}"}},
			}
			if got := projected([]v1alpha1.CatalogField{field("f", path)}, obj); got != nil {
				t.Fatalf("path %q projected %v", path, got)
			}
		})
	}
}

func TestCompileCatalogPath_Allowed(t *testing.T) {
	for _, path := range []string{
		"$.metadata.name",
		"$.metadata.labels['app']",
		"$.metadata.annotations['other']",
		"$['metadata']['annotations']['kubectl.kubernetes.io/other']",
		"$.items[-1]",
	} {
		if _, err := CompileCatalogPath(factory(), path); err != nil {
			t.Fatalf("path %q must be allowed: %v", path, err)
		}
	}
}

func TestProjectCatalogFields_Limits(t *testing.T) {
	obj := map[string]any{}
	var fields []v1alpha1.CatalogField
	want := map[string]string{}
	for i := range MaxCatalogFields + 2 {
		key := fmt.Sprintf("f%d", i)
		obj[key] = i
		fields = append(fields, field(key, "$."+key))
		if i < MaxCatalogFields {
			want[key] = fmt.Sprint(i)
		}
	}
	assertProjected(t, projected(fields, obj), want)
}

func TestProjectCatalogFields_DuplicateNameKeepsTheFirstEntry(t *testing.T) {
	obj := map[string]any{"a": "first", "b": "second"}
	assertProjected(t, projected([]v1alpha1.CatalogField{field("x", "$.a"), field("x", "$.b")}, obj), map[string]string{"x": `"first"`})
	// The first entry owns the name even when it yields nothing.
	assertProjected(t, projected([]v1alpha1.CatalogField{field("x", "$.missing"), field("x", "$.b")}, obj), nil)
}

// shippedDefinition returns the named GrantableClusterResourceDefinition of the module's own Helm chart.
func shippedDefinition(t *testing.T, name string) *v1alpha1.GrantableClusterResourceDefinition {
	t.Helper()
	for _, def := range testutil.RenderShipped[v1alpha1.GrantableClusterResourceDefinition](t, "GrantableClusterResourceDefinition") {
		if def.Name == name {
			return def
		}
	}
	t.Fatalf("definition %s is not in the rendered template", name)
	return nil
}

// TestShippedStorageClassCatalogFields projects the shipped storageclasses catalogFields onto a
// typical StorageClass. parameters must not leak: it can name secrets.
func TestShippedStorageClassCatalogFields(t *testing.T) {
	def := shippedDefinition(t, "storageclasses")
	reclaim := corev1.PersistentVolumeReclaimRetain
	binding := storagev1.VolumeBindingWaitForFirstConsumer
	expand := true
	sc := &storagev1.StorageClass{
		TypeMeta:             metav1.TypeMeta{APIVersion: "storage.k8s.io/v1", Kind: "StorageClass"},
		ObjectMeta:           metav1.ObjectMeta{Name: "fast"},
		Provisioner:          "rbd.csi.ceph.com",
		Parameters:           map[string]string{"csi.storage.k8s.io/provisioner-secret-name": "ceph-secret"},
		ReclaimPolicy:        &reclaim,
		VolumeBindingMode:    &binding,
		AllowVolumeExpansion: &expand,
	}
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(sc)
	if err != nil {
		t.Fatal(err)
	}
	assertProjected(t, projected(def.Spec.CatalogFields, obj), map[string]string{
		"provisioner":          `"rbd.csi.ceph.com"`,
		"reclaimPolicy":        `"Retain"`,
		"volumeBindingMode":    `"WaitForFirstConsumer"`,
		"allowVolumeExpansion": `true`,
	})
}

// crdsDir holds the module's CRDs, relative to this package.
var crdsDir = filepath.Join("..", "..", "..", "..", "..", "crds")

// TestCatalogLimitsMatchTheCRDs: MaxCatalogFields and MaxCatalogFieldValueBytes repeat the maxItems of
// spec.catalogFields and the numbers the CRD descriptions promise; nothing else keeps them in sync.
func TestCatalogLimitsMatchTheCRDs(t *testing.T) {
	read := func(name string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(crdsDir, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	gcrd := read("multitenancy.deckhouse.io_grantableclusterresourcedefinitions.yaml")
	crd := &apiextensionsv1.CustomResourceDefinition{}
	if err := yaml.Unmarshal([]byte(gcrd), crd); err != nil {
		t.Fatal(err)
	}
	for _, v := range crd.Spec.Versions {
		catalogFields := v.Schema.OpenAPIV3Schema.Properties["spec"].Properties["catalogFields"]
		if catalogFields.MaxItems == nil || *catalogFields.MaxItems != MaxCatalogFields {
			t.Fatalf("%s: spec.catalogFields.maxItems = %v, want %d", v.Name, catalogFields.MaxItems, MaxCatalogFields)
		}
	}
	if MaxCatalogFieldsBytes%1024 != 0 {
		t.Fatalf("MaxCatalogFieldsBytes = %d is not a whole number of KiB, as the CRDs state it", MaxCatalogFieldsBytes)
	}
	budgetKiB := MaxCatalogFieldsBytes / 1024
	for name, want := range map[string][]string{
		"multitenancy.deckhouse.io_grantableclusterresourcedefinitions.yaml": {
			fmt.Sprintf("At most %d fields", MaxCatalogFields), fmt.Sprintf("longer than %d bytes", MaxCatalogFieldValueBytes),
			fmt.Sprintf("more than %d KiB", budgetKiB),
		},
		"multitenancy.deckhouse.io_availableclusterresources.yaml": {
			fmt.Sprintf("longer than %d bytes", MaxCatalogFieldValueBytes), fmt.Sprintf("more than %d KiB", budgetKiB),
		},
	} {
		text := read(name)
		for _, w := range want {
			if !strings.Contains(text, w) {
				t.Errorf("%s does not say %q", name, w)
			}
		}
	}
	// The Russian descriptions are checked for the same numbers as standalone tokens rather than for
	// their wording: Go sources in this repository must stay free of Cyrillic (dmt no-cyrillic). The
	// number is looked up in the description of the field that states the limit only, so a number of
	// the same value elsewhere in the file does not satisfy the check.
	ruDescription := func(name string, field func(root apiextensionsv1.JSONSchemaProps) apiextensionsv1.JSONSchemaProps) string {
		t.Helper()
		doc := &apiextensionsv1.CustomResourceDefinition{}
		if err := yaml.Unmarshal([]byte(read(name)), doc); err != nil {
			t.Fatal(err)
		}
		if len(doc.Spec.Versions) == 0 || doc.Spec.Versions[0].Schema == nil || doc.Spec.Versions[0].Schema.OpenAPIV3Schema == nil {
			t.Fatalf("%s has no schema", name)
		}
		return field(*doc.Spec.Versions[0].Schema.OpenAPIV3Schema).Description
	}
	for _, c := range []struct {
		file, field string
		desc        func(apiextensionsv1.JSONSchemaProps) apiextensionsv1.JSONSchemaProps
		want        []int
	}{
		{
			file:  "doc-ru-multitenancy.deckhouse.io_grantableclusterresourcedefinitions.yaml",
			field: "spec.catalogFields",
			desc: func(root apiextensionsv1.JSONSchemaProps) apiextensionsv1.JSONSchemaProps {
				return root.Properties["spec"].Properties["catalogFields"]
			},
			want: []int{MaxCatalogFields},
		},
		{
			file:  "doc-ru-multitenancy.deckhouse.io_availableclusterresources.yaml",
			field: "status.available.items.fields",
			desc: func(root apiextensionsv1.JSONSchemaProps) apiextensionsv1.JSONSchemaProps {
				available := root.Properties["status"].Properties["available"]
				if available.Items == nil || available.Items.Schema == nil {
					return apiextensionsv1.JSONSchemaProps{}
				}
				return available.Items.Schema.Properties["fields"]
			},
			want: nil,
		},
	} {
		text := ruDescription(c.file, c.desc)
		for _, n := range c.want {
			if !regexp.MustCompile(`(^|\D)` + strconv.Itoa(n) + `(\D|$)`).MatchString(text) {
				t.Errorf("%s: the description of %s does not mention %d", c.file, c.field, n)
			}
		}
		// Both fields state the value limit and the budget. Their numbers are equal (512), so each is
		// looked up with its unit, bytes and KiB in Russian, spelled with escapes to keep this file free
		// of Cyrillic.
		if b := strconv.Itoa(MaxCatalogFieldValueBytes) + " \u0431\u0430\u0439\u0442"; !strings.Contains(text, b) {
			t.Errorf("%s: the description of %s does not mention %d bytes", c.file, c.field, MaxCatalogFieldValueBytes)
		}
		if kib := strconv.Itoa(budgetKiB) + " \u041a\u0438\u0411"; !strings.Contains(text, kib) {
			t.Errorf("%s: the description of %s does not mention %d KiB", c.file, c.field, budgetKiB)
		}
	}
}

// TestPathLimitsMatchTheCRDs: every JSONPath field in the CRDs has a maxLength that the parsed-path
// cache covers, so a stored ASCII path is always cached and no stored path (256 characters, at most
// 1024 bytes) hits jsonpath.MaxPathLen.
func TestPathLimitsMatchTheCRDs(t *testing.T) {
	load := func(name string) apiextensionsv1.JSONSchemaProps {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(crdsDir, name))
		if err != nil {
			t.Fatal(err)
		}
		crd := &apiextensionsv1.CustomResourceDefinition{}
		if err := yaml.Unmarshal(raw, crd); err != nil {
			t.Fatal(err)
		}
		if len(crd.Spec.Versions) != 1 {
			t.Fatalf("%s: %d versions, the test reads only one", name, len(crd.Spec.Versions))
		}
		return crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"]
	}
	items := func(p apiextensionsv1.JSONSchemaProps) apiextensionsv1.JSONSchemaProps {
		if p.Items == nil || p.Items.Schema == nil {
			return apiextensionsv1.JSONSchemaProps{}
		}
		return *p.Items.Schema
	}
	ref := items(load("multitenancy.deckhouse.io_grantableclusterresourcereferences.yaml").Properties["fieldPaths"])
	def := items(load("multitenancy.deckhouse.io_grantableclusterresourcedefinitions.yaml").Properties["catalogFields"])
	for name, field := range map[string]apiextensionsv1.JSONSchemaProps{
		"GrantableClusterResourceReference spec.fieldPaths[].path":            ref.Properties["path"],
		"GrantableClusterResourceReference spec.fieldPaths[].match.fieldPath": ref.Properties["match"].Properties["fieldPath"],
		"GrantableClusterResourceDefinition spec.catalogFields[].path":        def.Properties["path"],
	} {
		if field.MaxLength == nil || *field.MaxLength > jsonpath.MaxCachedPathLen || jsonpath.MaxCachedPathLen > jsonpath.MaxPathLen {
			t.Errorf("%s: maxLength = %v, want at most MaxCachedPathLen (%d) <= MaxPathLen (%d)", name, field.MaxLength, jsonpath.MaxCachedPathLen, jsonpath.MaxPathLen)
		}
	}
}

// TestCatalogSize_Boundary: CatalogSize counts the whole serialized list and accepts exactly
// MaxCatalogFieldsBytes.
func TestCatalogSize_Boundary(t *testing.T) {
	entry := func(pad int) []v1alpha1.AvailableObject {
		return []v1alpha1.AvailableObject{{Name: "a", Fields: map[string]apiextensionsv1.JSON{"f": {Raw: []byte(`"` + strings.Repeat("v", pad) + `"`)}}}}
	}
	overhead := len(`[{"name":"a","fields":{"f":""}}]`)
	for _, c := range []struct {
		size int
		fits bool
	}{{MaxCatalogFieldsBytes, true}, {MaxCatalogFieldsBytes + 1, false}} {
		got, fits := CatalogSize(entry(c.size - overhead))
		if got != c.size || fits != c.fits {
			t.Fatalf("CatalogSize = %d, %v; want %d, %v", got, fits, c.size, c.fits)
		}
	}
}

// TestCatalogSize_MatchesMarshal: counted entry by entry, the size of a catalog that fits is the length
// of json.Marshal of the whole list, and one over the budget stops counting early but is still refused.
func TestCatalogSize_MatchesMarshal(t *testing.T) {
	value := func(s string) apiextensionsv1.JSON { return apiextensionsv1.JSON{Raw: []byte(s)} }
	for _, list := range [][]v1alpha1.AvailableObject{
		{},
		{{Name: "a"}},
		{{Name: "a", Default: true}, {Name: "b<&>"}},
		{
			{Name: "a", Fields: map[string]apiextensionsv1.JSON{"n": value("1"), "s": value(`"x"`)}},
			{Name: "b", Default: true, Fields: map[string]apiextensionsv1.JSON{"o": value(`{"k":[1,true,null]}`)}},
			{Name: "c"},
		},
	} {
		raw, err := json.Marshal(list)
		if err != nil {
			t.Fatal(err)
		}
		if got, fits := CatalogSize(list); got != len(raw) || !fits {
			t.Fatalf("CatalogSize(%s) = %d, %v; want %d, true", raw, got, fits, len(raw))
		}
	}

	big := value(`"` + strings.Repeat("v", MaxCatalogFieldValueBytes-2) + `"`)
	var over []v1alpha1.AvailableObject
	for i := range 2 * MaxCatalogFieldsBytes / MaxCatalogFieldValueBytes {
		over = append(over, v1alpha1.AvailableObject{Name: fmt.Sprintf("o%d", i), Fields: map[string]apiextensionsv1.JSON{"f": big}})
	}
	raw, err := json.Marshal(over)
	if err != nil {
		t.Fatal(err)
	}
	if got, fits := CatalogSize(over); fits || got <= MaxCatalogFieldsBytes || got >= len(raw) {
		t.Fatalf("CatalogSize = %d, %v; want a count past %d that stops before %d", got, fits, MaxCatalogFieldsBytes, len(raw))
	}
}
