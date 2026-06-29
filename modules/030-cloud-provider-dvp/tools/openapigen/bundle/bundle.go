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

package bundle

import (
	"fmt"
	"os"
	"path/filepath"

	"openapigen"

	"github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/api/instanceclass/v1alpha1"
	"github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/api/settings"
)

// GenerateBundle generates all OpenAPI specs for the cloud-provider-dvp module.
// It writes:
//   - openapi/config-values.yaml         (ModuleConfigSettings schema)
//   - openapi/doc-ru-config-values.yaml  (ModuleConfigSettings ru descriptions)
//   - crds/instance_class.yaml           (DVPInstanceClass CRD)
//   - crds/doc-ru-instance_class.yaml    (DVPInstanceClass ru descriptions)
func GenerateBundle(moduleRoot string) error {
	steps := []struct {
		name string
		path string
		gen  func() ([]byte, error)
	}{
		{
			name: "config-values",
			path: filepath.Join(moduleRoot, "openapi", "config-values.yaml"),
			gen: func() ([]byte, error) {
				return openapigen.GenerateDeckhouseOpenAPISchema(settings.ModuleConfigSettings{})
			},
		},
		{
			name: "doc-ru-config-values",
			path: filepath.Join(moduleRoot, "openapi", "doc-ru-config-values.yaml"),
			gen: func() ([]byte, error) {
				return openapigen.GenerateDeckhouseDescriptionRu(settings.ModuleConfigSettings{})
			},
		},
		{
			name: "instance_class CRD",
			path: filepath.Join(moduleRoot, "crds", "instance_class.yaml"),
			gen: func() ([]byte, error) {
				return openapigen.GenerateCRD([]openapigen.VersionSpec{
					{Root: &v1alpha1.DVPInstanceClass{}},
				})
			},
		},
		{
			name: "doc-ru-instance_class",
			path: filepath.Join(moduleRoot, "crds", "doc-ru-instance_class.yaml"),
			gen: func() ([]byte, error) {
				return openapigen.GenerateCRDDescriptionRu([]openapigen.VersionSpec{
					{Root: &v1alpha1.DVPInstanceClass{}},
				})
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
