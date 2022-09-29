package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/werf/werf/pkg/path_matcher"
	"github.com/werf/werf/pkg/util"
	"gopkg.in/yaml.v2"
)

type Spec struct {
	Image        string       `yaml:"image"`
	Dependencies []Dependency `yaml:"dependencies"`
}

type Dependency struct {
	Imports []Import `yaml:"imports"`
}

type Import struct {
	TargetEnv string `yaml:"targetEnv"`
}

func myUsage() {
	fmt.Printf("Usage:\n")
	fmt.Printf("  fakewerf [OPTIONS] render\n\trender werf.yaml\n")
	fmt.Printf("  fakewerf [OPTIONS] images-tags\n\tgenerate images_tags.json\n")
	fmt.Printf("\nOptions:\n")
	flag.PrintDefaults()
}

func main() {

	flag.Usage = myUsage
	dir := flag.String("dir", ".", "Path to project")
	file := flag.String("config", "werf.yaml", "Relative path to werf.yaml")
	flag.Parse()
	args := flag.Args()
	if flag.NArg() != 1 || (args[0] != "render" && args[0] != "images-tags") {
		flag.Usage()
		os.Exit(1)
	}

	err := os.Chdir(*dir)
	if err != nil {
		panic(err)
	}

	tmpl := template.New("")
	tmpl.Funcs(funcMap(tmpl))
	tmpl, err = tmpl.ParseFiles(*file)
	if err != nil {
		panic(err)
	}

	templateData := make(map[string]interface{})
	templateData["Files"] = files{}

	var b bytes.Buffer
	err = tmpl.ExecuteTemplate(&b, *file, templateData)
	if err != nil {
		panic(err)
	}
	werfConfig := b.String()
	if args[0] == "render" {
		fmt.Println(werfConfig)
		os.Exit(0)
	}

	imagesTags := make(map[string]map[string]string)
	a := strings.NewReader(werfConfig)
	d := yaml.NewDecoder(a)
	for {
		spec := new(Spec)
		err := d.Decode(&spec)
		if spec == nil {
			continue
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			panic(err)
		}
		if spec.Image == "images-tags" {
			for _, dependency := range spec.Dependencies {
				for _, i := range dependency.Imports {
					s := strings.Split(i.TargetEnv, "_")
					module := s[3]
					tag := s[4]
					if imagesTags[module] == nil {
						imagesTags[module] = make(map[string]string)
					}
					imagesTags[module][tag] = "imageHash"
				}
			}
			break
		}
	}
	j, err := json.Marshal(imagesTags)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s\n", j)
}

func funcMap(tmpl *template.Template) template.FuncMap {
	funcMap := sprig.TxtFuncMap()
	delete(funcMap, "expandenv")

	funcMap["fromYaml"] = func(str string) (map[string]interface{}, error) {
		m := map[string]interface{}{}

		if err := yaml.Unmarshal([]byte(str), &m); err != nil {
			return nil, err
		}

		return m, nil
	}

	funcMap["include"] = func(name string, data interface{}) (string, error) {
		return executeTemplate(tmpl, name, data)
	}
	funcMap["tpl"] = func(templateContent string, data interface{}) (string, error) {
		templateName := util.GenerateConsistentRandomString(10)
		if err := addTemplate(tmpl, templateName, templateContent); err != nil {
			return "", err
		}

		return executeTemplate(tmpl, templateName, data)
	}

	funcMap["required"] = func(msg string, val interface{}) (interface{}, error) {
		if val == nil {
			return val, errors.New(msg)
		} else if _, ok := val.(string); ok {
			if val == "" {
				return val, errors.New(msg)
			}
		}
		return val, nil
	}

	return funcMap
}

func executeTemplate(tmpl *template.Template, name string, data interface{}) (string, error) {
	buf := bytes.NewBuffer(nil)
	if err := tmpl.ExecuteTemplate(buf, name, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func addTemplate(tmpl *template.Template, templateName, templateContent string) error {
	extraTemplate := tmpl.New(templateName)
	_, err := extraTemplate.Parse(templateContent)
	return err
}

type files struct{}

func (f files) Get(relPath string) string {
	data, _ := ioutil.ReadFile(relPath)
	//if err != nil {
	//	panic(err.Error())
	//}
	return string(data)
}

func (f files) Glob(pattern string) map[string]string {
	nf := make(map[string]string)

	pathMatcher := path_matcher.NewPathMatcher(path_matcher.PathMatcherOptions{
		BasePath:     ".",
		IncludeGlobs: []string{pattern},
	})

	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !pathMatcher.IsPathMatched(path) {
			return nil
		}

		if info.IsDir() {
			return nil
		}

		content, err := ioutil.ReadFile(path)
		if err != nil {
			panic(err.Error())
		}
		nf[path] = string(content)
		return nil
	})

	if err != nil {
		panic(err.Error())
	}

	return nf
}
