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

package crdenricher

import (
	"bytes"
	"fmt"
	"go/types"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"sigs.k8s.io/yaml"
)

// Options configures a single enrichment run.
type Options struct {
	// Paths is the list of Go package patterns that hold the API structs with
	// the markers, exactly like the controller-gen "paths" argument.
	Paths []string
	// CRDDir is the directory with the CRD YAML files produced by
	// controller-gen. The files are enriched in place.
	CRDDir string
	// Dir is the working directory used to resolve the package patterns.
	// When empty the current working directory is used.
	Dir string
}

// Enricher applies custom x-doc-* schema fields to controller-gen output based
// on the markers attached to the corresponding Go API structs.
type Enricher struct {
	// pkgByPath indexes every loaded API package by its import path so that
	// markers can be resolved while walking field types across packages.
	pkgByPath map[string]*packageInfo
	// rootsByVersion maps an API version and a CRD kind to the Go root type
	// that backs it.
	rootsByVersion map[string]map[string]*types.Named

	// warnings collects non-fatal problems, such as markers that point at a
	// schema node controller-gen did not emit.
	warnings []string

	// curatedStyle is set per file when the CRD opts into the hand-curated
	// deckhouse style via the x-doc-crd marker. Such files omit the leading
	// document separator.
	curatedStyle bool

	// exampleScope is set per file from the crd:exampleScope marker. It controls
	// where generated composite examples are attached: the default empty value
	// (and "root") attaches a single synthesized example to the CRD root, while
	// "tree" attaches a composite example to every object node as well.
	exampleScope string
}

// Run loads the API packages, then walks and enriches every CRD file in the
// configured directory. It returns the list of files that were modified.
func Run(opts Options) ([]string, error) {
	if len(opts.Paths) == 0 {
		return nil, fmt.Errorf("no package paths provided")
	}
	if opts.CRDDir == "" {
		return nil, fmt.Errorf("no CRD directory provided")
	}

	pkgByPath, err := loadPackages(opts.Dir, opts.Paths...)
	if err != nil {
		return nil, err
	}

	enr := &Enricher{
		pkgByPath:      pkgByPath,
		rootsByVersion: make(map[string]map[string]*types.Named),
	}
	for _, info := range pkgByPath {
		for kind, named := range info.roots {
			if enr.rootsByVersion[info.version] == nil {
				enr.rootsByVersion[info.version] = make(map[string]*types.Named)
			}
			enr.rootsByVersion[info.version][kind] = named
		}
	}

	files, err := crdFiles(opts.CRDDir)
	if err != nil {
		return nil, err
	}

	var changed []string
	for _, file := range files {
		modified, err := enr.enrichFile(file)
		if err != nil {
			return nil, fmt.Errorf("enrich %s: %w", file, err)
		}
		if modified {
			changed = append(changed, file)
		}
	}

	return changed, nil
}

// Warnings returns the non-fatal problems collected during the last Run.
func (e *Enricher) Warnings() []string {
	return e.warnings
}

// crdFiles returns the sorted list of YAML files in a directory.
func crdFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read CRD directory: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
			files = append(files, filepath.Join(dir, name))
		}
	}
	sort.Strings(files)
	return files, nil
}

// enrichFile parses a single CRD file, enriches its schemas and writes the
// result back when anything changed. Parsing and serialisation go through
// sigs.k8s.io/yaml, the same library controller-gen uses, so files without any
// markers are re-encoded byte for byte and left untouched.
func (e *Enricher) enrichFile(path string) (bool, error) {
	original, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	var crd map[string]any
	if err := yaml.Unmarshal(original, &crd); err != nil {
		return false, fmt.Errorf("parse yaml: %w", err)
	}

	if kind, _ := crd["kind"].(string); kind != "CustomResourceDefinition" {
		return false, nil
	}

	e.curatedStyle = false
	e.exampleScope = ""
	e.enrichCRD(crd)

	out, err := yaml.Marshal(crd)
	if err != nil {
		return false, fmt.Errorf("encode yaml: %w", err)
	}

	// controller-gen prefixes every CRD document with an explicit start marker;
	// keep the same shape so the diff stays minimal. Hand-curated CRDs (those
	// using the x-doc-crd marker) omit the separator, so drop it for them.
	if !e.curatedStyle && bytes.HasPrefix(original, []byte("---")) {
		out = append([]byte("---\n"), out...)
	}

	if bytes.Equal(original, out) {
		return false, nil
	}

	if err := os.WriteFile(path, out, 0o644); err != nil {
		return false, fmt.Errorf("write file: %w", err)
	}
	return true, nil
}

