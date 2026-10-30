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

package hooks

import (
	"fmt"

	"k8s.io/client-go/dynamic"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/modules/150-user-authn/hooks/internal/handover"
)

// Objects the chart used to render and user-authn-controller owns now; see package handover.
// A kind moved to the controller is added here. The hook acting on the list lives in
// handover_protect.go: addon-operator names a hook after its file, so a second hook registered
// in one file replaces the first.
var handoverKinds = []handover.Kind{
	dexClientSecrets,
	dexClientOAuth2Clients,
	kubernetesOAuth2Client,
	dexPasswords,
}

const handoverQueue = "/modules/user-authn/handover"

// newHandoverClient is overridable in tests.
var newHandoverClient = func(dc dependency.Container) (dynamic.Interface, error) {
	config, err := dc.GetClientConfig()
	if err != nil {
		return nil, fmt.Errorf("get client config: %w", err)
	}
	return dynamic.NewForConfig(config)
}
