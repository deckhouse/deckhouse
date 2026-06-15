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

import cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"

func getManagedCredentialSecrets(state *State) []cpapi.CredentialSecret {
	secrets := make([]cpapi.CredentialSecret, 0, len(state.CredentialSecrets))
	for _, secret := range state.CredentialSecrets {
		if secret.Namespace != "" && secret.Namespace != state.NamespaceName {
			continue
		}

		if !secret.IsManaged() {
			continue
		}

		secrets = append(secrets, secret)
	}

	return secrets
}

func findCredentialSecret(state *State, name string) (cpapi.CredentialSecret, bool) {
	for _, secret := range state.CredentialSecrets {
		if secret.Name != name {
			continue
		}

		if secret.Namespace != "" && secret.Namespace != state.NamespaceName {
			continue
		}

		return secret, true
	}

	return cpapi.CredentialSecret{}, false
}

func findNodeGroup(state *State, name string) (cpapi.NodeGroup, bool) {
	for _, nodeGroup := range state.NodeGroups {
		if nodeGroup.Name == name {
			return nodeGroup, true
		}
	}

	return cpapi.NodeGroup{}, false
}

func findInstanceClass(state *State, name string) (cpapi.InstanceClass, bool) {
	for _, class := range state.InstanceClasses {
		if class.Name == name {
			return class, true
		}
	}

	return cpapi.InstanceClass{}, false
}
