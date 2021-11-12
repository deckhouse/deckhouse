/*
Copyright 2021 Flant JSC

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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/go_lib/module"
	"github.com/deckhouse/deckhouse/go_lib/set"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterHelm: &go_hook.OrderedConfig{Order: 10},
}, deployDexAuthenticator)

type DexAuthenticator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec DexAuthenticatorSpec `json:"spec"`
}

type DexAuthenticatorSpec struct {
	ApplicationDomain                       string   `json:"applicationDomain"`
	ApplicationIngressCertificateSecretName string   `json:"applicationIngressCertificateSecretName"`
	ApplicationIngressClassName             string   `json:"applicationIngressClassName"`
	AllowedGroups                           []string `json:"allowedGroups,omitempty"`
}

func deployDexAuthenticator(input *go_hook.HookInput) error {
	if !input.Values.Get("global.clusterIsBootstrapped").Bool() {
		return nil
	}

	if input.Values.Exists("deckhouseWeb.internal.deployDexAuthenticator") {
		allowedGroups := set.NewFromValues(input.Values, "deckhouseWeb.auth.allowedUserGroups").Slice()
		dexAuthenticator := &DexAuthenticator{
			TypeMeta: metav1.TypeMeta{
				Kind:       "DexAuthenticator",
				APIVersion: "deckhouse.io/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "deckhouse-web",
				Namespace: "d8-system",
				Labels: map[string]string{
					"heritage": "deckhouse",
					"module":   "deckhouse-web",
					"app":      "dex-authenticator",
				},
			},
			Spec: DexAuthenticatorSpec{
				ApplicationDomain:                       module.GetPublicDomain("deckhouse", input),
				ApplicationIngressCertificateSecretName: module.GetHTTPSSecretName("ingress-tls", "deckhouseWeb", input),
				ApplicationIngressClassName:             module.GetIngressClass("deckhouseWeb", input),
				AllowedGroups:                           allowedGroups,
			},
		}
		input.PatchCollector.Create(dexAuthenticator, object_patch.UpdateIfExists())
	} else {
		input.PatchCollector.Delete("deckhouse.io/v1", "DexAuthenticator", "d8-system", "deckhouse-web")
	}
	return nil
}
