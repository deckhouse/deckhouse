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
	"fmt"
	"os"
	"regexp"
	"strings"
)

var (
	resourceFileRe = regexp.MustCompile(`openapi/config-values.y[a]?ml$|crds/.+.y[a]?ml$|openapi/cluster_configuration.y[a]?ml$|openapi/instance_class.y[a]?ml$|openapi/node_group.y[a]?ml$`)
	docFileRe      = regexp.MustCompile(`\.md$`)

	excludeFileRe = regexp.MustCompile("crds/native/.+.y[a]?ml$")
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
	return checkRelatedFileExists(fName, otherFileName, diffInfo)
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
