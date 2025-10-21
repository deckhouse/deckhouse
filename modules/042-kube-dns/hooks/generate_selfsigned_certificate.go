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
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	"github.com/deckhouse/deckhouse/go_lib/hooks/tls_certificate"
)

const (
	webhookServiceHost      = "d8-kube-dns-sts-pods-hosts-appender-webhook"
	webhookServiceNamespace = "kube-system"
)

var _ = tls_certificate.RegisterInternalTLSHook(tls_certificate.GenSelfSignedTLSHookConf{
	BeforeHookCheck: func(_ context.Context, input *go_hook.HookInput) bool {
		if len(input.Values.Get("kubeDns.clusterDomainAliases").Array()) == 0 {
			input.Logger.Debug("No Domain aliases provided. Interrupting hook execution.")
			return false
		}

		return true
	},

	SANs: tls_certificate.DefaultSANs([]string{
		webhookServiceHost,
		fmt.Sprintf(
			"%s.%s.svc",
			webhookServiceHost,
			webhookServiceNamespace,
		),
	}),
	CN: webhookServiceHost,

	Namespace:            webhookServiceNamespace,
	TLSSecretName:        "d8-kube-dns-sts-pods-hosts-appender-webhook",
	FullValuesPathPrefix: "kubeDns.internal.stsPodsHostsAppenderWebhook",
})
