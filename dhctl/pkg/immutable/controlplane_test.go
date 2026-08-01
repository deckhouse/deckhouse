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

package immutable

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

// TestCheckExtraFiles covers the case that put kube-apiserver in a crash loop:
// a cluster setting adds a flag pointing at an extra file, and on the immutable
// path nothing creates it — the module preparators that write those files on
// the classic path run over SSH and never run here.
func TestCheckExtraFiles(t *testing.T) {
	tests := []struct {
		name             string
		manifests        map[string]string
		extraFiles       map[string]string
		wantErrSubstring string
	}{
		{
			name:       "every referenced file is carried",
			manifests:  map[string]string{"kube-apiserver.yaml": "    - --authentication-config=" + extraFilesDir + "/authentication-config.yaml\n"},
			extraFiles: map[string]string{authenticationConfigFile: authenticationConfig},
		},
		{
			name: "the CSE encryption provider config nobody writes",
			manifests: map[string]string{
				"kube-apiserver.yaml": "    - --authentication-config=" + extraFilesDir + "/authentication-config.yaml\n" +
					"    - --encryption-provider-config=" + extraFilesDir + "/secret-encryption-config.yaml\n",
			},
			extraFiles:       map[string]string{authenticationConfigFile: authenticationConfig},
			wantErrSubstring: "secret-encryption-config.yaml (referenced by kube-apiserver.yaml)",
		},
		{
			name: "a file missed by two manifests names both",
			manifests: map[string]string{
				"kube-scheduler.yaml":          "    - --config=" + extraFilesDir + "/scheduler-config.yaml\n",
				"kube-controller-manager.yaml": "    - --config=" + extraFilesDir + "/scheduler-config.yaml\n",
			},
			wantErrSubstring: "scheduler-config.yaml (referenced by kube-controller-manager.yaml, kube-scheduler.yaml)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkExtraFiles(tt.manifests, tt.extraFiles)
			if tt.wantErrSubstring == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErrSubstring)
		})
	}
}

// TestCertSANs reads the same list control-plane-manager publishes under the
// "cert-sans" key of its config secret. The node issues the apiserver
// certificate itself, so a name missing here is a name the certificate does not
// cover until control-plane-manager reissues it.
func TestCertSANs(t *testing.T) {
	tests := []struct {
		name     string
		settings map[string]any
		expSANs  []string
	}{
		{
			name:     "no module config at all",
			settings: nil,
		},
		{
			name:     "module config without an apiserver section",
			settings: map[string]any{"encryptionAlgorithm": "ECDSA-P256"},
		},
		{
			name:     "the configured names",
			settings: map[string]any{"apiserver": map[string]any{"certSANs": []any{"ha.example.com", "192.168.1.100"}}},
			expSANs:  []string{"ha.example.com", "192.168.1.100"},
		},
		{
			name:     "an empty list is no list",
			settings: map[string]any{"apiserver": map[string]any{"certSANs": []any{}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metaConfig := testMetaConfig(t, "50Gi", "10Gi")
			if tt.settings != nil {
				metaConfig.ModuleConfigs = append(metaConfig.ModuleConfigs, &config.ModuleConfig{
					ObjectMeta: metav1.ObjectMeta{Name: "control-plane-manager"},
					Spec:       config.ModuleConfigSpec{Settings: tt.settings},
				})
			}

			require.Equal(t, tt.expSANs, certSANs(metaConfig))
		})
	}
}
