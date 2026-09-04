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

// Package crds_test validates the registry CRD manifests the way the API server
// would: the schemas must be structural, the CEL rules must compile, and the
// rules must actually reject what their messages claim they reject.
//
// The CEL rules are the part of a hand-written CRD most likely to be subtly
// wrong — "has(x) || y" reads fine and can still pass every case it was meant to
// catch — so they are exercised against a table of specs rather than merely
// compiled.
package crds_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsinternal "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	structuralcel "k8s.io/apiextensions-apiserver/pkg/apiserver/schema/cel"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/yaml"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
)

// expectedCRDs maps a manifest file to the CRD it must define. Listing them
// explicitly means a renamed or forgotten file fails the test instead of being
// silently skipped.
var expectedCRDs = map[string]struct {
	name string
	kind string
}{
	"registryconfig.yaml":   {name: "registryconfigs.deckhouse.io", kind: "RegistryConfig"},
	"registrystorage.yaml":  {name: "registrystorages.deckhouse.io", kind: "RegistryStorage"},
	"registryupstream.yaml": {name: "registryupstreams.deckhouse.io", kind: "RegistryUpstream"},
	"registrynode.yaml":     {name: "registrynodes.deckhouse.io", kind: "RegistryNode"},
}

// stripDocMarkers removes the `x-doc-*` keys the Deckhouse documentation tooling
// reads out of the manifest files. They are not fields of JSONSchemaProps, so the
// API server prunes them on decode; stripping them here reproduces that while
// leaving strict decoding in place for everything else, so a real typo such as
// `descriptionn` still fails the test instead of being silently dropped.
func stripDocMarkers(node any) any {
	switch v := node.(type) {
	case map[string]any:
		for key, value := range v {
			if strings.HasPrefix(key, "x-doc-") {
				delete(v, key)
				continue
			}
			v[key] = stripDocMarkers(value)
		}
		return v
	case []any:
		for i := range v {
			v[i] = stripDocMarkers(v[i])
		}
		return v
	default:
		return node
	}
}

func loadCRD(t *testing.T, file string) *apiextensionsv1.CustomResourceDefinition {
	t.Helper()

	data, err := os.ReadFile(file)
	require.NoError(t, err)

	var tree map[string]any
	require.NoError(t, yaml.Unmarshal(data, &tree), "%s is not valid YAML", file)

	stripped, err := json.Marshal(stripDocMarkers(tree))
	require.NoError(t, err)

	crd := &apiextensionsv1.CustomResourceDefinition{}
	require.NoError(t, yaml.UnmarshalStrict(stripped, crd), "%s is not a valid CustomResourceDefinition", file)
	return crd
}

// structuralOf converts a served version's schema into the structural form the
// API server builds internally. Anything the conversion or the structural
// validation rejects would be rejected on `kubectl apply` of the CRD itself.
func structuralOf(t *testing.T, crd *apiextensionsv1.CustomResourceDefinition, version string) *structuralschema.Structural {
	t.Helper()

	var v1Schema *apiextensionsv1.JSONSchemaProps
	for i := range crd.Spec.Versions {
		if crd.Spec.Versions[i].Name == version {
			require.NotNil(t, crd.Spec.Versions[i].Schema, "version %s has no schema", version)
			v1Schema = crd.Spec.Versions[i].Schema.OpenAPIV3Schema
		}
	}
	require.NotNil(t, v1Schema, "version %s not found", version)

	internal := &apiextensionsinternal.JSONSchemaProps{}
	require.NoError(t, apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(v1Schema, internal, nil))

	s, err := structuralschema.NewStructural(internal)
	require.NoError(t, err, "schema is not structural")
	require.Empty(t, structuralschema.ValidateStructural(nil, s).ToAggregate(), "schema failed structural validation")

	return s
}

func TestCRDsAreStructuralAndComplete(t *testing.T) {
	files, err := filepath.Glob("*.yaml")
	require.NoError(t, err)

	seen := make(map[string]bool, len(expectedCRDs))

	for _, file := range files {
		base := filepath.Base(file)
		if len(base) > 7 && base[:7] == "doc-ru-" {
			// Russian documentation overlays are read by the docs tooling, not by
			// the API server, and are deliberately partial.
			continue
		}

		want, ok := expectedCRDs[base]
		require.Truef(t, ok, "%s is not listed in expectedCRDs; add it or remove the file", base)
		seen[base] = true

		t.Run(base, func(t *testing.T) {
			crd := loadCRD(t, file)

			assert.Equal(t, want.name, crd.Name)
			assert.Equal(t, want.kind, crd.Spec.Names.Kind)
			assert.Equal(t, "deckhouse.io", crd.Spec.Group)
			assert.Equal(t, apiextensionsv1.ClusterScoped, crd.Spec.Scope,
				"registry CRDs are cluster-scoped: the agent reads its object by node name")
			assert.Equal(t, "deckhouse", crd.Labels["heritage"])
			assert.Equal(t, "registry", crd.Labels["module"])

			require.Len(t, crd.Spec.Versions, 1)
			version := crd.Spec.Versions[0]
			assert.Equal(t, "v1alpha1", version.Name)
			assert.True(t, version.Served)
			assert.True(t, version.Storage)
			require.NotNil(t, version.Subresources, "status must be a subresource: spec and status have different writers")
			require.NotNil(t, version.Subresources.Status)

			structuralOf(t, crd, version.Name)
		})
	}

	for base := range expectedCRDs {
		assert.Truef(t, seen[base], "%s is missing from the crds directory", base)
	}
}

