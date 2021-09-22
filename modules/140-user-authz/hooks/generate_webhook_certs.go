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
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	"github.com/deckhouse/deckhouse/modules/140-user-authz/hooks/internal"
)

type WebhookSecretData struct {
	CA     certificate.Authority
	Server certificate.Authority
}

const (
	webhookSnapshotTLS = "secrets"
)

func applyWebhookSecretRuleFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, err
	}

	ws := &WebhookSecretData{}

	webhookCA, ok := secret.Data["ca.crt"]
	if !ok {
		return nil, fmt.Errorf("'ca.crt' field not found")
	}
	webhookServerCrt, ok := secret.Data["webhook-server.crt"]
	if !ok {
		return nil, fmt.Errorf("'webhook-server.crt' field not found")
	}
	webhookServerKey, ok := secret.Data["webhook-server.key"]
	if !ok {
		return nil, fmt.Errorf("'webhook-server.key' field not found")
	}

	ws.CA.Cert = string(webhookCA)
	ws.Server.Cert = string(webhookServerCrt)
	ws.Server.Key = string(webhookServerKey)

	return ws, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Queue:        internal.Queue(webhookSnapshotTLS),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:              webhookSnapshotTLS,
			ApiVersion:        "v1",
			Kind:              "Secret",
			NamespaceSelector: internal.NsSelector(),
			NameSelector:      &types.NameSelector{MatchNames: []string{"user-authz-webhook"}},
			FilterFunc:        applyWebhookSecretRuleFilter,
		},
	},
}, webhookSecretsHandler)

func webhookSecretsHandler(input *go_hook.HookInput) error {
	var webhookCA string
	var webhookServerCrt string
	var webhookServerKey string

	snapshots := input.Snapshots[webhookSnapshotTLS]

	if len(snapshots) > 0 {
		snapshot := snapshots[0].(*WebhookSecretData)
		webhookCA = snapshot.CA.Cert
		webhookServerCrt = snapshot.Server.Cert
		webhookServerKey = snapshot.Server.Key
	} else {
		enableMultiTenancy := input.Values.Get("userAuthz.enableMultiTenancy").Bool()
		if !enableMultiTenancy {
			return nil
		}
		if input.Values.Exists("userAuthz.internal.webhookCA") {
			return nil
		}
		var selfSignedCA certificate.Authority
		selfSignedCA, err := certificate.GenerateCA(input.LogEntry, "user-authz-webhook")
		if err != nil {
			return fmt.Errorf("cannot generate selfsigned ca: %v", err)
		}
		webhookCert, err := certificate.GenerateSelfSignedCert(input.LogEntry, "user-authz-webhook", selfSignedCA, certificate.WithSANs("127.0.0.1"))
		if err != nil {
			return fmt.Errorf("cannot generate selfsigned cert: %v", err)
		}

		webhookCA = selfSignedCA.Cert
		webhookServerKey = webhookCert.Key
		webhookServerCrt = webhookCert.Cert
	}

	input.Values.Set("userAuthz.internal.webhookCA", webhookCA)
	input.Values.Set("userAuthz.internal.webhookServerCrt", webhookServerCrt)
	input.Values.Set("userAuthz.internal.webhookServerKey", webhookServerKey)

	return nil
}
