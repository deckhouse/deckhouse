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

// makeNER is a request for a registry package; the tests about the kind change it.
func makeNER(name, extensionName, digest string) *deckhousev1alpha1.NodeExtensionRequest {
	return &deckhousev1alpha1.NodeExtensionRequest{
		TypeMeta:   metav1.TypeMeta{APIVersion: "deckhouse.io/v1alpha1", Kind: "NodeExtensionRequest"},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: deckhousev1alpha1.NodeExtensionRequestSpec{
			Extension: deckhousev1alpha1.NodeExtension{Name: extensionName, Digest: digest, Kind: deckhousev1alpha1.ExtensionKindPackage},
		},
	}
}

func rawNER(t *testing.T, ner *deckhousev1alpha1.NodeExtensionRequest) runtime.RawExtension {
	t.Helper()
	raw, err := json.Marshal(ner)
	if err != nil {
		t.Fatal(err)
	}
	return runtime.RawExtension{Raw: raw}
}

func makeNERRequest(t *testing.T, op admissionv1.Operation, ner *deckhousev1alpha1.NodeExtensionRequest) admission.Request {
	t.Helper()
	return admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: op,
			Name:      ner.Name,
			Object:    rawNER(t, ner),
		},
	}
}

func makeNERUpdate(t *testing.T, old, updated *deckhousev1alpha1.NodeExtensionRequest) admission.Request {
	t.Helper()
	request := makeNERRequest(t, admissionv1.Update, updated)
	request.OldObject = rawNER(t, old)
	return request
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
			name:        "reserved extension name is denied",
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
		{
			// The agent is a platform extension like the other three. Left
			// unreserved, an older request of the same name would win the
			// cluster-wide contest and displace the platform's own.
			name:        "the agent's own name is denied",
			op:          admissionv1.Create,
			req:         makeNER("agent", "nodelet", digest),
			wantAllowed: false,
		},
		{
			// The platform's packages are named by the release, which admission does
			// not read: the controller settles it against the platform document.
			name:        "a platform package name is left to the controller",
			op:          admissionv1.Create,
			req:         makeNER("custom-jq", "jq", digest),
			wantAllowed: true,
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

// The refusal names the field the request is written with.
func TestNodeExtensionRequestValidatorNamesTheExtensionField(t *testing.T) {
	w := &NodeExtensionRequestValidator{decoder: admission.NewDecoder(nerScheme(t))}
	request := makeNER("shadow", "kubelet", "sha256:"+strings.Repeat("a", 64))

	resp := w.Handle(context.Background(), makeNERRequest(t, admissionv1.Create, request))

	if resp.Allowed {
		t.Fatal("a reserved name was admitted")
	}
	if !strings.Contains(denyMessage(resp), ".spec.extension.name") {
		t.Fatalf("the refusal names another field: %q", denyMessage(resp))
	}
}

// The kind is the requester's word and nothing guesses it: a request without one of the
// two is refused here as well as by the CRD.
func TestNodeExtensionRequestValidatorRequiresTheKind(t *testing.T) {
	w := &NodeExtensionRequestValidator{decoder: admission.NewDecoder(nerScheme(t))}
	for _, kind := range []deckhousev1alpha1.ExtensionKind{"", "Image"} {
		request := makeNER("fresh", "ceph", "sha256:"+strings.Repeat("a", 64))
		request.Spec.Extension.Kind = kind

		resp := w.Handle(context.Background(), makeNERRequest(t, admissionv1.Create, request))

		if resp.Allowed {
			t.Fatalf("kind %q was admitted", kind)
		}
		if !strings.Contains(denyMessage(resp), ".spec.extension.kind must be Package or SystemExtension") {
			t.Fatalf("kind %q: the refusal says something else: %q", kind, denyMessage(resp))
		}
	}
}

// The kind decides which nodes hold the artifact: changing it in place would leave it on
// the nodes of one type while installing it on the other, so the request is recreated.
func TestNodeExtensionRequestValidatorFreezesTheKind(t *testing.T) {
	w := &NodeExtensionRequestValidator{decoder: admission.NewDecoder(nerScheme(t))}
	old := makeNER("sds-utils", "sds-utils", "sha256:"+strings.Repeat("a", 64))

	changed := old.DeepCopy()
	changed.Spec.Extension.Kind = deckhousev1alpha1.ExtensionKindSystemExtension
	resp := w.Handle(context.Background(), makeNERUpdate(t, old, changed))
	if resp.Allowed {
		t.Fatal("a change of kind was admitted")
	}
	if !strings.Contains(denyMessage(resp), "delete the NodeExtensionRequest and create it again") {
		t.Fatalf("the refusal does not say how to change the kind: %q", denyMessage(resp))
	}

	rebuilt := old.DeepCopy()
	rebuilt.Spec.Extension.Digest = "sha256:" + strings.Repeat("b", 64)
	if resp := w.Handle(context.Background(), makeNERUpdate(t, old, rebuilt)); !resp.Allowed {
		t.Fatalf("an update that keeps the kind was refused: %q", denyMessage(resp))
	}
}
