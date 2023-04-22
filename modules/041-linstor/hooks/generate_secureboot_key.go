/*
Copyright 2023 Flant JSC

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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/gophercloud/gophercloud/acceptance/tools"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

/*
We need to generate drbd module for each kernel version dynamically. In case of SecureBoot we need to sign module,
and add our private key to secure store. This hook generate private key and passphrase.
*/

type SecureBootCertSnapshotContent struct {
	Der        string
	Key        string
	Passphrase string
}

type SecureBootCertSnapshot struct {
	Name string
	Data SecureBootCertSnapshotContent
}

func applySecureBootCertsFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert certificate secret to secret: %v", err)
	}

	return &SecureBootCertSnapshot{
		Name: secret.Name,
		Data: SecureBootCertSnapshotContent{
			Der:        string(secret.Data["secureboot.der"]),
			Key:        string(secret.Data["secureboot.key"]),
			Passphrase: string(secret.Data["passphrase"]),
		}}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "certs",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{linstorNamespace},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{secureBootSecretName},
			},
			FilterFunc: applySecureBootCertsFilter,
		},
	},
}, generateSecureBootCert)

func generateSecureBootCert(input *go_hook.HookInput) error {
	var cert SecureBootCertSnapshotContent

	snaps := input.Snapshots["certs"]
	for _, snap := range snaps {
		s := snap.(*SecureBootCertSnapshot)
		cert = s.Data
	}

	if cert.Passphrase == "" {
		bitSize := 2048
		passphrase := tools.RandomString("", 16)
		key, err := rsa.GenerateKey(rand.Reader, bitSize)
		if err != nil {
			return err
		}

		block := &pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(key),
		}

		block, err = x509.EncryptPEMBlock(rand.Reader, block.Type, block.Bytes, []byte(passphrase), x509.PEMCipherAES256)
		if err != nil {
			return err
		}

		pub := key.Public()
		derKey, err := x509.MarshalPKIXPublicKey(pub)

		input.Values.Set(secureBootDerPath, string(derKey))
		input.Values.Set(secureBootKeyPath, string(pem.EncodeToMemory(block)))
		input.Values.Set(secureBootPassphrasePath, passphrase)
	}

	return nil
}
