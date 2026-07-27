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

package dhregistry_test

import (
	"fmt"

	"github.com/deckhouse/deckhouse/pkg/registry/client"

	dhregistry "github.com/deckhouse/deckhouse/pkg/deckhouse-registry"
)

// Example shows the tree used purely as a path builder: no registry access is
// needed to resolve a reference.
func Example() {
	root := client.New("registry.deckhouse.io").WithSegment("deckhouse")
	reg := dhregistry.New(root, dhregistry.WithEdition(dhregistry.FEEdition))

	fmt.Println(reg.Deckhouse().Ref("v1.73.0"))
	fmt.Println(reg.Deckhouse().Releases().Ref(dhregistry.StableChannel.String()))
	fmt.Println(reg.Deckhouse().Install().Ref("v1.73.0"))
	fmt.Println(reg.Modules().Module("stronghold").Releases().Ref("alpha"))
	fmt.Println(reg.Modules().Module("neuvector").Extra().Image("scanner").Ref("3"))
	fmt.Println(reg.Packages().Package("elma").Versions().Ref("v1.0.1"))
	fmt.Println(reg.Security().Image("trivy-db").Ref("2"))
	fmt.Println(reg.CLI().Plugins().Plugin("package").Versions().Ref("v1.0.1"))
	fmt.Println(reg.Installer().Ref("v0.1.3"))

	// Output:
	// registry.deckhouse.io/deckhouse/fe:v1.73.0
	// registry.deckhouse.io/deckhouse/fe/release-channel:stable
	// registry.deckhouse.io/deckhouse/fe/install:v1.73.0
	// registry.deckhouse.io/deckhouse/fe/modules/stronghold/release:alpha
	// registry.deckhouse.io/deckhouse/fe/modules/neuvector/extra/scanner:3
	// registry.deckhouse.io/deckhouse/fe/packages/elma/version:v1.0.1
	// registry.deckhouse.io/deckhouse/fe/security/trivy-db:2
	// registry.deckhouse.io/deckhouse/fe/deckhouse-cli/plugins/package/version:v1.0.1
	// registry.deckhouse.io/deckhouse/installer:v0.1.3
}

// ExampleSplitEdition shows detecting the edition of a configured registry
// path, which is how a ModuleSource or an installer config is normalized.
func ExampleSplitEdition() {
	fmt.Println(dhregistry.SplitEdition("registry.deckhouse.io/deckhouse/ee/"))
	fmt.Println(dhregistry.SplitEdition("dev-registry.deckhouse.io/sys/deckhouse-oss"))

	// Output:
	// registry.deckhouse.io/deckhouse ee
	// dev-registry.deckhouse.io/sys/deckhouse-oss
}
