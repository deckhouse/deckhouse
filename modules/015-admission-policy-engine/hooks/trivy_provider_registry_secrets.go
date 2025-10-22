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
	"encoding/base64"
	"fmt"
	"time"

	"github.com/clarketm/json"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/tidwall/gjson"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

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
			FilterFunc: filterTrivyProviderSecret,
		},
	},
}, dependency.WithExternalDependencies(handleTrivyProviderSecrets))

type dockerConfig struct {
	Auths map[string]authn.AuthConfig `json:"auths"`
}

type valueDockerConfig struct {
	Auths map[string]authConfig `json:"auths"`
}

type authConfig struct {
	Username      string `json:"username,omitempty"`
	Password      string `json:"password,omitempty"`
	Auth          string `json:"auth,omitempty"`
	IdentityToken string `json:"identitytoken,omitempty"`
	RegistryToken string `json:"registrytoken,omitempty"`
}

func (a *authConfig) MarshalJSON() ([]byte, error) {
	if a.Username != "" && a.Password != "" {
		a.Auth = base64.StdEncoding.EncodeToString([]byte(a.Username + ":" + a.Password))
	}
	return json.Marshal(a)
}

func filterTrivyProviderSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := new(corev1.Secret)
	if err := sdk.FromUnstructured(obj, secret); err != nil {
		return nil, err
	}

	return dockerConfigBySecret(secret)
}

func dockerConfigBySecret(secret *corev1.Secret) (*dockerConfig, error) {
	if secret.Type != corev1.SecretTypeDockerConfigJson || secret.Data == nil {
		return nil, nil
	}

	raw, ok := secret.Data[corev1.DockerConfigJsonKey]
	if !ok {
		return nil, nil
	}

	config := new(dockerConfig)
	if err := json.Unmarshal(raw, config); err != nil {
		return nil, fmt.Errorf("unmarshal docker config: %v", err)
	}

	return config, nil
}

func handleTrivyProviderSecrets(_ context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	if !input.Values.Get("admissionPolicyEngine.denyVulnerableImages.enabled").Bool() {
		return nil
	}

	cfg := valueDockerConfig{Auths: make(map[string]authConfig)}

	authSnaps, err := sdkobjectpatch.UnmarshalToStruct[dockerConfig](input.Snapshots, "trivy_provider_secrets")
	if err != nil {
		return fmt.Errorf("failed to unmarshal trivy_provider_secrets snapshot: %w", err)
	}

	for _, auth := range authSnaps {
		if len(auth.Auths) == 0 {
			continue
		}
		for registry, config := range auth.Auths {
			cfg.Auths[registry] = authConfig{
				Username:      config.Username,
				Password:      config.Password,
				Auth:          config.Auth,
				IdentityToken: config.IdentityToken,
				RegistryToken: config.RegistryToken,
			}
		}
	}

	cli, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("get k8s client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()

	for _, value := range input.Values.Get("admissionPolicyEngine.denyVulnerableImages.registrySecrets").Array() {
		secret, err := dockerConfigByModuleValue(ctx, cli, value)
		if err != nil {
			return fmt.Errorf("get registry secret from the module values: %w", err)
		}

		for registry, config := range secret.Auths {
			cfg.Auths[registry] = authConfig{
				Username:      config.Username,
				Password:      config.Password,
				Auth:          config.Auth,
				IdentityToken: config.IdentityToken,
				RegistryToken: config.RegistryToken,
			}
		}
	}

	input.Values.Set("admissionPolicyEngine.internal.denyVulnerableImages.dockerConfigJson", cfg)

	return nil
}

func dockerConfigByModuleValue(ctx context.Context, cli k8s.Client, value gjson.Result) (*dockerConfig, error) {
	name, namespace, err := namespaceNameByModuleValue(value)
	if err != nil {
		return nil, fmt.Errorf("get name and namespace from registry secret: %w", err)
	}

	secret, err := cli.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get registry secret from namespace '%s': %w", namespace, err)
	}
	return dockerConfigBySecret(secret)
}

func namespaceNameByModuleValue(value gjson.Result) (string, string, error) {
	data := value.Map()
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