// enrichCRD walks every version schema of a CRD whose kind has a matching Go
// root type.
func (e *Enricher) enrichCRD(crd map[string]any) {
	spec := childMap(crd, "spec")
	if spec == nil {
		return
	}

	names := childMap(spec, "names")
	if names == nil {
		return
	}
	kind, _ := names["kind"].(string)
	if kind == "" {
		return
	}

	versions, ok := spec["versions"].([]any)
	if !ok {
		return
	}

	crdApplied := false
	for _, raw := range versions {
		version, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		name, _ := version["name"].(string)
		named, ok := e.rootsByVersion[name][kind]
		if !ok {
			continue
		}

		// CRD-level metadata (labels, preserveUnknownFields, normalizations) is
		// applied once from the root type markers.
		if !crdApplied {
			if info := e.infoFor(named); info != nil {
				e.applyCRDMarkers(crd, info.typeMarkers[named.Obj().Name()])
			}
			crdApplied = true
		}

		schema := childMap(version, "schema")
		if schema == nil {
			continue
		}
		openAPISchema := childMap(schema, "openAPIV3Schema")
		if openAPISchema == nil {
			continue
		}

		e.enrichType(openAPISchema, named)

		// Examples are generated bottom-up after every marker has been applied,
		// so explicit examples, defaults and enums are already in place.
		e.generateExamples(spec, names, name, openAPISchema)
	}
}

// applyCRDMarkers configures CRD-level settings that controller-gen cannot emit
// and normalises the document to the hand-curated deckhouse style. It runs when
// the root type carries an x-doc-crd marker. Labels and annotations are not
// handled here: they are emitted natively by controller-gen through the
// +kubebuilder:metadata:labels and +kubebuilder:metadata:annotations markers.
func (e *Enricher) applyCRDMarkers(crd map[string]any, markers []marker) {
	// Each CRD setting arrives as its own "crd:<key>=<value>" marker, mirroring
	// the kubebuilder marker style. The values are collected into a single
	// config map so the rest of the function can stay value-driven. A value-less
	// marker (for example "crd:minimal") is treated as the boolean true.
	config := map[string]any{}
	for _, m := range markers {
		if !m.isDoc() {
			continue
		}
		key, ok := strings.CutPrefix(m.name, crdMarker+":")
		if !ok {
			continue
		}
		var value any = true
		if m.hasValue {
			decoded, err := decodeValue(m.rawValue)
			if err != nil {
				e.warnings = append(e.warnings, err.Error())
				continue
			}
			value = decoded
		}
		config[key] = value
	}
	if len(config) == 0 {
		return
	}

	// exampleScope selects where generated examples are attached and is consumed
	// later by generateExamples; it is not written onto the CRD itself.
	if scope, ok := config["exampleScope"].(string); ok {
		e.exampleScope = scope
	}

	metadata := childMap(crd, "metadata")
	if metadata == nil {
		metadata = map[string]any{}
		crd["metadata"] = metadata
	}

	spec := childMap(crd, "spec")
	if pres, ok := config["preserveUnknownFields"]; ok && spec != nil {
		spec["preserveUnknownFields"] = pres
	}

	// The generator version annotation is dropped for every curated CRD; none
	// of them keep it.
	e.stripGeneratorAnnotation(metadata)

	// The "minimal" style strips what controller-gen injects that the
	// hand-curated CRDs omit: the listKind, the implicit apiVersion/kind/metadata
	// root properties and the leading document separator. CRDs that keep the
	// full controller-gen schema (only adding labels) leave minimal unset.
	if minimal, _ := config["minimal"].(bool); minimal && spec != nil {
		e.curatedStyle = true
		if names := childMap(spec, "names"); names != nil {
			delete(names, "listKind")
		}
		e.stripRootMeta(spec)
	}

	// stripFormat controls schema-level format stripping. Some curated CRDs drop
	// format entirely (stripFormat: true), some keep it (omit the key), and some
	// drop only specific formats such as int32 while keeping date-time
	// (stripFormat: [int32]).
	if sf, ok := config["stripFormat"]; ok && spec != nil {
		switch v := sf.(type) {
		case bool:
			if v {
				e.stripSchemaFormats(spec, nil)
			}
		case []any:
			only := map[string]bool{}
			for _, item := range v {
				if s, ok := item.(string); ok {
					only[s] = true
				}
			}
			e.stripSchemaFormats(spec, only)
		}
	}
}

