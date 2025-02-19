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

package requirements

import (
	"errors"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

const (
	CheckCloudProviderConfigRaw = "checkCloudProviderConfigRaw"
	CheckCloudProviderConfig    = "checkCloudProviderConfig"
)

func init() {
	requirements.RegisterCheck(CheckCloudProviderConfig, func(_ string, getter requirements.ValueGetter) (bool, error) {
		key, exists := getter.Get(CheckCloudProviderConfigRaw)
		if exists {
			if key.(bool) {
				return false, errors.New("The provider-cluster-configuration secret in the cluster contains fields outside the schema. Remove it from provider-cluster-configuration")
			}
		}
		return true, nil
	})
}
