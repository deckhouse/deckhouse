/*
Copyright 2021 Flant JSC

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
	"flag"
	"fmt"
	"os"
	"os/exec"
)

func main() {
	var validationType string
	flag.StringVar(&validationType, "type", "", "Validation type: copyright, cyrillic or doc-changes.")
	var patchFile string
	flag.StringVar(&patchFile, "file", "", "Patch file. git diff is executed if not passed.")
	var title string
	flag.StringVar(&title, "title", "", "Title string to check for cyrillic letters.")
	var description string
	flag.StringVar(&description, "description", "", "Description string to check for cyrillic letters.")
	flag.Parse()

	var diffInfo *DiffInfo
	var err error
	if patchFile != "" {
		// Parse file content.
		diffInfo, err = readFile(patchFile)
		if err != nil {
			fmt.Printf("Read file '%s': %v", patchFile, err)
			os.Exit(1)
		}
	} else {
		// Parse 'git diff' output.
		fmt.Printf("Run git diff ...\n")
		diffInfo, err = executeGitDiff()
		if err != nil {
			fmt.Printf("Execute git diff: %v", err)
			os.Exit(1)
		}
	}

	exitCode := 0
	switch validationType {
	case "copyright":
		exitCode = RunCopyrightValidation(diffInfo)
	case "no-cyrillic":
		exitCode = RunNoCyrillicValidation(diffInfo, title, description)
	case "doc-changes":
		exitCode = RunDocChangesValidation(diffInfo)
	case "grafana-dashboard":
		exitCode = RunGrafanaDashboardValidation(diffInfo)
	case "dump":
		fmt.Printf("%s\n", diffInfo.Dump())
	default:
		fmt.Printf("Unknown validation type '%s'\n", validationType)
		os.Exit(2)
	}

	if exitCode == 0 {
		fmt.Printf("Validation successful.\n")
	} else {
		fmt.Printf("Validation failed.\n")
	}
	os.Exit(exitCode)
}

func readFile(fName string) (*DiffInfo, error) {
	content, err := os.ReadFile(fName)
	if err != nil {
		return nil, err
	}

	br := bytes.NewReader(content)
	return ParseDiffOutput(br)
}

func executeGitDiff() (*DiffInfo, error) {
	gitCmd := exec.Command("git", "diff", "origin/main...", "-w", "--ignore-blank-lines")
	out, err := gitCmd.Output()
	if err != nil {
		return nil, err
	}

	br := bytes.NewReader(out)
	return ParseDiffOutput(br)
}
