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

package controller

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// secretAlphabet is the alphabet of the secrets the module generates: the one the module hooks used.
const secretAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// RandomSecret returns a random string of length letters and digits.
func RandomSecret(length int) (string, error) {
	limit := big.NewInt(int64(len(secretAlphabet)))
	out := make([]byte, length)
	for i := range out {
		n, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", fmt.Errorf("generate a secret: %w", err)
		}
		out[i] = secretAlphabet[n.Int64()]
	}
	return string(out), nil
}

// JoinRetryable joins errs so that one terminal error does not stop the retry of another:
// errors.Join of a terminal error is terminal as a whole.
func JoinRetryable(errs []error) error {
	var retry, terminal []error
	for _, err := range errs {
		switch {
		case err == nil:
		case errors.Is(err, reconcile.TerminalError(nil)):
			terminal = append(terminal, err)
		default:
			retry = append(retry, err)
		}
	}
	if len(retry) == 0 {
		return errors.Join(terminal...)
	}
	for _, err := range terminal {
		retry = append(retry, errors.New(err.Error()))
	}
	return errors.Join(retry...)
}

// ReadConditions reads status.conditions of an unstructured object.
func ReadConditions(obj *unstructured.Unstructured) []metav1.Condition {
	raw, _, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	conditions := make([]metav1.Condition, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		c := metav1.Condition{}
		c.Type, _ = m["type"].(string)
		status, _ := m["status"].(string)
		c.Status = metav1.ConditionStatus(status)
		c.Reason, _ = m["reason"].(string)
		c.Message, _ = m["message"].(string)
		// A typed client decodes the integer as int64, a JSON one as float64.
		switch g := m["observedGeneration"].(type) {
		case int64:
			c.ObservedGeneration = g
		case float64:
			c.ObservedGeneration = int64(g)
		}
		if t, ok := m["lastTransitionTime"].(string); ok {
			if parsed, err := time.Parse(time.RFC3339, t); err == nil {
				c.LastTransitionTime = metav1.NewTime(parsed)
			}
		}
		conditions = append(conditions, c)
	}
	return conditions
}

// ConditionItems turns conditions into the value of status.conditions of an unstructured object.
func ConditionItems(conditions []metav1.Condition) []any {
	items := make([]any, 0, len(conditions))
	for _, c := range conditions {
		item := map[string]any{
			"type":               c.Type,
			"status":             string(c.Status),
			"reason":             c.Reason,
			"lastTransitionTime": c.LastTransitionTime.UTC().Format(time.RFC3339),
			"observedGeneration": c.ObservedGeneration,
		}
		if c.Message != "" {
			item["message"] = c.Message
		}
		items = append(items, item)
	}
	return items
}
