// Copyright 2024 Flant JSC
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

package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
}, removeSecretGrfana)

type Secret struct {
	apiVersion string
	kind       string
	namespace  string
	name       string
}

func removeSecretGrfana(input *go_hook.HookInput) error {
	secret := Secret{
		apiVersion: "v1",
		kind:       "Secret",
		namespace:  "d8-monitoring",
		name:       "ingress-tls-v10",
	}

	input.PatchCollector.Delete(secret.apiVersion, secret.kind, secret.namespace, secret.name)

	return nil
}
