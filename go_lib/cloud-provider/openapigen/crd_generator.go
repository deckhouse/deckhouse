package openapigen

import (
	"fmt"
	"reflect"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"

	"openapigen/internal/deckhouse"
	"openapigen/internal/kube"
	"openapigen/markers"
)

// CRDMeta holds optional parameters for CRD generation.
// Group, kind, scope, version names, served, and storage are all derived
// from kubebuilder markers on the root types — matching controller-gen behavior.
type CRDMeta struct {
	// MaxDescriptionLength, if non-nil, trims all description fields to the given length.
	// Mirrors controller-gen's --max-desc-len flag. nil means no trimming.
	MaxDescriptionLength *int
}

// VersionSpec binds a Go root type to a CRD version entry.
// The version name, served/storage flags, scope, group, and kind are all read
// from kubebuilder markers on Root — exactly as controller-gen does.
type VersionSpec struct {
	// Root is the Go value of the CRD root type for this version.
	// Must embed metav1.TypeMeta and metav1.ObjectMeta.
	// Package must carry +groupName=<group>, type must carry +kubebuilder:object:root=true.
	Root any
}

// CRDGenerator generates Kubernetes CustomResourceDefinitions from Go types.
// It is stateless and safe for concurrent use after construction.
type CRDGenerator struct {
	cfg SchemaConfig
}

// NewCRDGenerator creates a new CRDGenerator with the given config.
// Returns an error if the config is invalid.
func NewCRDGenerator(cfg SchemaConfig) (*CRDGenerator, error) {
	if !cfg.EnableKubebuilderMarkers && !cfg.EnableDeckhouseMarkers {
		return nil, fmt.Errorf("SchemaConfig: at least one of EnableKubebuilderMarkers or EnableDeckhouseMarkers must be true")
	}
	return &CRDGenerator{cfg: cfg}, nil
}

// Generate produces a typed *apiextensionsv1.CustomResourceDefinition from kubebuilder markers.
// versions contains one root value per CRD version; all CRD identity
// (group, kind, scope, served, storage) is read from kubebuilder markers,
// exactly as controller-gen does.
// Note: deckhouse x-* extensions are not present in the returned typed struct
// (apiextensionsv1.JSONSchemaProps drops unknown keys on unmarshal).
// Use GenerateYAML to get the fully-enriched CRD with x-* extensions preserved.
func (g *CRDGenerator) Generate(meta CRDMeta, versions []VersionSpec) (*apiextensionsv1.CustomResourceDefinition, error) {
	if err := validateVersionSpecs(versions); err != nil {
		return nil, err
	}

	if !g.cfg.EnableKubebuilderMarkers {
		return nil, fmt.Errorf("CRDGenerator requires EnableKubebuilderMarkers: true (deckhouse markers are applied on top)")
	}

	roots := versionSpecRoots(versions)
	crdObj, err := kube.GetCRDFromRoots(roots, meta.MaxDescriptionLength)
	if err != nil {
		return nil, fmt.Errorf("kubebuilder CRD: %w", err)
	}
	return crdObj, nil
}

