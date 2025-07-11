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

package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
)

const (
	certCommonName = "kube-api-admission-ca"
	snapshotName   = "admission-webhook-client-ca-key-pair-secrets"
	secretName     = "admission-webhook-client-ca-key-pair"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       snapshotName,
			ApiVersion: "v1",
			Kind:       "Secret",
			FilterFunc: certificate.ApplyCaSelfSignedCertFilter,
			NameSelector: &types.NameSelector{
				MatchNames: []string{secretName},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
		},
	},
}, generateAdmissionWebhookClientCA)

func generateAdmissionWebhookClientCA(input *go_hook.HookInput) error {
	selfSignedCA, err := certificate.GetOrCreateCa(input, snapshotName, certCommonName)
	if err != nil {
		input.Logger.Fatal(fmt.Sprintf("cannot get or create CA for admission webhook client: %s", err))
		return err
	}

	input.Values.Set("global.internal.modules.admissionWebhookClientCA.cert", selfSignedCA.Cert)
	input.Values.Set("global.internal.modules.admissionWebhookClientCA.key", selfSignedCA.Key)

	return nil
}
