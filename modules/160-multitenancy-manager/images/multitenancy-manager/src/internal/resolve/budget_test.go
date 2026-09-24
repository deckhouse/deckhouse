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

package resolve

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"controller/api/v1alpha1"
	"controller/internal/engine"
	"controller/internal/jsonpath"
)

// budgetCatalog builds a resolved object-backed registration whose catalog, with every field, takes
// exactly target bytes in the serialization engine.MaxCatalogFieldsBytes bounds. The size is computed
// here from json.Marshal, not with engine.CatalogSize, and is linear in the value lengths (one more
// "v" is one more byte), so it can be padded to the byte.
func budgetCatalog(t *testing.T, target int) *Resolved {
	t.Helper()
	const base = 400 // value length; the padding keeps it under MaxCatalogFieldValueBytes
	reg := &v1alpha1.GrantableClusterResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "storageclasses"}}
	reg.Spec.GrantedResource = &v1alpha1.GrantedResource{APIGroup: "storage.k8s.io", Kind: "StorageClass"}
	for i := range engine.MaxCatalogFields {
		reg.Spec.CatalogFields = append(reg.Spec.CatalogFields, v1alpha1.CatalogField{Name: fmt.Sprintf("p%d", i), Path: fmt.Sprintf("$.p%d", i)})
	}
	lengths := [][]int{} // per object, per field
	size := func() int {
		list := make([]v1alpha1.AvailableObject, len(lengths))
		for o, ls := range lengths {
			fields := map[string]apiextensionsv1.JSON{}
			for f, l := range ls {
				fields[fmt.Sprintf("p%d", f)] = apiextensionsv1.JSON{Raw: []byte(`"` + strings.Repeat("v", l) + `"`)}
			}
			list[o] = v1alpha1.AvailableObject{Name: fmt.Sprintf("o%04d", o), Fields: fields}
		}
		raw, err := json.Marshal(list)
		if err != nil {
			t.Fatal(err)
		}
		return len(raw)
	}
	for {
		lengths = append(lengths, slices.Repeat([]int{base}, engine.MaxCatalogFields))
		if size() > target {
			lengths = lengths[:len(lengths)-1]
			break
		}
	}
	for delta, o, f := target-size(), 0, 0; delta > 0; f++ {
		if f == engine.MaxCatalogFields {
			o, f = o+1, 0
		}
		add := min(delta, engine.MaxCatalogFieldValueBytes-2-base)
		lengths[o][f] += add
		delta -= add
	}
	if got := size(); got != target {
		t.Fatalf("catalog takes %d bytes, want %d", got, target)
	}

	r := &Resolved{Reg: reg, allowed: map[string]struct{}{}, denied: map[string]struct{}{}, excluded: map[string]struct{}{},
		liveObjects: map[string]map[string]any{}}
	for o, ls := range lengths {
		name := fmt.Sprintf("o%04d", o)
		obj := map[string]any{}
		for f, l := range ls {
			obj[fmt.Sprintf("p%d", f)] = strings.Repeat("v", l)
		}
		r.liveNames = append(r.liveNames, name)
		r.liveObjects[name] = obj
	}
	return r
}

// TestAvailableWithFields_BudgetBoundary: a catalog of exactly MaxCatalogFieldsBytes keeps its fields,
// one byte more loses all of them.
func TestAvailableWithFields_BudgetBoundary(t *testing.T) {
	t.Run("at the budget", func(t *testing.T) {
		out, skips := budgetCatalog(t, engine.MaxCatalogFieldsBytes).AvailableWithFields(jsonpath.NewWithCache())
		if !skips.Empty() {
			t.Fatalf("skips = %+v", skips)
		}
		for _, e := range out {
			if len(e.Fields) != engine.MaxCatalogFields {
				t.Fatalf("%s carries %d fields, want %d", e.Name, len(e.Fields), engine.MaxCatalogFields)
			}
		}
	})

	t.Run("one byte over the budget", func(t *testing.T) {
		out, skips := budgetCatalog(t, engine.MaxCatalogFieldsBytes+1).AvailableWithFields(jsonpath.NewWithCache())
		if skips.OverBudgetBytes != engine.MaxCatalogFieldsBytes+1 || len(skips.Oversized) != 0 {
			t.Fatalf("skips = %+v", skips)
		}
		for _, e := range out {
			if e.Fields != nil {
				t.Fatalf("%s carries fields over the budget", e.Name)
			}
		}
	})
}
