/*
Copyright 2026 Flant JSC

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

	"github.com/deckhouse/deckhouse/go_lib/pwgen"
)

const (
	// sharedDexAuthenticatorRedisPasswordsPath holds the password of the Redis of each instance, by
	// the name of the instance. The Redis of the replicas of an instance talk to each other with it.
	sharedDexAuthenticatorRedisPasswordsPath = "userAuthn.internal.sharedDexAuthenticator.redisPasswords"

	sharedDexAuthenticatorNamespace = "d8-user-authn"

	sharedDexAuthenticatorRedisPasswordLength = 32
)

// sharedDexAuthenticatorInstances are the names of the instances: the one of the DexAuthenticator
// objects of the d8-* namespaces and the one of all the others.
var sharedDexAuthenticatorInstances = []string{"dex-authenticator-system", "dex-authenticator"}

// redisPasswordSecret is the Secret of an instance with the password of its Redis.
type redisPasswordSecret struct {
	Name     string
	Password string
}

func applyRedisPasswordSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	if err := sdk.FromUnstructured(obj, secret); err != nil {
		return nil, fmt.Errorf("convert the secret: %w", err)
	}

	return redisPasswordSecret{Name: secret.Name, Password: string(secret.Data["password"])}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "redis_passwords",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{MatchNames: []string{sharedDexAuthenticatorNamespace}},
			},
			NameSelector: &types.NameSelector{MatchNames: redisPasswordSecretNames()},
			FilterFunc:   applyRedisPasswordSecretFilter,
		},
	},
}, sharedDexAuthenticator)

func redisPasswordSecretNames() []string {
	names := make([]string, 0, len(sharedDexAuthenticatorInstances))
	for _, instance := range sharedDexAuthenticatorInstances {
		names = append(names, redisPasswordSecretName(instance))
	}

	return names
}

func redisPasswordSecretName(instance string) string {
	return instance + "-redis"
}

// sharedDexAuthenticator keeps the Redis password of each shared dex-authenticator instance across
// restarts: it reads the password back from the Secret the module renders and generates a new
// one only when there is none.
func sharedDexAuthenticator(_ context.Context, input *go_hook.HookInput) error {
	stored := map[string]string{}
	for secret, err := range sdkobjectpatch.SnapshotIter[redisPasswordSecret](input.Snapshots.Get("redis_passwords")) {
		if err != nil {
			return fmt.Errorf("unmarshal the 'redis_passwords' snapshot: %w", err)
		}
		stored[secret.Name] = secret.Password
	}

	// The whole map is set at once: the values have no map to add a key to before the first run.
	passwords := make(map[string]string, len(sharedDexAuthenticatorInstances))
	current := input.Values.Get(sharedDexAuthenticatorRedisPasswordsPath).Map()
	for _, instance := range sharedDexAuthenticatorInstances {
		password := current[instance].String()
		if password == "" {
			password = stored[redisPasswordSecretName(instance)]
		}
		if password == "" {
			password = pwgen.AlphaNum(sharedDexAuthenticatorRedisPasswordLength)
		}
		passwords[instance] = password
	}
	input.Values.Set(sharedDexAuthenticatorRedisPasswordsPath, passwords)

	return nil
}
