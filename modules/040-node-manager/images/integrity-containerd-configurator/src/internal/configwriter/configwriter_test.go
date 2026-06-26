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

package configwriter

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/require"
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
		wantErr  string
	}{
		{
			name:     "no policies",
			policies: nil,
			want:     nil,
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
			name: "empty CA",
			policies: []deckhousev1alpha1.ContainerdIntegrityPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "empty-ca"},
					Spec:       deckhousev1alpha1.ContainerdIntegrityPolicySpec{CA: "   "},
				},
			},
			wantErr: "empty spec.ca",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := AggregatePolicies(tt.policies)
			if tt.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestRenderNsToml(t *testing.T) {
	t.Parallel()

	cfg := &DesiredConfig{
		Namespaces: []string{"my-ns", "production", "kube-.+"},
		CACerts:    []string{"base64_ca_first", "base64_ca_second"},
	}

	got, err := RenderNsToml(cfg)
	require.NoError(t, err)

	var parsed nsTOML
	require.NoError(t, toml.Unmarshal(got, &parsed))
	require.Equal(t, cfg.Namespaces, parsed.Namespaces)
	require.Equal(t, cfg.CACerts, parsed.CACert)
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

	require.NoError(t, writer.Apply(config))

	nsTomlData, err := os.ReadFile(filepath.Join(dir, NsTomlFileName))
	require.NoError(t, err)

	expected, err := RenderNsToml(config)
	require.NoError(t, err)
	require.Equal(t, expected, nsTomlData)

	require.NoError(t, writer.Apply(nil))
	_, err = os.Stat(filepath.Join(dir, NsTomlFileName))
	require.True(t, os.IsNotExist(err))
}
