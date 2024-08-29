/*
Copyright 2024 Flant JSC

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

package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/yaml"
)

func cwd() string {
	_, f, _, ok := runtime.Caller(1)
	if !ok {
		panic("cannot get caller")
	}

	dir, err := filepath.Abs(f)
	if err != nil {
		panic(err)
	}

	for i := 0; i < 3; i++ { // ../../
		dir = filepath.Dir(dir)
	}

	// If deckhouse repo directory is symlinked (e.g. to /deckhouse), resolve the real path.
	// Otherwise, filepath.Walk will ignore all subdirectories.
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		panic(err)
	}

	return dir
}

func walkModules(workDir string) ([]settings, error) {
	var modules []settings
	err := filepath.Walk(workDir, func(path string, f os.FileInfo, err error) error {
		if f != nil && f.IsDir() {
			if slices.Contains([]string{"internal", "testdata", "docs", ".github"}, f.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		// In case of files inside `templates` directory we want only module path
		modulePath := strings.Split(filepath.Dir(path), "templates")[0]
		modulePath = strings.TrimRight(modulePath, "/")
		if filepath.Base(path) == "rbac.yaml" {
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			var moduleSettings settings
			if err = yaml.Unmarshal(raw, &moduleSettings); err != nil {
				return err
			}
			moduleSettings.path = modulePath
			for idx, crds := range moduleSettings.CRDs {
				moduleSettings.CRDs[idx] = filepath.Join(workDir, crds)
			}
			modules = append(modules, moduleSettings)
			return nil
		}
		return err
	})
	return modules, err
}

func main() {
	workDir := cwd()
	modules, err := walkModules(workDir)
	if err != nil {
		panic(err)
	}

	var documentation = docs{
		Scopes:  map[string]scopeDoc{},
		Modules: map[string]moduleDoc{},
	}

	ctx := context.Background()
	for _, module := range modules {
		generator, err := newModuleGenerator(module)
		if err != nil {
			panic(err)
		}
		documentation.Modules[module.Module], err = generator.generate(ctx)
		if err != nil {
			panic(err)
		}
		for _, scope := range module.Scopes {
			if found, ok := documentation.Scopes[scope]; ok {
				found.Modules = append(found.Modules, module.Module)
				if documentation.Modules[module.Module].Namespace != "none" {
					found.namespacesSet.Insert(documentation.Modules[module.Module].Namespace)
				}
				documentation.Scopes[scope] = found
			} else {
				doc := scopeDoc{Modules: []string{module.Module}, namespacesSet: sets.NewString()}
				if documentation.Modules[module.Module].Namespace != "none" {
					doc.namespacesSet.Insert(documentation.Modules[module.Module].Namespace)
				}
				documentation.Scopes[scope] = doc
			}
		}
	}
	for key, doc := range documentation.Scopes {
		if val, ok := documentation.Scopes[key]; ok {
			val.Namespaces = doc.namespacesSet.List()
			documentation.Scopes[key] = val
		}
	}

	marshaled, err := yaml.Marshal(documentation)
	if err != nil {
		panic(err)
	}

	var tmp interface{}
	if err = yaml.Unmarshal(marshaled, &tmp); err != nil {
		panic(err)
	}

	marshaled, err = yaml.Marshal(tmp)
	if err != nil {
		panic(err)
	}

	if err = os.WriteFile(filepath.Join(workDir, "docs", "documentation", "_data", "rbac.yaml"), marshaled, 0666); err != nil {
		panic(err)
	}
}
