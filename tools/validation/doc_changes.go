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
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var (
	resourceFileRe = regexp.MustCompile(`openapi/config-values.y[a]?ml$|crds/.+.y[a]?ml$|openapi/cluster_configuration.y[a]?ml$|openapi/instance_class.y[a]?ml$|openapi/node_group.y[a]?ml$`)
	docFileRe      = regexp.MustCompile(`\.md$`)

	excludeFileRe = regexp.MustCompile("crds/(gatekeeper|native|ratify|cert-manager|external)/.+.y[a]?ml$")

	moduleIncludeScriptRe = regexp.MustCompile(`(?s)<script\s+type=["']application/x-module-include["']\s*>(.*?)</script>`)
	moduleIncludeModuleRe = regexp.MustCompile(`^[a-z0-9-]+$`)
	moduleIncludeArtifactRe = regexp.MustCompile(`^[a-z0-9-][a-z0-9-./]*\.md$`)
)

func RunDocChangesValidation(info *DiffInfo) (exitCode int) {
	fmt.Printf("Run 'doc changes' validation ...\n")

	if len(info.Files) == 0 {
		fmt.Printf("Nothing to validate, diff is empty\n")
		return 0
	}

	exitCode = 0
	msgs := NewMessages()
	for _, fileInfo := range info.Files {
		if !fileInfo.HasContent() {
			continue
		}

		fileName := fileInfo.NewFileName

		if strings.Contains(fileName, "testdata") {
			msgs.Add(NewSkip(fileName, ""))
			continue
		}

		if docFileRe.MatchString(fileName) {
			msgs.Add(checkDocFile(fileName, info))
			continue
		}

		if resourceFileRe.MatchString(fileName) && !excludeFileRe.MatchString(fileName) {
			msgs.Add(checkResourceFile(fileName, info))
			continue
		}

		msgs.Add(NewSkip(fileName, ""))
	}
	msgs.PrintReport()

	if msgs.CountErrors() > 0 {
		exitCode = 1
	}

	return exitCode
}

var possibleDocRootsRe = regexp.MustCompile(`modules/[^/]+/docs/|docs/(site|documentation)/pages/`)
var excludedDocPathsRe = regexp.MustCompile(`docs/site/pages/(stronghold|code|virtualization-platform)`)
var docsDirAllowedFileRe = regexp.MustCompile(`modules/[^/]+/docs/(CLUSTER_CONFIGURATION|CONFIGURATION|CR|ISTIO-CR|FAQ|README|USAGE|EXAMPLES|ADVANCED_USAGE)(_RU)?.md`)
var docsDirFileRe = regexp.MustCompile(`/docs/[^/]+.md`)

func checkDocFile(fName string, diffInfo *DiffInfo) (msg Message) {
	if !possibleDocRootsRe.MatchString(fName) {
		return NewSkip(fName, "")
	}

	// Exclude specific paths
	if excludedDocPathsRe.MatchString(fName) {
		return NewSkip(fName, "")
	}

	if docsDirFileRe.MatchString(fName) && !docsDirAllowedFileRe.MatchString(fName) {
		return NewError(
			fName,
			"name is not allowed",
			`Rename this file or move it, for example, into 'internal' folder.
Only following file names are allowed in the module '/docs/' directory:
    CLUSTER_CONFIGURATION.md
    CONFIGURATION.md
    CR.md
    ISTIO-CR.md
    FAQ.md
    README.md
    USAGE.md
    EXAMPLES.md
		ADVANCED_USAGE.md
(also their Russian versions ended with '_RU.md')`,
		)
	}

	// Check if documentation for other language file is also modified.
	var otherFileName = fName
	if strings.HasSuffix(fName, `_RU.md`) {
		otherFileName = strings.TrimSuffix(fName, "_RU.md") + ".md"
	} else {
		otherFileName = strings.TrimSuffix(fName, ".md") + "_RU.md"
	}
	msg = checkRelatedFileExists(fName, otherFileName, diffInfo)
	if msg.IsError() {
		return msg
	}

	return checkModuleIncludePlaceholders(fName)
}

var docRuResourceRe = regexp.MustCompile(`doc-ru-.+.y[a]?ml$`)
var notDocRuResourceRe = regexp.MustCompile(`([^/]+\.y[a]?ml)$`)

