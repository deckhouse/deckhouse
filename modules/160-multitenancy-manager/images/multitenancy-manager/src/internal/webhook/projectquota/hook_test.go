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

package projectquota

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"controller/apis/deckhouse.io/v1alpha1"
)

func request(t *testing.T, operation admissionv1.Operation, pq, old *v1alpha1.ProjectQuota) admission.Request {
	t.Helper()
	raw := func(object *v1alpha1.ProjectQuota) runtime.RawExtension {
		if object == nil {
			return runtime.RawExtension{}
		}
		data, err := json.Marshal(object)
		require.NoError(t, err)
		return runtime.RawExtension{Raw: data}
	}
	return admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Operation: operation, Object: raw(pq), OldObject: raw(old)}}
}

func projectQuota(hard corev1.ResourceList, used corev1.ResourceList) *v1alpha1.ProjectQuota {
	return &v1alpha1.ProjectQuota{
		ObjectMeta: metav1.ObjectMeta{Name: "team"},
		Spec:       v1alpha1.ProjectQuotaSpec{Hard: hard},
		Status:     v1alpha1.ProjectQuotaStatus{Used: used},
	}
}

func rl(pairs ...string) corev1.ResourceList {
	out := corev1.ResourceList{}
	for i := 0; i+1 < len(pairs); i += 2 {
		out[corev1.ResourceName(pairs[i])] = resource.MustParse(pairs[i+1])
	}
	return out
}

func TestHandle(t *testing.T) {
	tests := []struct {
		name     string
		req      func(t *testing.T) admission.Request
		allowed  bool
		message  string
		warnings []string
	}{
		{
			name:    "accepted keys",
			req:     func(t *testing.T) admission.Request { return request(t, admissionv1.Create, projectQuota(rl("requests.cpu", "4", "projectNamespaces", "3"), nil), nil) },
			allowed: true,
		},
		{
			name:    "a refused key",
			req:     func(t *testing.T) admission.Request { return request(t, admissionv1.Create, projectQuota(rl("count/resourcequotas", "4"), nil), nil) },
			message: "count/resourcequotas",
		},
		{
			name: "a kept key on an update",
			req: func(t *testing.T) admission.Request {
				return request(t, admissionv1.Update, projectQuota(rl("example.com/widgets", "4", "pods", "5"), nil), projectQuota(rl("example.com/widgets", "4"), nil))
			},
			allowed:  true,
			warnings: []string{"example.com/widgets: a ResourceQuota does not count this key; see the list of accepted keys in the documentation of ProjectQuota; the key is kept because the object has it already"},
		},
		{
			name: "a limit lowered below the usage",
			req: func(t *testing.T) admission.Request {
				return request(t, admissionv1.Update, projectQuota(rl("pods", "5", "requests.cpu", "8"), nil), projectQuota(rl("pods", "20", "requests.cpu", "8"), rl("pods", "12", "requests.cpu", "9")))
			},
			allowed: true,
			warnings: []string{"pods: the limit 5 is below what the project uses, 12; nothing running is evicted, and creating more is denied until the usage is back within the limit"},
		},
		{
			name:    "a delete",
			req:     func(t *testing.T) admission.Request { return request(t, admissionv1.Delete, nil, projectQuota(rl("pods", "5"), nil)) },
			allowed: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := (&validator{}).Handle(context.Background(), tt.req(t))
			assert.Equal(t, tt.allowed, response.Allowed, response.Result)
			if tt.message != "" {
				assert.Contains(t, response.Result.Message, tt.message)
			}
			assert.Equal(t, tt.warnings, response.Warnings)
		})
	}
}
