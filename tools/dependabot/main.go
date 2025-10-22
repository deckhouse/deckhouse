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

package main

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

const templateFile = "dependabot/dependabot.tpl"

type Paths struct {
	GoMod []string
	NPM   []string
	PIP   []string
}

type Data struct {
	Paths Paths
}

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	cwd = filepath.Dir(cwd)
	log.Printf("repo dir: %s\n", cwd)

	tpl, err := template.New(filepath.Base(templateFile)).ParseFiles(templateFile)
	if err != nil {
		log.Fatal(err)
	}

	paths := Paths{
		GoMod: find(cwd, "go.mod"),
		PIP:   find(cwd, "requirements.txt"),
		NPM:   find(cwd, "yarn.lock"),
	}

	for _, path := range paths.GoMod {
		log.Printf("go.mod entry found: %s\n", path)
	}

	for _, path := range paths.PIP {
		log.Printf("pip entry found: %s\n", path)
	}

	for _, path := range paths.NPM {
		log.Printf("npm entry found: %s\n", path)
	}

	var res bytes.Buffer
	if err := tpl.Execute(&res, Data{Paths: paths}); err != nil {
		log.Fatal(err)
	}

	resFile := filepath.Join(cwd, ".github/dependabot.yml")

	log.Printf("saving to %s\n", resFile)
	if err := os.WriteFile(resFile, res.Bytes(), 0600); err != nil {
		log.Fatal(err)
	}
}

func find(cwd, file string) []string {
	var res []string

	err := filepath.Walk(cwd, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && info.Name() == file {
			// find relative path to the repo root, dependabot searches from the repo root
			relativePath := filepath.Join("/", strings.TrimPrefix(filepath.Dir(path), cwd))
			res = append(res, relativePath)
		}

		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	return res
}
