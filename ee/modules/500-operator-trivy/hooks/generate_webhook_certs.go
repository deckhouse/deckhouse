/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	"github.com/deckhouse/deckhouse/ee/modules/500-operator-trivy/hooks/internal/apis/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/hooks/tls_certificate"
)

const (
	certificateSecretName = "report-updater-webhook-ssl"
)

var _ = tls_certificate.RegisterInternalTLSHook(tls_certificate.GenSelfSignedTLSHookConf{
	BeforeHookCheck: func(_ context.Context, input *go_hook.HookInput) bool {
		var (
			secretExists         = len(input.Snapshots.Get(tls_certificate.SnapshotKey)) > 0
			reportUpdaterEnabled = input.Values.Get("operatorTrivy.linkCVEtoBDU").Bool()
		)

		return secretExists || reportUpdaterEnabled
	},

	SANs: tls_certificate.DefaultSANs([]string{
		"report-updater.d8-operator-trivy.svc",
		"report-updater.d8-operator-trivy",
		"report-updater",
	}),

	CN: "report-updater",

	Namespace:            v1alpha1.Namespace,
	TLSSecretName:        certificateSecretName,
	FullValuesPathPrefix: "operatorTrivy.internal.reportUpdater.webhookCertificate",
})
