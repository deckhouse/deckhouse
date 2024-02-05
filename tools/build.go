/*
Copyright 2021 Flant JSC

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
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
	"k8s.io/utils/pointer"
)

//go:generate go run ./build.go --edition all

const (
	modulesFileName            = "modules-%s.yaml"
	modulesWithExcludeFileName = "modules-with-exclude-%s.yaml"
	modulesWithDependencies    = "modules-with-dependencies-%s.yaml"
	candiFileName              = "candi-%s.yaml"
	modulesExcluded            = "modules-excluded-%s.yaml"
)

var workDir = cwd()

var defaultModulesExcludes = []string{
	"docs",
	"README.md",
	"images",
	"hooks/**/*.go",
	"hooks/*.go",
	"hack",
	"template_tests",
	".namespace",
	"values_matrix_test.yaml",
	".build.yaml",
}

var nothingButGoHooksExcludes = []string{
	"images",
	"templates",
	"charts",
	"crds",
	"docs",
	"monitoring",
	"openapi",
	"oss.yaml",
	"cloud-instance-manager",
	"values_matrix_test.yaml",
	"values.yaml",
	".helmignore",
	"candi",
	"Chart.yaml",
	".namespace",
	"**/*_test.go",
	"**/*.sh",
}

var stageDependencies = map[string][]string{
	"setup": {
		"**/*.go",
	},
}

type writeSettings struct {
	Edition           string
	Prefix            string
	Dir               string
	SaveTo            string
	ExcludePaths      []string
	StageDependencies map[string][]string
	ExcludedModules   map[string]struct{}
}

func writeExcludedModules(settings writeSettings, modules map[string]string, ed edition) {
	saveTo := fmt.Sprintf(settings.SaveTo, settings.Edition)

	if len(ed.ExcludeModules) == 0 {
		if err := writeToFile(saveTo, nil); err != nil {
			log.Fatal(err)
		}
		return
	}

	resultArr := make([]string, 0, len(ed.ExcludeModules))

	for _, name := range ed.ExcludeModules {
		modulePath, ok := modules[name]
		if !ok {
			log.Print(fmt.Sprintf("Not found module path for module %s\n", modulePath))
			continue
		}
		resultArr = append(resultArr, fmt.Sprintf("- %s/**", modulePath))
	}

	result := []byte(strings.Join(resultArr, "\n"))

	if err := writeToFile(saveTo, result); err != nil {
		log.Fatal(err)
	}
}

func writeSections(settings writeSettings) {
	saveTo := fmt.Sprintf(settings.SaveTo, settings.Edition)

	if settings.Dir == "" || settings.Prefix == "" {
		if err := writeToFile(saveTo, nil); err != nil {
			log.Fatal(err)
		}
		return
	}

	var addEntries []addEntry

	prefix := filepath.Join(workDir, settings.Prefix)
	searchDir := filepath.Join(prefix, settings.Dir, "*")

	files, err := filepath.Glob(searchDir)
	if err != nil {
		log.Fatalf("globbing: %v", err)
	}
	addNewFileEntry := func(file string) {
		hooksPathRegex := regexp.MustCompile(`\d+-[\w\-]+\/hooks`)
		// we do not want to add hooks to the modules-with-exclude include
		// this include is used in the dev-prebuild image, which does not use the hooks folder
		if settings.SaveTo == modulesWithExcludeFileName && hooksPathRegex.Match([]byte(file)) {
			return
		}
		addEntries = append(addEntries, addEntry{
			Add:               strings.TrimPrefix(file, workDir),
			To:                filepath.Join("/deckhouse", strings.TrimPrefix(file, prefix)),
			ExcludePaths:      settings.ExcludePaths,
			StageDependencies: settings.StageDependencies,
		})
	}

	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}

		if !info.IsDir() {
			continue
		}

		if len(settings.ExcludedModules) > 0 {
			moduleName := filepath.Base(file)[4:]
			// skip excluded modules
			if _, ok := settings.ExcludedModules[moduleName]; ok {
				continue
			}
		}

		buildFile := filepath.Join(file, ".build.yaml")

		ok, err := fileExists(buildFile)
		if err != nil {
			log.Fatal(err)
		}

		if ok {
			content, err := os.ReadFile(buildFile)
			if err != nil {
				log.Fatal(err)
			}

			if len(content) == 0 {
				// no need to add any files
				continue
			}

			// if build.yaml exists and not empty, try to add instruction
			// from it instead adding the entry for whole module
			scanner := bufio.NewScanner(bytes.NewReader(content))
			for scanner.Scan() {
				s := strings.TrimSpace(scanner.Text())
				additionalFiles, err := filepath.Glob(filepath.Join(file, s))
				if err != nil {
					log.Fatalf("globbing: %v", err)
				}

				for _, additionalFile := range additionalFiles {
					addNewFileEntry(additionalFile)
				}
			}
		} else {
			addNewFileEntry(file)
		}
	}

	var result []byte
	if len(addEntries) != 0 {
		result, err = yaml.Marshal(addEntries)
		if err != nil {
			log.Fatalf("converting entries to YAML: %v", err)
		}
	}

	if err := writeToFile(saveTo, result); err != nil {
		log.Fatal(err)
	}
}

func deleteRevisionFiles(edition string) {
	files, err := filepath.Glob(includePath(fmt.Sprintf("*-%s.yaml", edition)))
	if err != nil {
		log.Fatalf("globbing: %v", err)
	}

	for _, file := range files {
		_ = os.Remove(file)
	}
}

type addEntry struct {
	Add               string              `yaml:"add"`
	To                string              `yaml:"to"`
	ExcludePaths      []string            `yaml:"excludePaths,omitempty"`
	StageDependencies map[string][]string `yaml:"stageDependencies,omitempty"`
}

