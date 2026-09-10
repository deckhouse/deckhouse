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
	"context"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	"github.com/deckhouse/deckhouse/go_lib/hooks/tls_certificate"
)

const (
	yandexWebhookCN = "yandex-dns-webhook"
)

var _ = tls_certificate.RegisterInternalTLSHook(tls_certificate.GenSelfSignedTLSHookConf{
	BeforeHookCheck: func(_ context.Context, input *go_hook.HookInput) bool {
		folderID := input.Values.Get("certManager.yandexFolderID").String()
		saJSON := input.Values.Get("certManager.yandexServiceAccountJSON").String()
		return folderID != "" && saJSON != ""
	},

	SANs: tls_certificate.DefaultSANs([]string{
		"yandex-dns-webhook.d8-cert-manager.svc",
		"yandex-dns-webhook.d8-cert-manager",
		"yandex-dns-webhook",
	}),

	CN: yandexWebhookCN,

	Namespace:            "d8-cert-manager",
	TLSSecretName:        "yandex-dns-webhook-tls",
	FullValuesPathPrefix: "certManager.internal.yandexWebhookCert",
})
