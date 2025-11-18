// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package registry

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	registry_init "github.com/deckhouse/deckhouse/go_lib/registry/models/init"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

var (
	initSecretNamespace         = "d8-system"
	initSecretName              = "registry-init"
	initSecretAppliedAnnotation = "registry.deckhouse.io/is-applied"
)

func WaitForRegistryInitialization(ctx context.Context, kubeClient client.KubeClient, config Config) error {
	return retry.NewLoop("Check registry initialization", 20, 5*time.Second).
		RunContext(ctx, func() error {
			isExist, isApplied, err := initSecretStatus(ctx, kubeClient)
			if err != nil {
				return fmt.Errorf("failed to check registry init secret status: %w", err)
			}

			if !config.isModuleEnabled() || !isExist {
				if err := initSecretRemove(ctx, kubeClient); err != nil {
					return fmt.Errorf("failed to remove registry init secret: %w", err)
				}
				return nil
			}

			if !isApplied {
				return fmt.Errorf("registry is not initialized")
			}

			if err := initSecretRemove(ctx, kubeClient); err != nil {
				return fmt.Errorf("failed to remove registry init secret: %w", err)
			}
			return nil
		})
}

func initSecretFetch(ctx context.Context, kubeClient client.KubeClient) (registry_init.Config, error) {
	secret, err := kubeClient.CoreV1().Secrets(initSecretNamespace).Get(ctx, initSecretName, metav1.GetOptions{})
	if err != nil {
		return registry_init.Config{}, fmt.Errorf("failed to get secret '%s/%s': %w", initSecretNamespace, initSecretName, err)
	}

	var config registry_init.Config
	if err := yaml.Unmarshal(secret.Data["config"], &config); err != nil {
		return registry_init.Config{}, fmt.Errorf("failed to unmarshal secret config: %w", err)
	}

	return config, nil
}

func initSecretStatus(ctx context.Context, kubeClient client.KubeClient) (isExist bool, isApplied bool, err error) {
	secret, err := kubeClient.CoreV1().Secrets(initSecretNamespace).Get(ctx, initSecretName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, false, nil
		}

		return false, false, fmt.Errorf("failed to get secret '%s/%s': %w", initSecretNamespace, initSecretName, err)
	}

	_, isApplied = secret.Annotations[initSecretAppliedAnnotation]
	return true, isApplied, nil
}

func initSecretRemove(ctx context.Context, kubeClient client.KubeClient) error {
	err := kubeClient.CoreV1().Secrets(initSecretNamespace).Delete(ctx, initSecretName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to remove secret '%s/%s': %w", initSecretNamespace, initSecretName, err)
	}
	return nil
}
