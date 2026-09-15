/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package bundle

import (
	"fmt"
	"os"
	"path/filepath"

	"openapigen"

	zicv1 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/api/instanceclass/v1"
	zicv1alpha1 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/api/instanceclass/v1alpha1"
	zsettingsv2 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/api/settings/v2"
)

// GenerateBundle generates all OpenAPI specs for the cloud-provider-zvirt module.
// It writes:
//   - openapi/config-values.yaml         (ModuleConfigSettings schema)
//   - openapi/doc-ru-config-values.yaml  (ModuleConfigSettings ru descriptions)
//   - crds/instance_class.yaml           (ZvirtInstanceClass CRD)
//   - crds/doc-ru-instance_class.yaml    (ZvirtInstanceClass ru descriptions)
func GenerateBundle(moduleRoot string) error {
	instanceClassVersions := []openapigen.VersionSpec{
		{Root: &zicv1.ZvirtInstanceClass{}},
		{Root: &zicv1alpha1.ZvirtInstanceClass{}},
	}

	steps := []struct {
		name string
		path string
		gen  func() ([]byte, error)
	}{
		{
			name: "config-values",
			path: filepath.Join(moduleRoot, "openapi", "config-values.yaml"),
			gen: func() ([]byte, error) {
				return openapigen.GenerateDeckhouseOpenAPISchema(zsettingsv2.ModuleConfigSettings{})
			},
		},
		{
			name: "doc-ru-config-values",
			path: filepath.Join(moduleRoot, "openapi", "doc-ru-config-values.yaml"),
			gen: func() ([]byte, error) {
				return openapigen.GenerateDeckhouseDescriptionRu(zsettingsv2.ModuleConfigSettings{})
			},
		},
		{
			name: "instance_class CRD",
			path: filepath.Join(moduleRoot, "crds", "instance_class.yaml"),
			gen: func() ([]byte, error) {
				return openapigen.GenerateCRD(instanceClassVersions)
			},
		},
		{
			name: "doc-ru-instance_class",
			path: filepath.Join(moduleRoot, "crds", "doc-ru-instance_class.yaml"),
			gen: func() ([]byte, error) {
				return openapigen.GenerateCRDDescriptionRu(instanceClassVersions)
			},
		},
	}

	for _, s := range steps {
		data, err := s.gen()
		if err != nil {
			return fmt.Errorf("generate %s: %w", s.name, err)
		}
		if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
			return fmt.Errorf("mkdir for %s: %w", s.name, err)
		}
		if err := os.WriteFile(s.path, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", s.name, err)
		}
		fmt.Printf("generated: %s\n", s.path)
	}

	return nil
}
