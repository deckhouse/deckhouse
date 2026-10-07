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

package naming

import (
	"maps"
	"testing"
)

// TestSplitModuleOwnedLabels: of the module-owned labels, those the previous object already carried
// with the same value are kept; a new key or a new value is set anew. Other labels are in neither.
func TestSplitModuleOwnedLabels(t *testing.T) {
	tests := []struct {
		name         string
		labels       map[string]string
		previous     map[string]string
		expectedSet  map[string]string
		expectedKept map[string]string
	}{
		{
			name:         "no previous object",
			labels:       map[string]string{"security.deckhouse.io/pod-policy": "privileged", "team": "a"},
			expectedSet:  map[string]string{"security.deckhouse.io/pod-policy": "privileged"},
			expectedKept: map[string]string{},
		},
		{
			name:         "unchanged",
			labels:       map[string]string{"security.deckhouse.io/pod-policy": "privileged", "team": "b"},
			previous:     map[string]string{"security.deckhouse.io/pod-policy": "privileged", "team": "a"},
			expectedSet:  map[string]string{},
			expectedKept: map[string]string{"security.deckhouse.io/pod-policy": "privileged"},
		},
		{
			name:         "a new value",
			labels:       map[string]string{"security.deckhouse.io/pod-policy": "baseline"},
			previous:     map[string]string{"security.deckhouse.io/pod-policy": "privileged"},
			expectedSet:  map[string]string{"security.deckhouse.io/pod-policy": "baseline"},
			expectedKept: map[string]string{},
		},
		{
			name:         "a new key beside a kept one",
			labels:       map[string]string{"extended-monitoring.deckhouse.io/enabled": "true", "projects.deckhouse.io/project": "other"},
			previous:     map[string]string{"extended-monitoring.deckhouse.io/enabled": "true"},
			expectedSet:  map[string]string{"projects.deckhouse.io/project": "other"},
			expectedKept: map[string]string{"extended-monitoring.deckhouse.io/enabled": "true"},
		},
		{
			name:         "removed",
			labels:       map[string]string{"team": "a"},
			previous:     map[string]string{"security.deckhouse.io/pod-policy": "privileged"},
			expectedSet:  map[string]string{},
			expectedKept: map[string]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set, kept := SplitModuleOwnedLabels(tt.labels, tt.previous)
			if !maps.Equal(set, tt.expectedSet) {
				t.Errorf("set = %v, expected %v", set, tt.expectedSet)
			}
			if !maps.Equal(kept, tt.expectedKept) {
				t.Errorf("kept = %v, expected %v", kept, tt.expectedKept)
			}
		})
	}
}

// TestIsGitOpsTrackingLabel: the default tracking keys of Argo CD, Flux and kapp are never inherited by
// additional namespaces; other keys under the same domains, and ordinary labels, are.
func TestIsGitOpsTrackingLabel(t *testing.T) {
	for key, expected := range map[string]bool{
		"app.kubernetes.io/instance":          true,
		"argocd.argoproj.io/instance":         true,
		"kustomize.toolkit.fluxcd.io/name":    true,
		"helm.toolkit.fluxcd.io/namespace":    true,
		"kapp.k14s.io/app":                    true,
		"kapp.k14s.io/association":            true,
		"source.toolkit.fluxcd.io/name":       false,
		"app.kubernetes.io/name":              false,
		"team":                                false,
		"example.com/argocd.argoproj.io-like": false,
	} {
		if got := IsGitOpsTrackingLabel(key); got != expected {
			t.Errorf("IsGitOpsTrackingLabel(%q) = %v, expected %v", key, got, expected)
		}
	}
}

func TestProjectUser(t *testing.T) {
	if got := ProjectUser("proj"); got != "system:multitenancy-manager:project:proj" {
		t.Fatalf("ProjectUser(proj) = %q", got)
	}
}
