/*
Copyright 2023 Flant JSC

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

package conversion

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"tools/helm_generate/helper"

	"gopkg.in/yaml.v3"
)

const (
	conversionFolder = "/openapi/conversions"
)

var regexVersionFile = regexp.MustCompile("v([1-9]|[1-9][0-9]|[1-9][0-9][0-9]).yaml")

type module struct {
	Name string
	Path string
}

type moduleFile struct {
	Name string `yaml:"name"`
}

type conversion struct {
	Version     *int         `yaml:"version,omitempty"`
	Description *description `yaml:"description,omitempty"`
}

type description struct {
	English string `yaml:"en,omitempty"`
	Russian string `yaml:"ru,omitempty"`
}

func run() error {
	var err error

	deckhouseRoot, err := helper.DeckhouseRoot()
	if err != nil {
		return fmt.Errorf("deckhouse root: %w", err)
	}

	modules := modules(deckhouseRoot)

	result := make(map[string][]conversion, len(modules))

	for _, module := range modules {
		folder := filepath.Join(deckhouseRoot, module.Path, conversionFolder)

		stat, err := os.Stat(folder)
		if err != nil && !os.IsNotExist(err) {
			panic(err)
		}

		if os.IsNotExist(err) || !stat.IsDir() {
			continue
		}

		_, ok := result[module.Name]
		if ok {
			panic("duplicate module name, probably we have collisions")
		}

		filepath.Walk(folder, func(path string, info fs.FileInfo, _ error) error {
			if !regexVersionFile.MatchString(filepath.Base(path)) {
				return nil
			}

			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("open file to read conversion: %w", err)
			}

			c := new(conversion)
			err = yaml.NewDecoder(file).Decode(c)
			if err != nil {
				return fmt.Errorf("yaml decode: %w", err)
			}

			conversions, ok := result[module.Name]
			if !ok {
				conversions = make([]conversion, 0, 1)
			}

			conversions = append(conversions, *c)

			result[module.Name] = conversions

			return nil
		})
	}

	fileName := filepath.Join(deckhouseRoot, "docs/documentation/_data/conversions.yml")

	file, err := os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return fmt.Errorf("open file to write conversions: %w", err)
	}
	defer file.Close()

	err = yaml.NewEncoder(file).Encode(result)
	if err != nil {
		return fmt.Errorf("yaml encode: %w", err)
	}

	return nil
}

func modules(deckhouseRoot string) []module {
	modules := make([]module, 0)

	// ce modules
	modules = append(modules, parseModules(deckhouseRoot, "modules")...)
	// ee modules
	modules = append(modules, parseModules(deckhouseRoot, "ee/modules")...)
	// be modules
	modules = append(modules, parseModules(deckhouseRoot, "ee/be/modules")...)
	// fe modules
	modules = append(modules, parseModules(deckhouseRoot, "ee/fe/modules")...)
	// se modules
	modules = append(modules, parseModules(deckhouseRoot, "ee/se/modules")...)
	modules = append(modules, parseModules(deckhouseRoot, "ee/se-plus/modules")...)

	return modules
}

func parseModules(deckhouseRoot string, folder string) []module {
	modules := make([]module, 0)

	files, _ := os.ReadDir(filepath.Join(deckhouseRoot, folder))
	for _, file := range files {
		if file.IsDir() {
			md := module{
				Name: file.Name(),
				Path: filepath.Join(folder, file.Name()),
			}

			// if we found module.yaml in folder - parse name from it
			moduleFilePath := filepath.Join(deckhouseRoot, folder, file.Name(), "module.yaml")
			_, err := os.Stat(moduleFilePath)
			if err == nil {
				f, err := os.OpenFile(moduleFilePath, os.O_RDONLY, 0666)
				if err != nil {
					panic(err)
				}

				mf := new(moduleFile)
				err = yaml.NewDecoder(f).Decode(&mf)
				if err != nil {
					panic(err)
				}

				if mf.Name != "" {
					md.Name = mf.Name
				}
			}

			modules = append(modules, md)
		}
	}

	return modules
}
