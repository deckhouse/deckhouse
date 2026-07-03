/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package configapplier

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/require"

	deckhousev1alpha1 "integrity-controller/api/deckhouse.io/v1alpha1"
)

func TestAggregatePolicies(t *testing.T) {
	t.Parallel()

	ca := "-----BEGIN CERTIFICATE-----\nabc\n-----END CERTIFICATE-----"
	otherCA := "-----BEGIN CERTIFICATE-----\ndef\n-----END CERTIFICATE-----"

	tests := []struct {
		name     string
		policies []deckhousev1alpha1.ContainerdIntegrityPolicy
		want     *DesiredConfig
	}{
		{
			name:     "no policies",
			policies: nil,
			want: &DesiredConfig{
				Namespaces: []string{},
				CACerts:    []string{},
			},
		},
		{
			name: "single policy",
			policies: []deckhousev1alpha1.ContainerdIntegrityPolicy{
				{
					Spec: deckhousev1alpha1.ContainerdIntegrityPolicySpec{
						CA: ca,
					},
					Status: deckhousev1alpha1.ContainerdIntegrityPolicyStatus{
						ProtectedNamespaces: []string{"baz", "qwerty"},
					},
				},
			},
			want: &DesiredConfig{
				Namespaces: []string{"baz", "qwerty"},
				CACerts:    []string{base64.StdEncoding.EncodeToString([]byte(ca))},
			},
		},
		{
			name: "merge namespaces and collect unique CAs from multiple policies",
			policies: []deckhousev1alpha1.ContainerdIntegrityPolicy{
				{
					Spec: deckhousev1alpha1.ContainerdIntegrityPolicySpec{CA: ca},
					Status: deckhousev1alpha1.ContainerdIntegrityPolicyStatus{
						ProtectedNamespaces: []string{"production"},
					},
				},
				{
					Spec: deckhousev1alpha1.ContainerdIntegrityPolicySpec{CA: otherCA},
					Status: deckhousev1alpha1.ContainerdIntegrityPolicyStatus{
						ProtectedNamespaces: []string{"my-ns", "production", "kube-.+"},
					},
				},
			},
			want: &DesiredConfig{
				Namespaces: []string{"kube-.+", "my-ns", "production"},
				CACerts: []string{
					base64.StdEncoding.EncodeToString([]byte(ca)),
					base64.StdEncoding.EncodeToString([]byte(otherCA)),
				},
			},
		},
		{
			name: "deduplicate identical CAs",
			policies: []deckhousev1alpha1.ContainerdIntegrityPolicy{
				{
					Spec: deckhousev1alpha1.ContainerdIntegrityPolicySpec{CA: ca},
				},
				{
					Spec: deckhousev1alpha1.ContainerdIntegrityPolicySpec{CA: ca},
				},
			},
			want: &DesiredConfig{
				Namespaces: []string{},
				CACerts:    []string{base64.StdEncoding.EncodeToString([]byte(ca))},
			},
		},
		{
			name: "include policy with empty CA",
			policies: []deckhousev1alpha1.ContainerdIntegrityPolicy{
				{
					Spec: deckhousev1alpha1.ContainerdIntegrityPolicySpec{CA: ""},
					Status: deckhousev1alpha1.ContainerdIntegrityPolicyStatus{
						ProtectedNamespaces: []string{"skipped-ns"},
					},
				},
			},
			want: &DesiredConfig{
				Namespaces: []string{"skipped-ns"},
				CACerts:    []string{base64.StdEncoding.EncodeToString([]byte(""))},
			},
		},
		{
			name: "include empty CA and valid policy",
			policies: []deckhousev1alpha1.ContainerdIntegrityPolicy{
				{
					Spec: deckhousev1alpha1.ContainerdIntegrityPolicySpec{CA: ""},
					Status: deckhousev1alpha1.ContainerdIntegrityPolicyStatus{
						ProtectedNamespaces: []string{"skipped-ns"},
					},
				},
				{
					Spec: deckhousev1alpha1.ContainerdIntegrityPolicySpec{CA: ca},
					Status: deckhousev1alpha1.ContainerdIntegrityPolicyStatus{
						ProtectedNamespaces: []string{"my-ns"},
					},
				},
			},
			want: &DesiredConfig{
				Namespaces: []string{"my-ns", "skipped-ns"},
				CACerts: []string{
					base64.StdEncoding.EncodeToString([]byte("")),
					base64.StdEncoding.EncodeToString([]byte(ca)),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := AggregatePolicies(tt.policies)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestRenderIntegrityToml(t *testing.T) {
	t.Parallel()

	cfg := &DesiredConfig{
		Namespaces: []string{"my-ns", "production", "kube-.+"},
		CACerts:    []string{"base64_ca_first", "base64_ca_second"},
	}

	got, err := RenderIntegrityToml(cfg)
	require.NoError(t, err)

	var parsed integrityTOML
	require.NoError(t, toml.Unmarshal(got, &parsed))
	require.Equal(t, cfg.Namespaces, parsed.Namespaces)
	require.Equal(t, cfg.CACerts, parsed.CACerts)
}

func TestFSApplierApplyAndRemove(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	applier := NewFSApplier(dir)

	ca := "-----BEGIN CERTIFICATE-----\nabc\n-----END CERTIFICATE-----"
	config := &DesiredConfig{
		Namespaces: []string{"my-ns", "production"},
		CACerts:    []string{base64.StdEncoding.EncodeToString([]byte(ca))},
	}

	_, err := applier.Apply(config)
	require.NoError(t, err)

	IntegrityTomlPath := filepath.Join(dir, IntegrityConfigFile)
	IntegrityTomlData, err := os.ReadFile(IntegrityTomlPath)
	require.NoError(t, err)

	expected, err := RenderIntegrityToml(config)
	require.NoError(t, err)
	require.Equal(t, expected, IntegrityTomlData)

	_, err = applier.Apply(config)
	require.NoError(t, err)
	unchanged, err := os.ReadFile(IntegrityTomlPath)
	require.NoError(t, err)
	require.Equal(t, expected, unchanged)

	require.NoError(t, os.WriteFile(IntegrityTomlPath, []byte("stale"), 0o644))
	_, err = applier.Apply(config)
	require.NoError(t, err)
	restored, err := os.ReadFile(IntegrityTomlPath)
	require.NoError(t, err)
	require.Equal(t, expected, restored)

	_, err = applier.Apply(nil)
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(dir, IntegrityConfigFile))
	require.True(t, os.IsNotExist(err))

	_, err = applier.Apply(config)
	require.NoError(t, err)
	_, err = applier.Apply(&DesiredConfig{})
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(dir, IntegrityConfigFile))
	require.True(t, os.IsNotExist(err))
}
