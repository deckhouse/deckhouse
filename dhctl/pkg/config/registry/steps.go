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
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

const (
	secretsNamespace            = "d8-system"
	initSecretName              = "registry-init"
	stateSecretName             = "registry-state"
	initSecretAppliedAnnotation = "registry.deckhouse.io/is-applied"
)

func WaitForRegistryInitialization(ctx context.Context, kubeClient client.KubeClient, config Config) error {
	return retry.NewLoop("Waiting for Registry to become Ready", 100, 20*time.Second).RunContext(ctx, func() error {
		return checkRegistryInitialization(ctx, kubeClient, config)
	})
}

func checkRegistryInitialization(ctx context.Context, kubeClient client.KubeClient, config Config) error {
	if config.ModuleEnabled {
		if err := checkInit(ctx, kubeClient); err != nil {
			log.DebugF("Error while checking registry init: %v\n", err)
			return ErrIsNotReady
		}

		msg, err := checkReady(ctx, kubeClient)
		if err != nil {
			if msg != "" {
				err := fmt.Errorf("%s\n%s", ErrIsNotReady.Error(), msg)
				log.DebugF("Error while checking registry ready: %v\n", err)
				return err
			}
			log.DebugF("Error while checking registry ready: %v\n", err)
			return ErrIsNotReady
		}
	}

	if err := removeInitSecret(ctx, kubeClient); err != nil {
		log.DebugF("Error while removing registry init secret: %v\n", err)
		return ErrIsNotReady
	}

	return nil
}

func checkInit(ctx context.Context, kubeClient client.KubeClient) error {
	exists, applied, err := getInitSecretStatus(ctx, kubeClient)
	if err != nil {
		return err
	}

	if exists && !applied {
		return ErrNotInitialized
	}
	return nil
}

func checkReady(ctx context.Context, kubeClient client.KubeClient) (string, error) {
	conditions, err := getStateSecret(ctx, kubeClient)
	if err != nil {
		return "", err
	}

	if len(conditions) == 0 {
		return "", ErrIsNotReady
	}

	var msg strings.Builder
	ready := true

	for _, condition := range conditions {
		if condition.Status == metav1.ConditionTrue {
			continue
		}
		ready = false
		if condition.Type == "Ready" {
			continue
		}

		if msg.Len() > 0 {
			msg.WriteString("\n")
		}
		fmt.Fprintf(&msg, "* %s: %s",
			condition.Type,
			strings.TrimSpace(strings.ReplaceAll(condition.Message, "\n", " ")),
		)
	}

	if ready {
		return "", nil
	}
	return msg.String(), ErrIsNotReady
}

func getStateSecret(ctx context.Context, kubeClient client.KubeClient) ([]metav1.Condition, error) {
	secret, err := kubeClient.CoreV1().Secrets(secretsNamespace).Get(ctx, stateSecretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get secret '%s/%s': %w", secretsNamespace, stateSecretName, err)
	}

	var conditions []metav1.Condition

	conditionRaw, exists := secret.Data["conditions"]
	if !exists {
		return conditions, nil
	}

	if err := yaml.Unmarshal(conditionRaw, &conditions); err != nil {
		return nil, fmt.Errorf("unmarshal secret data: %w", err)
	}
	return conditions, nil
}

func getInitSecretStatus(ctx context.Context, kubeClient client.KubeClient) (bool, bool, error) {
	secret, err := kubeClient.CoreV1().Secrets(secretsNamespace).Get(ctx, initSecretName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, false, nil
		}
		return false, false, fmt.Errorf("get secret '%s/%s': %w", secretsNamespace, initSecretName, err)
	}
	_, applied := secret.Annotations[initSecretAppliedAnnotation]
	return true, applied, nil
}

func removeInitSecret(ctx context.Context, kubeClient client.KubeClient) error {
	err := kubeClient.CoreV1().Secrets(secretsNamespace).Delete(ctx, initSecretName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("remove secret '%s/%s': %w", secretsNamespace, initSecretName, err)
	}
	return nil
}
