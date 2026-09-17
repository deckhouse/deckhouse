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

package policystatus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	controllerClient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	policyGroup   = "deckhouse.io"
	policyVersion = "v1alpha1"

	// lastUpdateTimeField holds the moment the summary last changed, not the moment it was last
	// checked. The writer stores a summary only when it differs from the one already in the object.
	lastUpdateTimeField = "lastUpdateTime"
)

// Writer stores the summaries in the status of the policy resources.
type Writer struct {
	client controllerClient.Client
	now    func() time.Time
}

// NewWriter returns a Writer that talks to the cluster through the given client.
func NewWriter(client controllerClient.Client) *Writer {
	return &Writer{client: client, now: time.Now}
}

// Sync stores every summary whose contents changed.
//
// A policy that has disappeared is skipped rather than reported as an error: the audit works on a
// snapshot of the constraints, so a policy deleted between the audit and this call is expected.
// A failure on one policy does not stop the rest, and Sync reports how many policies it could
// not update.
func (w *Writer) Sync(ctx context.Context, summaries map[Owner]Summary) error {
	var failed int

	for owner, summary := range summaries {
		if err := w.syncOne(ctx, owner, summary); err != nil {
			failed++
			slog.Warn("update policy status failed",
				"kind", owner.Kind, "name", owner.Name, "error", err)
		}
	}

	if failed > 0 {
		return fmt.Errorf("failed to update the status of %d policies", failed)
	}
	return nil
}

func (w *Writer) syncOne(ctx context.Context, owner Owner, summary Summary) error {
	desired, err := toUnstructuredMap(summary)
	if err != nil {
		return fmt.Errorf("encode summary: %w", err)
	}

	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   policyGroup,
			Version: policyVersion,
			Kind:    owner.Kind,
		})

		if err := w.client.Get(ctx, types.NamespacedName{Name: owner.Name}, obj); err != nil {
			if apierrors.IsNotFound(err) {
				slog.Debug("policy is gone, skipping its status",
					"kind", owner.Kind, "name", owner.Name)
				return nil
			}
			return err
		}

		current, _, err := unstructured.NestedMap(obj.Object, "status", "violations")
		if err != nil {
			// A status that cannot be read is a status worth overwriting.
			slog.Warn("read current policy status failed, overwriting it",
				"kind", owner.Kind, "name", owner.Name, "error", err)
			current = nil
		}

		if sameSummary(current, desired) {
			return nil
		}

		stored := make(map[string]interface{}, len(desired)+1)
		for k, v := range desired {
			stored[k] = v
		}
		stored[lastUpdateTimeField] = w.now().UTC().Format(time.RFC3339)

		if err := unstructured.SetNestedMap(obj.Object, stored, "status", "violations"); err != nil {
			return fmt.Errorf("set status: %w", err)
		}

		return w.client.Status().Update(ctx, obj)
	})
}

// sameSummary reports whether the stored summary already says what the new one says.
//
// lastUpdateTime is left out of the comparison on purpose: it is the one field that would otherwise
// differ on every audit cycle and turn an unchanged summary into a write.
func sameSummary(current, desired map[string]interface{}) bool {
	if current == nil {
		return false
	}

	stripped := make(map[string]interface{}, len(current))
	for k, v := range current {
		if k == lastUpdateTimeField {
			continue
		}
		stripped[k] = v
	}

	return reflect.DeepEqual(stripped, desired)
}

// toUnstructuredMap converts a summary into the shape the API server stores.
//
// It goes through JSON rather than through runtime.DefaultUnstructuredConverter, so that the numbers
// end up as int64 and compare equal to the numbers read back from the API server.
func toUnstructuredMap(summary Summary) (map[string]interface{}, error) {
	encoded, err := json.Marshal(summary)
	if err != nil {
		return nil, err
	}

	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()

	var raw map[string]interface{}
	if err := decoder.Decode(&raw); err != nil {
		return nil, err
	}

	return normalizeNumbers(raw).(map[string]interface{}), nil
}

// normalizeNumbers turns every json.Number into an int64, which is what the unstructured helpers and
// the API server both use for an integer field.
func normalizeNumbers(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		for key, item := range v {
			v[key] = normalizeNumbers(item)
		}
		return v
	case []interface{}:
		for i, item := range v {
			v[i] = normalizeNumbers(item)
		}
		return v
	case json.Number:
		if number, err := v.Int64(); err == nil {
			return number
		}
		return v.String()
	default:
		return value
	}
}