// validateSpec runs the CEL rules of the CRD against a spec, exactly as the API
// server does on write.
func validateSpec(t *testing.T, s *structuralschema.Structural, spec map[string]any) field.ErrorList {
	t.Helper()

	obj := map[string]any{"spec": spec}
	validator := structuralcel.NewValidator(s, true, celconfig.PerCallLimit)
	require.NotNil(t, validator, "no CEL rules found in the schema")

	errs, _ := validator.Validate(
		context.Background(), field.NewPath(""), s, obj, nil, celconfig.RuntimeCELCostBudget,
	)
	return errs
}

// TestRegistryConfigTwoAxes covers the whole truth table of the two
// configuration axes that replaced the old mode state machine, including the
// fourth combination that leaves nodes with no source of images.
func TestRegistryConfigTwoAxes(t *testing.T) {
	s := structuralOf(t, loadCRD(t, "registryconfig.yaml"), "v1alpha1")

	upstream := map[string]any{"scheme": "HTTPS", "host": "registry.deckhouse.io", "path": "/deckhouse/ee"}
	source := map[string]any{"bundleRef": "d8-mirror-bundle", "expectedDigests": int64(459)}

	tests := []struct {
		name    string
		spec    map[string]any
		wantErr string
	}{
		{
			name: "cache off with upstream forwards directly",
			spec: map[string]any{
				"mode":    "Managed",
				"primary": map[string]any{"upstream": upstream},
				"storage": map[string]any{"cache": false},
			},
		},
		{
			name: "cache on with upstream is a pass-through cache",
			spec: map[string]any{
				"mode":    "Managed",
				"primary": map[string]any{"upstream": upstream},
				"storage": map[string]any{"cache": true, "size": "50Gi"},
			},
		},
		{
			name: "cache on without upstream is air-gap and needs a source",
			spec: map[string]any{
				"mode":    "Managed",
				"storage": map[string]any{"cache": true, "source": source},
			},
		},
		{
			name: "air-gap without a source is rejected",
			spec: map[string]any{
				"mode":    "Managed",
				"storage": map[string]any{"cache": true},
			},
			wantErr: "storage.source is required",
		},
		{
			name: "no upstream and no cache leaves nothing to pull from",
			spec: map[string]any{
				"mode":    "Managed",
				"storage": map[string]any{"cache": false},
			},
			wantErr: "needs a source of images",
		},
		{
			name:    "an empty Managed spec is rejected too",
			spec:    map[string]any{"mode": "Managed"},
			wantErr: "needs a source of images",
		},
		{
			name: "an empty primary object is rejected the same as an absent one",
			spec: map[string]any{
				"mode":    "Managed",
				"primary": map[string]any{},
				"storage": map[string]any{"cache": false},
			},
			wantErr: "needs a source of images",
		},
		{
			name: "Unmanaged needs nothing: the design must survive the absence of configuration",
			spec: map[string]any{"mode": "Unmanaged"},
		},
		{
			name: "Unmanaged with an empty storage is still fine",
			spec: map[string]any{"mode": "Unmanaged", "storage": map[string]any{"cache": false}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validateSpec(t, s, tt.spec)

			if tt.wantErr == "" {
				assert.Empty(t, errs.ToAggregate(), "spec should be accepted")
				return
			}

			require.NotEmpty(t, errs, "spec should be rejected")
			assert.Contains(t, errs.ToAggregate().Error(), tt.wantErr)
		})
	}
}

// TestRegistryConfigListMapKeys guards the merge semantics of the status
// conditions: without a listType=map the conditions of different writers would
// clobber each other on apply.
func TestRegistryConfigListMapKeys(t *testing.T) {
	for file, path := range map[string][]string{
		"registryconfig.yaml":   {"status", "conditions"},
		"registryupstream.yaml": {"status", "conditions"},
		"registrystorage.yaml":  {"status", "replicas"},
		"registrynode.yaml":     {"spec", "backends"},
	} {
		t.Run(file, func(t *testing.T) {
			s := structuralOf(t, loadCRD(t, file), "v1alpha1")

			node := s
			for _, key := range path {
				require.Contains(t, node.Properties, key)
				next := node.Properties[key]
				node = &next
			}

			require.NotNil(t, node.XListType, "%s.%v must declare a list type", file, path)
			assert.Equal(t, "map", *node.XListType)
			assert.NotEmpty(t, node.XListMapKeys)
		})
	}
}

// TestRegistryNodeCarriesNoStorageCompleteness locks in the invariant that
// completeness is a property of the storage and is reported in exactly one
// place. A node-side "full" field would invite gating the air-gap transition on
// the wrong object.
func TestRegistryNodeCarriesNoStorageCompleteness(t *testing.T) {
	s := structuralOf(t, loadCRD(t, "registrynode.yaml"), "v1alpha1")

	status, ok := s.Properties["status"]
	require.True(t, ok)

	for _, forbidden := range []string{"full", "filled", "verifiedDigests", "safeToDropUpstream", "authoritative", "replicas"} {
		assert.NotContainsf(t, status.Properties, forbidden,
			"RegistryNode.status must not carry storage completeness (%q); it lives in RegistryStorage.status.replicas", forbidden)
	}
}

// TestRegistryUpstreamRequiresMatchAndUpstream keeps the multi-writer contract
// honest: an upstream without a match would be routed nowhere, and a match
// without an upstream would intercept pulls and drop them.
func TestRegistryUpstreamRequiresMatchAndUpstream(t *testing.T) {
	crd := loadCRD(t, "registryupstream.yaml")
	spec := crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"]

	assert.ElementsMatch(t, []string{"match", "upstream"}, spec.Required)
	assert.Equal(t, []string{"host"}, spec.Properties["upstream"].Required)
}
