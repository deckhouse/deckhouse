/*
Copyright 2026 Flant JSC

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
	"regexp"
	"strings"
)

// The cloud-config of a node is assembled by dhctl, and cloud-init keeps one users key per
// document: a second one silently drops the first, taking the account converge logs in
// with. A provider declares the account its image needs in candi/node-users.yml instead.
var providerTerraformRe = regexp.MustCompile(`/candi/(terraform-modules|layouts)/.+\.(tf|tftpl|tpl)$`)

var (
	usersKeyRe      = regexp.MustCompile(`(^|[^\w-])"users"\s*[:=]|^\s*-?\s*users:\s*($|[\[&*#])`)
	payloadDecodeRe = regexp.MustCompile(`(yamldecode|jsondecode)\s*\(\s*base64decode\s*\(\s*var\.(cloudConfig|cloud_config)`)
)

var skipProviderCloudConfigSelfRe = regexp.MustCompile(`provider_cloud_config(_test)?\.go$`)

const providerCloudConfigHint = `the cloud-config of a node is assembled by dhctl.
    The account your image needs goes to candi/node-users.yml, next to cni-bootstrap.yml.
    Other keys — hostname, ssh_* — may still be appended after the payload; never "users",
    and never decode the payload: cloud-init keeps one users key per document.`

func RunProviderCloudConfigValidation(info *DiffInfo) (exitCode int) {
	fmt.Printf("Run 'provider cloud-config' validation ...\n")

	if len(info.Files) == 0 {
		fmt.Printf("OK, diff is empty\n")
		return 0
	}

	msgs := NewMessages()

	for _, fileInfo := range info.Files {
		if !fileInfo.HasContent() {
			continue
		}

		if !fileInfo.IsAdded() && !fileInfo.IsModified() {
			continue
		}

		fileName := fileInfo.NewFileName

		if skipProviderCloudConfigSelfRe.MatchString(fileName) {
			msgs.Add(NewSkip(fileName, "self"))
			continue
		}

		if !providerTerraformRe.MatchString(fileName) {
			continue
		}

		found := findCloudConfigOwnership(fileInfo.NewLines())
		if found != "" {
			msgs.Add(NewError(fileName, "must not take over the cloud-config users key", found))
			continue
		}

		msgs.Add(NewOK(fileName))
	}

	msgs.PrintReport()

	if msgs.CountErrors() > 0 {
		return 1
	}

	return 0
}

func findCloudConfigOwnership(lines []string) string {
	var found []string

	for _, line := range lines {
		switch {
		case usersKeyRe.MatchString(line):
			found = append(found, fmt.Sprintf("  %s\n    %s", strings.TrimSpace(line), providerCloudConfigHint))
		case payloadDecodeRe.MatchString(line):
			found = append(found, fmt.Sprintf("  %s\n    %s", strings.TrimSpace(line), providerCloudConfigHint))
		}
	}

	return strings.Join(found, "\n")
}
