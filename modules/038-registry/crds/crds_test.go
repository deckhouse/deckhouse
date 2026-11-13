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
	apiextensionsvalidation "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/validation"
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
	"registryconfig.yaml":    {name: "registryconfigs.deckhouse.io", kind: "RegistryConfig"},
	"registrystorage.yaml":   {name: "registrystorages.deckhouse.io", kind: "RegistryStorage"},
	"registryupstream.yaml":  {name: "registryupstreams.deckhouse.io", kind: "RegistryUpstream"},
	"registrynode.yaml":      {name: "registrynodes.deckhouse.io", kind: "RegistryNode"},
	"registryoperation.yaml": {name: "registryoperations.deckhouse.io", kind: "RegistryOperation"},
}

// stripDeckhouseMarkers removes the schema extensions that are understood by
// Deckhouse but are not fields of the upstream JSONSchemaProps Go type used by
// this test. Strict decoding remains in place for everything else, so a real
// typo such as `descriptionn` still fails instead of being silently dropped.
func stripDeckhouseMarkers(node any) any {
	switch v := node.(type) {
	case map[string]any:
		for key, value := range v {
			if strings.HasPrefix(key, "x-doc-") || key == "x-kubernetes-sensitive-data" {
				delete(v, key)
				continue
			}
			v[key] = stripDeckhouseMarkers(value)
		}
		return v
	case []any:
		for i := range v {
			v[i] = stripDeckhouseMarkers(v[i])
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

	stripped, err := json.Marshal(stripDeckhouseMarkers(tree))
	require.NoError(t, err)

	crd := &apiextensionsv1.CustomResourceDefinition{}
	require.NoError(t, yaml.UnmarshalStrict(stripped, crd), "%s is not a valid CustomResourceDefinition", file)
	return crd
}

func rawSpecProperties(t *testing.T, file string) map[string]any {
	t.Helper()

	data, err := os.ReadFile(file)
	require.NoError(t, err)

	var tree map[string]any
	require.NoError(t, yaml.Unmarshal(data, &tree), "%s is not valid YAML", file)

	spec, ok := tree["spec"].(map[string]any)
	require.True(t, ok, "%s has no spec", file)
	versions, ok := spec["versions"].([]any)
	require.True(t, ok, "%s has no spec.versions", file)
	require.Len(t, versions, 1, "%s must have exactly one version", file)
	version, ok := versions[0].(map[string]any)
	require.True(t, ok)
	properties, ok := version["schema"].(map[string]any)["openAPIV3Schema"].(map[string]any)["properties"].(map[string]any)["spec"].(map[string]any)["properties"].(map[string]any)
	require.True(t, ok, "cannot reach spec.properties in %s", file)
	return properties
}

func schemaProperty(t *testing.T, root map[string]any, path ...string) map[string]any {
	t.Helper()

	var current any = root
	for _, segment := range path {
		object, ok := current.(map[string]any)
		require.True(t, ok, "schema path %s reaches a non-object before %s", strings.Join(path, "."), segment)
		current, ok = object[segment]
		require.True(t, ok, "schema path %s is missing %s", strings.Join(path, "."), segment)
	}
	property, ok := current.(map[string]any)
	require.True(t, ok, "schema path %s is not an object", strings.Join(path, "."))
	return property
}

func TestRegistryAuthenticationFieldsAreSensitive(t *testing.T) {
	tests := []struct {
		file      string
		authRoots [][]string
	}{
		{
			file: "registryconfig.yaml",
			authRoots: [][]string{
				{"primary", "properties", "upstream", "properties", "auth", "properties"},
				{"primary", "properties", "upstream", "properties", "mirrors", "items", "properties", "auth", "properties"},
			},
		},
		{
			file: "registryupstream.yaml",
			authRoots: [][]string{
				{"upstream", "properties", "auth", "properties"},
				{"upstream", "properties", "mirrors", "items", "properties", "auth", "properties"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.file, func(t *testing.T) {
			properties := rawSpecProperties(t, test.file)
			for _, root := range test.authRoots {
				for _, field := range []string{"password", "auth"} {
					path := append(append([]string{}, root...), field)
					assert.Equal(t, true, schemaProperty(t, properties, path...)["x-kubernetes-sensitive-data"],
						"spec.%s must be masked without access to the sensitive subresource", strings.Join(path, "."))
				}

				usernamePath := append(append([]string{}, root...), "username")
				assert.NotContains(t, schemaProperty(t, properties, usernamePath...), "x-kubernetes-sensitive-data",
					"spec.%s must stay visible", strings.Join(usernamePath, "."))
			}
		})
	}
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
		"registryconfig.yaml":    {"status", "conditions"},
		"registryupstream.yaml":  {"status", "conditions"},
		"registrystorage.yaml":   {"status", "replicas"},
		"registrynode.yaml":      {"spec", "backends"},
		"registryoperation.yaml": {"status", "replicas"},
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

// TestStorageReplicasCarryTheHealthError: the syncers tell a replica that cannot serve from one whose
// copy failed by this field alone. Missing from the schema, the API server prunes it without a word,
// and every replica looks healthy to the others when nobody holds the whole set and nobody leads.
func TestStorageReplicasCarryTheHealthError(t *testing.T) {
	s := structuralOf(t, loadCRD(t, "registrystorage.yaml"), "v1alpha1")

	status, ok := s.Properties["status"]
	require.True(t, ok)
	replicas, ok := status.Properties["replicas"]
	require.True(t, ok)
	require.NotNil(t, replicas.Items)

	for _, field := range []string{"error", "healthError"} {
		property, ok := replicas.Items.Properties[field]
		require.Truef(t, ok, "RegistryStorage.status.replicas[] must carry %q", field)
		assert.Equal(t, "string", property.Type)
	}
}

// TestStorageReplicasCarryTheSyncerStart: the controller drops the upstream only on a lease holder's
// entry stamped by the syncer running on its node now. A schema without the field has the API server
// prune the stamp from every report, and the transition is then held for good.
func TestStorageReplicasCarryTheSyncerStart(t *testing.T) {
	s := structuralOf(t, loadCRD(t, "registrystorage.yaml"), "v1alpha1")

	status, ok := s.Properties["status"]
	require.True(t, ok)
	replicas, ok := status.Properties["replicas"]
	require.True(t, ok)
	require.NotNil(t, replicas.Items)

	property, ok := replicas.Items.Properties["syncerStartedAt"]
	require.True(t, ok, "RegistryStorage.status.replicas[] must carry syncerStartedAt")
	assert.Equal(t, "string", property.Type)
	require.NotNil(t, property.ValueValidation)
	assert.Equal(t, "date-time", property.ValueValidation.Format)
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

// validateUpdate runs the CEL rules of the CRD against an update from old to obj, so the transition
// rules — the ones that read oldSelf — are exercised too.
func validateUpdate(t *testing.T, s *structuralschema.Structural, obj, old map[string]any) field.ErrorList {
	t.Helper()

	validator := structuralcel.NewValidator(s, true, celconfig.PerCallLimit)
	require.NotNil(t, validator, "no CEL rules found in the schema")

	errs, _ := validator.Validate(
		context.Background(), field.NewPath(""), s, obj, old, celconfig.RuntimeCELCostBudget,
	)
	return errs
}

// TestRegistryOperationTransitions keeps an operation what it was: the spec does not change after
// creation, and a finished operation is not reopened by a late writer — a replica reporting after the
// controller has already given up on it.
func TestRegistryOperationTransitions(t *testing.T) {
	s := structuralOf(t, loadCRD(t, "registryoperation.yaml"), "v1alpha1")

	gc := map[string]any{"type": "GarbageCollection", "cause": "Scheduled"}
	status := func(phase string) map[string]any {
		return map[string]any{"spec": gc, "status": map[string]any{"phase": phase}}
	}

	tests := []struct {
		name     string
		old, obj map[string]any
		wantErr  string
	}{
		{name: "pending to running", old: status("Pending"), obj: status("Running")},
		{name: "running to succeeded", old: status("Running"), obj: status("Succeeded")},
		{name: "running to canceled", old: status("Running"), obj: status("Canceled")},
		{name: "first status", old: map[string]any{"spec": gc}, obj: status("Pending")},
		{
			name: "succeeded reopened", old: status("Succeeded"), obj: status("Running"),
			wantErr: "a finished operation cannot be reopened",
		},
		{
			name: "failed reopened", old: status("Failed"), obj: status("Pending"),
			wantErr: "a finished operation cannot be reopened",
		},
		{
			name: "canceled turned into success", old: status("Canceled"), obj: status("Succeeded"),
			wantErr: "a finished operation cannot be reopened",
		},
		{
			name: "cause changed", old: map[string]any{"spec": gc},
			obj:     map[string]any{"spec": map[string]any{"type": "GarbageCollection", "cause": "Manual"}},
			wantErr: "the spec of an operation is immutable",
		},
		{
			name: "cause added", old: map[string]any{"spec": map[string]any{"type": "GarbageCollection"}},
			obj:     map[string]any{"spec": gc},
			wantErr: "the spec of an operation is immutable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validateUpdate(t, s, tt.obj, tt.old)
			if tt.wantErr == "" {
				assert.Empty(t, errs, "update should be accepted")
				return
			}
			require.NotEmpty(t, errs, "update should be rejected")
			assert.Contains(t, errs.ToAggregate().Error(), tt.wantErr)
		})
	}
}

// TestRegistryOperationIsSelectableByTypeAndPhase: a UI lists the operations of one type, or the ones
// still running, without reading them all.
func TestRegistryOperationIsSelectableByTypeAndPhase(t *testing.T) {
	crd := loadCRD(t, "registryoperation.yaml")

	var paths []string
	for _, field := range crd.Spec.Versions[0].SelectableFields {
		paths = append(paths, field.JSONPath)
	}
	assert.ElementsMatch(t, []string{".spec.type", ".status.phase"}, paths)
}

// TestCRDsAreAcceptedByTheAPIServer runs the validation the API server runs when a CRD is created,
// the CEL cost estimate included.
//
// Compiling and executing the rules, as the tests above do, says nothing about their estimated cost,
// and the cost is checked only on create: a rule comparing optional strings drops their maxLength from
// the estimate, the CRD is refused, and the module's ensure-CRDs task fails until Deckhouse never gets
// Ready — which is how RegistryOperation first met a cluster.
func TestCRDsAreAcceptedByTheAPIServer(t *testing.T) {
	for file := range expectedCRDs {
		t.Run(file, func(t *testing.T) {
			crd := loadCRD(t, file)
			// The defaults the API server fills in before it validates: listKind among them.
			apiextensionsv1.SetObjectDefaults_CustomResourceDefinition(crd)

			internal := &apiextensionsinternal.CustomResourceDefinition{}
			require.NoError(t, apiextensionsv1.Convert_v1_CustomResourceDefinition_To_apiextensions_CustomResourceDefinition(crd, internal, nil))
			// What the API server holds for a CRD on create: the one version stored.
			internal.Status.StoredVersions = []string{internal.Spec.Versions[0].Name}

			errs := apiextensionsvalidation.ValidateCustomResourceDefinition(context.Background(), internal)
			assert.Empty(t, errs.ToAggregate(), "the API server would refuse %s", file)
		})
	}
}

// TestRegistryUpstreamLimits pins the per-field limits every copy of the spec shares: the
// arbitration (go_lib/registry/apis/upstreams, Max*) and NodeConfig spec.registry.upstreams
// declare the same, so an object the API server stores is never one a node refuses.
func TestRegistryUpstreamLimits(t *testing.T) {
	crd := loadCRD(t, "registryupstream.yaml")
	spec := crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"]
	upstream := spec.Properties["upstream"]
	mirror := upstream.Properties["mirrors"].Items.Schema

	maxLength := func(props apiextensionsv1.JSONSchemaProps) int64 {
		require.NotNil(t, props.MaxLength)
		return *props.MaxLength
	}
	assert.Equal(t, int64(255), maxLength(spec.Properties["match"]))
	for name, endpoint := range map[string]apiextensionsv1.JSONSchemaProps{"upstream": upstream, "mirror": *mirror} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, int64(255), maxLength(endpoint.Properties["host"]))
			assert.Equal(t, int64(1024), maxLength(endpoint.Properties["path"]))
			assert.Equal(t, int64(65536), maxLength(endpoint.Properties["ca"]))
			auth := endpoint.Properties["auth"]
			assert.Equal(t, int64(1024), maxLength(auth.Properties["username"]))
			assert.Equal(t, int64(4096), maxLength(auth.Properties["password"]))
			assert.Equal(t, int64(4096), maxLength(auth.Properties["auth"]))
		})
	}
	require.NotNil(t, upstream.Properties["mirrors"].MaxItems)
	assert.Equal(t, int64(8), *upstream.Properties["mirrors"].MaxItems)
}

// TestAVerifyAndARepairSayWhatTheyAreAbout: a Repair without digests is nothing to run, a digest list on
// a garbage collection is a request nobody would honour, and neither may change once created.
func TestAVerifyAndARepairSayWhatTheyAreAbout(t *testing.T) {
	s := structuralOf(t, loadCRD(t, "registryoperation.yaml"), "v1alpha1")
	digest := "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	for name, spec := range map[string]map[string]any{
		"a verify of everything": {"type": "Verify"},
		"a verify of one node":   {"type": "Verify", "verify": map[string]any{"nodes": []any{"master-0"}}},
		"a repair":               {"type": "Repair", "repair": map[string]any{"digests": []any{digest}}},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Empty(t, validateSpec(t, s, spec))
		})
	}

	for name, tt := range map[string]struct {
		spec    map[string]any
		wantErr string
	}{
		"a repair of nothing": {
			spec: map[string]any{"type": "Repair"}, wantErr: "spec.repair is required for a Repair",
		},
		"a collection with a repair": {
			spec:    map[string]any{"type": "GarbageCollection", "repair": map[string]any{"digests": []any{digest}}},
			wantErr: "spec.repair is required for a Repair and allowed for nothing else",
		},
		"a repair with a verify": {
			spec: map[string]any{
				"type": "Repair", "repair": map[string]any{"digests": []any{digest}}, "verify": map[string]any{},
			},
			wantErr: "spec.verify is allowed only for a Verify",
		},
	} {
		t.Run(name, func(t *testing.T) {
			errs := validateSpec(t, s, tt.spec)
			require.NotEmpty(t, errs)
			assert.Contains(t, errs.ToAggregate().Error(), tt.wantErr)
		})
	}

	repair := func(digests ...any) map[string]any {
		return map[string]any{"spec": map[string]any{"type": "Repair", "repair": map[string]any{"digests": digests}}}
	}
	other := "sha256:fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
	errs := validateUpdate(t, s, repair(digest, other), repair(digest))
	require.NotEmpty(t, errs, "the digests of a repair do not change")
	assert.Contains(t, errs.ToAggregate().Error(), "the spec of an operation is immutable")
	assert.Empty(t, validateUpdate(t, s, repair(digest), repair(digest)))
}
