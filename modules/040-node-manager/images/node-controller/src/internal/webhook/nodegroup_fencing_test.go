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
	"testing"

	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/testenv"
)

func TestValidation_FencingWatchdogWarning(t *testing.T) {
	scheme := newScheme()
	validator := &NodeGroupValidator{Client: fake.NewClientBuilder().WithScheme(scheme).Build(), decoder: admission.NewDecoder(scheme)}

	for _, tc := range []struct {
		mode v1.FencingMode
		warn bool
	}{
		{mode: v1.FencingModeWatchdog, warn: true},
		{mode: v1.FencingModeSlow},
		{mode: v1.FencingModeDisabled},
	} {
		for _, operation := range []admissionv1.Operation{admissionv1.Create, admissionv1.Update} {
			t.Run(string(tc.mode)+"/"+string(operation), func(t *testing.T) {
				ng := baseNodeGroup("worker", v1.NodeTypeStatic)
				ng.Spec.Fencing = &v1.FencingSpec{Mode: tc.mode}
				var oldNG *v1.NodeGroup
				if operation == admissionv1.Update {
					oldNG = baseNodeGroup("worker", v1.NodeTypeStatic)
				}

				response := validator.Handle(context.Background(), makeAdmissionRequest(t, operation, ng, oldNG))

				require.True(t, response.Allowed, "%v", response.Result)
				if tc.warn {
					require.Len(t, response.Warnings, 1)
					require.Contains(t, response.Warnings[0], "Watchdog is deprecated and works as Slow")
				} else {
					require.Empty(t, response.Warnings)
				}
			})
		}
	}
}

// The CRD schema of spec.fencing, run by a real apiserver without the webhook.
func TestNodeGroupFencingCRDRules(t *testing.T) {
	if !testenv.AssetsAvailable() {
		t.Skip("envtest assets not found; run `make envtest` or set KUBEBUILDER_ASSETS")
	}
	env, _, c, err := testenv.Start(runtime.NewScheme(), testenv.CRDPaths(testenv.WithNodeGroupCRDFile())...)
	if err != nil {
		t.Fatalf("start envtest: %v", err)
	}
	t.Cleanup(func() {
		if err := env.Stop(); err != nil {
			t.Errorf("stop envtest: %v", err)
		}
	})

	for _, tc := range []struct {
		name    string
		fencing map[string]any
		wantErr string
	}{
		{name: "Watchdog", fencing: map[string]any{"mode": "Watchdog"}},
		{name: "Watchdog with watchdog.timeout", fencing: map[string]any{"mode": "Watchdog", "watchdog": map[string]any{"timeout": "60s"}}},
		{name: "Slow with watchdog.timeout", fencing: map[string]any{"mode": "Slow", "watchdog": map[string]any{"timeout": "5m"}}},
		{name: "Critical", fencing: map[string]any{"mode": "Critical"}},
		{name: "unknown mode", fencing: map[string]any{"mode": "Foo"}, wantErr: "Unsupported value"},
		{name: "bad watchdog.timeout", fencing: map[string]any{"mode": "Slow", "watchdog": map[string]any{"timeout": "soon"}}, wantErr: "should match"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ng := nodeGroupObject("v1", testenv.UniqueName("ng"), map[string]any{"nodeType": "Static", "fencing": tc.fencing})
			err := c.Create(t.Context(), ng)
			if err == nil {
				t.Cleanup(func() { _ = c.Delete(t.Context(), ng) })
			}
			checkRefusal(t, err, tc.wantErr)
		})
	}
}
