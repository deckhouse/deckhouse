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
var titlesFS embed.FS

type Locale string

const (
	ENLocale Locale = "en"
	RULocale Locale = "ru"
)

func AllLocales() []Locale {
	return []Locale{ENLocale, RULocale}
}

// Titles stores all phase and subphase titles.
type Titles struct {
	phase    map[Locale]map[OperationPhase]string
	subPhase map[Locale]map[OperationSubPhase]string
}

func LoadTitles() (*Titles, error) {
	ret := &Titles{
		phase:    make(map[Locale]map[OperationPhase]string),
		subPhase: make(map[Locale]map[OperationSubPhase]string),
	}

	for _, locale := range AllLocales() {
		titles := make(map[OperationPhase]string)
		path := fmt.Sprintf("i18n/phases.%s.yaml", locale)

		if err := readYamlFile(path, &titles); err != nil {
			return nil, fmt.Errorf("load phase titles: %w", err)
		}
		ret.phase[locale] = titles
	}

	for _, locale := range AllLocales() {
		titles := make(map[OperationSubPhase]string)
		path := fmt.Sprintf("i18n/subphases.%s.yaml", locale)

		if err := readYamlFile(path, &titles); err != nil {
			return nil, fmt.Errorf("load subphase titles: %w", err)
		}
		ret.subPhase[locale] = titles
	}

	return ret, nil
}

// Phase returns the English title of a phase by its code.
func (t *Titles) Phase(phase OperationPhase) string {
	return t.phase[ENLocale][phase]
}

// SubPhase returns the English title of a subphase by its code.
func (t *Titles) SubPhase(subPhase OperationSubPhase) string {
	return t.subPhase[ENLocale][subPhase]
}

// TitlesCatalog represents the transport-neutral representation of the full set
// of titles.
type TitlesCatalog struct {
	Phases    map[string]LocaleTitles `json:"phases"`
	SubPhases map[string]LocaleTitles `json:"subPhases"`
}

type LocaleTitles struct {
	ByLocale map[string]string `json:"byLocale"`
}

// ToCatalog returns the titles as code -> LocaleTitles maps.
// Both namespaces are kept separate because phase and subphase codes overlap
// by string value (e.g. "InstallDeckhouse", "BaseInfra", "Check") with
// different titles. The returned maps are defensive copies.
func (t *Titles) ToCatalog() TitlesCatalog {
	return TitlesCatalog{
		Phases:    mapToCatalog(t.phase),
		SubPhases: mapToCatalog(t.subPhase),
	}
}

// mapToCatalog converts a map[Locale]map[K]string to map[string]LocaleTitles.
// It supports any key type that has string as its underlying type.
func mapToCatalog[K ~string](in map[Locale]map[K]string) map[string]LocaleTitles {
	if in == nil {
		return nil
	}

	ret := make(map[string]LocaleTitles)

	for locale, titles := range in {
		for code, title := range titles {
			codeStr := string(code)
			localeStr := string(locale)

			lt, ok := ret[codeStr]
			if !ok {
				lt = LocaleTitles{ByLocale: make(map[string]string)}
				ret[codeStr] = lt
			}
			lt.ByLocale[localeStr] = title
		}
	}

	return ret
}

// readYamlFile reads and unmarshals a title file directly into the provided map.
func readYamlFile(path string, target any) error {
	data, err := titlesFS.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, target); err != nil {
		return fmt.Errorf("parse file %s: %w", path, err)
	}
	return nil
}
