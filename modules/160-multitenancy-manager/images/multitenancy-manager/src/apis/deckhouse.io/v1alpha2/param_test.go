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

package v1alpha2

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParamUnmarshal(t *testing.T) {
	t.Run("literal string", func(t *testing.T) {
		var p Param[string]
		require.NoError(t, json.Unmarshal([]byte(`"Baseline"`), &p))
		assert.Empty(t, p.Ref())
		v, ok, err := p.Resolve(nil)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, "Baseline", v)
	})

	t.Run("fromParam reference", func(t *testing.T) {
		var p Param[string]
		require.NoError(t, json.Unmarshal([]byte(`{"fromParam":"podSecurityProfile"}`), &p))
		assert.Equal(t, "podSecurityProfile", p.Ref())
	})

	t.Run("null is zero", func(t *testing.T) {
		var p Param[string]
		require.NoError(t, json.Unmarshal([]byte(`null`), &p))
		assert.True(t, p.IsZero())
	})

	t.Run("literal map is not mistaken for a reference", func(t *testing.T) {
		var p Param[map[string]string]
		require.NoError(t, json.Unmarshal([]byte(`{"team":"a","env":"prod"}`), &p))
		assert.Empty(t, p.Ref())
		v, ok, err := p.Resolve(nil)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, map[string]string{"team": "a", "env": "prod"}, v)
	})

	t.Run("non-string fromParam is rejected", func(t *testing.T) {
		var p Param[string]
		assert.Error(t, json.Unmarshal([]byte(`{"fromParam":123}`), &p))
	})
}

func TestParamMarshalRoundTrip(t *testing.T) {
	for _, raw := range []string{`"Baseline"`, `{"fromParam":"x"}`, `null`} {
		var p Param[string]
		require.NoError(t, json.Unmarshal([]byte(raw), &p))
		out, err := json.Marshal(p)
		require.NoError(t, err)
		assert.JSONEq(t, raw, string(out))
	}
}

func TestParamResolve(t *testing.T) {
	params := map[string]any{
		"podSecurityProfile": "Restricted",
		"namespace": map[string]any{
			"labels": map[string]any{"team": "a"},
		},
		"nullable": nil,
	}

	t.Run("scalar from params", func(t *testing.T) {
		p := FromParamRef[string]("podSecurityProfile")
		v, ok, err := p.Resolve(params)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, "Restricted", v)
	})

	t.Run("dotted path into nested map", func(t *testing.T) {
		p := FromParamRef[map[string]string]("namespace.labels")
		v, ok, err := p.Resolve(params)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, map[string]string{"team": "a"}, v)
	})

	t.Run("missing parameter is unset, not an error", func(t *testing.T) {
		p := FromParamRef[string]("absent")
		_, ok, err := p.Resolve(params)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("null parameter is unset", func(t *testing.T) {
		p := FromParamRef[string]("nullable")
		_, ok, err := p.Resolve(params)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("type mismatch is an error", func(t *testing.T) {
		p := FromParamRef[int64]("podSecurityProfile")
		_, _, err := p.Resolve(params)
		assert.Error(t, err)
	})
}

func TestFromParamRefs(t *testing.T) {
	spec := ProjectTemplateSpec{
		PodSecurityStandard: FromParamRef[string]("podSecurityProfile"),
		NetworkPolicy:       &NetworkPolicySpec{Mode: FromParamRef[string]("networkPolicy")},
		NamespaceMetadata: &NamespaceMetadata{
			Labels:      FromParamRef[map[string]string]("namespace.labels"),
			Annotations: LiteralParam(map[string]string{"a": "b"}),
		},
		Features: &FeaturesSpec{Monitoring: FromParamRef[bool]("extendedMonitoringEnabled")},
	}

	refs := spec.FromParamRefs()
	got := map[string]string{}
	for _, r := range refs {
		got[r.Field] = r.Param
	}

	assert.Equal(t, map[string]string{
		"podSecurityStandard":      "podSecurityProfile",
		"networkPolicy.mode":       "networkPolicy",
		"namespaceMetadata.labels": "namespace.labels",
		"features.monitoring":      "extendedMonitoringEnabled",
	}, got)
}
