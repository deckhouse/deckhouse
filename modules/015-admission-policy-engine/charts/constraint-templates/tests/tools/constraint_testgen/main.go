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

// constraint_testgen: verify ConstraintTestProfile vs test_suite; generate from ConstraintTestMatrix.
package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "verify":
		root := defaultTestsRoot()
		for i := 2; i < len(os.Args); i++ {
			if os.Args[i] == "-tests-root" && i+1 < len(os.Args) {
				root = os.Args[i+1]
				i++
			}
		}
		if err := verify(root); err != nil {
			fmt.Fprintf(os.Stderr, "constraint test profiles: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("constraint test profiles: OK")
	case "generate":
		var (
			testsRoot  = defaultTestsRoot()
			bundlePath string
		)
		for i := 2; i < len(os.Args); i++ {
			switch os.Args[i] {
			case "-tests-root":
				if i+1 < len(os.Args) {
					testsRoot = os.Args[i+1]
					i++
				}
			case "-bundle":
				if i+1 < len(os.Args) {
					bundlePath = os.Args[i+1]
					i++
				}
			}
		}
		if bundlePath == "" {
			fmt.Fprintf(os.Stderr, "generate requires -bundle <test-matrix.yaml>\n")
			usage()
			os.Exit(2)
		}
		if !filepath.IsAbs(bundlePath) {
			if cwd, e := os.Getwd(); e == nil {
				bundlePath = filepath.Clean(filepath.Join(cwd, bundlePath))
			}
		}
		if err := generateFromInputFile(bundlePath, testsRoot); err != nil {
			fmt.Fprintf(os.Stderr, "constraint_testgen generate: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("constraint_testgen generate: OK")
	case "coverage":
		var (
			testsRoot  = defaultTestsRoot()
			format     = "table"
			constraint string
		)
		for i := 2; i < len(os.Args); i++ {
			switch os.Args[i] {
			case "-tests-root":
				if i+1 < len(os.Args) {
					testsRoot = os.Args[i+1]
					i++
				}
			case "-format":
				if i+1 < len(os.Args) {
					format = os.Args[i+1]
					i++
				}
			case "-constraint":
				if i+1 < len(os.Args) {
					constraint = os.Args[i+1]
					i++
				}
			}
		}
		if err := runCoverage(testsRoot, format, constraint); err != nil {
			fmt.Fprintf(os.Stderr, "constraint_testgen coverage: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("constraint_testgen coverage: OK")
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	b := filepath.Base(os.Args[0])
	fmt.Fprintf(os.Stderr, `usage:
	 %s verify [-tests-root <path>]
	 %s generate -bundle <test-matrix.yaml> [-tests-root <path>]
	 %s coverage [-tests-root <path>] [-constraint <name|path>] [-format table|json|markdown]
`, b, b, b)
}

func defaultTestsRoot() (dir string) {
	cwd, err := os.Getwd()
	if err != nil {
		return "charts/constraint-templates/tests"
	}
	candidates := []string{
		filepath.Join(cwd, "charts", "constraint-templates", "tests"),
		filepath.Join(cwd, "..", "charts", "constraint-templates", "tests"),
		// cwd is tools/constraint_testgen when running `go run` from that directory
		filepath.Join(cwd, "..", "..", "charts", "constraint-templates", "tests"),
	}
	for _, c := range candidates {
		if st, e := os.Stat(c); e == nil && st.IsDir() {
			return c
		}
	}
	return filepath.Join(cwd, "charts", "constraint-templates", "tests")
}
