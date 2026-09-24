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

package grantableclusterresourcedefinition

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery/cached/memory"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/restmapper"
	clienttesting "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	grantsv1alpha1 "controller/api/v1alpha1"
	"controller/internal/jsonpath"
	"controller/internal/resolve"
	"controller/internal/testutil"
)

// testMapper knows the kinds the module ships definitions for, all cluster-scoped, and Secret, which is
// namespaced.
func testMapper() meta.RESTMapper {
	root := []schema.GroupVersionKind{
		{Group: "storage.k8s.io", Version: "v1", Kind: "StorageClass"},
		{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRole"},
		{Group: "cert-manager.io", Version: "v1", Kind: "ClusterIssuer"},
	}
	secret := schema.GroupVersionKind{Version: "v1", Kind: "Secret"}
	var versions []schema.GroupVersion
	for _, gvk := range append(root, secret) {
		versions = append(versions, gvk.GroupVersion())
	}
	m := meta.NewDefaultRESTMapper(versions)
	for _, gvk := range root {
		m.Add(gvk, meta.RESTScopeRoot)
	}
	m.Add(secret, meta.RESTScopeNamespace)
	return m
}

func newValidator() *validator {
	return &validator{factory: jsonpath.NewWithCache(), mapper: testMapper()}
}

// failingMapper fails every mapping with an error that is not "no matches for kind".
type failingMapper struct{ meta.RESTMapper }

func (failingMapper) RESTMapping(schema.GroupKind, ...string) (*meta.RESTMapping, error) {
	return nil, errors.New("discovery is down")
}

func raw(t *testing.T, def *grantsv1alpha1.GrantableClusterResourceDefinition) []byte {
	t.Helper()
	b, err := json.Marshal(def)
	require.NoError(t, err)
	return b
}

func request(t *testing.T, operation admissionv1.Operation, def *grantsv1alpha1.GrantableClusterResourceDefinition) admission.Request {
	t.Helper()
	return admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Operation: operation,
		Name:      def.Name,
		Object:    runtime.RawExtension{Raw: raw(t, def)},
	}}
}

func update(t *testing.T, old, def *grantsv1alpha1.GrantableClusterResourceDefinition) admission.Request {
	t.Helper()
	req := request(t, admissionv1.Update, def)
	req.OldObject = runtime.RawExtension{Raw: raw(t, old)}
	return req
}

// objectBacked is a StorageClass definition with the given catalogFields.
func objectBacked(fields ...grantsv1alpha1.CatalogField) *grantsv1alpha1.GrantableClusterResourceDefinition {
	return &grantsv1alpha1.GrantableClusterResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "storageclasses"},
		Spec: grantsv1alpha1.GrantableClusterResourceDefinitionSpec{
			GrantedResource: &grantsv1alpha1.GrantedResource{APIGroup: "storage.k8s.io", Kind: "StorageClass"},
			CatalogFields:   fields,
		},
	}
}

// valueBacked is a definition without grantedResource, with the given catalogFields.
func valueBacked(fields ...grantsv1alpha1.CatalogField) *grantsv1alpha1.GrantableClusterResourceDefinition {
	def := objectBacked(fields...)
	def.Name = "loadbalancerclasses"
	def.Spec.GrantedResource = nil
	return def
}

func field(name, path string) grantsv1alpha1.CatalogField {
	return grantsv1alpha1.CatalogField{Name: name, Path: path}
}

var (
	good         = field("provisioner", "$.provisioner")
	uncompilable = field("broken", "$.foo-bar")
	notSingular  = field("everything", "$.parameters[*]")
	forbidden    = field("managed", "$.metadata.managedFields[0]")
)

