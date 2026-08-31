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

// Command aiexport builds the AI export — the per-page Markdown, external-llms.txt and
// external-corpus.json — from an already rendered Hugo site.
//
// In production this is a step of the docs-builder build, see
// `internal/docs.Service.Build`. Locally docs-builder is not run at all (it is
// an HTTP service and it wants a Kubernetes API), so `make ai-export` renders
// the site with plain Hugo and then calls this command over the resulting
// `public` directory.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/flant/docs-builder/internal/aiexport"
)

func main() {
	publicDir := flag.String("public", "public", "Directory with the site rendered by Hugo")
	langs := flag.String("langs", "en,ru", "Comma-separated list of languages to export")
	verbose := flag.Bool("v", false, "Log every skipped and untitled page")
	flag.Parse()

	// The same lines the build writes, but on stderr, so that the paths this
	// command prints on stdout stay usable in a pipe.
	level := log.LevelInfo
	if *verbose {
		level = log.LevelDebug
	}

	logger := log.NewLogger(
		log.WithLevel(level.Level()),
		log.WithHandlerType(log.TextHandlerType),
		log.WithOutput(os.Stderr),
	)

	failed := false

	for _, lang := range strings.Split(*langs, ",") {
		lang = strings.TrimSpace(lang)
		if lang == "" {
			continue
		}

		manifest := filepath.Join(*publicDir, lang, "ai", "ai.json")
		if _, err := os.Stat(manifest); err != nil {
			// A missing manifest makes Export a no-op: inside docs-builder a
			// broken AI export must not fail the whole site build. Run by
			// hand the tradeoff is the opposite — it means Hugo has not run
			// yet, and silence would be indistinguishable from success.
			fmt.Fprintf(os.Stderr, "%s: no manifest at %s, render the site with Hugo first\n", lang, manifest)
			failed = true

			continue
		}

		if err := aiexport.Export(*publicDir, lang, logger); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", lang, err)
			failed = true

			continue
		}

		fmt.Printf("%s: %s\n", lang, filepath.Join(*publicDir, lang, "modules", "external-llms.txt"))
	}

	if failed {
		os.Exit(1)
	}
}
