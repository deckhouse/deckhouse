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

package webhook

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
)

func nerScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := deckhousev1alpha1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	return s
}

func makeNER(name, sysextName, digest string) *deckhousev1alpha1.NodeExtensionRequest {
	return &deckhousev1alpha1.NodeExtensionRequest{
		TypeMeta:   metav1.TypeMeta{APIVersion: "deckhouse.io/v1alpha1", Kind: "NodeExtensionRequest"},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: deckhousev1alpha1.NodeExtensionRequestSpec{
			Sysext: deckhousev1alpha1.Sysext{Name: sysextName, Digest: digest},
		},
	}
}

func makeNERRequest(t *testing.T, op admissionv1.Operation, ner *deckhousev1alpha1.NodeExtensionRequest) admission.Request {
	t.Helper()
	raw, err := json.Marshal(ner)
	if err != nil {
		t.Fatal(err)
	}
	return admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: op,
			Name:      ner.Name,
			Object:    runtime.RawExtension{Raw: raw},
		},
	}
}

func denyMessage(resp admission.Response) string {
	if resp.Result == nil {
		return ""
	}
	return resp.Result.Message
}

// Only the reserved-name rule is admission's: a name or digest already claimed
// by another request is settled cluster-wide by the nodeconfig controller, which
// reports it on the request that lost.
func TestNodeExtensionRequestValidator(t *testing.T) {
	s := nerScheme(t)
	digest := "sha256:" + strings.Repeat("a", 64)

	tests := []struct {
		name        string
		op          admissionv1.Operation
		req         *deckhousev1alpha1.NodeExtensionRequest
		wantAllowed bool
	}{
		{
			name:        "a name of its own is allowed",
			op:          admissionv1.Create,
			req:         makeNER("fresh", "ceph", digest),
			wantAllowed: true,
		},
		{
			name:        "reserved sysext name is denied",
			op:          admissionv1.Create,
			req:         makeNER("shadow", "kubelet", digest),
			wantAllowed: false,
		},
		{
			name:        "a reserved name is refused on UPDATE too",
			op:          admissionv1.Update,
			req:         makeNER("existing", "containerd", digest),
			wantAllowed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &NodeExtensionRequestValidator{decoder: admission.NewDecoder(s)}
			resp := w.Handle(context.Background(), makeNERRequest(t, tt.op, tt.req))
			if resp.Allowed != tt.wantAllowed {
				t.Fatalf("Allowed = %v, want %v (message: %q)", resp.Allowed, tt.wantAllowed, denyMessage(resp))
			}
		})
	}
}
