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

// Package bootstrapsecrets keeps the per-NodeGroup bootstrap tokens a joining
// node authenticates with. Ported from the order_bootstrap_token hook.
package bootstrapsecrets

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	nodecommon "github.com/deckhouse/node-controller/internal/common"
)

const (
	// A token is replaced once less than this is left of it, so that a node
	// which started bootstrapping with it still finishes. Both values are the
	// hook's: modules/040-node-manager/hooks/order_bootstrap_token.go.
	rotateWhenLeftLessThan = 3 * time.Hour
	tokenTTL               = 4 * time.Hour

	tokenIDLength     = 6
	tokenSecretLength = 16

	secretNamePrefix = "bootstrap-token-"
	expirationKey    = "expiration"
)

// EnsureToken returns the bootstrap token of the group, minting a new secret
// when the group has none or the current one expires too soon. The old secret
// is left alone until CollectExpiredTokens sweeps it.
func EnsureToken(ctx context.Context, c client.Client, ngName string) (string, error) {
	tokens, err := nodecommon.BootstrapTokens(ctx, c)
	if err != nil {
		return "", fmt.Errorf("read bootstrap tokens: %w", err)
	}

	if token, ok := tokens[ngName]; ok {
		fresh, err := validLongEnough(ctx, c, token)
		if err != nil {
			return "", err
		}
		if fresh {
			return token, nil
		}
	}

	return mintToken(ctx, c, ngName)
}

// CollectExpiredTokens deletes the bootstrap token secrets whose expiration has
// passed. Tokens of deleted NodeGroups are not collected early — as in the hook,
// they go on the next sweep after their own expiration.
func CollectExpiredTokens(ctx context.Context, c client.Client) error {
	secrets := &corev1.SecretList{}
	if err := c.List(ctx, secrets,
		client.InNamespace(nodecommon.KubeSystemNamespace),
		client.HasLabels{nodecommon.BootstrapTokenNodeGroupLabel},
	); err != nil {
		return fmt.Errorf("list bootstrap tokens: %w", err)
	}

	var deletions []error
	for i := range secrets.Items {
		secret := &secrets.Items[i]
		if !expired(secret) {
			continue
		}
		if err := c.Delete(ctx, secret); err != nil && !apierrors.IsNotFound(err) {
			deletions = append(deletions, fmt.Errorf("delete bootstrap token %s: %w", secret.Name, err))
		}
	}
	return errors.Join(deletions...)
}

// validLongEnough reports whether the token outlives the rotation threshold.
// The bootstrap token authenticator only accepts a secret named after the token
// id, so the id is enough to find the secret carrying this token's expiration.
func validLongEnough(ctx context.Context, r client.Reader, token string) (bool, error) {
	id, _, found := strings.Cut(token, ".")
	if !found {
		return false, nil
	}

	name := secretNamePrefix + id
	secret := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{Namespace: nodecommon.KubeSystemNamespace, Name: name}, secret)
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read bootstrap token %s: %w", name, err)
	}

	// A missing or unreadable expiration is not an error but an answer: such a
	// token is rotated out, exactly as the hook did with its zero validity.
	expiration, err := time.Parse(time.RFC3339, string(secret.Data[expirationKey]))
	if err != nil {
		return false, nil
	}
	return time.Until(expiration) > rotateWhenLeftLessThan, nil
}

func expired(secret *corev1.Secret) bool {
	if secret.Type != corev1.SecretTypeBootstrapToken {
		return false
	}
	raw, ok := secret.Data[expirationKey]
	if !ok {
		return false
	}
	expiration, err := time.Parse(time.RFC3339, string(raw))
	if err != nil {
		return false
	}
	return time.Until(expiration) < 0
}

func mintToken(ctx context.Context, c client.Client, ngName string) (string, error) {
	id, tokenSecret := tokenMaterial()
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretNamePrefix + id,
			Namespace: nodecommon.KubeSystemNamespace,
			Labels: map[string]string{
				"heritage":                              "deckhouse",
				"module":                                "node-manager",
				nodecommon.BootstrapTokenNodeGroupLabel: ngName,
			},
		},
		Type: corev1.SecretTypeBootstrapToken,
		Data: map[string][]byte{
			expirationKey:                    []byte(time.Now().Add(tokenTTL).Format(time.RFC3339)),
			"token-id":                       []byte(id),
			"token-secret":                   []byte(tokenSecret),
			"auth-extra-groups":              []byte("system:bootstrappers:d8-node-manager"),
			"usage-bootstrap-authentication": []byte("true"),
			"usage-bootstrap-signing":        []byte("true"),
		},
	}

	if err := c.Create(ctx, secret); err != nil {
		return "", fmt.Errorf("mint bootstrap token for node group %s: %w", ngName, err)
	}
	return id + "." + tokenSecret, nil
}

// tokenMaterial returns the id and the secret half of a token. rand.Text gives
// at least 26 unbiased base32 characters; lowercased they are a subset of the
// [a-z0-9] the bootstrap token format allows.
func tokenMaterial() (string, string) {
	material := strings.ToLower(rand.Text())
	return material[:tokenIDLength], material[tokenIDLength : tokenIDLength+tokenSecretLength]
}