func cwd() string {
	_, f, _, ok := runtime.Caller(1)
	if !ok {
		panic("cannot get caller")
	}

	dir, err := filepath.Abs(f)
	if err != nil {
		panic(err)
	}

	for i := 0; i < 2; i++ { // ../
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

type buildIncludes struct {
	SkipCandi   *bool `yaml:"skipCandi,omitempty"`
	SkipModules *bool `yaml:"skipModules,omitempty"`
}

type edition struct {
	Name           string         `yaml:"name,omitempty"`
	ModulesDir     string         `yaml:"modulesDir,omitempty"`
	BuildIncludes  *buildIncludes `yaml:"buildIncludes,omitempty"`
	ExcludeModules []string       `yaml:"excludeModules,omitempty"`
}

type editions struct {
	Editions []edition `yaml:"editions,omitempty"`
}

type executor struct {
	Editions []edition
}

func newExecutor() *executor {
	content, err := os.ReadFile(workDir + "/editions.yaml")
	if err != nil {
		panic(fmt.Sprintf("cannot read editions file: %v", err))
	}

	e := editions{}
	err = yaml.Unmarshal(content, &e)
	if err != nil {
		panic(fmt.Errorf("cannot unmarshal editions file: %v", err))
	}

	for i, ed := range e.Editions {
		if ed.Name == "" {
			panic(fmt.Sprintf("name for %d index is empty", i))
		}
	}

	return &executor{
		Editions: e.Editions,
	}
}

func main() {
	var editionStr string
	flag.StringVar(&editionStr, "edition", "", "Deckhouse edition")

	flag.Parse()

	e := newExecutor()

	if editionStr == "all" {
		for _, ed := range e.Editions {
			e.executeEdition(ed.Name)
		}
	} else {
		for _, ed := range e.Editions {
			if ed.Name == editionStr {
				e.executeEdition(editionStr)
			}
		}

		log.Fatalf("Incorrect edition %q", editionStr)
	}
}

func (e *executor) executeEdition(editionName string) {
	deleteRevisionFiles(editionName)
	modulesDict := make(map[string]string)

	for _, ed := range e.Editions {
		// get moduleName => path dict
		searchDir := filepath.Join(workDir, ed.ModulesDir)
		entries, err := os.ReadDir(searchDir)
		if err != nil {
			log.Fatalf("cannot read dir: %s", err)
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			moduleName := e.Name()[4:]
			modulesDict[moduleName] = path.Join(ed.ModulesDir, e.Name())
		}

		// convert excluded modules to dict
		excludeModules := make(map[string]struct{})
		for _, moduleName := range ed.ExcludeModules {
			excludeModules[moduleName] = struct{}{}
		}

		bi := ed.BuildIncludes
		if bi == nil {
			bi = &buildIncludes{
				SkipCandi:   pointer.Bool(false),
				SkipModules: pointer.Bool(false),
			}
		}

		prefix := strings.TrimPrefix(strings.TrimSuffix(ed.ModulesDir, "modules"), "/")

		writeSettingCandi := writeSettings{
			Edition: editionName,
			SaveTo:  candiFileName,
		}
		if bi.SkipCandi == nil || !*bi.SkipCandi {
			writeSettingCandi.Prefix = prefix
			writeSettingCandi.Dir = "candi"
		}

		writeSettingsModules := writeSettings{
			Edition:         editionName,
			SaveTo:          modulesFileName,
			ExcludedModules: excludeModules,
		}

		writeSettingsExcludeFileName := writeSettings{
			Edition:         editionName,
			SaveTo:          modulesWithExcludeFileName,
			ExcludedModules: excludeModules,
		}

		writeSettingStageDeps := writeSettings{
			Edition:         editionName,
			SaveTo:          modulesWithDependencies,
			ExcludedModules: excludeModules,
		}

		if bi.SkipModules == nil || !*bi.SkipModules {
			writeSettingsModules.Prefix = prefix
			writeSettingsModules.Dir = "modules"
			writeSettingsModules.StageDependencies = stageDependencies

			writeSettingsExcludeFileName.Prefix = prefix
			writeSettingsExcludeFileName.Dir = "modules"
			writeSettingsExcludeFileName.ExcludePaths = defaultModulesExcludes

			writeSettingStageDeps.Prefix = prefix
			writeSettingStageDeps.Dir = "modules"
			writeSettingStageDeps.StageDependencies = stageDependencies
			writeSettingStageDeps.ExcludePaths = nothingButGoHooksExcludes

		}

		writeSections(writeSettingsModules)
		writeSections(writeSettingsExcludeFileName)
		writeSections(writeSettingStageDeps)
		writeSections(writeSettingCandi)

		if ed.Name == editionName {
			// only for one edition
			writeExcludedModules(writeSettings{
				SaveTo:  modulesExcluded,
				Edition: editionName,
			}, modulesDict, ed)
			return
		}
	}

	log.Fatalf("Unknown Deckhouse edition %q", editionName)
}

func writeToFile(path string, content []byte) error {
	f, err := os.OpenFile(includePath(path), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// Only write header once
	if stat, _ := f.Stat(); stat.Size() == 0 {
		_, err = f.Write([]byte("# Code generated by tools/build.go; DO NOT EDIT.\n"))
		if err != nil {
			return err
		}
	}

	_, err = f.Write(content)
	return err
}

// includePath returns absolute path for build_includes directory (destination)
func includePath(path string) string {
	return filepath.Join(workDir, "tools", "build_includes", path)
}

func fileExists(parts ...string) (bool, error) {
	_, err := os.Stat(filepath.Join(parts...))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
