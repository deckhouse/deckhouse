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
	"slices"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func sensitiveUser(password string) *unstructured.Unstructured {
	u := Object(UserGVK)
	u.SetName("alice")
	u.Object["spec"] = map[string]any{"email": "alice@example.com", "password": password}
	return u
}

// countingReader counts the reads that go past the cache.
type countingReader struct {
	client.Reader
	gets int
}

func (r *countingReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	r.gets++
	return r.Reader.Get(ctx, key, obj, opts...)
}

func newSensitiveClient(obj *unstructured.Unstructured) client.Client {
	return fake.NewClientBuilder().WithScheme(runtime.NewScheme()).WithObjects(obj).Build()
}

func TestGetUnmasked(t *testing.T) {
	t.Parallel()

	const hash = "JDJ5JDEwJGRNWGVGUVBkdUdYYVMyWDFPcGdZdk9HSy81LkdsNm5sdU9mUkhnNWlQdDhuSlh6SzhpeS5H"
	key := client.ObjectKey{Name: "alice"}

	tests := []struct {
		name      string
		cached    string
		api       string
		want      string
		wantErr   error
		wantReads int
	}{
		{name: "the cache holds the value", cached: hash, api: hash, want: hash, wantReads: 0},
		{name: "the cache holds the masked value from before the grant", cached: MaskedValue, api: hash, want: hash, wantReads: 1},
		{name: "the API server masks the value too", cached: MaskedValue, api: MaskedValue, wantErr: ErrSensitiveMasked, wantReads: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			api := &countingReader{Reader: newSensitiveClient(sensitiveUser(tt.api))}
			obj := Object(UserGVK)
			err := GetUnmasked(t.Context(), newSensitiveClient(sensitiveUser(tt.cached)), api, nil, key, obj)
			if api.gets != tt.wantReads {
				t.Errorf("reads past the cache = %d, want %d", api.gets, tt.wantReads)
			}
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("GetUnmasked: %v", err)
			}
			if got, _, _ := unstructured.NestedString(obj.Object, "spec", "password"); got != tt.want {
				t.Errorf("spec.password = %q, want %q", got, tt.want)
			}
		})
	}
}

// A value the API server masks too is retried by the reconcile without any other change, so every
// such read is counted and recorded on the object.
func TestGetUnmaskedReportsMaskedValue(t *testing.T) {
	for _, tt := range []struct {
		name   string
		obj    *unstructured.Unstructured
		gvk    schema.GroupVersionKind
		events []string
	}{
		{
			name:   "masked",
			obj:    sensitiveUser(MaskedValue),
			gvk:    UserGVK,
			events: []string{"Warning SensitiveFieldMasked spec.password reads as <omitted>: user-authn-controller is not allowed to read the users/sensitive subresource and retries until it can"},
		},
		{name: "unmasked", obj: sensitiveUser("JDJ5JDEw"), gvk: UserGVK},
	} {
		t.Run(tt.name, func(t *testing.T) {
			counter := sensitiveMaskedTotal.WithLabelValues(tt.gvk.Kind)
			before := counterValue(t, counter)
			recorder := events.NewFakeRecorder(10)
			c := newSensitiveClient(tt.obj)

			_ = GetUnmasked(t.Context(), c, c, recorder, client.ObjectKeyFromObject(tt.obj), Object(tt.gvk))

			if got := counterValue(t, counter) - before; got != float64(len(tt.events)) {
				t.Errorf("%s{kind=%q} grew by %v, want %d", sensitiveMaskedMetricName, tt.gvk.Kind, got, len(tt.events))
			}
			close(recorder.Events)
			var got []string
			for event := range recorder.Events {
				got = append(got, event)
			}
			if strings.Join(got, "\n") != strings.Join(tt.events, "\n") {
				t.Errorf("events = %q, want %q", got, tt.events)
			}
		})
	}
}

func TestSensitiveMaskedTotalReportsEveryKind(t *testing.T) {
	t.Parallel()

	ch := make(chan prometheus.Metric, 10)
	newSensitiveMaskedTotal().Collect(ch)
	close(ch)
	var kinds []string
	for metric := range ch {
		var out dto.Metric
		if err := metric.Write(&out); err != nil {
			t.Fatal(err)
		}
		if out.GetCounter().GetValue() != 0 {
			t.Errorf("a new counter is %v", out.GetCounter().GetValue())
		}
		for _, label := range out.GetLabel() {
			kinds = append(kinds, label.GetName()+"="+label.GetValue())
		}
	}
	slices.Sort(kinds)
	if want := []string{"kind=DexProvider", "kind=User", "kind=UserOperation"}; !slices.Equal(kinds, want) {
		t.Errorf("series = %v, want %v", kinds, want)
	}
}

func counterValue(t *testing.T, counter prometheus.Counter) float64 {
	t.Helper()

	var out dto.Metric
	if err := counter.Write(&out); err != nil {
		t.Fatal(err)
	}
	return out.GetCounter().GetValue()
}

func TestMaskedSensitiveField(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		gvk  schema.GroupVersionKind
		path []string
	}{
		{gvk: UserGVK, path: []string{"spec", "password"}},
		{gvk: UserOperationGVK, path: []string{"spec", "resetPassword", "newPasswordHash"}},
		{gvk: DexProviderGVK, path: []string{"spec", "github", "clientSecret"}},
		{gvk: DexProviderGVK, path: []string{"spec", "gitlab", "clientSecret"}},
		{gvk: DexProviderGVK, path: []string{"spec", "bitbucketCloud", "clientSecret"}},
		{gvk: DexProviderGVK, path: []string{"spec", "crowd", "clientSecret"}},
		{gvk: DexProviderGVK, path: []string{"spec", "oidc", "clientSecret"}},
		{gvk: DexProviderGVK, path: []string{"spec", "ldap", "bindPW"}},
	} {
		t.Run(tt.gvk.Kind+"/"+strings.Join(tt.path, "."), func(t *testing.T) {
			t.Parallel()

			obj := Object(tt.gvk)
			if _, masked := maskedSensitiveField(obj); masked {
				t.Fatal("an absent field is reported as masked")
			}
			if err := unstructured.SetNestedField(obj.Object, MaskedValue, tt.path...); err != nil {
				t.Fatal(err)
			}
			if path, masked := maskedSensitiveField(obj); !masked || path != strings.Join(tt.path, ".") {
				t.Errorf("maskedSensitiveField = %q, %v, want %q, true", path, masked, strings.Join(tt.path, "."))
			}
			if err := unstructured.SetNestedField(obj.Object, "dh-test-secret-0123456789abcdef", tt.path...); err != nil {
				t.Fatal(err)
			}
			if _, masked := maskedSensitiveField(obj); masked {
				t.Error("an unmasked field is reported as masked")
			}
		})
	}
}

func TestGetUnmaskedAcceptsOrdinaryMaskedText(t *testing.T) {
	t.Parallel()

	provider := Object(DexProviderGVK)
	provider.SetName("ldap")
	provider.Object["spec"] = map[string]any{
		"type":        "LDAP",
		"displayName": MaskedValue,
		"ldap": map[string]any{
			"usernamePrompt": MaskedValue,
		},
	}
	api := &countingReader{Reader: newSensitiveClient(provider)}
	obj := Object(DexProviderGVK)
	err := GetUnmasked(
		t.Context(),
		newSensitiveClient(provider),
		api,
		nil,
		client.ObjectKeyFromObject(provider),
		obj,
	)
	if err != nil {
		t.Fatalf("GetUnmasked: %v", err)
	}
	if api.gets != 0 {
		t.Errorf("reads past the cache = %d, want 0", api.gets)
	}
}
