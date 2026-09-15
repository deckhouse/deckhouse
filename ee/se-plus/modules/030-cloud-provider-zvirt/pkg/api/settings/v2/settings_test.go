/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package v2

import "testing"

func TestSectionPredicatesOnNil(t *testing.T) {
	var s *ModuleConfigSettings

	if s.HasProviderSection() || s.HasNodesSection() || s.HasStorageSection() || s.HasCCMSection() {
		t.Fatal("section predicates must report false on a nil receiver")
	}
}

func TestSectionPredicatesOnZeroValue(t *testing.T) {
	s := &ModuleConfigSettings{}

	if s.HasProviderSection() || s.HasNodesSection() || s.HasStorageSection() || s.HasCCMSection() {
		t.Fatal("section predicates must report false on a zero value")
	}
}

func TestSectionPredicatesOnFilledSections(t *testing.T) {
	tests := []struct {
		name     string
		settings *ModuleConfigSettings
		got      func(*ModuleConfigSettings) bool
	}{
		{
			name:     "provider",
			settings: &ModuleConfigSettings{Provider: Provider{Parameters: ProviderParameters{Server: "https://zvirt.example.com"}}},
			got:      (*ModuleConfigSettings).HasProviderSection,
		},
		{
			name:     "nodes",
			settings: &ModuleConfigSettings{Nodes: Nodes{Parameters: NodesParameters{Layout: "Standard"}}},
			got:      (*ModuleConfigSettings).HasNodesSection,
		},
		{
			name:     "storage",
			settings: &ModuleConfigSettings{Storage: Storage{Parameters: StorageParameters{ExcludedStorageClasses: []string{"slow"}}}},
			got:      (*ModuleConfigSettings).HasStorageSection,
		},
		{
			name:     "ccm",
			settings: &ModuleConfigSettings{CCM: CCM{Disabled: true}},
			got:      (*ModuleConfigSettings).HasCCMSection,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.got(tt.settings) {
				t.Fatalf("%s section must be reported as set", tt.name)
			}
		})
	}
}
