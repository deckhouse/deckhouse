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

func nsprManifest(pod string) string {
	return "apiVersion: v1\nkind: Pod\nmetadata:\n  name: " + pod + "\n  namespace: d8-system\n" +
		"spec:\n  hostNetwork: true\n  containers:\n  - name: main\n    image: deckhouse.local/images:main\n"
}

func makeNSPR(name, manifest string) *deckhousev1alpha1.NodeStaticPodRequest {
	return &deckhousev1alpha1.NodeStaticPodRequest{
		TypeMeta:   metav1.TypeMeta{APIVersion: "deckhouse.io/v1alpha1", Kind: "NodeStaticPodRequest"},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       deckhousev1alpha1.NodeStaticPodRequestSpec{Manifest: manifest},
	}
}

func makeNSPRRequest(t *testing.T, op admissionv1.Operation, nspr *deckhousev1alpha1.NodeStaticPodRequest) admission.Request {
	t.Helper()
	raw, err := json.Marshal(nspr)
	if err != nil {
		t.Fatal(err)
	}
	return admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: op,
			Name:      nspr.Name,
			Object:    runtime.RawExtension{Raw: raw},
		},
	}
}

// The manifest is checked where it is written. A node that refuses it reports
// the entry failed on every node of the group at once.
// What is NOT checked here is the collision between two objects on one pod: a
// webhook would have to list the others live to see it, and the losing object
// still needs a status to be told in — so the controller settles it and writes
// the reason, the same split NodeExtensionRequest makes.
func TestNodeStaticPodRequestValidator(t *testing.T) {
	s := runtime.NewScheme()
	if err := deckhousev1alpha1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		op          admissionv1.Operation
		nspr        *deckhousev1alpha1.NodeStaticPodRequest
		wantAllowed bool
		wantMessage string
	}{
		{
			name:        "a Pod with a name and a namespace is allowed",
			op:          admissionv1.Create,
			nspr:        makeNSPR("registry-agent", nsprManifest("registry-agent")),
			wantAllowed: true,
		},
		{
			// The object's name is the file name on the node; kubelet takes the
			// mirror pod's name from the document. The two have no reason to agree,
			// and requiring it was this webhook's one wrong rule before 3.1.
			name:        "a pod named nothing like its object is allowed",
			op:          admissionv1.Create,
			nspr:        makeNSPR("agent-request", nsprManifest("registry-agent")),
			wantAllowed: true,
		},
		{
			// A CR name is a DNS subdomain and the field it becomes is a DNS label:
			// the API server admits this one, every node it reached would refuse the
			// whole NodeConfig, and the rollout would stop there.
			name:        "a name the NodeConfig field would not take is denied",
			op:          admissionv1.Create,
			nspr:        makeNSPR("registry-agent.v2", nsprManifest("registry-agent")),
			wantAllowed: false,
			wantMessage: "metadata.name",
		},
		{
			name:        "a name longer than a DNS label is denied",
			op:          admissionv1.Create,
			nspr:        makeNSPR(strings.Repeat("a", 64), nsprManifest("registry-agent")),
			wantAllowed: false,
			wantMessage: "metadata.name",
		},
		{
			// The object name is what is reserved, whatever pod the manifest names.
			name:        "a reserved control-plane name is denied",
			op:          admissionv1.Create,
			nspr:        makeNSPR("kube-apiserver", nsprManifest("registry-agent")),
			wantAllowed: false,
			wantMessage: "the node agent or a bashible step writes itself",
		},
		{
			// The bashible steps write these into the same directory: 051 the API
			// proxy, 052 the registry proxy, 020/070 the node services.
			name:        "a name a bashible step writes itself is denied",
			op:          admissionv1.Create,
			nspr:        makeNSPR("kubernetes-api-proxy", nsprManifest("registry-agent")),
			wantAllowed: false,
			wantMessage: "the node agent or a bashible step writes itself",
		},
		{
			// kubelet collides on namespace/name, whatever the file is called.
			name:        "a reserved control-plane pod under another name is denied",
			op:          admissionv1.Create,
			nspr:        makeNSPR("aaa", strings.Replace(nsprManifest("kube-apiserver"), "d8-system", "kube-system", 1)),
			wantAllowed: false,
			wantMessage: "the pod kube-system/kube-apiserver already has a manifest",
		},
		{
			name:        "a pod a bashible step writes, under another name, is denied",
			op:          admissionv1.Create,
			nspr:        makeNSPR("nodeservices", nsprManifest("registry-nodeservices")),
			wantAllowed: false,
			wantMessage: "the pod d8-system/registry-nodeservices already has a manifest",
		},
		{
			name:        "a reserved name is refused on UPDATE too",
			op:          admissionv1.Update,
			nspr:        makeNSPR("etcd", nsprManifest("etcd")),
			wantAllowed: false,
		},
		{
			name:        "a manifest that is not a valid Pod is denied",
			op:          admissionv1.Create,
			nspr:        makeNSPR("registry-agent", "apiVersion: apps/v1\nkind: Deployment\n"),
			wantAllowed: false,
			wantMessage: ".spec.manifest is refused: manifest is not a valid Pod: ",
		},
		{
			// A core/v1 kind decodes cleanly and is still not a Pod.
			name:        "a ConfigMap is denied too",
			op:          admissionv1.Create,
			nspr:        makeNSPR("registry-agent", "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: x\n  namespace: d8-system\n"),
			wantAllowed: false,
		},
		{
			// The node asks for both, and a node that refuses the manifest reports
			// the entry failed on every node of the group.
			name:        "a pod with no name is denied",
			op:          admissionv1.Create,
			nspr:        makeNSPR("registry-agent", "apiVersion: v1\nkind: Pod\nmetadata:\n  namespace: d8-system\n"),
			wantAllowed: false,
		},
		{
			name:        "a pod with no namespace is denied",
			op:          admissionv1.Create,
			nspr:        makeNSPR("registry-agent", "apiVersion: v1\nkind: Pod\nmetadata:\n  name: registry-agent\n"),
			wantAllowed: false,
		},
		{
			// Loose parsing: the document is executed by a kubelet of its own
			// version, so a field newer than this binary's k8s.io/api must not be
			// refused here — that would refuse it on every node it reaches.
			name:        "a field the vendored Pod type does not know is allowed",
			op:          admissionv1.Create,
			nspr:        makeNSPR("registry-agent", "apiVersion: v1\nkind: Pod\nmetadata:\n  name: registry-agent\n  namespace: d8-system\nspec:\n  brandNewField: true\n"),
			wantAllowed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &NodeStaticPodRequestValidator{decoder: admission.NewDecoder(s)}
			resp := w.Handle(context.Background(), makeNSPRRequest(t, tt.op, tt.nspr))
			if resp.Allowed != tt.wantAllowed {
				t.Fatalf("Allowed = %v, want %v (message: %q)", resp.Allowed, tt.wantAllowed, denyMessage(resp))
			}
			if tt.wantMessage != "" && !strings.Contains(denyMessage(resp), tt.wantMessage) {
				t.Fatalf("message = %q, want it to contain %q", denyMessage(resp), tt.wantMessage)
			}
		})
	}
}
