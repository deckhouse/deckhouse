/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
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
)

func applyCertFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert registry ca secret to secret: %v", err)
	}

	return certificate.Authority{
		Cert: string(secret.Data["registry-ca.crt"]),
		Key:  string(secret.Data["registry-ca.key"]),
	}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "cert",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"registry-pki"},
			},
			FilterFunc: applyCertFilter,
		},
	},
}, generateRegistryCA)

func generateRegistryCA(input *go_hook.HookInput) error {
	const (
		certPath = "systemRegistry.internal.registryCA.cert"
		keyPath  = "systemRegistry.internal.registryCA.key"
	)

	var registryCA certificate.Authority

	certs := input.Snapshots["cert"]
	if len(certs) == 1 {
		var ok bool
		registryCA, ok = certs[0].(certificate.Authority)
		if !ok {
			return fmt.Errorf("cannot convert registry certificate to certificate authority")
		}
	} else {
		var err error
		registryCA, err = certificate.GenerateCA(input.LogEntry, "embedded-registry-ca")
		if err != nil {
			return fmt.Errorf("cannot generate registry ca: %v", err)
		}
	}

	input.Values.Set(certPath, registryCA.Cert)
	input.Values.Set(keyPath, registryCA.Key)
	return nil
}