// Check if resource for other language is also modified.
func checkResourceFile(fName string, diffInfo *DiffInfo) (msg Message) {
	otherFileName := fName
	if docRuResourceRe.MatchString(fName) {
		otherFileName = strings.Replace(fName, "doc-ru-", "", 1)
	} else {
		otherFileName = notDocRuResourceRe.ReplaceAllString(fName, `doc-ru-$1`)
	}
	return checkRelatedFileExists(fName, otherFileName, diffInfo)
}

func checkRelatedFileExists(origName string, otherName string, diffInfo *DiffInfo) Message {
	file, err := os.Open(otherName)
	if err != nil {
		return NewError(origName, "related is absent", fmt.Sprintf(`Documentation or resource file is changed
while related language file '%s' is absent.`, otherName))
	}
	defer file.Close()

	for _, fileInfo := range diffInfo.Files {
		if fileInfo.NewFileName == otherName {
			return NewOK(origName)
		}
	}
	return NewError(origName, "related not changed", fmt.Sprintf(`Documentation or resource file is changed
while related language file '%s' is not changed`, otherName))
}

type moduleIncludePlaceholder struct {
	Module   string `json:"module"`
	Channel  string `json:"channel"`
	Artifact string `json:"artifact"`
	OnError  string `json:"onError"`
	Fallback string `json:"fallback"`
}

var allowedModuleIncludeChannels = map[string]struct{}{
	"alpha":        {},
	"beta":         {},
	"early-access": {},
	"stable":       {},
	"rock-solid":   {},
	"latest":       {},
}

var allowedModuleIncludeOnError = map[string]struct{}{
	"":         {},
	"skip":     {},
	"fallback": {},
}

func checkModuleIncludePlaceholders(fileName string) Message {
	content, err := os.ReadFile(fileName)
	if err != nil {
		return NewError(fileName, "cannot read file", err.Error())
	}

	errors := validateModuleIncludePlaceholders(string(content))
	if len(errors) == 0 {
		return NewOK(fileName)
	}

	return NewError(
		fileName,
		"invalid reusable content placeholder",
		strings.Join(errors, "\n"),
	)
}

func validateModuleIncludePlaceholders(content string) []string {
	matches := moduleIncludeScriptRe.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}

	errors := make([]string, 0)
	for idx, match := range matches {
		if len(match) < 2 {
			continue
		}

		placeholder := moduleIncludePlaceholder{}
		raw := strings.TrimSpace(match[1])
		if err := json.Unmarshal([]byte(raw), &placeholder); err != nil {
			errors = append(errors, fmt.Sprintf("placeholder #%d: invalid JSON: %v", idx+1, err))
			continue
		}

		if placeholder.Module == "" || !moduleIncludeModuleRe.MatchString(placeholder.Module) {
			errors = append(errors, fmt.Sprintf("placeholder #%d: module must match ^[a-z0-9-]+$", idx+1))
		}

		if placeholder.Artifact == "" || !moduleIncludeArtifactRe.MatchString(placeholder.Artifact) {
			errors = append(errors, fmt.Sprintf("placeholder #%d: artifact must match ^[a-z0-9-][a-z0-9-./]*\\.md$", idx+1))
		} else if strings.HasSuffix(placeholder.Artifact, ".ru.md") {
			errors = append(errors, fmt.Sprintf("placeholder #%d: artifact must not contain language suffix", idx+1))
		}

		channel := placeholder.Channel
		if channel == "" {
			channel = "stable"
		}
		if _, ok := allowedModuleIncludeChannels[channel]; !ok {
			errors = append(errors, fmt.Sprintf("placeholder #%d: unsupported channel %q", idx+1, placeholder.Channel))
		}

		if _, ok := allowedModuleIncludeOnError[placeholder.OnError]; !ok {
			errors = append(errors, fmt.Sprintf("placeholder #%d: unsupported onError %q", idx+1, placeholder.OnError))
		}

		if placeholder.OnError == "fallback" && strings.TrimSpace(placeholder.Fallback) == "" {
			errors = append(errors, fmt.Sprintf("placeholder #%d: fallback content is required when onError=fallback", idx+1))
		}
	}

	return errors
}
