package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	yaml "gopkg.in/yaml.v3"
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
			if f.Name() == "internal" {
				return filepath.SkipDir
			}
			if f.Name() == "testdata" {
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
			log.Printf("found module with settings: %v\n", moduleSettings)
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
	ctx := context.Background()
	for _, module := range modules {
		generator, err := newModuleGenerator(module)
		if err != nil {
			panic(err)
		}
		if err = generator.generate(ctx); err != nil {
			panic(err)
		}
	}
}
