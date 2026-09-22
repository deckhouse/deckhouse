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
	// GenerateExamples enables the automatic bottom-up synthesis of
	// x-doc-examples. It is off by default: explicit examples markers are always
	// applied, but composite examples (the synthesized root example and, under
	// crd:exampleScope=tree, the per-node examples) are only produced when this
	// flag is set.
	GenerateExamples bool
	// Reindent switches the output encoder to the goyaml.v3 layout, which indents
	// every block sequence under its parent key (SetIndent(2)). It is off by
	// default: the enricher then keeps the sigs.k8s.io/yaml layout (block
	// sequences flush with their parent key), leaving files byte-identical to
	// controller-gen output except at the enriched nodes. Key ordering (authored
	// order for examples, sorted elsewhere) is the same either way.
	Reindent bool
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

	// generateExamplesEnabled turns the automatic example synthesis on. It mirrors
	// Options.GenerateExamples and is off by default, so composite examples are
	// only produced when the caller opts in.
	generateExamplesEnabled bool

	// reindent mirrors Options.Reindent. When set, every document is encoded with
	// the goyaml.v3 indented layout instead of the flush sigs.k8s.io/yaml one.
	reindent bool
}

// fileState is the part of an enrichment that belongs to one CRD document: the
// switches its crd: markers flip, and the identity a warning needs to be worth
// reading. It is built per file in enrichFile, so a switch cannot leak into the
// next document -- which a hand-written reset block on the Enricher stopped
// guaranteeing the moment anyone added a switch and forgot to list it there.
type fileState struct {
	// path is the manifest being enriched. Warnings quote its base name.
	// The CRD kind needs no field of its own: a root type only matches a
	// document whose names.kind equals the type's own name, so nodeCtx.typeName
	// already carries it.
	path string

	// curatedStyle is set when the CRD opts into the hand-curated deckhouse
	// style via crd:minimal. Such files omit the leading document separator.
	curatedStyle bool

	// exampleScope is set from the crd:exampleScope marker. It controls where
	// generated composite examples are attached: the default empty value (and
	// "root") attaches a single synthesized example to the CRD root, while
	// "tree" attaches a composite example to every object node as well.
	exampleScope string

	// orderedExamples is set when an example marker produced an
	// order-preserving orderedMap. Such files are encoded with the
	// order-preserving marshaller so the authored field order survives; files
	// without ordered examples keep the default sigs.k8s.io/yaml encoding.
	orderedExamples bool
}

// nodeCtx is where the walk currently is. The file half is shared by pointer
// (the switches are flipped deep in the tree and read back in enrichFile); the
// Go type and field are copied as the walk descends, so a warning can name the
// declaration the marker was written on instead of leaving the reader to grep a
// field name that occurs in nine CRDs.
//
// The zero value is usable and simply produces unprefixed warnings.
type nodeCtx struct {
	file     *fileState
	typeName string
	field    string
}

// onType returns the context of a type's own schema node, dropping any field.
func (c nodeCtx) onType(typeName string) nodeCtx {
	c.typeName = typeName
	c.field = ""
	return c
}

// at returns the context of one field of the current type.
func (c nodeCtx) at(typeName, field string) nodeCtx {
	c.typeName = typeName
	c.field = field
	return c
}

// where renders the location prefix of a warning: the manifest, and the Go
// declaration the markers came from once the walk knows it.
func (c nodeCtx) where() string {
	var parts []string
	if c.file != nil && c.file.path != "" {
		parts = append(parts, filepath.Base(c.file.path))
	}
	switch {
	case c.typeName != "" && c.field != "":
		parts = append(parts, c.typeName+"."+c.field)
	case c.typeName != "":
		parts = append(parts, c.typeName)
	}
	return strings.Join(parts, ": ")
}

// warnf records a non-fatal problem together with where it was found. Every
// warning goes through here: a message that does not say which manifest, kind
// and field it is about cannot be acted on when the same marker appears in a
// dozen CRDs, which is the normal case.
func (e *Enricher) warnf(ctx nodeCtx, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if where := ctx.where(); where != "" {
		msg = where + ": " + msg
	}
	e.warnings = append(e.warnings, msg)
}

