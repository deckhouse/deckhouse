// Copyright 2025 Flant JSC
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

package vcd

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/settings"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

var versionsForTest = []string{legacyVersion, "3.14.1"}

func testGetLegacyAPI(_ *config.MetaConfig, _ log.Logger) (string, error) {
	return "36.2", nil
}

func testGetCurrentAPI(_ *config.MetaConfig, _ log.Logger) (string, error) {
	return "38.0", nil
}

func TestVersionsContentLegacy(t *testing.T) {
	set := &settings.Simple{
		VersionsVal:  &versionsForTest,
		NamespaceVal: pointer.String("vmware"),
		TypeVal:      pointer.String("vcd"),
	}

	content, version, err := versionContentProviderWithAPI(testGetLegacyAPI, set, &config.MetaConfig{}, log.GetDefaultLogger())

	require.NoError(t, err)
	require.Equal(t, version, legacyVersion)
	require.Equal(t, string(content), fmt.Sprintf(`
terraform {
  required_version = ">= 0.14.8"
  required_providers {
    vcd = {
      source  = "vmware/vcd"
      version = ">= %s"
    }
  }
}
`, legacyVersion))
}

func TestVersionsContentCurrent(t *testing.T) {
	set := &settings.Simple{
		VersionsVal:  &versionsForTest,
		NamespaceVal: pointer.String("vmware"),
		TypeVal:      pointer.String("vcd"),
	}

	content, version, err := versionContentProviderWithAPI(testGetCurrentAPI, set, &config.MetaConfig{}, log.GetDefaultLogger())

	require.NoError(t, err)
	require.Equal(t, version, versionsForTest[1])
	require.Equal(t, string(content), fmt.Sprintf(`
terraform {
  required_version = ">= 0.14.8"
  required_providers {
    vcd = {
      source  = "vmware/vcd"
      version = ">= %s"
    }
  }
}
`, versionsForTest[1]))
}
