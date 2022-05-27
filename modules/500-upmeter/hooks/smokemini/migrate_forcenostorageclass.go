// Copyright 2021 Flant JSC
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

package smokemini

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

// Enforce upmeter.smokeMini.storageClass = false by deleting it (false is the default in the config schema)
// TODO: Delete this hook in Deckhouse v1.34
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
}, func(input *go_hook.HookInput) error {
	const (
		smokeminiKey    = "upmeter.smokeMini"
		storageClassKey = smokeminiKey + ".storageClass"
	)

	if !input.ConfigValues.Exists(storageClassKey) {
		return nil
	}

	// Clean empty object in config
	v := input.ConfigValues.Get(smokeminiKey)
	if len(v.Map()) <= 1 {
		// Only storage can be there
		input.ConfigValues.Remove(smokeminiKey)
	} else {
		// Remove the explicit storage class settings
		input.ConfigValues.Remove(storageClassKey)
	}

	return nil
})
