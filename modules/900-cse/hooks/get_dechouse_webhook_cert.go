// Copyright 2023 Flant JSC
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

// this hook figure out minimal ingress controller version at the beginning and on IngressNginxController creation
// this version is used on requirements check on Deckhouse update
// Deckhouse would not update minor version before pod is ready, so this hook will execute at least once (on sync)

package hooks

import (
	"fmt"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/cse",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "admission-webhook-certs",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"admission-webhook-certs"},
			},
			ExecuteHookOnSynchronization: pointer.Bool(false),
			ExecuteHookOnEvents:          pointer.Bool(false),
			FilterFunc:                   applyAdmissionWebhookCertsSecretFilter,
		},
	},
}, getAddmissionWebhookCertHandler)

func applyAdmissionWebhookCertsSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret v1.Secret
	err := sdk.FromUnstructured(obj, &secret)
	if err != nil {
		return nil, err
	}

	return certificate.Certificate{
		CA:   string(secret.Data["ca.crt"]),
		Cert: string(secret.Data["tls.crt"]),
		Key:  string(secret.Data["tls.key"]),
	}, nil
}

func getAddmissionWebhookCertHandler(input *go_hook.HookInput) error {
	snapshots, ok := input.Snapshots["admission-webhook-certs"]
	if !ok {
		input.LogEntry.Info("No admission-webhook-certs received")
		return nil
	}

	if len(snapshots) == 0 {
		return fmt.Errorf("no admission-webhook-certs received")
	}

	cert := snapshots[0].(certificate.Certificate)

	input.Values.Set(
		"cse.internal.admissionWebhookCert",
		certValues{CA: cert.CA, Crt: cert.Cert, Key: cert.Key},
	)
	return nil
}

type certValues struct {
	CA  string `json:"ca"`
	Crt string `json:"crt"`
	Key string `json:"key"`
}
