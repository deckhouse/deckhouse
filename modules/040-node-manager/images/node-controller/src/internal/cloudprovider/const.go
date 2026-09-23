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

package cloudprovider

import (
	"slices"
	"strings"

	"github.com/deckhouse/node-controller/internal/common"
)

// The registration Secret a provider module publishes.
const (
	RegistrationSecretNamespace = common.CloudProviderSecretNamespace
	RegistrationSecretLabel     = common.CloudProviderRegistrationLabel
	RegistrationSecretBaseName  = common.CloudProviderSecretName
)

// Keys of the registration Secret that callers outside this package name.
const (
	InstanceClassKindKey       = common.InstanceClassKindKey
	InstanceClassAPIVersionKey = common.InstanceClassAPIVersionKey
)

func isStatic(pType string) bool {
	return slices.Contains([]string{"none", ""}, strings.ToLower(pType))
}
