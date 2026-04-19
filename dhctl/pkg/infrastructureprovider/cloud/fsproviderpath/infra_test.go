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

package fsproviderpath

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud/settings"
)

func TestPluginDestination(t *testing.T) {
	tofuSettings := &settings.Simple{
		UseOpenTofuVal:       pointer.Bool(true),
		NamespaceVal:         pointer.String("yandex-cloud"),
		TypeVal:              pointer.String("yandex"),
		DestinationBinaryVal: pointer.String("terraform-provider-yandex"),
	}

	tofuPlugin := GetPluginDir("/plugins", tofuSettings, "0.83.0", "linux_amd64")
	// get from make build-render
	require.Equal(t, tofuPlugin, "/plugins/registry.opentofu.org/yandex-cloud/yandex/0.83.0/linux_amd64/terraform-provider-yandex")

	terraformSettings := &settings.Simple{
		UseOpenTofuVal:       pointer.Bool(false),
		NamespaceVal:         pointer.String("terraform-provider-openstack"),
		TypeVal:              pointer.String("openstack"),
		DestinationBinaryVal: pointer.String("terraform-provider-openstack"),
	}

	terraformPlugin := GetPluginDir("/plugins", terraformSettings, "1.54.1", "linux_amd64")
	// get from make build-render
	require.Equal(t, terraformPlugin, "/plugins/registry.terraform.io/terraform-provider-openstack/openstack/1.54.1/linux_amd64/terraform-provider-openstack")
}
