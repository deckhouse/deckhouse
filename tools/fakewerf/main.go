/*
Copyright 2022 Flant JSC

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
	"bytes"
	"errors"
	"flag"
	"io/ioutil"
	"os"
	"path/filepath"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/werf/werf/pkg/path_matcher"
	"github.com/werf/werf/pkg/util"
	"gopkg.in/yaml.v2"
)

/*
fakewerf is a simple werf.yaml config renderer

It is used by matrix tests to generate fake images tags.
Hovewer it also can be used for initial validation of werf configuration.

USAGE:

  ./fakewerf -dir /deckhouse -config werf.yaml -env FE
*/

func main() {
	env := flag.String("env", ".", "Enviroment name")
	dir := flag.String("dir", ".", "Path to project")
	file := flag.String("config", "werf.yaml", "Relative path to werf.yaml")
	flag.Parse()

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
	templateData["Env"] = *env

	err = tmpl.ExecuteTemplate(os.Stdout, *file, templateData)
	if err != nil {
		panic(err)
	}
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
	data, err := ioutil.ReadFile(relPath)
	if err != nil {
		panic(err.Error())
	}
	return string(data)
}

func (f files) Glob(pattern string) map[string]interface{} {
	nf := make(map[string]interface{})

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
