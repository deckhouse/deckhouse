// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// fakePatchingValidator stands in for vcd: the only provider that patches its
// own parsed config through the optional PatchProviderClusterConfig method.
type fakePatchingValidator struct {
	patch map[string]any
}

func (p *fakePatchingValidator) Validate(_ context.Context, _ ProviderInput) error {
	return nil
}

func (p *fakePatchingValidator) PatchProviderClusterConfig(_ context.Context, _ ProviderInput) (map[string]any, error) {
	return p.patch, nil
}

func fakeValidatorProvider(v MetaConfigValidator) MetaConfigValidatorProvider {
	return func(_ context.Context, _ string) MetaConfigValidator { return v }
}

func TestPatchMergeContract(t *testing.T) {
	validator := &fakePatchingValidator{patch: map[string]interface{}{
		"replaced": map[string]interface{}{"new": true},
		"added":    "fresh",
	}}

	m := &MetaConfig{
		ProviderName: "mergetest",
		ProviderClusterConfig: map[string]json.RawMessage{
			"kept":     json.RawMessage(`{"old":1}`),
			"replaced": json.RawMessage(`{"old":1,"gone":"yes"}`),
		},
	}

	_, err := validateProviderConfig(context.Background(), fakeValidatorProvider(validator), m)
	require.NoError(t, err)

	require.JSONEq(t, `{"old":1}`, string(m.ProviderClusterConfig["kept"]), "keys absent from the result must stay untouched")
	require.JSONEq(t, `{"new":true}`, string(m.ProviderClusterConfig["replaced"]), "returned keys must replace the old value wholesale")
	require.JSONEq(t, `"fresh"`, string(m.ProviderClusterConfig["added"]))
}

func TestPatchRevalidatesAgainstSchema(t *testing.T) {
	dir := t.TempDir()
	writeTestProviderSchema(t, dir, "MutProvConfiguration")
	// NewSchemaStore is a process-wide singleton: loading here is what makes
	// the revalidation inside validateProviderConfig see the schema.
	store := NewSchemaStore(nil)
	require.NoError(t, store.LoadProviderDir("mutprov", "sha256:mut1", dir))

	validator := &fakePatchingValidator{patch: map[string]interface{}{
		"bogus": "not allowed by additionalProperties: false",
	}}

	m := &MetaConfig{
		ProviderName: "mutprov",
		ProviderClusterConfig: map[string]json.RawMessage{
			"apiVersion": json.RawMessage(`"deckhouse.io/v1"`),
			"kind":       json.RawMessage(`"MutProvConfiguration"`),
			"layout":     json.RawMessage(`"Standard"`),
		},
	}

	_, err := validateProviderConfig(context.Background(), fakeValidatorProvider(validator), m)
	require.Error(t, err)
	require.Contains(t, err.Error(), "patched provider cluster configuration into an invalid state")
	require.Contains(t, err.Error(), "bogus")
}

func TestPatchValidResultPasses(t *testing.T) {
	dir := t.TempDir()
	writeTestProviderSchema(t, dir, "MutProvOkConfiguration")
	store := NewSchemaStore(nil)
	require.NoError(t, store.LoadProviderDir("mutprovok", "sha256:mut2", dir))

	validator := &fakePatchingValidator{patch: map[string]interface{}{
		"layout": "Amended",
	}}

	m := &MetaConfig{
		ProviderName: "mutprovok",
		ProviderClusterConfig: map[string]json.RawMessage{
			"apiVersion": json.RawMessage(`"deckhouse.io/v1"`),
			"kind":       json.RawMessage(`"MutProvOkConfiguration"`),
			"layout":     json.RawMessage(`"Standard"`),
		},
	}

	_, err := validateProviderConfig(context.Background(), fakeValidatorProvider(validator), m)
	require.NoError(t, err)
	require.JSONEq(t, `"Amended"`, string(m.ProviderClusterConfig["layout"]))
}