func TestHandle_Refusals(t *testing.T) {
	ctx := context.Background()
	v := newValidator()

	t.Run("allowed: valid paths", func(t *testing.T) {
		resp := v.Handle(ctx, request(t, admissionv1.Create, objectBacked(good, field("expand", "$.allowVolumeExpansion"))))
		assert.True(t, resp.Allowed, resp.Result.Message)
	})

	t.Run("allowed: value-backed without catalogFields", func(t *testing.T) {
		assert.True(t, v.Handle(ctx, request(t, admissionv1.Create, valueBacked())).Allowed)
	})

	for _, tc := range []struct {
		name  string
		def   *grantsv1alpha1.GrantableClusterResourceDefinition
		wants []string
	}{
		{"a path that does not compile", objectBacked(good, uncompilable),
			[]string{`'spec.catalogFields[1].path' "$.foo-bar": not a valid RFC 9535 JSONPath`}},
		{"a path that is not a singular query", objectBacked(notSingular),
			[]string{`'spec.catalogFields[0].path' "$.parameters[*]": not a singular query`}},
		{"a forbidden path", objectBacked(forbidden),
			[]string{`'spec.catalogFields[0].path' "$.metadata.managedFields[0]": overlaps $['metadata']['managedFields']`}},
		{"a parent of a forbidden path", objectBacked(field("meta", "$.metadata")),
			[]string{`'spec.catalogFields[0].path' "$.metadata": overlaps`}},
		{"catalogFields on a value-backed definition", valueBacked(good),
			[]string{`'spec.catalogFields' is set, but the definition is value-backed`}},
	} {
		t.Run("denied: "+tc.name, func(t *testing.T) {
			resp := v.Handle(ctx, request(t, admissionv1.Create, tc.def))
			require.False(t, resp.Allowed)
			assert.Contains(t, resp.Result.Message, "the '"+tc.def.Name+"' GrantableClusterResourceDefinition is invalid: ")
			for _, want := range tc.wants {
				assert.Contains(t, resp.Result.Message, want)
			}
		})
	}

	t.Run("denied: every problem in one refusal, with its index", func(t *testing.T) {
		resp := v.Handle(ctx, request(t, admissionv1.Create, valueBacked(uncompilable, good, notSingular, forbidden)))
		require.False(t, resp.Allowed)
		msg := resp.Result.Message
		for _, want := range []string{
			`'spec.catalogFields' is set, but the definition is value-backed`,
			`'spec.catalogFields[0].path' "$.foo-bar"`,
			`'spec.catalogFields[2].path' "$.parameters[*]"`,
			`'spec.catalogFields[3].path' "$.metadata.managedFields[0]"`,
		} {
			assert.Contains(t, msg, want)
		}
		assert.NotContains(t, msg, "spec.catalogFields[1]")
		assert.Equal(t, 3, strings.Count(msg, "; "), msg)
	})
}

// granting returns a StorageClass-like definition of the given kind.
func granting(group, kind string) *grantsv1alpha1.GrantableClusterResourceDefinition {
	def := objectBacked(good)
	def.Name = strings.ToLower(kind) + "s"
	def.Spec.GrantedResource = &grantsv1alpha1.GrantedResource{APIGroup: group, Kind: kind}
	return def
}

func TestHandle_GrantedResourceScope(t *testing.T) {
	ctx := context.Background()
	v := newValidator()
	secrets := granting("", "Secret")

	t.Run("denied: a namespaced kind", func(t *testing.T) {
		resp := v.Handle(ctx, request(t, admissionv1.Create, secrets))
		require.False(t, resp.Allowed)
		assert.Equal(t, "the 'secrets' GrantableClusterResourceDefinition is invalid: "+
			"'spec.grantedResource' Secret is namespaced: only cluster-scoped resources can be granted", resp.Result.Message)
	})

	t.Run("denied: the namespaced kind first, with the catalogFields problems", func(t *testing.T) {
		def := granting("", "Secret")
		def.Spec.CatalogFields = append(def.Spec.CatalogFields, uncompilable)
		resp := v.Handle(ctx, request(t, admissionv1.Create, def))
		require.False(t, resp.Allowed)
		assert.Regexp(t, `invalid: 'spec.grantedResource' Secret is namespaced: .*; 'spec.catalogFields\[1\].path'`, resp.Result.Message)
	})

	t.Run("allowed: a kind the mapper does not know yet", func(t *testing.T) {
		resp := v.Handle(ctx, request(t, admissionv1.Create, granting("example.com", "Widget")))
		assert.True(t, resp.Allowed, resp.Result.Message)
	})

	t.Run("allowed: the mapping fails for another reason", func(t *testing.T) {
		broken := &validator{factory: jsonpath.NewWithCache(), mapper: failingMapper{}}
		resp := broken.Handle(ctx, request(t, admissionv1.Create, secrets))
		assert.True(t, resp.Allowed, resp.Result.Message)
	})

	t.Run("allowed: metadata-only update of a stored namespaced definition", func(t *testing.T) {
		updated := granting("", "Secret")
		updated.Labels = map[string]string{"edited": "true"}
		assert.True(t, v.Handle(ctx, update(t, secrets, updated)).Allowed)
	})

	t.Run("denied: grantedResource changed to a namespaced kind", func(t *testing.T) {
		updated := objectBacked(good)
		updated.Spec.GrantedResource = &grantsv1alpha1.GrantedResource{Kind: "Secret"}
		resp := v.Handle(ctx, update(t, objectBacked(good), updated))
		require.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "is namespaced")
	})
}