// stripSchemaFormats removes schema-level "format" keys from the openAPIV3Schema
// of each version. controller-gen infers format from Go types (int32 for uint32,
// date-time for metav1.Time), but the hand-curated CRDs use it inconsistently.
// When only is nil every format is dropped; otherwise only the listed format
// values are dropped. Printer column formats live outside the schema and are
// left intact.
func (e *Enricher) stripSchemaFormats(spec map[string]any, only map[string]bool) {
	versions, ok := spec["versions"].([]any)
	if !ok {
		return
	}
	for _, raw := range versions {
		version, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		schema := childMap(version, "schema")
		if schema == nil {
			continue
		}
		if openAPISchema := childMap(schema, "openAPIV3Schema"); openAPISchema != nil {
			stripFormatRecursive(openAPISchema, only)
		}
	}
}

// stripFormatRecursive deletes the "format" key from every nested mapping. When
// only is non-nil, the key is removed only when its value is in the set.
func stripFormatRecursive(node any, only map[string]bool) {
	switch typed := node.(type) {
	case map[string]any:
		if f, ok := typed["format"]; ok {
			if only == nil {
				delete(typed, "format")
			} else if s, ok := f.(string); ok && only[s] {
				delete(typed, "format")
			}
		}
		for _, v := range typed {
			stripFormatRecursive(v, only)
		}
	case []any:
		for _, v := range typed {
			stripFormatRecursive(v, only)
		}
	}
}

// stripGeneratorAnnotation removes the controller-gen version annotation, and
// the annotations map itself when it becomes empty.
func (e *Enricher) stripGeneratorAnnotation(metadata map[string]any) {
	annotations := childMap(metadata, "annotations")
	if annotations == nil {
		return
	}
	delete(annotations, "controller-gen.kubebuilder.io/version")
	if len(annotations) == 0 {
		delete(metadata, "annotations")
	}
}

// stripRootMeta removes the apiVersion, kind and metadata properties that
// controller-gen always injects into the root object schema, matching the
// hand-curated CRDs that omit them.
func (e *Enricher) stripRootMeta(spec map[string]any) {
	versions, ok := spec["versions"].([]any)
	if !ok {
		return
	}
	for _, raw := range versions {
		version, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		schema := childMap(version, "schema")
		if schema == nil {
			continue
		}
		openAPISchema := childMap(schema, "openAPIV3Schema")
		if openAPISchema == nil {
			continue
		}
		properties := childMap(openAPISchema, "properties")
		if properties == nil {
			continue
		}
		delete(properties, "apiVersion")
		delete(properties, "kind")
		delete(properties, "metadata")
	}
}

// enrichType applies the type-level markers of a named type to the given
// schema node and then descends into its struct fields.
func (e *Enricher) enrichType(schema map[string]any, named *types.Named) {
	info := e.infoFor(named)
	if info != nil {
		e.applyMarkers(schema, info.typeMarkers[named.Obj().Name()])
	}
	e.enrichStruct(schema, named)
}

// enrichStruct walks the fields of a struct type, applying field markers and
// recursing into the matching schema children.
func (e *Enricher) enrichStruct(schema map[string]any, named *types.Named) {
	structType, ok := named.Underlying().(*types.Struct)
	if !ok {
		return
	}

	info := e.infoFor(named)
	properties := childMap(schema, "properties")

	for i := 0; i < structType.NumFields(); i++ {
		field := structType.Field(i)
		jsonName, inline, skip := parseJSONTag(structType.Tag(i))
		if skip {
			continue
		}

		// Embedded structs flattened with ",inline" (or with no JSON name at
		// all) merge their fields into the current schema node.
		if field.Embedded() && (inline || jsonName == "") {
			if embedded := namedOf(field.Type()); embedded != nil {
				e.enrichStruct(schema, embedded)
			}
			continue
		}

		if jsonName == "" {
			continue
		}

		child := childMap(properties, jsonName)

		if info != nil {
			markers := info.fieldMarkers[named.Obj().Name()][field.Name()]
			if len(markers) > 0 {
				if child == nil {
					e.warnings = append(e.warnings, fmt.Sprintf(
						"%s.%s: marker present but schema has no property %q",
						named.Obj().Name(), field.Name(), jsonName))
				} else {
					e.applyMarkers(child, markers)
				}
			}
		}

		if child != nil {
			e.enrichValue(child, field.Type())
		}
	}
}

// enrichValue follows the structure of a Go field type into the schema:
// pointers are dereferenced, slices descend into "items", maps into
// "additionalProperties" and named structs into their nested properties.
func (e *Enricher) enrichValue(schema map[string]any, typ types.Type) {
	for {
		switch t := typ.(type) {
		case *types.Pointer:
			typ = t.Elem()
		case *types.Named:
			switch t.Underlying().(type) {
			case *types.Struct:
				e.enrichType(schema, t)
				return
			default:
				typ = t.Underlying()
			}
		case *types.Slice:
			if items := childMap(schema, "items"); items != nil {
				e.enrichValue(items, t.Elem())
			}
			return
		case *types.Array:
			if items := childMap(schema, "items"); items != nil {
				e.enrichValue(items, t.Elem())
			}
			return
		case *types.Map:
			if additional := childMap(schema, "additionalProperties"); additional != nil {
				e.enrichValue(additional, t.Elem())
			}
			return
		default:
			return
		}
	}
}

