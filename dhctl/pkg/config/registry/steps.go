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

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

const (
	secretsNamespace = "d8-system"
	stateSecretName  = "registry-state"
	initSecretName   = "registry-init"

	conditionTypeReady = "Ready"
)

// WaitForRegistryInitialization waits for the registry to become fully initialized and ready.
// Parameters:
//   - ctx: context for cancellation and timeouts
//   - kubeClient: Kubernetes client for API operations
//   - config: configuration with registry settings
//
// Returns:
//   - err: error from the operation
func WaitForRegistryInitialization(ctx context.Context, kubeClient client.KubeClient, config Config) error {
	return retry.
		NewLoop("Waiting for Registry to become Ready", 100, 20*time.Second).
		RunContext(ctx, func() error {
			return checkRegistryInitialization(ctx, kubeClient, config)
		})
}

// checkRegistryInitialization checks whether the registry is ready, unless legacy mode is enabled.
// Parameters:
//   - ctx: context for cancellation and timeouts
//   - kubeClient: Kubernetes client for API operations
//   - config: configuration with registry settings
//
// Returns:
//   - err: error from the operation
func checkRegistryInitialization(ctx context.Context, kubeClient client.KubeClient, config Config) error {
	if config.LegacyMode {
		return nil
	}

	logger := dhlog.FromContext(ctx)

	conditions, err := getConditions(ctx, kubeClient)
	if err != nil {
		logger.DebugContext(ctx, fmt.Sprintf("Error while checking registry ready: %v", err))
		return ErrIsNotReady
	}

	if !isConditionsReady(conditions) {
		if msg := formatNotReadyMessage(conditions); msg != "" {
			err := fmt.Errorf("%s\n%s", ErrIsNotReady.Error(), msg)
			logger.DebugContext(ctx, fmt.Sprintf("Error while checking registry ready: %v", err))
			return err
		}

		return ErrIsNotReady
	}

	return nil
}

// formatNotReadyMessage builds a human-readable message listing all non-True
// conditions (excluding the Ready condition itself).
func formatNotReadyMessage(conditions []metav1.Condition) string {
	if len(conditions) == 0 {
		return ""
	}

	var msg strings.Builder

	for _, condition := range conditions {
		if condition.Type == conditionTypeReady {
			continue
		}

		if condition.Status == metav1.ConditionTrue {
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

	return msg.String()
}

// isConditionsReady checks whether the registry is ready based on its conditions.
// It returns true only if the Ready condition is present and set to True.
func isConditionsReady(conditions []metav1.Condition) bool {
	if len(conditions) == 0 {
		return false
	}

	for _, condition := range conditions {
		if condition.Type == conditionTypeReady {
			return condition.Status == metav1.ConditionTrue
		}
	}

	return false
}

// getConditions retrieves and parses the registry state conditions.
// Parameters:
//   - ctx: context for cancellation and timeouts
//   - kubeClient: Kubernetes client for API operations
//
// Returns:
//   - []metav1.Condition: registry state conditions
//   - err: error from the operation
func getConditions(ctx context.Context, kubeClient client.KubeClient) ([]metav1.Condition, error) {
	var conditions []metav1.Condition

	secret, err := kubeClient.
		CoreV1().
		Secrets(secretsNamespace).
		Get(ctx, stateSecretName, metav1.GetOptions{})

	if err != nil {
		if apierrors.IsNotFound(err) {
			return conditions, nil
		}
		return nil, fmt.Errorf(
			"get secret '%s/%s': %w",
			secretsNamespace,
			stateSecretName,
			err,
		)
	}

	conditionRaw, exists := secret.Data["conditions"]
	if !exists {
		return conditions, nil
	}

	if err := yaml.Unmarshal(conditionRaw, &conditions); err != nil {
		return nil, fmt.Errorf(
			"unmarshal secret '%s/%s' conditions: %w",
			secretsNamespace,
			stateSecretName,
			err,
		)
	}

	return conditions, nil
}
