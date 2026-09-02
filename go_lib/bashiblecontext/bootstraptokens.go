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

package bashiblecontext

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BootstrapTokens returns the newest unexpired bootstrap token of every
// NodeGroup that has one, keyed by group. Kept in step with
// groupBootstrapToken in dhctl's pkg/operations/bootstrap/steps_immutable_join.go.
func BootstrapTokens(ctx context.Context, r client.Reader) (map[string]string, error) {
	secrets := &corev1.SecretList{}
	if err := r.List(ctx, secrets,
		client.InNamespace(kubeSystemNS),
		client.HasLabels{bootstrapTokenNGLabel},
	); err != nil {
		return nil, fmt.Errorf("list bootstrap tokens: %w", err)
	}

	tokens := make(map[string]string, len(secrets.Items))
	newest := make(map[string]time.Time, len(secrets.Items))
	for i := range secrets.Items {
		sec := &secrets.Items[i]
		ng := sec.Labels[bootstrapTokenNGLabel]
		if sec.Type != corev1.SecretTypeBootstrapToken || ng == "" {
			continue
		}
		if raw, ok := sec.Data["expiration"]; ok {
			expire, err := time.Parse(time.RFC3339, string(raw))
			if err != nil || time.Until(expire) < 0 {
				continue
			}
		}
		id, hasID := sec.Data["token-id"]
		secretPart, hasSecret := sec.Data["token-secret"]
		if !hasID || !hasSecret {
			continue
		}
		if _, seen := tokens[ng]; seen && !sec.CreationTimestamp.After(newest[ng]) {
			continue
		}
		tokens[ng] = string(id) + "." + string(secretPart)
		newest[ng] = sec.CreationTimestamp.Time
	}
	return tokens, nil
}