// Run loads the API packages, then walks and enriches every CRD file in the
// configured directory. It returns the list of files that were modified.
//
// Warnings collected along the way are dropped, so new code should reach for
// RunWithWarnings instead: every warning means a marker did not do what its
// author wrote it to do, and nothing else in the run says so. Run is kept for
// callers that already depend on this signature.
func Run(opts Options) ([]string, error) {
	changed, _, err := RunWithWarnings(opts)
	return changed, err
}

// RunWithWarnings is Run, returning the non-fatal problems it collected as well:
// markers naming a field the schema has no property for, raw and unset paths
// that do not resolve, sensitive-data on a root type. None of them stops the
// enrichment, and every one of them means a marker did not do what its author
// wrote it to do, so a caller that has somewhere to print them should.
func RunWithWarnings(opts Options) ([]string, []string, error) {
	if len(opts.Paths) == 0 {
		return nil, nil, fmt.Errorf("no package paths provided")
	}
	if opts.CRDDir == "" {
		return nil, nil, fmt.Errorf("no CRD directory provided")
	}

	pkgByPath, err := loadPackages(opts.Dir, opts.Paths...)
	if err != nil {
		return nil, nil, err
	}

	enr := &Enricher{
		pkgByPath:               pkgByPath,
		rootsByVersion:          make(map[string]map[string]*types.Named),
		generateExamplesEnabled: opts.GenerateExamples,
		reindent:                opts.Reindent,
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
		return nil, nil, err
	}

	var changed []string
	for _, file := range files {
		modified, err := enr.enrichFile(file)
		if err != nil {
			return nil, nil, fmt.Errorf("enrich %s: %w", file, err)
		}
		if modified {
			changed = append(changed, file)
		}
	}

	return changed, enr.Warnings(), nil
}

// Warnings returns the non-fatal problems collected during the last Run, in the
// order they were found, with duplicates removed: the same marker is visited once
// per version of every CRD document that names its kind, so an unfiltered list
// repeats each problem as many times as there are versions.
//
// The result is a copy, so a caller sorting or trimming it cannot disturb a run
// still in progress.
func (e *Enricher) Warnings() []string {
	seen := make(map[string]struct{}, len(e.warnings))
	out := make([]string, 0, len(e.warnings))
	for _, w := range e.warnings {
		if _, dup := seen[w]; dup {
			continue
		}
		seen[w] = struct{}{}
		out = append(out, w)
	}
	return out
}

