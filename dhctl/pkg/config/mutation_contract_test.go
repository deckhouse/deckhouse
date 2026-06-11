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

	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/providerdata"
)

type fakeMutatingPreparator struct {
	result providerdata.PrepareResult
}

func (p *fakeMutatingPreparator) Validate(_ context.Context, _ ProviderInput) error {
	return nil
}

func (p *fakeMutatingPreparator) Prepare(_ context.Context, _ ProviderInput) (providerdata.PrepareResult, error) {
	return p.result, nil
}

func fakePreparatorProvider(p MetaConfigPreparator) MetaConfigPreparatorProvider {
	return func(_, _ string) MetaConfigPreparator { return p }
}

func TestPrepareMutationMergeContract(t *testing.T) {
	vars := &providerdata.CloudProviderVars{Settings: map[string]interface{}{"zone": "b"}}
	preparator := &fakeMutatingPreparator{result: providerdata.PrepareResult{
		Vars: vars,
		ProviderClusterConfig: map[string]interface{}{
			"replaced": map[string]interface{}{"new": true},
			"added":    "fresh",
		},
	}}

	m := &MetaConfig{
		ProviderName: "mergetest",
		ProviderClusterConfig: map[string]json.RawMessage{
			"kept":     json.RawMessage(`{"old":1}`),
			"replaced": json.RawMessage(`{"old":1,"gone":"yes"}`),
		},
	}

	_, err := validateAndPrepareMetaConfig(context.Background(), fakePreparatorProvider(preparator), m)
	require.NoError(t, err)

	require.JSONEq(t, `{"old":1}`, string(m.ProviderClusterConfig["kept"]), "keys absent from the result must stay untouched")
	require.JSONEq(t, `{"new":true}`, string(m.ProviderClusterConfig["replaced"]), "returned keys must replace the old value wholesale")
	require.JSONEq(t, `"fresh"`, string(m.ProviderClusterConfig["added"]))
	require.Same(t, vars, m.CloudProviderVars, "non-nil Vars must replace CloudProviderVars wholesale")
}

func TestPrepareMutationRevalidatesAgainstSchema(t *testing.T) {
	dir := t.TempDir()
	writeTestProviderSchema(t, dir, "MutProvConfiguration")
	// NewSchemaStore is a process-wide singleton: loading here is what makes
	// the revalidation inside validateAndPrepareMetaConfig see the schema.
	store := NewSchemaStore(nil)
	require.NoError(t, store.LoadProviderDir("mutprov", "sha256:mut1", dir))

	preparator := &fakeMutatingPreparator{result: providerdata.PrepareResult{
		ProviderClusterConfig: map[string]interface{}{
			"bogus": "not allowed by additionalProperties: false",
		},
	}}

	m := &MetaConfig{
		ProviderName: "mutprov",
		ProviderClusterConfig: map[string]json.RawMessage{
			"apiVersion": json.RawMessage(`"deckhouse.io/v1"`),
			"kind":       json.RawMessage(`"MutProvConfiguration"`),
			"layout":     json.RawMessage(`"Standard"`),
		},
	}

	_, err := validateAndPrepareMetaConfig(context.Background(), fakePreparatorProvider(preparator), m)
	require.Error(t, err)
	require.Contains(t, err.Error(), "mutated provider cluster configuration into an invalid state")
	require.Contains(t, err.Error(), "bogus")
}

func TestPrepareMutationValidResultPasses(t *testing.T) {
	dir := t.TempDir()
	writeTestProviderSchema(t, dir, "MutProvOkConfiguration")
	store := NewSchemaStore(nil)
	require.NoError(t, store.LoadProviderDir("mutprovok", "sha256:mut2", dir))

	preparator := &fakeMutatingPreparator{result: providerdata.PrepareResult{
		ProviderClusterConfig: map[string]interface{}{
			"layout": "Amended",
		},
	}}

	m := &MetaConfig{
		ProviderName: "mutprovok",
		ProviderClusterConfig: map[string]json.RawMessage{
			"apiVersion": json.RawMessage(`"deckhouse.io/v1"`),
			"kind":       json.RawMessage(`"MutProvOkConfiguration"`),
			"layout":     json.RawMessage(`"Standard"`),
		},
	}

	_, err := validateAndPrepareMetaConfig(context.Background(), fakePreparatorProvider(preparator), m)
	require.NoError(t, err)
	require.JSONEq(t, `"Amended"`, string(m.ProviderClusterConfig["layout"]))
}
