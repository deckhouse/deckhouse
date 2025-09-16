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
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/module"
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

	cert := secret.Data["ca.crt"]
	if len(cert) == 0 {
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

func discoverDexCA(_ context.Context, input *go_hook.HookInput) error {
	const (
		dexCAPath = "userAuthn.internal.discoveredDexCA"

		dexCAModePath           = "userAuthn.controlPlaneConfigurator.dexCAMode"
		customCAPath            = "userAuthn.controlPlaneConfigurator.dexCustomCA"
		configuratorEnabledPath = "userAuthn.controlPlaneConfigurator.enabled"
	)

	configuratorEnabled := input.Values.Get(configuratorEnabledPath).Bool()
	if !configuratorEnabled {
		input.Values.Remove(dexCAPath)
		return nil
	}

	var (
		dexCA     string
		secretKey string
	)

	dexCAModeFromConfig := input.Values.Get(dexCAModePath).String()

	switch module.GetHTTPSMode("userAuthn", input) {
	case "CertManager":
		secretKey = "ingress-tls"
	case "CustomCertificate":
		secretKey = "ingress-tls-customcertificate"
	}

	switch dexCAModeFromConfig {
	case doNotNeedCAMode:
		input.Values.Remove(dexCAPath)
	case fromIngressSecretCAMode:
		dexCASnapshots := input.Snapshots.Get("secret")
		for dexCAFromSnapshot, err := range sdkobjectpatch.SnapshotIter[DexCA](dexCASnapshots) {
			if err != nil {
				return fmt.Errorf("cannot convert dex ca certificate from snaphots: failed to iterate over 'secret' snapshot: %w", err)
			}

			if dexCAFromSnapshot.Name == secretKey {
				dexCA = string(dexCAFromSnapshot.Data)
				break
			}
		}

		if dexCA == "" {
			input.Logger.Warn("cannot get ca.crt or tls.crt from secret, his is ok for first run")
			input.Values.Remove(dexCAPath)
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