// decodeMarkerValue decodes a marker value and records the non-fatal problems
// worth telling the author about. The boolean result is false when the value
// could not be decoded at all, in which case the marker has to be skipped.
func (e *Enricher) decodeMarkerValue(m marker, ctx nodeCtx) (any, bool) {
	value, err := decodeValue(m.rawValue)
	if err != nil {
		e.warnf(ctx, "%s", err.Error())
		return nil, false
	}
	if warn := unquotedMappingWarning(m.name, m.rawValue, value); warn != "" {
		e.warnf(ctx, "%s", warn)
	}
	return value, true
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

	// Every switch this document flips lives on its own fileState, so nothing
	// carries over into the next file and adding a switch cannot reopen that.
	fs := &fileState{path: path}
	e.enrichCRD(crd, fs)

	// Documents that carry authored (ordered) examples are encoded with the
	// order-preserving marshaller so the example fields keep their authored
	// order; everything else keeps the default sigs.k8s.io/yaml encoding, which
	// leaves files without markers byte for byte identical. The reindent flag
	// overrides both and encodes every document with the goyaml.v3 indented
	// layout (it also preserves ordered examples).
	marshal := yaml.Marshal
	switch {
	case e.reindent:
		marshal = marshalIndented
	case fs.orderedExamples:
		marshal = marshalOrdered
	}
	out, err := marshal(crd)
	if err != nil {
		return false, fmt.Errorf("encode yaml: %w", err)
	}

	// controller-gen prefixes every CRD document with an explicit start marker;
	// keep the same shape so the diff stays minimal. Hand-curated CRDs (those
	// using the x-doc-crd marker) omit the separator, so drop it for them.
	if !fs.curatedStyle && bytes.HasPrefix(original, []byte("---")) {
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
func (e *Enricher) enrichCRD(crd map[string]any, fs *fileState) {
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

	ctx := nodeCtx{file: fs}
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
				rootName := named.Obj().Name()
				e.applyCRDMarkers(crd, info.typeMarkers[rootName], ctx.onType(rootName))
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

		e.enrichType(openAPISchema, named, ctx)

		// x-kubernetes-sensitive-data must not sit on the schema root: the root
		// also covers the system apiVersion, kind and metadata fields, which the
		// apiserver cannot encrypt. Drop it with a warning if a root type carries
		// the marker by mistake.
		if _, ok := openAPISchema[sensitiveDataKey]; ok {
			delete(openAPISchema, sensitiveDataKey)
			e.warnf(ctx.onType(named.Obj().Name()),
				"sensitive-data marker is not allowed on the root type; apply it to spec or a field")
		}

		// Examples are generated bottom-up after every marker has been applied,
		// so explicit examples, defaults and enums are already in place. This is
		// opt-in: without the flag only the explicit examples markers survive.
		if e.generateExamplesEnabled {
			e.generateExamples(spec, names, name, openAPISchema, fs)
		}
	}
}

// applyCRDMarkers configures CRD-level settings that controller-gen cannot emit
// and normalises the document to the hand-curated deckhouse style. It runs when
// the root type carries an x-doc-crd marker. Labels and annotations are not
// handled here: they are emitted natively by controller-gen through the
// +kubebuilder:metadata:labels and +kubebuilder:metadata:annotations markers.
func (e *Enricher) applyCRDMarkers(crd map[string]any, markers []marker, ctx nodeCtx) {
	// The two settings below are file switches. A caller outside enrichFile has
	// no file to flip them on, so give it a throwaway one rather than
	// dereferencing nil: the settings then go nowhere, which is what such a
	// caller wants.
	if ctx.file == nil {
		ctx.file = &fileState{}
	}

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
			decoded, valid := e.decodeMarkerValue(m, ctx)
			if !valid {
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
		ctx.file.exampleScope = scope
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
		ctx.file.curatedStyle = true
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
func (e *Enricher) enrichType(schema map[string]any, named *types.Named, ctx nodeCtx) {
	name := named.Obj().Name()
	info := e.infoFor(named)
	if info != nil {
		e.applyMarkers(schema, info.typeMarkers[name], ctx.onType(name))
	}
	e.enrichStruct(schema, named, ctx)
}

// enrichStruct walks the fields of a struct type, applying field markers and
// recursing into the matching schema children.
func (e *Enricher) enrichStruct(schema map[string]any, named *types.Named, ctx nodeCtx) {
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
				e.enrichStruct(schema, embedded, ctx)
			}
			continue
		}

		if jsonName == "" {
			continue
		}

		child := childMap(properties, jsonName)

		fieldCtx := ctx.at(named.Obj().Name(), field.Name())

		if info != nil {
			markers := info.fieldMarkers[named.Obj().Name()][field.Name()]
			if len(markers) > 0 {
				// Only the enricher's own markers are worth complaining about. A
				// field carrying nothing but +optional or a kubebuilder marker
				// loses nothing when controller-gen emits no property for it --
				// which is the normal case for the ObjectMeta a curated CRD
				// strips -- and warning about it teaches everyone to ignore the
				// channel.
				switch {
				case child != nil:
					e.applyMarkers(child, markers, fieldCtx)
				case hasEnricherMarker(markers):
					e.warnf(fieldCtx, "marker present but schema has no property %q", jsonName)
				}
			}
		}

		if child != nil {
			e.enrichValue(child, field.Type(), fieldCtx)
		}
	}
}

