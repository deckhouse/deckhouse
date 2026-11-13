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

package docs

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// SummaryFile is the file next to the pages of a rendered version with its
// Summary.
const SummaryFile = "module.json"

// Summary is what the list of modules shows of a module version: the title
// and the description per language, as the list page took them from the
// pages of the module, and the lifecycle stage.
type Summary struct {
	Title       map[string]string `json:"title,omitempty"`
	Description map[string]string `json:"description,omitempty"`
	Stage       string            `json:"stage,omitempty"`
}

// ReadSummary reads the summary of the rendered version in dir. A version
// rendered before summaries existed has none: the error is fs.ErrNotExist.
func ReadSummary(dir string) (Summary, error) {
	var s Summary
	raw, err := os.ReadFile(filepath.Join(dir, SummaryFile))
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		return s, fmt.Errorf("parse %s: %w", SummaryFile, err)
	}

	return s, nil
}

// writeSummary takes the summary from the unpacked documentation of the
// version in siteDir: the title is menuTitle, linkTitle or title of the
// README, the description is descriptions.<lang> of module.yaml or else the
// description of the README. A broken module.yaml or front matter only
// loses its part of the summary, with a warning.
func writeSummary(siteDir, outDir, module, version string, logger *log.Logger) error {
	var meta struct {
		Stage        string            `yaml:"stage"`
		Descriptions map[string]string `yaml:"descriptions"`
	}
	raw, err := os.ReadFile(filepath.Join(siteDir, modulesDir, module, version, "module.yaml"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read module.yaml: %w", err)
	}
	if err := yaml.Unmarshal(raw, &meta); err != nil {
		logger.Warn("module summary: parse module.yaml", slog.String("module", module), slog.String("version", version), log.Err(err))
	}

	s := Summary{Title: map[string]string{}, Description: map[string]string{}, Stage: meta.Stage}
	for _, lang := range Languages {
		name := "README.md"
		if lang != "en" {
			name = "README." + lang + ".md"
		}
		front, err := readFrontMatter(filepath.Join(siteDir, contentDir, module, version, name))
		if err != nil {
			logger.Warn("module summary: read the front matter of "+name, slog.String("module", module), slog.String("version", version), log.Err(err))
		}
		if title := cmp.Or(front.MenuTitle, front.LinkTitle, front.Title); title != "" {
			s.Title[lang] = title
		}
		if description := cmp.Or(meta.Descriptions[lang], front.Description); description != "" {
			s.Description[lang] = description
		}
	}

	raw, err = json.Marshal(s)
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(outDir, SummaryFile), raw, 0600)
}

type frontMatter struct {
	Title       string `yaml:"title"`
	LinkTitle   string `yaml:"linkTitle"`
	MenuTitle   string `yaml:"menuTitle"`
	Description string `yaml:"description"`
}

// readFrontMatter reads the YAML front matter of a Markdown page; a page
// without one, or that is missing, gives an empty one.
func readFrontMatter(path string) (frontMatter, error) {
	var front frontMatter
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return front, nil
	}
	if err != nil {
		return front, err
	}
	rest, ok := bytes.CutPrefix(bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf")), []byte("---"))
	if !ok {
		return front, nil
	}
	if end := bytes.Index(rest, []byte("\n---")); end >= 0 {
		if err := yaml.Unmarshal(rest[:end], &front); err != nil {
			return frontMatter{}, fmt.Errorf("parse: %w", err)
		}
	}

	return front, nil
}
