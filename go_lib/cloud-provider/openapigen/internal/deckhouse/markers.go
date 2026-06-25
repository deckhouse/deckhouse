package deckhouse

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3gen"
	"sigs.k8s.io/controller-tools/pkg/loader"
	ctmarkers "sigs.k8s.io/controller-tools/pkg/markers"

	"openapigen/markers"
)

type pkgMarkers struct {
	mv    ctmarkers.MarkerValues
	types map[string]typeMarkers
}

type typeMarkers struct {
	mv     ctmarkers.MarkerValues
	fields map[string]fieldRef
}

type fieldRef struct {
	mv       *ctmarkers.MarkerValues
	jsonName string
}

func buildMarkersSchemaCustomizer(root any, reg *ctmarkers.Registry) (openapi3gen.SchemaCustomizerFn, error) {
	collector := &ctmarkers.Collector{Registry: reg}

	rt := reflect.TypeOf(root)
	rt = normalizeStructType(rt)
	if rt.Kind() != reflect.Struct {
		return nil, fmt.Errorf("openapigen markers: root must be a struct, got %T", root)
	}

	pkgPaths := collectPackages(root)

	bind, err := buildOpenAPIMarkerBindings(collector, pkgPaths)
	if err != nil {
		return nil, err
	}

	return openAPIMarkerSchemaObjectCustomizerFn(bind), nil
}

func buildOpenAPIMarkerBindings(col *ctmarkers.Collector, pkgsPaths []string) (map[string]pkgMarkers, error) {
	out := make(map[string]pkgMarkers, len(pkgsPaths))

	pkgs, err := loader.LoadRoots(pkgsPaths...)
	if err != nil {
		return nil, fmt.Errorf("load packages %v: %w", pkgsPaths, err)
	}

	for _, pkg := range pkgs {
		packageMarkers, err := ctmarkers.PackageMarkers(col, pkg)
		if err != nil {
			return nil, fmt.Errorf("collect package markers for package %s: %w", pkg.PkgPath, err)
		}

		if err := normalizeMarkerValues(packageMarkers); err != nil {
			return nil, fmt.Errorf("normalize package %s markers: %w", pkg.PkgPath, err)
		}

		out[pkg.PkgPath] = pkgMarkers{
			mv:    packageMarkers,
			types: make(map[string]typeMarkers),
		}

		var normErr error
		err = ctmarkers.EachType(col, pkg, func(info *ctmarkers.TypeInfo) {
			if normErr != nil {
				return
			}

			if err := normalizeMarkerValues(info.Markers); err != nil {
				normErr = fmt.Errorf("normalize type %s markers: %w", info.Name, err)
				return
			}

			out[pkg.PkgPath].types[info.Name] = typeMarkers{
				mv:     info.Markers,
				fields: make(map[string]fieldRef),
			}

			for _, f := range info.Fields {
				if f.Name == "" {
					continue
				}
				jname, ok := jsonNameFromTag(f.Tag)
				if !ok || jname == "" {
					continue
				}

				if err := normalizeMarkerValues(f.Markers); err != nil {
					normErr = fmt.Errorf("normalize field %s.%s markers: %w", info.Name, f.Name, err)
					return
				}

				out[pkg.PkgPath].types[info.Name].fields[f.Name] = fieldRef{
					&f.Markers,
					jname}
			}
		})

		if err != nil {
			return nil, err
		}
		if normErr != nil {
			return nil, normErr
		}
	}

	return out, nil
}

func normalizeMarkerValues(mv ctmarkers.MarkerValues) error {
	for name, occurrences := range mv {
		if len(occurrences) == 0 {
			continue
		}
		mm, ok := occurrences[0].(markers.MergeableSchemaMarker)
		if !ok {
			continue
		}
		merged, err := mm.MergeFrom(occurrences)
		if err != nil {
			return fmt.Errorf("merge '%s' marker: %w", name, err)
		}
		mv[name] = []any{merged}
	}
	return nil
}

