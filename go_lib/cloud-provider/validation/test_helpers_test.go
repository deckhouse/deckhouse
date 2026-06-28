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

package validation

import (
	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

func credentialContentState(secrets []cpapi.CredentialSecret) *State {
	return &State{
		NamespaceName:     "d8-cloud-provider-test",
		CredentialSecrets: secrets,
	}
}

func instanceClassState(kind string, nodeGroups []cpapi.NodeGroup, classes []cpapi.InstanceClass) *State {
	return &State{
		InstanceClassKind: kind,
		NodeGroups:        nodeGroups,
		InstanceClasses:   classes,
	}
}

func hasViolationCode(result Result, code string) bool {
	for _, violation := range result.Errors() {
		if violation.Code == code {
			return true
		}
	}
	return false
}