// GenerateYAML serializes a fully-enriched CRD to YAML, prepending the default header.
// Unlike Generate, this preserves deckhouse x-* extensions in the openAPIV3Schema
// by working with raw YAML maps (bypassing JSONSchemaProps typed unmarshaling).
func (g *CRDGenerator) GenerateYAML(meta CRDMeta, versions []VersionSpec) ([]byte, error) {
	if err := validateVersionSpecs(versions); err != nil {
		return nil, err
	}

	if !g.cfg.EnableKubebuilderMarkers {
		return nil, fmt.Errorf("CRDGenerator requires EnableKubebuilderMarkers: true (deckhouse markers are applied on top)")
	}

	roots := versionSpecRoots(versions)
	crdObj, err := kube.GetCRDFromRoots(roots, meta.MaxDescriptionLength)
	if err != nil {
		return nil, fmt.Errorf("kubebuilder CRD: %w", err)
	}

	// Serialize kubebuilder CRD to a raw map first so we can patch x-* extensions.
	// apiextensionsv1.JSONSchemaProps drops unknown fields on unmarshal, so we
	// must perform deckhouse enrichment at the raw-map level.
	crdRaw, err := anyToRawMap(crdObj)
	if err != nil {
		return nil, fmt.Errorf("serialize CRD to raw map: %w", err)
	}

	if g.cfg.EnableDeckhouseMarkers {
		reg := g.cfg.DeckhouseRegistry
		if reg == nil {
			reg, err = markers.BuildDeckhouseOpenAPIMarkerRegistry()
			if err != nil {
				return nil, fmt.Errorf("build deckhouse marker registry: %w", err)
			}
		}

		rootByPkg := buildRootByPkgMap(versions)

		versionsRaw := crdRawVersions(crdRaw)
		for i, versionRaw := range versionsRaw {
			vMap, ok := versionRaw.(map[string]any)
			if !ok {
				continue
			}
			versionName, _ := vMap["name"].(string)

			root, ok := rootByPkg[versionName]
			if !ok {
				root = versions[0].Root
			}

			rootSchemaMap := crdVersionOpenAPISchema(vMap)
			if rootSchemaMap == nil {
				continue
			}

			specVal, specFieldName, err := extractSpecField(root)
			if err != nil {
				return nil, fmt.Errorf("extract spec field for version %s: %w", versionName, err)
			}

			deckhouseSpecSchema, err := deckhouse.BuildSchema(specVal, reg)
			if err != nil {
				return nil, fmt.Errorf("deckhouse schema for version %s: %w", versionName, err)
			}

			deckhouseSpecRaw, err := anyToRawMap(deckhouseSpecSchema)
			if err != nil {
				return nil, fmt.Errorf("serialize deckhouse spec schema for version %s: %w", versionName, err)
			}

			propsMap := ensureMap(rootSchemaMap, "properties")
			existingSpecMap, _ := propsMap[specFieldName].(map[string]any)
			if existingSpecMap == nil {
				propsMap[specFieldName] = deckhouseSpecRaw
			} else {
				mergeRawMaps(existingSpecMap, deckhouseSpecRaw)
				propsMap[specFieldName] = existingSpecMap
			}

			versionsRaw[i] = vMap
		}
	}

	out, err := yaml.Marshal(crdRaw)
	if err != nil {
		return nil, fmt.Errorf("marshal CRD: %w", err)
	}
	return append([]byte(defaultHeader), out...), nil
}

// reflectPkgPath returns the package path of the given value's type.
func reflectPkgPath(v any) string {
	t := reflect.TypeOf(v)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.PkgPath()
}

// pkgPathToName returns the last path segment of a package import path.
func pkgPathToName(pkgPath string) string {
	parts := strings.Split(pkgPath, "/")
	if len(parts) == 0 {
		return pkgPath
	}
	return parts[len(parts)-1]
}

// extractSpecField finds the first non-embedded struct field tagged "spec" in the root type
// and returns a zero value of that field's type along with its json tag name.
// CRD roots embed TypeMeta/ObjectMeta which cannot be passed to deckhouse.BuildSchema directly.
func extractSpecField(root any) (any, string, error) {
	t := reflect.TypeOf(root)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, "", fmt.Errorf("root must be a struct, got %s", t.Kind())
	}
	for i := range t.NumField() {
		f := t.Field(i)
		if f.Anonymous {
			continue
		}
		jsonTag := f.Tag.Get("json")
		name, _, _ := strings.Cut(jsonTag, ",")
		if name == "spec" {
			fv := reflect.New(f.Type).Interface()
			return fv, name, nil
		}
	}
	return nil, "", fmt.Errorf("no 'spec' field found in type %s", t.Name())
}

// anyToRawMap serializes any value to a raw map[string]any via YAML round-trip.
// Preserves x-* extension keys that typed structs would drop on unmarshal.
func anyToRawMap(v any) (map[string]any, error) {
	data, err := yaml.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	var out map[string]any
	if err := yaml.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("unmarshal to map: %w", err)
	}
	return out, nil
}

