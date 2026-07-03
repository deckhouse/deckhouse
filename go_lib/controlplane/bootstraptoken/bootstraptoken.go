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

package bootstraptoken

import (
	"context"
	"crypto/rand"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	namespace                = "kube-system"
	alphaNumLowerCaseCharset = "0123456789abcdefghijklmnopqrstuvwxyz"
)

// randString mirrors go_lib/pwgen.AlphaNumLowerCase
func randString(length int) string {
	buf := make([]byte, length)
	_, _ = rand.Read(buf)
	op := byte(len(alphaNumLowerCaseCharset))
	for i, b := range buf {
		buf[i] = alphaNumLowerCaseCharset[b%op]
	}
	return string(buf)
}

// Generate returns a fresh (token-id, token-secret)
func Generate() (id, secret string) {
	return randString(6), randString(16)
}

func BuildSecret(id, secret string, groups []string, ttl time.Duration, labels map[string]string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("bootstrap-token-%s", id),
			Namespace: namespace,
			Labels:    labels,
		},
		Type: corev1.SecretTypeBootstrapToken,
		Data: map[string][]byte{
			"expiration":                     []byte(time.Now().Add(ttl).Format(time.RFC3339)),
			"token-id":                       []byte(id),
			"token-secret":                   []byte(secret),
			"auth-extra-groups":              []byte(strings.Join(groups, ",")),
			"usage-bootstrap-authentication": []byte("true"),
		},
	}
}

// ParseToken extracts "id.secret" and expiry from a bootstrap-token Secret
func ParseToken(sec *corev1.Secret) (token string, expiresAt time.Time, ok bool) {
	id, hasID := sec.Data["token-id"]
	s, hasSecret := sec.Data["token-secret"]
	if !hasID || !hasSecret {
		return "", time.Time{}, false
	}
	exp := time.Time{}
	if raw, has := sec.Data["expiration"]; has {
		if t, err := time.Parse(time.RFC3339, string(raw)); err == nil {
			exp = t
		}
	}
	return fmt.Sprintf("%s.%s", id, s), exp, true
}

// EnsureValid guarantees a bootstrap-token Secret matching labelSelector exists in kube-system and is valid for at least regenBelow.
// Expired tokens are deleted; the freshest valid token is reused; otherwise a new one (valid ttl) is created.
// Returns the usable "id.secret" token.
func EnsureValid(ctx context.Context, c kubernetes.Interface, labelSelector string, groups []string, ttl, regenBelow time.Duration, extraLabels map[string]string) (string, error) {
	list, err := c.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return "", fmt.Errorf("list bootstrap-token secrets: %w", err)
	}

	type candidate struct {
		token   string
		expires time.Time
	}
	var valid []candidate
	for i := range list.Items {
		sec := &list.Items[i]
		if sec.Type != corev1.SecretTypeBootstrapToken {
			continue
		}
		token, exp, ok := ParseToken(sec)
		if !ok {
			continue
		}
		if time.Until(exp) <= 0 {
			_ = c.CoreV1().Secrets(namespace).Delete(ctx, sec.Name, metav1.DeleteOptions{})
			continue
		}
		valid = append(valid, candidate{token: token, expires: exp})
	}

	sort.Slice(valid, func(i, j int) bool { return valid[i].expires.After(valid[j].expires) })
	if len(valid) > 0 && time.Until(valid[0].expires) > regenBelow {
		return valid[0].token, nil
	}

	id, secret := Generate()
	newSec := BuildSecret(id, secret, groups, ttl, extraLabels)
	if _, err := c.CoreV1().Secrets(namespace).Create(ctx, newSec, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return "", fmt.Errorf("create bootstrap-token secret: %w", err)
	}
	return fmt.Sprintf("%s.%s", id, secret), nil
}
