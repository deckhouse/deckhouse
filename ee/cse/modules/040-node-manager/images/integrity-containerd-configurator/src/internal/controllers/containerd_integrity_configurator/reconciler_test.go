/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package containerdintegrityconfigurator

import (
	"encoding/base64"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/require"

	deckhousev1alpha1 "integrity-controller/api/deckhouse.io/v1alpha1"
)

func TestBuildDesiredConfig(t *testing.T) {
	t.Parallel()

	ca := "-----BEGIN CERTIFICATE-----\nabc\n-----END CERTIFICATE-----"
	otherCA := "-----BEGIN CERTIFICATE-----\ndef\n-----END CERTIFICATE-----"

	tests := []struct {
		name     string
		policies []deckhousev1alpha1.ContainerdIntegrityPolicy
		want     *desiredConfig
	}{
		{
			name:     "no policies",
			policies: nil,
			want: &desiredConfig{
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
			want: &desiredConfig{
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
			want: &desiredConfig{
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
			want: &desiredConfig{
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
			want: &desiredConfig{
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
			want: &desiredConfig{
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

			got := makeDesiredConfig(tt.policies)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestRenderIntegrityToml(t *testing.T) {
	t.Parallel()

	cfg := &desiredConfig{
		Namespaces: []string{"my-ns", "production", "kube-.+"},
		CACerts:    []string{"base64_ca_first", "base64_ca_second"},
	}

	got, err := renderIntegrityToml(cfg)
	require.NoError(t, err)

	var parsed integrityTOML
	require.NoError(t, toml.Unmarshal(got, &parsed))
	require.Equal(t, cfg.Namespaces, parsed.Namespaces)
	require.Equal(t, cfg.CACerts, parsed.CACerts)
}

func TestIsIntegrityConfigFileInSync(t *testing.T) {
	t.Parallel()

	require.True(t, isIntegrityConfigFileInSync([]byte("same"), []byte("same")))
	require.False(t, isIntegrityConfigFileInSync([]byte("old"), []byte("new")))
}
