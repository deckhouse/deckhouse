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
			Name:       "deckhouse_webhook_cert",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"webhook-handler-certs"},
			},
			ExecuteHookOnSynchronization: pointer.Bool(false),
			ExecuteHookOnEvents:          pointer.Bool(false),
			FilterFunc:                   applyDeckhouseWebhookSecretFilter,
		},
	},
}, getDeckhouseWebhookCertHandler)

func applyDeckhouseWebhookSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, err
	}

	if secret.Name != "webhook-handler-certs" {
		return nil, nil
	}

	return secret, nil
}

func getDeckhouseWebhookCertHandler(input *go_hook.HookInput) error {
	snapshots, ok := input.Snapshots["deckhouse_webhook_cert"]
	if !ok {
		input.LogEntry.Info("No webhook-handler-certs received")
		return nil
	}

	if len(snapshots) == 0 {
		return fmt.Errorf("no webhook-handler-certs received")
	}

	secret := snapshots[0].(*v1.Secret)

	ca, ok := secret.Data["ca.crt"]
	if !ok {
		return fmt.Errorf("no webhook-handler-certs received")
	}

	input.Values.Set("deckhouse.internal.admissionWebhookCert.ca", string(ca))
	return nil
}
