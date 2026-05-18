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
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/name212/govalue"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/settings"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

var versionsForTest = []string{legacyVersion, "3.14.1"}

type testCloudClient struct {
	version string
}

func newTestCloudClient(version string) *testCloudClient {
	return &testCloudClient{
		version: version,
	}
}

func (c *testCloudClient) GetVersion(context.Context) (string, error) {
	return c.version, nil
}

func testGetLegacyClient() cloudClient {
	return newTestCloudClient("36.2")
}

func testGetCurrentClient() cloudClient {
	return newTestCloudClient("38.0")
}

func TestVersionsContentLegacy(t *testing.T) {
	set := &settings.Simple{
		VersionsVal:  &versionsForTest,
		NamespaceVal: pointer.String("vmware"),
		TypeVal:      pointer.String("vcd"),
	}

	content, version, err := versionContentProviderWithClient(context.TODO(), testGetLegacyClient(), set, log.GetDefaultLogger())

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

	content, version, err := versionContentProviderWithClient(context.TODO(), testGetCurrentClient(), set, log.GetDefaultLogger())

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

func TestVCDClientProvider(t *testing.T) {
	makeInputWithServer := func(url string) map[string]json.RawMessage {
		pc, err := json.Marshal(providerConfig{Server: url, Insecure: true})
		require.NoError(t, err)
		return map[string]json.RawMessage{"provider": pc}
	}

	assertError := func(t *testing.T, pcc map[string]json.RawMessage) {
		_, err := newVcdCloudClient(pcc, log.GetDefaultLogger())
		require.Error(t, err)
	}

	// no provider key
	assertError(t, nil)
	assertError(t, map[string]json.RawMessage{})

	// invalid url
	assertError(t, makeInputWithServer(":-//blah"))

	// valid url
	pcc := makeInputWithServer("https://my-server:8080")
	c, err := newVcdCloudClient(pcc, log.GetDefaultLogger())
	require.NoError(t, err)
	require.False(t, govalue.IsNil(c))
}
