// Copyright 2025 Flant JSC
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

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

const pathToSidebars = "/app/sidebars"
const pathToResult = "/app/results"
const breadcrumbsFileName = "breadcrumbs.yml"

const templateString = `{{ .Url }}:
  title:
    en: {{ .En }}
    ru: {{ .Ru }}
`

type data struct {
	Url string
	En  string
	Ru  string
}

type Sidebar struct {
	Entries []Entry `yaml:"entries"`
}

type Entry struct {
	Title struct {
		En string `yaml:"en"`
		Ru string `yaml:"ru"`
	} `yaml:"title"`
	URL     string  `yaml:"url,omitempty"`
	Folders []Entry `yaml:"folders,omitempty"`
}

func main() {
	if err := os.Mkdir(pathToResult, os.ModePerm); err != nil {
		log.Fatal(err)
	}
	files, err := os.ReadDir(pathToSidebars)
	if err != nil {
		log.Fatal(err)
	}
	for _, f := range files {
		var sidebar Sidebar

		file, err := os.ReadFile(filepath.Join(pathToSidebars, f.Name()))
		if err != nil {
			log.Fatal(err)
		}
		err = yaml.Unmarshal(file, &sidebar)
		if err != nil {
			fmt.Println(err)
			return
		}

		pathToBreadcrumbsFile := filepath.Join(pathToResult, strings.TrimSuffix(f.Name(), filepath.Ext(f.Name())))
		err = os.MkdirAll(pathToBreadcrumbsFile, os.ModePerm)
		if err != nil {
			log.Fatal(err)
		}
		pathToFile := filepath.Join(pathToBreadcrumbsFile, breadcrumbsFileName)
		breadcrumbsFile, err := os.Create(pathToFile)
		if err != nil {
			fmt.Println(err)
		}
		defer breadcrumbsFile.Close()

		for _, entry := range sidebar.Entries {
			if entry.Folders != nil {
				readFolders(entry.Title.En, entry.Title.Ru, entry.Folders, *breadcrumbsFile)
			}
		}
	}
}

func getLevelUrl(folders []Entry) string {
	index := strings.LastIndex(folders[0].URL, "/")
	if index != -1 {
		return folders[0].URL[:index]
	}
	return ""
}

func readFolders(titleEn string, titleRu string, folders []Entry, breadcrumbsFile os.File) {
	for _, entry := range folders {
		if entry.Folders != nil {
			readFolders(entry.Title.En, entry.Title.Ru, entry.Folders, breadcrumbsFile)
		}
	}
	if checkURLs(folders) {
		url := getLevelUrl(folders)
		if url != "" {
			var dt data
			dt.Url = url
			dt.En = titleEn
			dt.Ru = titleRu

			tmpl, err := template.New("struct").Parse(templateString)
			if err != nil {
				panic(err)
			}

			err = tmpl.Execute(&breadcrumbsFile, dt)
			if err != nil {
				panic(err)
			}
		}
	}
}

func checkURLs(folders []Entry) bool {
	result := false
	for _, entity := range folders {
		if entity.URL != "" {
			result = true
		}
	}
	return result
}