// TestHandle_UnknownKindsDoNotRediscover: with the grants REST mapper, made-up groups in the requests
// cost no discovery beyond the one fill after a reset, a namespaced kind is still refused, and a kind
// served after a reset is seen.
func TestHandle_UnknownKindsDoNotRediscover(t *testing.T) {
	ctx := context.Background()
	disc := &fakediscovery.FakeDiscovery{Fake: &clienttesting.Fake{Resources: []*metav1.APIResourceList{
		{GroupVersion: "v1", APIResources: []metav1.APIResource{{Name: "secrets", Kind: "Secret", Namespaced: true}}},
		{GroupVersion: "storage.k8s.io/v1", APIResources: []metav1.APIResource{{Name: "storageclasses", Kind: "StorageClass"}}},
	}}}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(disc))
	v := &validator{factory: jsonpath.NewWithCache(), mapper: mapper}

	probe := func() {
		for i := range 20 {
			resp := v.Handle(ctx, request(t, admissionv1.Create, granting(fmt.Sprintf("probe%d.example.com", i), "Widget")))
			require.True(t, resp.Allowed, resp.Result.Message)
		}
	}

	require.False(t, v.Handle(ctx, request(t, admissionv1.Create, granting("", "Secret"))).Allowed)
	require.True(t, v.Handle(ctx, request(t, admissionv1.Create, granting("storage.k8s.io", "StorageClass"))).Allowed)
	filled := len(disc.Actions())
	require.NotZero(t, filled)
	probe()
	require.False(t, v.Handle(ctx, request(t, admissionv1.Create, granting("", "Secret"))).Allowed)
	assert.Equal(t, filled, len(disc.Actions()), "misses are answered from the cache")

	disc.Resources = append(disc.Resources, &metav1.APIResourceList{GroupVersion: "late.example.com/v1",
		APIResources: []metav1.APIResource{{Name: "widgets", Kind: "Widget", Namespaced: true}}})
	assert.True(t, v.Handle(ctx, request(t, admissionv1.Create, granting("late.example.com", "Widget"))).Allowed,
		"a kind served after the fill is unknown until the reset")
	assert.Equal(t, filled, len(disc.Actions()))
	mapper.Reset()
	resp := v.Handle(ctx, request(t, admissionv1.Create, granting("late.example.com", "Widget")))
	require.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "is namespaced")
	refilled := len(disc.Actions())
	probe()
	assert.Equal(t, refilled, len(disc.Actions()), "one fill per reset")
}

func TestHandle_Undecodable(t *testing.T) {
	v := newValidator()
	req := request(t, admissionv1.Create, objectBacked(good))
	req.Object.Raw = []byte("{not json")
	resp := v.Handle(context.Background(), req)
	assert.False(t, resp.Allowed)
	assert.EqualValues(t, http.StatusBadRequest, resp.Result.Code)

	req = update(t, objectBacked(good), objectBacked(good))
	req.OldObject.Raw = []byte("{not json")
	resp = v.Handle(context.Background(), req)
	assert.False(t, resp.Allowed)
	assert.EqualValues(t, http.StatusBadRequest, resp.Result.Code)
}

