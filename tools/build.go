package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:generate go run ./build.go --edition all

const (
	modulesFileName            = "modules-%s.yaml"
	modulesWithExcludeFileName = "modules-with-exclude-%s.yaml"
	cloudProvidersFileName     = "cloud-providers-%s.yaml"
)

var workDir = cwd()

var defaultModulesExcludes = []string{
	"docs",
	"README.md",
	"images",
	"hooks/**/*.go",
	"template_tests",
	".namespace",
	"values_matrix_test.yaml",
	".build.yaml",
}

type writeSettings struct {
	Edition      string
	Prefix       string
	Dir          string
	SaveTo       string
	ExcludePaths []string
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
		addEntries = append(addEntries, addEntry{
			Add:          strings.TrimPrefix(file, workDir),
			To:           filepath.Join("/deckhouse", strings.TrimPrefix(file, prefix)),
			ExcludePaths: settings.ExcludePaths,
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

		buildFile := filepath.Join(file, ".build.yaml")

		ok, err := fileExists(buildFile)
		if err != nil {
			log.Fatal(err)
		}

		if ok {
			content, err := ioutil.ReadFile(buildFile)
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
	Add          string   `yaml:"add"`
	To           string   `yaml:"to"`
	ExcludePaths []string `yaml:"excludePaths,omitempty"`
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
	return dir
}

func main() {
	var edition string
	flag.StringVar(&edition, "edition", "", "Deckhouse edition")

	flag.Parse()

	if edition == "all" {
		executeEdition("CE")
		executeEdition("EE")
		executeEdition("FE")
	} else {
		executeEdition(edition)
	}
}

func executeEdition(edition string) {
	deleteRevisionFiles(edition)

	switch edition {
	case "FE":
		writeSections(writeSettings{
			Edition: edition,
			Prefix:  "ee/fe",
			Dir:     "modules",
			SaveTo:  modulesFileName,
		})
		writeSections(writeSettings{
			Edition:      edition,
			Prefix:       "ee/fe",
			Dir:          "modules",
			SaveTo:       modulesWithExcludeFileName,
			ExcludePaths: defaultModulesExcludes,
		})
		writeSections(writeSettings{
			Edition: edition,
			SaveTo:  cloudProvidersFileName,
		})
		fallthrough
	case "EE":
		writeSections(writeSettings{
			Edition: edition,
			Prefix:  "ee",
			Dir:     "modules",
			SaveTo:  modulesFileName,
		})
		writeSections(writeSettings{
			Edition:      edition,
			Prefix:       "ee",
			Dir:          "modules",
			SaveTo:       modulesWithExcludeFileName,
			ExcludePaths: defaultModulesExcludes,
		})
		writeSections(writeSettings{
			Edition: edition,
			Prefix:  "ee",
			Dir:     "candi/cloud-providers",
			SaveTo:  cloudProvidersFileName,
		})
	case "CE":
		writeSections(writeSettings{
			Edition: edition,
			SaveTo:  modulesFileName,
		})
		writeSections(writeSettings{
			Edition: edition,
			SaveTo:  modulesWithExcludeFileName,
		})
		writeSections(writeSettings{
			Edition: edition,
			SaveTo:  cloudProvidersFileName,
		})
	default:
		log.Fatalf("Unknown Deckhouse edition %q", edition)
	}
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
