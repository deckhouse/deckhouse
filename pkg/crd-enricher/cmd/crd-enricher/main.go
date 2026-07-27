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

// Command crd-enricher enriches controller-gen output with custom x-doc-*
// schema fields. It mirrors the controller-gen invocation contract:
//
//	crd-enricher paths="./pkg/apis/..." crds=bin/crd/bases
//
// where "paths" selects the Go packages with the API structs and "crds" (or
// the controller-gen style "output:crd:artifacts:config") points at the
// directory with the CRD YAML files to enrich in place.
package main

import (
	"fmt"
	"os"
	"strings"

	crdenricher "github.com/deckhouse/deckhouse/pkg/crd-enricher"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	var opts crdenricher.Options

	for _, arg := range args {
		switch {
		case arg == "-h", arg == "--help", arg == "help":
			usage()
			return nil

		case strings.HasPrefix(arg, "paths="):
			value := trimQuotes(strings.TrimPrefix(arg, "paths="))
			for _, p := range strings.Split(value, ",") {
				if p = strings.TrimSpace(p); p != "" {
					opts.Paths = append(opts.Paths, p)
				}
			}

		case strings.HasPrefix(arg, "crds="):
			opts.CRDDir = trimQuotes(strings.TrimPrefix(arg, "crds="))

		// Accept the controller-gen style output rule as an alias so the same
		// argument can be reused for both tools.
		case strings.HasPrefix(arg, "output:crd:artifacts:config="):
			opts.CRDDir = trimQuotes(strings.TrimPrefix(arg, "output:crd:artifacts:config="))

		case strings.HasPrefix(arg, "dir="):
			opts.Dir = trimQuotes(strings.TrimPrefix(arg, "dir="))

		default:
			return fmt.Errorf("unknown argument %q", arg)
		}
	}

	changed, err := crdenricher.Run(opts)
	if err != nil {
		return err
	}

	for _, file := range changed {
		fmt.Printf("enriched %s\n", file)
	}
	if len(changed) == 0 {
		fmt.Println("no CRDs required enrichment")
	}

	return nil
}

func trimQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func usage() {
	fmt.Print(`crd-enricher enriches controller-gen CRDs with custom x-doc-* schema fields.

Usage:
  crd-enricher paths=<go-packages> crds=<crd-dir> [dir=<workdir>]

Arguments:
  paths=   Comma separated Go package patterns with the API structs (repeatable).
  crds=    Directory with the CRD YAML files produced by controller-gen.
           The controller-gen alias output:crd:artifacts:config=<dir> is accepted too.
  dir=     Optional working directory used to resolve the package patterns.

Example:
  crd-enricher paths="./deckhouse-controller/pkg/apis/deckhouse.io/..." crds=bin/crd/bases
`)
}
