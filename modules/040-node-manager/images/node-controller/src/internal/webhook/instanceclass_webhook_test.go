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
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func makeInstanceClassDeleteRequest(t *testing.T, kind, name string, consumers []string, withStatus bool) admission.Request {
	t.Helper()
	oldObject := map[string]any{
		"apiVersion": "deckhouse.io/v1",
		"kind":       kind,
		"metadata":   map[string]any{"name": name},
		"spec":       map[string]any{"cores": 2},
	}
	if withStatus {
		oldObject["status"] = map[string]any{"nodeGroupConsumers": consumers}
	}
	raw, err := json.Marshal(oldObject)
	if err != nil {
		t.Fatal(err)
	}
	return admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Delete,
			Name:      name,
			Kind:      metav1.GroupVersionKind{Group: "deckhouse.io", Version: "v1", Kind: kind},
			OldObject: runtime.RawExtension{Raw: raw},
		},
	}
}

func TestInstanceClassDelete_DenyWhenHasNodeGroupConsumers(t *testing.T) {
	w := &InstanceClassDeleteValidator{}
	for _, kind := range []string{"YandexInstanceClass", "AWSInstanceClass", "GCPInstanceClass", "AzureInstanceClass", "OpenstackInstanceClass"} {
		t.Run(kind, func(t *testing.T) {
			req := makeInstanceClassDeleteRequest(t, kind, "worker-test", []string{"nodegroup1", "nodegroup2"}, true)
			resp := w.Handle(t.Context(), req)
			if resp.Allowed {
				t.Fatal("expected denied: instance class has NodeGroup consumers")
			}
			expMessage := fmt.Sprintf("%s/worker-test cannot be deleted because it is being used by NodeGroup: nodegroup1, nodegroup2", kind)
			if resp.Result == nil || resp.Result.Message != expMessage {
				t.Fatalf("expected message %q, got: %v", expMessage, resp.Result)
			}
		})
	}
}

func TestInstanceClassDelete_AllowWhenNoConsumers(t *testing.T) {
	w := &InstanceClassDeleteValidator{}
	req := makeInstanceClassDeleteRequest(t, "YandexInstanceClass", "worker-test", []string{}, true)
	resp := w.Handle(t.Context(), req)
	if !resp.Allowed {
		t.Fatalf("expected allowed for empty nodeGroupConsumers, got: %v", resp.Result)
	}
}

func TestInstanceClassDelete_AllowWhenStatusMissing(t *testing.T) {
	w := &InstanceClassDeleteValidator{}
	req := makeInstanceClassDeleteRequest(t, "YandexInstanceClass", "worker-test", nil, false)
	resp := w.Handle(t.Context(), req)
	if !resp.Allowed {
		t.Fatalf("expected allowed when status is absent, got: %v", resp.Result)
	}
}

func TestInstanceClassDelete_AllowWhenOldObjectMissing(t *testing.T) {
	w := &InstanceClassDeleteValidator{}
	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Delete,
			Name:      "worker-test",
			Kind:      metav1.GroupVersionKind{Group: "deckhouse.io", Version: "v1", Kind: "YandexInstanceClass"},
		},
	}
	resp := w.Handle(t.Context(), req)
	if !resp.Allowed {
		t.Fatalf("expected allowed when oldObject is absent, got: %v", resp.Result)
	}
}

func TestInstanceClassDelete_NonDeleteOperationErrored(t *testing.T) {
	w := &InstanceClassDeleteValidator{}
	req := makeInstanceClassDeleteRequest(t, "YandexInstanceClass", "worker-test", nil, true)
	req.Operation = admissionv1.Create
	resp := w.Handle(t.Context(), req)
	if resp.Allowed {
		t.Fatal("expected error response for non-DELETE operation")
	}
	if resp.Result == nil || resp.Result.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-DELETE operation, got: %v", resp.Result)
	}
}
