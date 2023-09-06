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
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/clarketm/json"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/tidwall/gjson"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/admission-policy-engine/trivy_provider_secrets",
	OnBeforeHelm: &go_hook.OrderedConfig{
		Order: 30,
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "trivy_provider_secrets",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-admission-policy-engine"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"deckhouse-registry"},
			},
			FilterFunc: fileterTrivyProviderSecrets,
		},
	},
}, dependency.WithExternalDependencies(handleTrivyProviderSecrets))

type dockerConfig struct {
	Auths map[string]authn.AuthConfig `json:"auths"`
}

func fileterTrivyProviderSecrets(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret corev1.Secret
	err := sdk.FromUnstructured(obj, &secret)
	if err != nil {
		return nil, err
	}
	config, err := registrySecretToAuthnConfig(&secret)
	if err != nil {
		if errors.Is(err, ErrSecretWithNoData) || errors.Is(err, ErrNotDockerCfgJSONType) || errors.Is(err, ErrNoDockerConfigJSONKey) {
			return nil, nil
		}
		return nil, err
	}
	return config.Auths, nil
}

var (
	ErrSecretWithNoData      = errors.New("secret has nil data")
	ErrNotDockerCfgJSONType  = fmt.Errorf("secret is not '%s' type", corev1.SecretTypeDockerConfigJson)
	ErrNoDockerConfigJSONKey = fmt.Errorf("secret doesn't have '%s' key", corev1.DockerConfigJsonKey)
)

func registrySecretToAuthnConfig(secret *corev1.Secret) (*dockerConfig, error) {
	if secret.Type != corev1.SecretTypeDockerConfigJson {
		return nil, fmt.Errorf("%w: name=%s, namespace=%s", ErrNotDockerCfgJSONType, secret.GetName(), secret.GetNamespace())
	}

	if secret.Data == nil {
		return nil, fmt.Errorf("%w: name=%s, namespace=%s", ErrSecretWithNoData, secret.GetName(), secret.GetNamespace())
	}

	rawCreds, ok := secret.Data[corev1.DockerConfigJsonKey]
	if !ok {
		return nil, fmt.Errorf("%w: name=%s, namespace=%s", ErrNoDockerConfigJSONKey, secret.GetName(), secret.GetNamespace())
	}

	config := new(dockerConfig)
	if err := json.Unmarshal(rawCreds, config); err != nil {
		return nil, fmt.Errorf("cannot decode docker config JSON: %v", err)
	}
	return config, nil
}

func handleTrivyProviderSecrets(input *go_hook.HookInput, dc dependency.Container) error {
	if !input.Values.Get("admissionPolicyEngine.denyVulnerableImages.enabled").Bool() {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()

	resultCfg := dockerConfig{Auths: make(map[string]authn.AuthConfig)}
	for _, authSnap := range input.Snapshots["trivy_provider_secrets"] {
		err := convertSnapToAuthnConfig(authSnap, &resultCfg)
		if err != nil && !errors.Is(err, ErrNilSnapshot) {
			return err
		}
	}

	k8sClient, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("can't get k8s client for retrieving registry secrets: %w", err)
	}

	registrySecretsValues := input.Values.Get("admissionPolicyEngine.denyVulnerableImages.registrySecrets").Array()
	for _, registrySecretValue := range registrySecretsValues {
		registrySecret, err := registrySecretValueToAuthnConfig(ctx, registrySecretValue, k8sClient)
		if err != nil {
			return fmt.Errorf("can't get registry secret from module values: %w", err)
		}
		err = convertSnapToAuthnConfig(registrySecret.Auths, &resultCfg)
		if err != nil && !errors.Is(err, ErrNilSnapshot) {
			return err
		}
	}
	input.Values.Set("admissionPolicyEngine.internal.denyVulnerableImages.dockerConfigJson", resultCfg)
	return nil
}

var ErrNilSnapshot = errors.New("nil snapshot")

func convertSnapToAuthnConfig(authSnap interface{}, resultCfg *dockerConfig) error {
	if authSnap == nil {
		return fmt.Errorf("%w: %v", ErrNilSnapshot, authSnap)
	}

	auths, ok := authSnap.(map[string]authn.AuthConfig)
	if !ok {
		return fmt.Errorf("can't convert auths snaphsot to map[string]authn.AuthConfig{}: %v", authSnap)
	}

	for registry, config := range auths {
		resultCfg.Auths[registry] = config
	}
	return nil
}

func registrySecretValueToAuthnConfig(ctx context.Context, registrySecretValue gjson.Result, k8sClient k8s.Client) (*dockerConfig, error) {
	name, namespace, err := registrySecretValueToNamespaceName(registrySecretValue)
	if err != nil {
		return nil, fmt.Errorf("can't retrieve name and namespace from registry secret entry: %w", err)
	}
	registrySecret, err := k8sClient.CoreV1().Secrets(namespace).Get(ctx, name, v1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("can't get registry secret from namespace '%s': %w", namespace, err)
	}
	return registrySecretToAuthnConfig(registrySecret)
}

func registrySecretValueToNamespaceName(registrySecretValue gjson.Result) (string, string, error) {
	data := registrySecretValue.Map()
	if len(data) == 0 {
		return "", "", fmt.Errorf("no data found from registrySecret value")
	}

	name, ok := data["name"]
	if !ok {
		return "", "", fmt.Errorf("no name found for registry secret")
	}

	namespace, ok := data["namespace"]
	if !ok {
		return "", "", fmt.Errorf("no namespace found for registry secret")
	}
	return name.String(), namespace.String(), nil
}
