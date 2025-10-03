// Copyright 2024 Flant JSC
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

package crds

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/project"
)

func List() ([]apiextensionsv1.CustomResourceDefinition, error) {
	projectDir, err := project.Dir()
	if err != nil {
		return nil, fmt.Errorf("get project directory: %w", err)
	}

	fsys := os.DirFS(filepath.Join(projectDir, "deckhouse-controller", "crds"))
	var result []apiextensionsv1.CustomResourceDefinition
	err = fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk dir: %w", err)
		}

		if _, ok := map[string]struct{}{
			".yaml": {},
			".yml":  {},
		}[filepath.Ext(path)]; !ok {
			return nil
		}

		if d.IsDir() {
			return nil
		}

		if strings.HasPrefix(path, "doc-ru-") {
			return nil
		}

		rawData, err := fs.ReadFile(fsys, path)
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}

		var crd apiextensionsv1.CustomResourceDefinition
		err = yaml.Unmarshal(rawData, &crd)
		if err != nil {
			return fmt.Errorf("unmarshal crd in %s: %w", path, err)
		}

		result = append(result, crd)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk dir: %w", err)
	}
	return result, nil
}