// enrichValue follows the structure of a Go field type into the schema:
// pointers are dereferenced, slices descend into "items", maps into
// "additionalProperties" and named structs into their nested properties.
func (e *Enricher) enrichValue(schema map[string]any, typ types.Type, ctx nodeCtx) {
	for {
		switch t := typ.(type) {
		case *types.Pointer:
			typ = t.Elem()
		case *types.Named:
			switch t.Underlying().(type) {
			case *types.Struct:
				e.enrichType(schema, t, ctx)
				return
			default:
				typ = t.Underlying()
			}
		case *types.Slice:
			if items := childMap(schema, "items"); items != nil {
				e.enrichValue(items, t.Elem(), ctx)
			}
			return
		case *types.Array:
			if items := childMap(schema, "items"); items != nil {
				e.enrichValue(items, t.Elem(), ctx)
			}
			return
		case *types.Map:
			if additional := childMap(schema, "additionalProperties"); additional != nil {
				e.enrichValue(additional, t.Elem(), ctx)
			}
			return
		default:
			return
		}
	}
}

// applyMarkers writes the x-doc-* keys described by the markers into a schema
// node. examplesMarker accumulates a list (each entry optionally described by a
// following examples-description marker), value-less markers become boolean
// flags and everything else stores its parsed YAML value.
func (e *Enricher) applyMarkers(schema map[string]any, markers []marker, ctx nodeCtx) {
	if schema == nil {
		return
	}

	var examples []exampleEntry
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
			// Examples are decoded with the order-preserving decoder so an
			// example object keeps its authored field order when rendered. Each
			// value becomes its own entry so a following examples-description
			// marker can attach to it.
			value, err := decodeOrderedValue(m.rawValue)
			if err != nil {
				e.warnf(ctx, "%s", err.Error())
				continue
			}
			// An example authored in ascending key order renders identically
			// under the default encoder, so collapse it to the plain (sorted)
			// model. This keeps the whole document on the default sigs.k8s.io/yaml
			// encoding instead of switching to the order-preserving one, which
			// would reindent every sequence in the file. Examples that
			// deliberately reorder keys stay ordered and keep their authored order.
			if plain, ok := plainIfSorted(value); ok {
				value = plain
			}
			if list, ok := value.([]any); ok {
				for _, item := range list {
					examples = append(examples, exampleEntry{value: item})
				}
			} else {
				examples = append(examples, exampleEntry{value: value})
			}

		case m.name == examplesDescriptionMarker:
			// A description attaches to the example introduced by the preceding
			// examples marker. Without one there is nothing to describe.
			if len(examples) == 0 {
				e.warnf(ctx, "%s marker has no preceding %s marker to attach to",
					examplesDescriptionMarker, examplesMarker)
				continue
			}
			value, ok := e.decodeMarkerValue(m, ctx)
			if !ok {
				continue
			}
			last := &examples[len(examples)-1]
			if last.hasDescription {
				e.warnf(ctx, "%s marker overrides an earlier description for the same example",
					examplesDescriptionMarker)
			}
			last.description = value
			last.hasDescription = true

		case m.name == examplesNameMarker:
			// A name attaches to the example introduced by the preceding examples
			// marker, exactly like a description.
			if len(examples) == 0 {
				e.warnf(ctx, "%s marker has no preceding %s marker to attach to",
					examplesNameMarker, examplesMarker)
				continue
			}
			value, ok := e.decodeMarkerValue(m, ctx)
			if !ok {
				continue
			}
			last := &examples[len(examples)-1]
			if last.hasName {
				e.warnf(ctx, "%s marker overrides an earlier name for the same example", examplesNameMarker)
			}
			last.name = value
			last.hasName = true

		case strings.HasPrefix(m.name, rawMarkerPrefix):
			// raw:<key> injects a standard schema field named <key> directly
			// (not under an x-doc-* key). It is used for fields controller-gen
			// cannot emit on some types (for example a pattern on a Duration).
			// A dotted <key> walks into nested schema maps, which lets a field
			// override descriptions that controller-gen pulls from a shared type
			// (for example items.description on a []metav1.Condition field).
			key := strings.TrimPrefix(m.name, rawMarkerPrefix)
			if key == "" {
				e.warnf(ctx, "raw marker has no key; write raw:<field>=<value> or raw:<node>.<field>=<value>")
				continue
			}
			// A value-less raw: marker would decode to null and write it, which is
			// never what its author meant -- and the one thing they might have
			// meant, taking the field out, is what unset: is for.
			if !m.hasValue {
				e.warnf(ctx, "raw marker for %q has no value and would write null; use unset:%s to remove the field", key, key)
				continue
			}
			value, ok := e.decodeMarkerValue(m, ctx)
			if !ok {
				continue
			}
			if strings.Contains(key, ".") {
				if !setNested(schema, strings.Split(key, "."), value) {
					e.warnf(ctx, "raw path %q does not resolve to a schema node", key)
				}
			} else {
				schema[key] = value
			}

		case strings.HasPrefix(m.name, unsetMarkerPrefix):
			// unset:<key> deletes a standard schema field, the mirror of raw:<key>.
			// It is how a field takes out a node controller-gen filled in from a
			// vendored type -- items.description on a []metav1.Condition field --
			// which raw: can only overwrite, never remove. It carries no value: a
			// key set to null is not the same schema as a key that is not there.
			key := strings.TrimPrefix(m.name, unsetMarkerPrefix)
			if key == "" {
				e.warnf(ctx, "unset marker has no key; write unset:<field> or unset:<node>.<field>")
				continue
			}
			if m.hasValue {
				e.warnf(ctx, "unset marker for %q takes no value, ignoring %q", key, m.rawValue)
			}
			path := strings.Split(key, ".")
			// Refuse to remove a field the structural schema requires. Obeying
			// would produce a manifest the apiserver rejects at apply time, with
			// nothing to connect the refusal back to this marker.
			last := path[len(path)-1]
			if structuralKeys[last] {
				e.warnf(ctx, "unset path %q would remove %q, which a structural schema requires; the apiserver would reject the CRD", key, last)
				continue
			}
			// Removing a validation key is legal and the apiserver will accept the
			// result, which is exactly why it is worth a line: the API then admits
			// values it used to reject, and the crds/ re-render gate cannot see it
			// -- both sides of that comparison come from this same marker.
			if validationKeys[last] {
				e.warnf(ctx, "unset path %q removes the validation key %q; the apiserver will accept values it rejected before", key, last)
			}
			switch deleteNested(schema, path) {
			case unsetNotFound:
				e.warnf(ctx, "unset path %q is not present in the schema", key)
			case unsetWouldEmptyParent:
				e.warnf(ctx, "unset path %q would leave its parent node empty, which the apiserver rejects (\"must not be empty for specified object fields\"); refusing", key)
			}

		case m.name == sensitiveDataMarker:
			// sensitive-data renders to the kube-apiserver flag
			// x-kubernetes-sensitive-data rather than an x-doc-* key. On an
			// object or array node it marks the whole subtree as sensitive. It
			// is a value-less flag by default but accepts an explicit boolean.
			if m.hasValue {
				value, ok := e.decodeMarkerValue(m, ctx)
				if !ok {
					continue
				}
				schema[sensitiveDataKey] = value
			} else {
				schema[sensitiveDataKey] = true
			}

		case !m.hasValue:
			// A value-less entity (for example deprecated) becomes a boolean
			// x-doc-<entity> flag.
			schema[docKeyPrefix+m.name] = true

		default:
			// A valued simple entity (for example default) stores its parsed
			// YAML value under x-doc-<entity>.
			value, ok := e.decodeMarkerValue(m, ctx)
			if !ok {
				continue
			}
			schema[docKeyPrefix+m.name] = value
		}
	}

	if len(examples) > 0 {
		schema[docKeyPrefix+examplesMarker] = e.buildExamples(examples, ctx)
	}
}

