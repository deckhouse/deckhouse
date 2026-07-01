/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package configwriter

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/pkg/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
			name: "skip policy with empty CA",
			policies: []deckhousev1alpha1.ContainerdIntegrityPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "empty-ca"},
					Spec:       deckhousev1alpha1.ContainerdIntegrityPolicySpec{CA: "   "},
					Status: deckhousev1alpha1.ContainerdIntegrityPolicyStatus{
						ProtectedNamespaces: []string{"skipped-ns"},
					},
				},
			},
			want: &DesiredConfig{
				Namespaces: []string{},
				CACerts:    []string{},
			},
		},
		{
			name: "skip empty CA and apply valid policy",
			policies: []deckhousev1alpha1.ContainerdIntegrityPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "empty-ca"},
					Spec:       deckhousev1alpha1.ContainerdIntegrityPolicySpec{CA: "   "},
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
				Namespaces: []string{"my-ns"},
				CACerts:    []string{base64.StdEncoding.EncodeToString([]byte(ca))},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := AggregatePolicies(log.NewNop(), tt.policies)
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

func TestWriterApplyAndRemove(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writer := NewWriter(dir)

	ca := "-----BEGIN CERTIFICATE-----\nabc\n-----END CERTIFICATE-----"
	config := &DesiredConfig{
		Namespaces: []string{"my-ns", "production"},
		CACerts:    []string{base64.StdEncoding.EncodeToString([]byte(ca))},
	}

	require.NoError(t, writer.Apply(log.NewNop(), config))

	IntegrityTomlPath := filepath.Join(dir, IntegrityConfigFile)
	IntegrityTomlData, err := os.ReadFile(IntegrityTomlPath)
	require.NoError(t, err)

	expected, err := RenderIntegrityToml(config)
	require.NoError(t, err)
	require.Equal(t, expected, IntegrityTomlData)

	require.NoError(t, writer.Apply(log.NewNop(), config))
	unchanged, err := os.ReadFile(IntegrityTomlPath)
	require.NoError(t, err)
	require.Equal(t, expected, unchanged)

	require.NoError(t, os.WriteFile(IntegrityTomlPath, []byte("stale"), 0o644))
	require.NoError(t, writer.Apply(log.NewNop(), config))
	restored, err := os.ReadFile(IntegrityTomlPath)
	require.NoError(t, err)
	require.Equal(t, expected, restored)

	require.NoError(t, writer.Apply(log.NewNop(), nil))
	_, err = os.Stat(filepath.Join(dir, IntegrityConfigFile))
	require.True(t, os.IsNotExist(err))

	require.NoError(t, writer.Apply(log.NewNop(), config))
	require.NoError(t, writer.Apply(log.NewNop(), &DesiredConfig{}))
	_, err = os.Stat(filepath.Join(dir, IntegrityConfigFile))
	require.True(t, os.IsNotExist(err))
}
