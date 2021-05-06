package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	doNotNeedCAMode         = "DoNotNeed"
	fromIngressSecretCAMode = "FromIngressSecret"
	customCAMode            = "Custom"
)

type DexCA struct {
	Name string `json:"name"`
	Data []byte `json:"data"`
}

func applyDexCAFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes secret to secret: %v", err)
	}

	cert, ok := secret.Data["ca.crt"]
	if !ok {
		cert = secret.Data["tls.crt"]
	}

	return DexCA{Name: obj.GetName(), Data: cert}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "secret",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-user-authn"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"ingress-tls", "ingress-tls-customcertificate"},
			},
			FilterFunc: applyDexCAFilter,
		},
	},
}, discoverDexCA)

func discoverDexCA(input *go_hook.HookInput) error {
	const (
		dexCAPath     = "userAuthn.internal.discoveredDexCA"
		dexCAModePath = "userAuthn.controlPlaneConfigurator.dexCAMode"
		customCAPath  = "userAuthn.controlPlaneConfigurator.dexCustomCA"
	)

	configuratorEnabled := input.Values.Get("userAuthn.controlPlaneConfigurator.enabled").Bool()
	if !configuratorEnabled {
		if input.ConfigValues.Exists(dexCAPath) {
			input.Values.Remove(dexCAPath)
		}
		return nil
	}

	var dexCA string

	dexCAModeFromConfig := input.Values.Get(dexCAModePath).String()

	switch dexCAModeFromConfig {
	case doNotNeedCAMode:
		if input.ConfigValues.Exists(dexCAPath) {
			input.Values.Remove(dexCAPath)
		}
	case fromIngressSecretCAMode:
		dexCASnapshots := input.Snapshots["secret"]
		if len(dexCASnapshots) > 0 {
			dexCAFromSnapshot, ok := dexCASnapshots[0].(DexCA)
			if !ok {
				return fmt.Errorf("cannot convert dex ca certificate from snaphots")
			}

			dexCA = string(dexCAFromSnapshot.Data)
		}

		if dexCA == "" {
			input.LogEntry.Warnln("cannot get ca.crt or tls.crt from secret, his is ok for first run")
			if input.ConfigValues.Exists(dexCAPath) {
				input.Values.Remove(dexCAPath)
			}
			return nil
		}
	case customCAMode:
		if !input.Values.Exists(customCAPath) {
			return fmt.Errorf("dexCustomCA parameter is mandatory with dexCAMode = 'Custom'")
		}

		dexCA = input.Values.Get(customCAPath).String()
	}

	input.Values.Set(dexCAPath, dexCA)
	return nil
}
