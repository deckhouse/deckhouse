/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package pricing

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type DeckhouseSecret struct {
	Bundle         []byte
	ReleaseChannel []byte
}

func ApplyPricingDeckhouseSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes secret to secret: %v", err)
	}

	return DeckhouseSecret{Bundle: secret.Data["bundle"], ReleaseChannel: secret.Data["releaseChannel"]}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:              "deckhouseSecret",
			ApiVersion:        "v1",
			Kind:              "Secret",
			NamespaceSelector: &types.NamespaceSelector{NameSelector: &types.NameSelector{MatchNames: []string{"d8-system"}}},
			NameSelector:      &types.NameSelector{MatchNames: []string{"deckhouse-discovery"}},
			FilterFunc:        ApplyPricingDeckhouseSecretFilter,
		},
	},
}, deckhouseSecret)

func deckhouseSecret(input *go_hook.HookInput) error {
	snaps, ok := input.Snapshots["deckhouseSecret"]
	if !ok {
		input.LogEntry.Info("No deckhouse secret received, skipping setting values")
		return nil
	}
	ds := snaps[0].(DeckhouseSecret)
	input.Values.Set("flantIntegration.internal.bundle", string(ds.Bundle))
	input.Values.Set("flantIntegration.internal.releaseChannel", string(ds.ReleaseChannel))

	return nil
}