// exampleEntry is one authored example together with the optional name and
// description gathered from following examples-name and examples-description
// markers.
type exampleEntry struct {
	value          any
	name           any
	hasName        bool
	description    any
	hasDescription bool
}

// buildExamples renders the collected entries into the x-doc-examples list. If
// no entry carries a name or a description the list stays a plain list of
// values, exactly as before. As soon as any entry has a name or a description,
// every entry switches to the wrapper form {x-description, x-name, x-example}
// (an entry missing either attribute omits its key), so the array stays
// homogeneous for consumers. Wrapping (and any ordered example value) forces the
// order-preserving encoder so the attributes keep their place ahead of the
// example.
func (e *Enricher) buildExamples(entries []exampleEntry, ctx nodeCtx) []any {
	// As in applyCRDMarkers: a caller outside enrichFile has no file to flip the
	// encoder switch on, and a throwaway one keeps that harmless.
	if ctx.file == nil {
		ctx.file = &fileState{}
	}

	wrap := false
	for _, entry := range entries {
		if entry.hasName || entry.hasDescription {
			wrap = true
			break
		}
	}

	out := make([]any, 0, len(entries))
	for _, entry := range entries {
		if containsOrdered(entry.value) {
			ctx.file.orderedExamples = true
		}
		if !wrap {
			out = append(out, entry.value)
			continue
		}

		ctx.file.orderedExamples = true
		wrapper := make(orderedMap, 0, 3)
		if entry.hasDescription {
			wrapper = append(wrapper, orderedEntry{key: docDescriptionKey, val: entry.description})
		}
		if entry.hasName {
			wrapper = append(wrapper, orderedEntry{key: docNameKey, val: entry.name})
		}
		wrapper = append(wrapper, orderedEntry{key: docExampleKey, val: entry.value})
		out = append(out, wrapper)
	}
	return out
}