// applyMarkers writes the x-doc-* keys described by the markers into a schema
// node. examplesMarker accumulates a list, value-less markers become boolean
// flags and everything else stores its parsed YAML value.
func (e *Enricher) applyMarkers(schema map[string]any, markers []marker) {
	if schema == nil {
		return
	}

	var examples []any
	for _, m := range markers {
		if !m.isDoc() {
			continue
		}
		// CRD-level markers are handled separately and must not leak into the
		// schema node.
		if isCRDMarker(m.name) {
			continue
		}

		switch {
		case m.name == examplesMarker:
			value, err := decodeValue(m.rawValue)
			if err != nil {
				e.warnings = append(e.warnings, err.Error())
				continue
			}
			if list, ok := value.([]any); ok {
				examples = append(examples, list...)
			} else {
				examples = append(examples, value)
			}

		case strings.HasPrefix(m.name, rawMarkerPrefix):
			value, err := decodeValue(m.rawValue)
			if err != nil {
				e.warnings = append(e.warnings, err.Error())
				continue
			}
			// raw:<key> injects a standard schema field named <key> directly
			// (not under an x-doc-* key). It is used for fields controller-gen
			// cannot emit on some types (for example a pattern on a Duration).
			// A dotted <key> walks into nested schema maps, which lets a field
			// override descriptions that controller-gen pulls from a shared type
			// (for example items.description on a []metav1.Condition field).
			key := strings.TrimPrefix(m.name, rawMarkerPrefix)
			if strings.Contains(key, ".") {
				if !setNested(schema, strings.Split(key, "."), value) {
					e.warnings = append(e.warnings, fmt.Sprintf("raw path %q does not resolve to a schema node", key))
				}
			} else {
				schema[key] = value
			}

		case !m.hasValue:
			// A value-less entity (for example deprecated) becomes a boolean
			// x-doc-<entity> flag.
			schema[docKeyPrefix+m.name] = true

		default:
			// A valued simple entity (for example default) stores its parsed
			// YAML value under x-doc-<entity>.
			value, err := decodeValue(m.rawValue)
			if err != nil {
				e.warnings = append(e.warnings, err.Error())
				continue
			}
			schema[docKeyPrefix+m.name] = value
		}
	}

	if len(examples) > 0 {
		schema[docKeyPrefix+examplesMarker] = examples
	}
}

// setNested walks an existing schema sub-tree along path and sets the final
// key to value. Intermediate segments must already exist and be maps (the
// nodes controller-gen emitted); it returns false otherwise so a mistyped path
// surfaces as a warning rather than silently growing the schema.
func setNested(schema map[string]any, path []string, value any) bool {
	node := schema
	for _, key := range path[:len(path)-1] {
		child, ok := node[key].(map[string]any)
		if !ok {
			return false
		}
		node = child
	}
	node[path[len(path)-1]] = value
	return true
}

// infoFor returns the packageInfo of a named type, or nil when the type lives
// in a package that is not part of the enriched API (and therefore carries no
// markers).
func (e *Enricher) infoFor(named *types.Named) *packageInfo {
	obj := named.Obj()
	if obj == nil || obj.Pkg() == nil {
		return nil
	}
	return e.pkgByPath[obj.Pkg().Path()]
}

// namedOf unwraps pointers and returns the underlying named type, or nil.
func namedOf(typ types.Type) *types.Named {
	for {
		switch t := typ.(type) {
		case *types.Pointer:
			typ = t.Elem()
		case *types.Named:
			return t
		default:
			return nil
		}
	}
}

// parseJSONTag extracts the JSON property name and the inline flag from a
// struct tag, reporting whether the field is skipped from JSON entirely.
func parseJSONTag(tag string) (string, bool, bool) {
	value := reflect.StructTag(tag).Get("json")
	if value == "" {
		return "", false, false
	}

	parts := strings.Split(value, ",")
	name := parts[0]
	inline := false
	for _, opt := range parts[1:] {
		if opt == "inline" {
			inline = true
		}
	}

	if name == "-" && len(parts) == 1 {
		return "", false, true
	}
	if name == "-" {
		name = ""
	}
	return name, inline, false
}
