/*
Copyright 2023 Flant JSC

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

// Don't forget to add "embeddedVirtualizationMustBeDisabled": "true" to release.yaml

package requirements

import (
	"errors"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

const embeddedVirtualizationEnabled = "embeddedVirtualization:enabled"
const requirementsKey = "embeddedVirtualizationMustBeDisabled"

func init() {
	f := func(_ string, getter requirements.ValueGetter) (bool, error) {
		if _, ok := getter.Get(embeddedVirtualizationEnabled); ok {
			return false, errors.New("embedded virtualization module must be disabled")
		}
		return true, nil
	}

	requirements.RegisterCheck(requirementsKey, f)
}
