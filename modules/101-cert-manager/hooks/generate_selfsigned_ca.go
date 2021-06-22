package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	"github.com/deckhouse/deckhouse/modules/101-cert-manager/hooks/internal"
)

const (
	caSnapshot           = "cert"
	selfSignedSecretName = "selfsigned-ca-key-pair"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       caSnapshot,
			ApiVersion: "v1",
			Kind:       "Secret",
			FilterFunc: certificate.ApplyCaSelfSignedCertFilter,
			NameSelector: &types.NameSelector{
				MatchNames: []string{selfSignedSecretName},
			},
			NamespaceSelector: internal.NsSelector(),
		},
	},
}, generateSelfSignedCA)

func generateSelfSignedCA(input *go_hook.HookInput) error {
	selfSignedCA, err := certificate.GetOrCreateCa(input, caSnapshot, "cluster-selfsigned-ca")
	if err != nil {
		return err
	}

	input.Values.Set("certManager.internal.selfSignedCA.cert", selfSignedCA.Cert)
	input.Values.Set("certManager.internal.selfSignedCA.key", selfSignedCA.Key)

	return nil
}