func openAPIMarkerSchemaObjectCustomizerFn(bind map[string]pkgMarkers) openapi3gen.SchemaCustomizerFn {
	return func(name string, ft reflect.Type, tag reflect.StructTag, schema *openapi3.Schema) error {
		if schema.Type.Is(openapi3.TypeObject) || name == "_root" {
			nt := normalizeStructType(ft)

			if nt.Kind() == reflect.Map {
				return nil
			}

			if nt.Kind() != reflect.Struct {
				return fmt.Errorf("expected struct, got %s", nt.Kind())
			}

			jname, ok := jsonNameFromTag(tag)
			if (!ok || jname == "") && name != "_root" {
				return fmt.Errorf("type '%s' does not have json tag", nt.Name())
			}

			pkgMarker, ok := bind[nt.PkgPath()]
			if !ok {
				return fmt.Errorf("could not find parsed markers for package %s", nt.PkgPath())
			}

			typeMarker, ok := pkgMarker.types[nt.Name()]
			if !ok {
				return fmt.Errorf("could not find parsed markers for type %s", nt.Name())
			}

			if schema.Extensions == nil {
				schema.Extensions = make(map[string]any)
			}

			if mv := typeMarker.mv; len(mv) > 0 {
				err := applyMarkerValuesToSchema(schema, mv)
				if err != nil {
					return err
				}
			}

			for schemaName, field := range schema.Properties {
				fieldRef, err := typeMarker.getFieldRefByJSONTag(schemaName)
				if err != nil {
					return fmt.Errorf("type '%s': %w", name, err)
				}

				if field.Value != nil {
					if field.Value.Extensions == nil {
						field.Value.Extensions = make(map[string]any)
					}
					err = applyMarkerValuesToSchema(field.Value, *fieldRef.mv)
					if err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
}

func applyMarkerValuesToSchema(schema *openapi3.Schema, mv ctmarkers.MarkerValues) error {
	if len(mv) == 0 {
		return nil
	}
	names := slices.Sorted(maps.Keys(mv))
	for _, name := range names {
		for _, raw := range mv[name] {
			if raw == nil {
				continue
			}
			sm, isSchemaMarker := raw.(markers.SchemaMarker)
			if !isSchemaMarker {
				return fmt.Errorf("marker value %T does not implement schemaMarker interface", raw)
			}
			if err := sm.ApplyToSchema(schema); err != nil {
				return fmt.Errorf("apply '%s' marker: %w", name, err)
			}
		}
	}
	return nil
}

func (t *typeMarkers) getFieldRefByJSONTag(jname string) (*fieldRef, error) {
	result := make([]fieldRef, 0)
	for _, field := range t.fields {
		if field.jsonName == jname {
			result = append(result, field)
		}
	}

	switch {
	case len(result) == 0:
		return nil, fmt.Errorf("field with tag '%s' not found", jname)
	case len(result) > 1:
		return nil, fmt.Errorf("found %d field with '%s' tag", len(result), jname)
	default:
		return &result[0], nil
	}
}

func jsonNameFromTag(tag reflect.StructTag) (string, bool) {
	s := tag.Get("json")
	if s == "" || s == "-" {
		return "", false
	}
	name := strings.Split(s, ",")[0]
	if name == "" {
		return "", false
	}
	return name, true
}

func normalizeStructType(t reflect.Type) reflect.Type {
	for t != nil && t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t
}

func collectPackages(v any) []string {
	visited := make(map[reflect.Type]struct{})
	pkgs := make(map[string]struct{})

	if v == nil {
		return nil
	}

	var walk func(t reflect.Type)
	walk = func(t reflect.Type) {
		for t.Kind() == reflect.Pointer {
			t = t.Elem()
		}

		if _, ok := visited[t]; ok {
			return
		}
		visited[t] = struct{}{}

		if pkg := t.PkgPath(); pkg != "" {
			pkgs[pkg] = struct{}{}
		}

		switch t.Kind() {
		case reflect.Struct:
			for i := range t.NumField() {
				walk(t.Field(i).Type)
			}
		case reflect.Slice, reflect.Array:
			walk(t.Elem())
		case reflect.Map:
			walk(t.Key())
			walk(t.Elem())
		}
	}

	walk(reflect.TypeOf(v))

	result := make([]string, 0, len(pkgs))
	for p := range pkgs {
		result = append(result, p)
	}

	slices.Sort(result)

	return result
}
