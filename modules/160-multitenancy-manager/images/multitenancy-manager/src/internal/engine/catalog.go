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
	"errors"
	"fmt"
	"slices"

	"github.com/theory/jsonpath"
	"github.com/theory/jsonpath/spec"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"controller/api/v1alpha1"
	jsonpathfactory "controller/internal/jsonpath"
)

// This file projects the catalogFields of a GrantableClusterResourceDefinition onto a granted object.
// The limits mirror the CRD schema, so an object that bypassed it (built in code, stored before the
// schema had the limits) is still bounded here.
const (
	// MaxCatalogFields is the number of catalogFields entries that are projected; the rest is ignored.
	MaxCatalogFields = 10
	// MaxCatalogFieldValueBytes is the largest JSON serialization of a value that is projected.
	MaxCatalogFieldValueBytes = 512
)

// forbiddenCatalogPaths lists the member-name paths a catalog field may not read: managedFields and
// the last-applied annotation hold a copy of the whole object, whatever else it may contain. A path
// is refused when it overlaps one of them in either direction -- it reads under it ($.metadata.
// managedFields[0]) or it reads a parent that contains it ($.metadata, $.metadata.annotations, $).
var forbiddenCatalogPaths = [][]string{
	{"metadata", "managedFields"},
	{"metadata", "annotations", "kubectl.kubernetes.io/last-applied-configuration"},
}

// CompileCatalogPath compiles a catalogFields path with the given factory and returns the error that
// makes it unusable: it does not compile, it is not a singular query (RFC 9535 section 2.3.5.1: member
// names and indexes only, so it selects at most one value) or it reads a forbidden path.
func CompileCatalogPath(factory jsonpathfactory.Factory, path string) (*jsonpath.Path, error) {
	parsed, err := factory.Path(path)
	if err != nil {
		return nil, fmt.Errorf("not a valid RFC 9535 JSONPath: %w", err)
	}
	query := parsed.Query()
	if query.Singular() == nil {
		return nil, errors.New("not a singular query: wildcards, descendant segments, slices, filters and several selectors in one segment are not allowed")
	}
	// A singular query has one Name or Index selector per segment; an Index never equals a member
	// name, so comparing Name selectors is enough.
	segs := query.Segments()
	for _, forbidden := range forbiddenCatalogPaths {
		n := min(len(segs), len(forbidden))
		overlaps := true
		for i := 0; i < n && overlaps; i++ {
			name, ok := segs[i].Selectors()[0].(spec.Name)
			overlaps = ok && string(name) == forbidden[i]
		}
		if overlaps {
			return nil, fmt.Errorf("overlaps %s, which holds a copy of the whole object", namesOf(forbidden))
		}
	}
	return parsed, nil
}

func namesOf(path []string) spec.NormalizedPath {
	out := make(spec.NormalizedPath, len(path))
	for i, name := range path {
		out[i] = spec.Name(name)
	}
	return out
}

// ProjectCatalogFields returns the values of the declared fields in obj, keyed by field name, or nil
// when none is present. It never fails: an entry beyond MaxCatalogFields, an entry whose path
// CompileCatalogPath refuses, an absent or null value and a value longer than
// MaxCatalogFieldValueBytes are left out. A name belongs to its first entry: a later entry of the same
// name is ignored even when the first one yields nothing.
func ProjectCatalogFields(factory jsonpathfactory.Factory, fields []v1alpha1.CatalogField, obj map[string]any) map[string]apiextensionsv1.JSON {
	var out map[string]apiextensionsv1.JSON
	seen := make([]string, 0, min(len(fields), MaxCatalogFields))
	for _, f := range fields[:min(len(fields), MaxCatalogFields)] {
		if slices.Contains(seen, f.Name) {
			continue
		}
		seen = append(seen, f.Name)
		parsed, err := CompileCatalogPath(factory, f.Path)
		if err != nil {
			continue
		}
		nodes := parsed.Select(obj)
		if len(nodes) != 1 || nodes[0] == nil {
			continue
		}
		raw, err := json.Marshal(nodes[0])
		if err != nil || len(raw) > MaxCatalogFieldValueBytes {
			continue
		}
		if out == nil {
			out = map[string]apiextensionsv1.JSON{}
		}
		out[f.Name] = apiextensionsv1.JSON{Raw: raw}
	}
	return out
}
