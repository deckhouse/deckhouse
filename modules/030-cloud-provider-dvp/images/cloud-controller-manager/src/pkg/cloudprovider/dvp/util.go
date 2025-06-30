/*
Copyright 2025 Flant JSC

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

package dvp

import (
	"fmt"
	"regexp"

	"k8s.io/apimachinery/pkg/types"
)

var regExpProviderID = regexp.MustCompile(`^` + providerName + `://(.+)$`)

func MapNodeNameToVMName(nodeName types.NodeName) string {
	return string(nodeName)
}

func ParseProviderID(providerID string) (string, error) {
	matches := regExpProviderID.FindStringSubmatch(providerID)
	if len(matches) == 2 {
		return matches[1], nil
	}

	return "", fmt.Errorf("can't parse providerID %q", providerID)
}
