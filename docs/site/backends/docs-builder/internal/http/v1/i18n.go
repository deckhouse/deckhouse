// Copyright 2026 Flant JSC
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

package v1

import (
	"embed"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/flant/docs-builder/internal/docs"
)

//go:embed i18n/*.yaml
var i18nFS embed.FS

type pageTexts struct {
	Placeholder placeholderTexts `yaml:"placeholder"`
	Versions    versionsTexts    `yaml:"versions"`
	ModulesList modulesListTexts `yaml:"modulesList"`
}

// texts are the texts of the on-demand pages per language, from i18n/<lang>.yaml.
var texts = mustLoadTexts()

func mustLoadTexts() map[string]pageTexts {
	all := make(map[string]pageTexts, len(docs.Languages))
	for _, lang := range docs.Languages {
		raw, err := i18nFS.ReadFile("i18n/" + lang + ".yaml")
		if err != nil {
			panic(fmt.Sprintf("read %s texts: %v", lang, err))
		}

		var t pageTexts
		if err := yaml.Unmarshal(raw, &t); err != nil {
			panic(fmt.Sprintf("parse %s texts: %v", lang, err))
		}
		all[lang] = t
	}

	return all
}

// textsIn returns the texts of a language; an unknown language gets English.
func textsIn(lang string) pageTexts {
	if t, ok := texts[lang]; ok {
		return t
	}

	return texts["en"]
}

// fill puts the module name and version into a text.
func fill(s, module, version string) string {
	return strings.NewReplacer("{module}", module, "{version}", version).Replace(s)
}
