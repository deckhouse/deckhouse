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
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
)

// TestWarnAboutKubernetesVersion covers the two things the schema enum cannot say: that a version
// it lists has reached end of life, and that the field being edited is not the one that decides.
func TestWarnAboutKubernetesVersion(t *testing.T) {
	versionMap := map[string]any{
		"k8s": map[string]any{
			"1.32": map[string]any{"status": endOfLifeStatus},
			"1.33": map[string]any{"status": "available"},
		},
	}

	moduleConfig := func(version string) []*ModuleConfig {
		mc := &ModuleConfig{Spec: ModuleConfigSpec{Settings: SettingsValues{"kubernetesVersion": version}}}
		mc.ObjectMeta = metav1.ObjectMeta{Name: "control-plane-manager"}
		return []*ModuleConfig{mc}
	}

	tests := []struct {
		name          string
		version       string
		moduleConfigs []*ModuleConfig
		wantWarnings  []string
		wantQuiet     bool
	}{
		{
			name:         "an end-of-life version",
			version:      "1.32",
			wantWarnings: []string{"Kubernetes 1.32 has reached end of life"},
		},
		{
			name:      "a supported version",
			version:   "1.33",
			wantQuiet: true,
		},
		{
			name:      "Automatic",
			version:   "Automatic",
			wantQuiet: true,
		},
		{
			// The ModuleConfig setting wins whenever it is set, so editing the other one does
			// nothing — which is exactly the shape of this mistake.
			name:          "the two disagree",
			version:       "1.33",
			moduleConfigs: moduleConfig("1.34"),
			wantWarnings:  []string{"the cluster will be created on 1.34"},
		},
		{
			name:          "the two agree",
			version:       "1.33",
			moduleConfigs: moduleConfig("1.33"),
			wantQuiet:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := json.Marshal(tt.version)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var logged bytes.Buffer
			ctx := dhlog.ToContext(t.Context(), slog.New(slog.NewTextHandler(&logged, nil)))

			warnAboutKubernetesVersion(ctx, &MetaConfig{
				ClusterConfig: map[string]json.RawMessage{"kubernetesVersion": encoded},
				ModuleConfigs: tt.moduleConfigs,
				VersionMap:    versionMap,
			})

			if tt.wantQuiet {
				if strings.Contains(logged.String(), "WARN") {
					t.Errorf("nothing to warn about, got:\n%s", logged.String())
				}
				return
			}
			for _, want := range tt.wantWarnings {
				if !strings.Contains(logged.String(), want) {
					t.Errorf("want a warning containing %q, got:\n%s", want, logged.String())
				}
			}
		})
	}
}
