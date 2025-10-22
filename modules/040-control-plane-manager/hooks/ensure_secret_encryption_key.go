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
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
)

type SecretEncryptionKey []byte

const (
	secretEncryptionKeySecretName          = "d8-secret-encryption-key"
	secretEncryptionKeySecretKey           = "secretEncryptionKey"
	secretEncryptionKeyValuePath           = "controlPlaneManager.internal.secretEncryptionKey"
	secretEncryptionEnabledConfigValuePath = "controlPlaneManager.apiserver.encryptionEnabled"
	kubeSystemNS                           = "kube-system"
)

var (
	secretLabels = map[string]string{
		"heritage": "deckhouse",
		"module":   "control-plane-manager",
		"name":     "d8-secret-encryption-key",
	}
)

func extractEncryptionSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert incoming object to Secret: %v", err)
	}

	return secret.Data[secretEncryptionKeySecretKey], nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/control-plane-manager",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "secret_encryption_key",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{kubeSystemNS},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{secretEncryptionKeySecretName},
			},
			FilterFunc: extractEncryptionSecret,
		},
	},
}, ensureEncryptionSecretKey)

func ensureEncryptionSecretKey(_ context.Context, input *go_hook.HookInput) error {
	keys := input.Snapshots.Get("secret_encryption_key")

	secretKey := make([]byte, 0)
	if len(keys) > 0 {
		err := keys[0].UnmarshalTo(&secretKey)

		if err != nil {
			return fmt.Errorf("failed to unmarshal 'secret_encryption_key' snapshot: %w", err)
		}
	}

	if len(secretKey) == 0 {
		if !input.Values.Get(secretEncryptionEnabledConfigValuePath).Bool() {
			return nil
		}

		key, err := generateSecretEncryptionKey()
		if err != nil {
			return err
		}
		secretKey = key

		newCM := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretEncryptionKeySecretName,
				Namespace: kubeSystemNS,
				Labels:    secretLabels,
			},
			Data: map[string][]byte{secretEncryptionKeySecretKey: key},
		}

		gvks, _, err := scheme.Scheme.ObjectKinds(newCM)
		if err != nil {
			return fmt.Errorf("missing apiVersion or kind and cannot assign it; %w", err)
		}

		for _, gvk := range gvks {
			if len(gvk.Kind) == 0 {
				continue
			}
			if len(gvk.Version) == 0 || gvk.Version == runtime.APIVersionInternal {
				continue
			}
			newCM.SetGroupVersionKind(gvk)
			break
		}

		input.PatchCollector.CreateOrUpdate(newCM)
	}

	input.Values.Set(secretEncryptionKeyValuePath, base64.StdEncoding.EncodeToString(secretKey))

	return nil
}

func generateSecretEncryptionKey() ([]byte, error) {
	secret := make([]byte, 32)
	_, err := rand.Read(secret)
	if err != nil {
		return []byte{}, err
	}

	return secret, nil
}