// setNested walks an existing schema sub-tree along path and sets the final
// key to value. Intermediate segments must already exist and be maps (the
// nodes controller-gen emitted); it returns false otherwise so a mistyped path
// surfaces as a warning rather than silently growing the schema.
func setNested(schema map[string]any, path []string, value any) bool {
	if len(path) == 0 {
		return false
	}
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

// unsetOutcome is what deleteNested did. The three cases each want saying
// differently, and as a type the impossible fourth -- emptied a parent without
// removing anything -- cannot be spelled.
type unsetOutcome int

const (
	// unsetNotFound means the path did not resolve, or the field was not there,
	// and nothing changed. The caller wants to hear about it: the marker was
	// written to remove something, so removing nothing means it has outlived its
	// target -- the vendored godoc changed, or the path was mistyped.
	unsetNotFound unsetOutcome = iota
	// unsetRemoved means the field is gone and its parent still has other keys.
	unsetRemoved
	// unsetWouldEmptyParent means the removal was refused: it would have left the
	// parent an empty mapping. An empty node is not the same schema as an absent
	// one, and for the nodes apiextensions constrains it is not a valid one
	// either -- the apiserver answers "must not be empty for specified object
	// fields" at apply time, with nothing in the failure pointing back at the
	// marker. Refused rather than done-and-warned for the same reason
	// structuralKeys are: a warning is not fatal by default, so obeying would ship
	// a document the apiserver rejects.
	unsetWouldEmptyParent
)

// deleteNested removes the field named by the last path element, walking the
// mappings named by the ones before it. path must be non-empty. The schema is
// left untouched unless the outcome is unsetRemoved.
func deleteNested(schema map[string]any, path []string) unsetOutcome {
	if len(path) == 0 {
		return unsetNotFound
	}
	node := schema
	for _, key := range path[:len(path)-1] {
		child, ok := node[key].(map[string]any)
		if !ok {
			return unsetNotFound
		}
		node = child
	}
	last := path[len(path)-1]
	if _, ok := node[last]; !ok {
		return unsetNotFound
	}
	// The schema root is not a "parent node" in this sense: emptying it means the
	// document has no schema left, which a single-element path cannot express.
	if len(path) > 1 && len(node) == 1 {
		return unsetWouldEmptyParent
	}
	delete(node, last)
	return unsetRemoved
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