func TestHandle_Ratcheting(t *testing.T) {
	ctx := context.Background()
	v := newValidator()
	stored := objectBacked(good, uncompilable, forbidden)

	t.Run("allowed: metadata-only update of an object with stored invalid entries", func(t *testing.T) {
		updated := objectBacked(good, uncompilable, forbidden)
		updated.Labels = map[string]string{"edited": "true"}
		assert.True(t, v.Handle(ctx, update(t, stored, updated)).Allowed)
	})

	t.Run("allowed: unchanged invalid entries at shifted indexes", func(t *testing.T) {
		updated := objectBacked(forbidden, field("mode", "$.volumeBindingMode"), uncompilable)
		assert.True(t, v.Handle(ctx, update(t, stored, updated)).Allowed)
	})

	t.Run("denied: a new invalid entry next to stored ones", func(t *testing.T) {
		resp := v.Handle(ctx, update(t, stored, objectBacked(good, uncompilable, forbidden, notSingular)))
		require.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "spec.catalogFields[3]")
		assert.NotContains(t, resp.Result.Message, "spec.catalogFields[1]")
		assert.NotContains(t, resp.Result.Message, "spec.catalogFields[2]")
	})

	t.Run("denied: a stored invalid entry changed but still invalid", func(t *testing.T) {
		changed := uncompilable
		changed.Path = "$.foo-baz"
		resp := v.Handle(ctx, update(t, stored, objectBacked(good, changed, forbidden)))
		require.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, `'spec.catalogFields[1].path' "$.foo-baz"`)
	})

	t.Run("allowed: a stored invalid entry fixed", func(t *testing.T) {
		fixed := uncompilable
		fixed.Path = "$.reclaimPolicy"
		assert.True(t, v.Handle(ctx, update(t, stored, objectBacked(good, fixed, forbidden))).Allowed)
	})

	t.Run("denied: invalid entries on CREATE", func(t *testing.T) {
		assert.False(t, v.Handle(ctx, request(t, admissionv1.Create, stored)).Allowed)
	})

	storedValueBacked := valueBacked(good)

	t.Run("allowed: metadata-only update of a stored value-backed definition with catalogFields", func(t *testing.T) {
		updated := valueBacked(good)
		updated.Annotations = map[string]string{"edited": "true"}
		assert.True(t, v.Handle(ctx, update(t, storedValueBacked, updated)).Allowed)
	})

	t.Run("denied: catalogFields changed on a value-backed definition", func(t *testing.T) {
		resp := v.Handle(ctx, update(t, storedValueBacked, valueBacked(good, field("expand", "$.allowVolumeExpansion"))))
		require.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "value-backed")
	})

	t.Run("denied: grantedResource dropped from a definition with catalogFields", func(t *testing.T) {
		resp := v.Handle(ctx, update(t, objectBacked(good), valueBacked(good)))
		require.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "value-backed")
	})

	t.Run("allowed: a value-backed definition fixed by setting grantedResource", func(t *testing.T) {
		assert.True(t, v.Handle(ctx, update(t, storedValueBacked, objectBacked(good))).Allowed)
	})

	t.Run("allowed: an object being deleted, even with a new invalid entry", func(t *testing.T) {
		deleting := valueBacked(good, uncompilable, notSingular)
		now := metav1.Now()
		deleting.DeletionTimestamp = &now
		assert.True(t, v.Handle(ctx, update(t, stored, deleting)).Allowed)
	})
}

// TestHandle_ShippedDefinitions runs every GrantableClusterResourceDefinition the module ships in
// templates/cluster-objects-controller/grantable-resources.yaml through this webhook: the module must
// not be able to block its own Helm release.
func TestHandle_ShippedDefinitions(t *testing.T) {
	defs := testutil.RenderShipped[grantsv1alpha1.GrantableClusterResourceDefinition](t, "GrantableClusterResourceDefinition")

	names := make([]string, 0, len(defs))
	for _, def := range defs {
		names = append(names, def.Name)
	}
	// Guard the rendering itself: a template that renders to nothing must not pass as "nothing to check".
	require.ElementsMatch(t, []string{"storageclasses", "loadbalancerclasses", "clusterissuers", "clusterroles"}, names)

	v := newValidator()
	for _, def := range defs {
		t.Run(def.Name, func(t *testing.T) {
			// The mapper must know the kind: an unknown one is let through, and this test would not see
			// the module ship a namespaced kind.
			_, err := resolve.GrantedResourceProblem(v.mapper, def)
			require.NoError(t, err)
			resp := v.Handle(context.Background(), request(t, admissionv1.Create, def))
			assert.True(t, resp.Allowed, "shipped definition %q must pass validation: %s", def.Name, resp.Result.Message)
		})
	}
}
