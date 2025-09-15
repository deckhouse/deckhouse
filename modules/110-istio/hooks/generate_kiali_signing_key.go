/*
Copyright 2024 Flant JSC

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
	"math/rand"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib"
)

type kialiSecret struct {
	SigningKey string
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "kiali_signing_key_secret",
			ApiVersion: "v1",
			Kind:       "Secret",
			FilterFunc: applyKialiSecretFilter,
			NameSelector: &types.NameSelector{
				MatchNames: []string{"kiali-signing-key"},
			},
			NamespaceSelector: lib.NsSelector(),
		},
	},
}, generateKialiSigningKey)

func applyKialiSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kiali secret object to structured secret: %v", err)
	}

	signingKey := secret.Data["key"]

	return kialiSecret{
		SigningKey: string(signingKey),
	}, nil
}

func generateKialiSigningKey(_ context.Context, input *go_hook.HookInput) error {
	kialiSigningKey := ""
	snapshots := input.Snapshots.Get("kiali_signing_key_secret")
	if len(snapshots) == 1 {
		var secret kialiSecret
		err := snapshots[0].UnmarshalTo(&secret)
		if err != nil {
			return fmt.Errorf("failed to unmarshal 'kiali_signing_key_secret' snapshot: %w", err)
		}
		kialiSigningKey = secret.SigningKey
	}
	if len(kialiSigningKey) != 32 {
		kialiSigningKey = randomString(32)
	}
	input.Values.Set("istio.internal.kialiSigningKey", kialiSigningKey)
	return nil
}

func randomString(n int) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	s := make([]rune, n)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return string(s)
}