// mergeRawMaps deep-merges src into dst. For nested maps, recurses. src values override dst.
func mergeRawMaps(dst, src map[string]any) {
	for k, sv := range src {
		dv, exists := dst[k]
		if !exists {
			dst[k] = sv
			continue
		}
		dstMap, dstIsMap := dv.(map[string]any)
		srcMap, srcIsMap := sv.(map[string]any)
		if dstIsMap && srcIsMap {
			mergeRawMaps(dstMap, srcMap)
			dst[k] = dstMap
			continue
		}
		dst[k] = sv
	}
}

// validateVersionSpecs validates versions slice for Generate/GenerateYAML.
func validateVersionSpecs(versions []VersionSpec) error {
	if len(versions) == 0 {
		return fmt.Errorf("at least one VersionSpec is required")
	}
	for i, v := range versions {
		if v.Root == nil {
			return fmt.Errorf("VersionSpec[%d].Root must not be nil", i)
		}
	}
	return nil
}

// versionSpecRoots extracts the Root values from a []VersionSpec.
func versionSpecRoots(versions []VersionSpec) []any {
	roots := make([]any, len(versions))
	for i, v := range versions {
		roots[i] = v.Root
	}
	return roots
}

// buildRootByPkgMap maps package name → root value for each version.
func buildRootByPkgMap(versions []VersionSpec) map[string]any {
	m := make(map[string]any, len(versions))
	for _, v := range versions {
		pkgPath := reflectPkgPath(v.Root)
		m[pkgPathToName(pkgPath)] = v.Root
	}
	return m
}

// crdRawVersions returns the versions slice from a raw CRD map.
func crdRawVersions(crdRaw map[string]any) []any {
	specMap, _ := crdRaw["spec"].(map[string]any)
	if specMap == nil {
		return nil
	}
	versions, _ := specMap["versions"].([]any)
	return versions
}

// crdVersionOpenAPISchema returns the openAPIV3Schema map from a raw version map.
func crdVersionOpenAPISchema(vMap map[string]any) map[string]any {
	schemaMap, _ := vMap["schema"].(map[string]any)
	if schemaMap == nil {
		return nil
	}
	rootSchemaMap, _ := schemaMap["openAPIV3Schema"].(map[string]any)
	return rootSchemaMap
}

// ensureMap returns the sub-map for key in m, creating it if absent.
func ensureMap(m map[string]any, key string) map[string]any {
	v, _ := m[key].(map[string]any)
	if v == nil {
		v = make(map[string]any)
		m[key] = v
	}
	return v
}

// GenerateCRD generates a full Kubernetes CRD YAML with enriched openAPIV3Schema
// (kubebuilder + deckhouse markers) for each version.
// All CRD identity (group, kind, scope, served, storage) is read from kubebuilder markers
// on the root types — matching controller-gen behavior.
func GenerateCRD(versions []VersionSpec) ([]byte, error) {
	reg, err := markers.BuildDeckhouseOpenAPIMarkerRegistry()
	if err != nil {
		return nil, err
	}
	gen, err := NewCRDGenerator(SchemaConfig{
		EnableKubebuilderMarkers: true,
		EnableDeckhouseMarkers:   true,
		DeckhouseRegistry:        reg,
	})
	if err != nil {
		return nil, err
	}
	return gen.GenerateYAML(CRDMeta{}, versions)
}

// GenerateCRDDescriptionRu generates a CRD-shaped ru-description overlay YAML,
// containing only description fields derived from deckhouse ru:description markers.
func GenerateCRDDescriptionRu(versions []VersionSpec) ([]byte, error) {
	reg, err := markers.BuildDeckhouseDescriptionRuOpenAPIMarkerRegistry()
	if err != nil {
		return nil, err
	}
	gen, err := NewCRDGenerator(SchemaConfig{
		EnableKubebuilderMarkers: true,
		EnableDeckhouseMarkers:   true,
		DeckhouseRegistry:        reg,
	})
	if err != nil {
		return nil, err
	}
	return gen.GenerateYAML(CRDMeta{}, versions)
}
