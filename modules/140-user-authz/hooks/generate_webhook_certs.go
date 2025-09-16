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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	"github.com/deckhouse/deckhouse/go_lib/hooks/tls_certificate"
	"github.com/deckhouse/deckhouse/modules/140-user-authz/hooks/internal"
)

const (
	certificateSecretName = "user-authz-webhook"
)

var _ = tls_certificate.RegisterInternalTLSHook(tls_certificate.GenSelfSignedTLSHookConf{
	BeforeHookCheck: func(_ context.Context, input *go_hook.HookInput) bool {
		var (
			secretExists        = len(input.Snapshots.Get(tls_certificate.SnapshotKey)) > 0
			multitenancyEnabled = input.Values.Get("userAuthz.enableMultiTenancy").Bool()
		)

		return secretExists || multitenancyEnabled
	},

	SANs: tls_certificate.DefaultSANs([]string{"127.0.0.1"}),
	CN:   "127.0.0.1",

	Namespace:            internal.Namespace,
	TLSSecretName:        certificateSecretName,
	FullValuesPathPrefix: "userAuthz.internal.webhookCertificate",
})
