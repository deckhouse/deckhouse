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

type PublishAPICert struct {
	Name string `json:"name"`
	Data []byte `json:"data"`
}

func applyPublishAPICertFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	s := &v1.Secret{}
	err := sdk.FromUnstructured(obj, s)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes secret to secret: %v", err)
	}

	return PublishAPICert{Name: obj.GetName(), Data: s.Data["ca.crt"]}, nil
}

var possiblePublishAPISecretNames = []string{
	"kubernetes-tls",
	"kubernetes-tls-selfsigned",
	"kubernetes-tls-customcertificate",
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
				MatchNames: possiblePublishAPISecretNames,
			},
			FilterFunc: applyPublishAPICertFilter,
		},
	},
}, discoverPublishAPICA)

func discoverPublishAPICA(_ context.Context, input *go_hook.HookInput) error {
	var (
		secretPath     = "userAuthn.internal.publishedAPIKubeconfigGeneratorMasterCA"
		modePath       = "userAuthn.publishAPI.https.mode"
		globalOptsPath = "userAuthn.publishAPI.https.global.kubeconfigGeneratorMasterCA"
		kubeCAPath     = "global.discovery.kubernetesCA"
	)

	caCertificates := make(map[string][]byte)
	for publishCert, err := range sdkobjectpatch.SnapshotIter[PublishAPICert](input.Snapshots.Get("secret")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'secret' snapshot: %w", err)
		}

		caCertificates[publishCert.Name] = publishCert.Data
	}

	var cert string
	var err error
	switch input.Values.Get(modePath).String() {
	case "Global":
		if input.Values.Exists(globalOptsPath) {
			cert = input.Values.Get(globalOptsPath).String()
		} else {
			switch module.GetHTTPSMode("userAuthn", input) {
			case "CertManager":
				cert, err = getCert(input, "kubernetes-tls")
				if err != nil {
					return fmt.Errorf("failed to get cert from 'kubernetes-tls' secret: %w", err)
				}
			case "CustomCertificate":
				cert, err = getCert(input, "kubernetes-tls-customcertificate")
				if err != nil {
					return fmt.Errorf("failed to get cert from 'kubernetes-tls-customcertificate' secret: %w", err)
				}
			case "OnlyInURI", "Disabled":
			}
			if cert == "" {
				cert = input.Values.Get(kubeCAPath).String()
			}
		}
	case "SelfSigned":
		cert, err = getCert(input, "kubernetes-tls-selfsigned")
		if err != nil {
			return fmt.Errorf("failed to get cert from 'kubernetes-tls-selfsigned' secret: %w", err)
		}
		if cert == "" {
			cert = input.Values.Get(kubeCAPath).String()
		}
	}

	input.Values.Set(secretPath, cert)
	return nil
}

func getCert(input *go_hook.HookInput, secretKey string) (string, error) {
	caCertificates := make(map[string][]byte)

	var cert string
	for publishCert, err := range sdkobjectpatch.SnapshotIter[PublishAPICert](input.Snapshots.Get("secret")) {
		if err != nil {
			return "", fmt.Errorf("failed to iterate over 'secret' snapshot: %w", err)
		}

		caCertificates[publishCert.Name] = publishCert.Data
	}

	for _, name := range possiblePublishAPISecretNames {
		if name == secretKey {
			cert = string(caCertificates[name])
			continue
		}
		input.PatchCollector.DeleteInBackground("v1", "Secret", "d8-user-authn", name)
	}
	return cert, nil
}
