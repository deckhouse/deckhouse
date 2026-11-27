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
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

// MaskedValue is what kube-apiserver returns instead of a field marked x-kubernetes-sensitive-data
// to a caller without the <resource>/sensitive subresource. In a sensitive field, a create with it
// is refused and an update that sends it back keeps the stored value.
const MaskedValue = "<omitted>"

// ErrSensitiveMasked reports a spec that still reads as MaskedValue from the API server: the
// controller is not allowed to read the sensitive subresource of the resource.
var ErrSensitiveMasked = errors.New("a sensitive field reads as " + MaskedValue +
	": the controller is not allowed to read the sensitive subresource of the resource")

const (
	sensitiveMaskedMetricName = "d8_user_authn_sensitive_field_masked_total"
	// ReasonSensitiveFieldMasked is the reason of the Warning event on an object whose sensitive
	// field the controller cannot read.
	ReasonSensitiveFieldMasked = "SensitiveFieldMasked"
	actionRead                 = "Read"
)

// sensitiveFields are the fields marked x-kubernetes-sensitive-data in the module CRDs, by the kind
// of the object, with the resource whose sensitive subresource unmasks them.
var sensitiveFields = map[string]struct {
	resource string
	paths    [][]string
}{
	UserGVK.Kind: {resource: "users", paths: [][]string{{"spec", "password"}}},
	UserOperationGVK.Kind: {resource: "useroperations", paths: [][]string{
		{"spec", "resetPassword", "newPasswordHash"},
	}},
	DexProviderGVK.Kind: {resource: "dexproviders", paths: [][]string{
		{"spec", "github", "clientSecret"},
		{"spec", "gitlab", "clientSecret"},
		{"spec", "bitbucketCloud", "clientSecret"},
		{"spec", "crowd", "clientSecret"},
		{"spec", "oidc", "clientSecret"},
		{"spec", "ldap", "bindPW"},
	}},
}

var (
	registerSensitiveMetricsOnce sync.Once

	sensitiveMaskedTotal = newSensitiveMaskedTotal()
)

func newSensitiveMaskedTotal() *prometheus.CounterVec {
	c := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: sensitiveMaskedMetricName,
		Help: "Reads of an object whose sensitive field the API server returned as " + MaskedValue +
			" past the cache too, by kind: the controller cannot read the sensitive subresource and retries.",
	}, []string{"kind"})
	// Every kind is reported from the start, so the first masked read is an increase.
	for kind := range sensitiveFields {
		c.WithLabelValues(kind)
	}
	return c
}

// RegisterSensitiveMetrics registers d8_user_authn_sensitive_field_masked_total in the registry
// of controller-runtime.
func RegisterSensitiveMetrics() error {
	var regErr error
	registerSensitiveMetricsOnce.Do(func() {
		if err := ctrlmetrics.Registry.Register(sensitiveMaskedTotal); err != nil {
			var already prometheus.AlreadyRegisteredError
			if errors.As(err, &already) {
				return
			}
			regErr = fmt.Errorf("register %s: %w", sensitiveMaskedMetricName, err)
		}
	})
	return regErr
}

// GetUnmasked reads obj through c, the cache. The cache keeps the objects as they came when the
// controller did not have the sensitive subresource yet: the CRDs with the marker are applied
// before the module release grants it, and a cached object changes only with its next event. When
// a sensitive field holds MaskedValue, GetUnmasked reads the object again through apiReader, past the cache,
// and fails with ErrSensitiveMasked if it is still masked.
//
// The reconcile retries such an object with a growing delay until the controller can read the
// subresource, for example after RBAC rolled back to a release without it. Nothing else changes
// meanwhile, so each such read is counted in d8_user_authn_sensitive_field_masked_total and
// recorded as a Warning event on the object through recorder (nil records none).
func GetUnmasked(ctx context.Context, c, apiReader client.Reader, recorder events.EventRecorder, key client.ObjectKey, obj *unstructured.Unstructured) error {
	if err := c.Get(ctx, key, obj); err != nil {
		return err
	}
	if _, masked := maskedSensitiveField(obj); !masked {
		return nil
	}

	if err := apiReader.Get(ctx, key, obj); err != nil {
		return err
	}
	path, masked := maskedSensitiveField(obj)
	if !masked {
		return nil
	}

	kind := obj.GetKind()
	sensitiveMaskedTotal.WithLabelValues(kind).Inc()
	if recorder != nil {
		recorder.Eventf(obj, nil, corev1.EventTypeWarning, ReasonSensitiveFieldMasked, actionRead,
			"%s reads as %s: user-authn-controller is not allowed to read the %s/sensitive subresource "+
				"and retries until it can", path, MaskedValue, sensitiveFields[kind].resource)
	}
	return fmt.Errorf("%s %s: %w", kind, key.Name, ErrSensitiveMasked)
}

// maskedSensitiveField returns the first sensitive field of obj that holds MaskedValue.
func maskedSensitiveField(obj *unstructured.Unstructured) (string, bool) {
	if obj.GroupVersionKind().Group != UserGVK.Group {
		return "", false
	}
	for _, path := range sensitiveFields[obj.GetKind()].paths {
		value, _, err := unstructured.NestedString(obj.Object, path...)
		if err == nil && value == MaskedValue {
			return strings.Join(path, "."), true
		}
	}
	return "", false
}
