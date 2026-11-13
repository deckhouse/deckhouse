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

package fencing

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	agentNamespace        = "d8-cloud-instance-manager"
	agentVersionLabel     = "node-manager.deckhouse.io/fencing-agent-version"
	keySecretName         = "fencing-memberlist-key"
	keyPrimary            = "primary"
	keySecondary          = "secondary"
	keySize               = 32
	keyRegeneratedAtAnnot = "node-manager.deckhouse.io/fencing-key-regenerated-at"
	keyRegeneratedMetric  = "d8_fencing_key_regenerated_timestamp_seconds"

	keySecretSnapshot = "key_secret"
	agentPodsSnapshot = "agent_pods"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/node-manager/fencing",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:              keySecretSnapshot,
			ApiVersion:        "v1",
			Kind:              "Secret",
			NamespaceSelector: &types.NamespaceSelector{NameSelector: &types.NameSelector{MatchNames: []string{agentNamespace}}},
			NameSelector:      &types.NameSelector{MatchNames: []string{keySecretName}},
			FilterFunc:        applyKeySecretFilter,
		},
		// Agent pods tell a deleted Secret from a first install: pods of version 2 exist only
		// when the key was in use.
		{
			Name:                         agentPodsSnapshot,
			ApiVersion:                   "v1",
			Kind:                         "Pod",
			NamespaceSelector:            &types.NamespaceSelector{NameSelector: &types.NameSelector{MatchNames: []string{agentNamespace}}},
			LabelSelector:                &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: agentVersionLabel, Operator: metav1.LabelSelectorOpExists}}},
			ExecuteHookOnSynchronization: ptr.To(false),
			ExecuteHookOnEvents:          ptr.To(false),
			FilterFunc:                   nameFilter,
		},
	},
}, dependency.WithExternalDependencies(handleMemberlistKey))

type keySecret struct {
	RegeneratedAt string
	Valid         bool
}

func applyKeySecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret corev1.Secret
	if err := sdk.FromUnstructured(obj, &secret); err != nil {
		return nil, err
	}

	valid := len(secret.Data[keyPrimary]) == keySize
	if second, ok := secret.Data[keySecondary]; ok {
		valid = valid && len(second) == keySize
	}

	return keySecret{RegeneratedAt: secret.Annotations[keyRegeneratedAtAnnot], Valid: valid}, nil
}

// handleMemberlistKey keeps the Secret with the memberlist key. The hook does not write values:
// the key must not appear in them.
func handleMemberlistKey(_ context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	input.MetricsCollector.Expire(keyRegeneratedMetric)

	secrets, err := sdkobjectpatch.UnmarshalToStruct[keySecret](input.Snapshots, keySecretSnapshot)
	if err != nil {
		return fmt.Errorf("failed to unmarshal '%s' snapshot: %w", keySecretSnapshot, err)
	}

	if len(secrets) == 0 {
		if !fencingEnabled(input) {
			return nil
		}
		return createKeySecret(input, dc.GetClock().Now(), len(input.Snapshots.Get(agentPodsSnapshot)) > 0)
	}

	secret := secrets[0]
	// An administrator owns the keys of an existing Secret, also a wrong one.
	if !secret.Valid {
		input.Logger.Error("The memberlist key Secret holds a key of a wrong format, the hook does not change it",
			slog.String("secret", keySecretName))
	}
	if secret.RegeneratedAt == "" {
		return nil
	}

	regeneratedAt, err := time.Parse(time.RFC3339, secret.RegeneratedAt)
	if err != nil {
		input.Logger.Error("Cannot parse the key regeneration time of the memberlist key Secret",
			slog.String("annotation", keyRegeneratedAtAnnot), log.Err(err))
		return nil
	}
	input.MetricsCollector.Set(keyRegeneratedMetric, float64(regeneratedAt.Unix()), nil, metrics.WithGroup(keyRegeneratedMetric))

	return nil
}

// fencingEnabled reports whether any NodeGroup runs an agent. Values hold the modes
// that handleNodeGroupModes has already normalized.
func fencingEnabled(input *go_hook.HookInput) bool {
	for _, ng := range input.Values.Get("nodeManager.internal.fencing.nodeGroups").Map() {
		if ng.Get("mode").String() != modeDisabled {
			return true
		}
	}
	return false
}

// createKeySecret writes a new key. The regeneration time goes into the Secret only when agents
// work: they used a key that is lost now, so the administrator must learn about the new one.
func createKeySecret(input *go_hook.HookInput, now time.Time, regenerated bool) error {
	key := make([]byte, keySize)
	if _, err := rand.Read(key); err != nil {
		return fmt.Errorf("generate the memberlist key: %w", err)
	}

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      keySecretName,
			Namespace: agentNamespace,
			// No heritage label: the label would forbid the administrator to rotate the key.
			Labels: map[string]string{"app": "fencing-agent", "module": "node-manager"},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{keyPrimary: key},
	}
	if regenerated {
		secret.Annotations = map[string]string{keyRegeneratedAtAnnot: now.UTC().Format(time.RFC3339)}
		input.MetricsCollector.Set(keyRegeneratedMetric, float64(now.Unix()), nil, metrics.WithGroup(keyRegeneratedMetric))
	}

	input.PatchCollector.CreateIfNotExists(secret)
	return nil
}
