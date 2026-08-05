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

package v2

import (
	"testing"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

func TestModuleConfigSettingsImplementsContract(t *testing.T) {
	t.Parallel()

	var _ cpapi.ModuleSettingsObject = (*ModuleConfigSettings)(nil)

	settings := &ModuleConfigSettings{
		Provider: Provider{Parameters: ProviderParameters{CloudID: "cloud", FolderID: "folder"}},
		Nodes:    Nodes{Parameters: NodesParameters{SSHPublicKey: "ssh-rsa AAA", Layout: "Standard"}},
	}

	if !settings.HasProviderSection() {
		t.Fatal("HasProviderSection() = false, want true")
	}
	if !settings.HasNodesSection() {
		t.Fatal("HasNodesSection() = false, want true")
	}
	if settings.HasStorageSection() {
		t.Fatal("HasStorageSection() = true, want false")
	}
	if settings.HasCCMSection() {
		t.Fatal("HasCCMSection() = true, want false")
	}
}

func TestModuleConfigSettingsSectionsAbsent(t *testing.T) {
	t.Parallel()

	empty := &ModuleConfigSettings{}
	if empty.HasProviderSection() || empty.HasNodesSection() || empty.HasStorageSection() || empty.HasCCMSection() {
		t.Fatal("zero-value sections must be reported as absent")
	}

	var nilSettings *ModuleConfigSettings
	if nilSettings.HasProviderSection() || nilSettings.HasNodesSection() ||
		nilSettings.HasStorageSection() || nilSettings.HasCCMSection() {
		t.Fatal("nil settings must report all sections as absent")
	}
}

func TestModuleConfigSettingsStorageSectionPresentWhenDisabled(t *testing.T) {
	t.Parallel()

	settings := &ModuleConfigSettings{Storage: Storage{Disabled: true}}
	if !settings.HasStorageSection() {
		t.Fatal("HasStorageSection() = false, want true when storage.disabled is set")
	}
}
