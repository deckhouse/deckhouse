/*
Copyright 2024 Flant JSC

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

package template

import (
	"fmt"
	"strings"
)

// Parses resource name for nodegroup bundles that is expected to be of form {os}.{target} with hyphens as delimiters,
// e.g.
//
//	`ubuntu-lts.master`  for nodegroup bundles
func ParseName(name string) (string, string, error) {
	parts := strings.Split(name, ".")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("name: %q must comply with format {os}.{target} using hyphens as innner delimiters", name)
	}

	os, target := parts[0], parts[1]

	return os, target, nil
}

// GetNodegroupContextKey parses context secretKey for nodegroup bundles
func GetNodegroupContextKey(name string) (string, error) {
	os, ng, err := ParseName(name)
	if err != nil {
		return "", fmt.Errorf("bad os name: %v", err)
	}
	return fmt.Sprintf("bundle-%s-%s", os, ng), nil
}

// GetBashibleContextKey parses context secretKey bashible
func GetBashibleContextKey(name string) (string, error) {
	os, nodegroup, err := ParseName(name)
	if err != nil {
		return "", fmt.Errorf("bad bashible name: %v", err)
	}
	return fmt.Sprintf("bashible-%s-%s", os, nodegroup), nil
}

// GetBootstrapContextKey parses context secretKey bootstrap
func GetBootstrapContextKey(nodegroup string) (string, error) {
	return fmt.Sprintf("bashible-common-%s", nodegroup), nil
}
