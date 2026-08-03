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

package phases

import (
	"embed"
	"fmt"

	"gopkg.in/yaml.v3"
)

//go:embed i18n/*.yaml
var TitlesFS embed.FS

type Language string

const (
	ENLanguage Language = "en"
	RULanguage Language = "ru"
)

var Languages = []Language{ENLanguage, RULanguage}

// Titles stores all phase and subphase titles.
// It is immutable after LoadTitles returns.
type Titles struct {
	phase    map[Language]map[OperationPhase]string
	subPhase map[Language]map[OperationSubPhase]string
}

func LoadTitles() (*Titles, error) {
	ret := &Titles{
		phase:    make(map[Language]map[OperationPhase]string),
		subPhase: make(map[Language]map[OperationSubPhase]string),
	}

	for _, lang := range Languages {
		titles := make(map[OperationPhase]string)
		path := fmt.Sprintf("i18n/phases.%s.yaml", lang)

		if err := readYamlFile(path, &titles); err != nil {
			return nil, fmt.Errorf("load phase titles: %w", err)
		}
		ret.phase[lang] = titles
	}

	for _, lang := range Languages {
		titles := make(map[OperationSubPhase]string)
		path := fmt.Sprintf("i18n/subphases.%s.yaml", lang)

		if err := readYamlFile(path, &titles); err != nil {
			return nil, fmt.Errorf("load subphase titles: %w", err)
		}
		ret.subPhase[lang] = titles
	}

	return ret, nil
}

// Phase returns the English title of a phase by its code.
func (t *Titles) Phase(phase OperationPhase) string {
	return t.phase[ENLanguage][phase]
}

// SubPhase returns the English title of a subphase by its code.
func (t *Titles) SubPhase(subPhase OperationSubPhase) string {
	return t.subPhase[ENLanguage][subPhase]
}

// TitlesCatalog represents the transport-neutral representation of the full set
// of titles. It is consumed by both the gRPC GetPhaseCatalog handler and the
// phase-catalog CLI command. Keeping output generation in one place prevents
// the two transports from diverging.
type TitlesCatalog struct {
	Phases    map[string]map[string]string `json:"phases"`
	SubPhases map[string]map[string]string `json:"subPhases"`
}

// ToCatalog returns the titles as code -> locale -> title maps.
// Both namespaces are kept separate because phase and subphase codes overlap
// by string value (e.g. "InstallDeckhouse", "BaseInfra", "Check") with
// different titles. The returned maps are defensive copies.
func (t *Titles) ToCatalog() TitlesCatalog {
	return TitlesCatalog{
		Phases:    mapToCatalog(t.phase),
		SubPhases: mapToCatalog(t.subPhase),
	}
}

// mapToCatalog converts a map[Language]map[K]string to map[string]map[string]string.
// It supports any key type that has string as its underlying type.
func mapToCatalog[K ~string](in map[Language]map[K]string) map[string]map[string]string {
	if in == nil {
		return nil
	}

	ret := make(map[string]map[string]string)

	for lang, titles := range in {
		for code, title := range titles {
			codeStr := string(code)
			langStr := string(lang)

			if ret[codeStr] == nil {
				ret[codeStr] = make(map[string]string)
			}
			ret[codeStr][langStr] = title
		}
	}

	return ret
}

// readYamlFile reads and unmarshals a title file directly into the provided map.
func readYamlFile(path string, target any) error {
	data, err := TitlesFS.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, target); err != nil {
		return fmt.Errorf("parse file %s: %w", path, err)
	}
	return nil
}
