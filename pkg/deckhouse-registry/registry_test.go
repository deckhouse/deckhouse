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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/deckhouse/pkg/registry/client"

	dhregistry "github.com/deckhouse/deckhouse/pkg/deckhouse-registry"
)

// newFE builds a registry over registry.deckhouse.io/deckhouse scoped to fe.
// The real client is used because it is the one that implements the host+segments
// contract of GetRegistry; it performs no I/O until a request is made.
func newFE(t *testing.T) *dhregistry.Registry {
	t.Helper()

	root := client.New("registry.deckhouse.io").WithSegment("deckhouse")

	return dhregistry.New(root, dhregistry.WithEdition(dhregistry.FEEdition))
}

// TestPaths pins every path in the Deckhouse registry structure against the
// documented examples. This is the contract the package exists to encode, so it
// is asserted exhaustively.
func TestPaths(t *testing.T) {
	reg := newFE(t)

	tests := []struct {
		name string
		got  string
		want string
	}{
		// Roots
		{"root", reg.Root(), "registry.deckhouse.io/deckhouse"},
		{"edition root", reg.EditionRoot(), "registry.deckhouse.io/deckhouse/fe"},

		// Deckhouse
		{"deckhouse", reg.Deckhouse().Path(), "registry.deckhouse.io/deckhouse/fe"},
		{"deckhouse releases", reg.Deckhouse().Releases().Path(), "registry.deckhouse.io/deckhouse/fe/release-channel"},
		{"deckhouse install", reg.Deckhouse().Install().Path(), "registry.deckhouse.io/deckhouse/fe/install"},
		{"deckhouse install-standalone", reg.Deckhouse().InstallStandalone().Path(), "registry.deckhouse.io/deckhouse/fe/install-standalone"},

		// Security
		{"security", reg.Security().Path(), "registry.deckhouse.io/deckhouse/fe/security"},
		{"security image", reg.Security().Image("trivy-db").Path(), "registry.deckhouse.io/deckhouse/fe/security/trivy-db"},

		// Modules
		{"modules catalog", reg.Modules().Path(), "registry.deckhouse.io/deckhouse/fe/modules"},
		{"module", reg.Modules().Module("stronghold").Path(), "registry.deckhouse.io/deckhouse/fe/modules/stronghold"},
		{"module releases", reg.Modules().Module("stronghold").Releases().Path(), "registry.deckhouse.io/deckhouse/fe/modules/stronghold/release"},
		{"module extra", reg.Modules().Module("neuvector").Extra().Path(), "registry.deckhouse.io/deckhouse/fe/modules/neuvector/extra"},
		{"module extra image", reg.Modules().Module("neuvector").Extra().Image("scanner").Path(), "registry.deckhouse.io/deckhouse/fe/modules/neuvector/extra/scanner"},

		// Packages
		{"packages catalog", reg.Packages().Path(), "registry.deckhouse.io/deckhouse/fe/packages"},
		{"package", reg.Packages().Package("stronghold").Path(), "registry.deckhouse.io/deckhouse/fe/packages/stronghold"},
		{"package versions", reg.Packages().Package("stronghold").Versions().Path(), "registry.deckhouse.io/deckhouse/fe/packages/stronghold/version"},
		{"package extra", reg.Packages().Package("neuvector").Extra().Path(), "registry.deckhouse.io/deckhouse/fe/packages/neuvector/extra"},
		{"package extra image", reg.Packages().Package("neuvector").Extra().Image("scanner").Path(), "registry.deckhouse.io/deckhouse/fe/packages/neuvector/extra/scanner"},

		// Installer (edition-independent)
		{"installer", reg.Installer().Path(), "registry.deckhouse.io/deckhouse/installer"},

		// deckhouse-cli
		{"cli", reg.CLI().Path(), "registry.deckhouse.io/deckhouse/fe/deckhouse-cli"},
		{"cli versions", reg.CLI().Versions().Path(), "registry.deckhouse.io/deckhouse/fe/deckhouse-cli/version"},
		{"plugins catalog", reg.CLI().Plugins().Path(), "registry.deckhouse.io/deckhouse/fe/deckhouse-cli/plugins"},
		{"plugin", reg.CLI().Plugins().Plugin("package").Path(), "registry.deckhouse.io/deckhouse/fe/deckhouse-cli/plugins/package"},
		{"plugin versions", reg.CLI().Plugins().Plugin("package").Versions().Path(), "registry.deckhouse.io/deckhouse/fe/deckhouse-cli/plugins/package/version"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.got)
		})
	}
}

// TestRefs covers the documented example references, tag and digest alike.
func TestRefs(t *testing.T) {
	reg := newFE(t)

	const digest = "sha256:0000000000000000000000000000000000000000000000000000000000000000"

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"deckhouse version", reg.Deckhouse().Ref("v1.73.0"), "registry.deckhouse.io/deckhouse/fe:v1.73.0"},
		{"deckhouse channel", reg.Deckhouse().Ref("lts"), "registry.deckhouse.io/deckhouse/fe:lts"},
		{"release by channel", reg.Deckhouse().Releases().Ref("alpha"), "registry.deckhouse.io/deckhouse/fe/release-channel:alpha"},
		{"release by version", reg.Deckhouse().Releases().Ref("v1.73.0"), "registry.deckhouse.io/deckhouse/fe/release-channel:v1.73.0"},
		{"install", reg.Deckhouse().Install().Ref("v1.73.0"), "registry.deckhouse.io/deckhouse/fe/install:v1.73.0"},
		{"install standalone", reg.Deckhouse().InstallStandalone().Ref("v1.73.0"), "registry.deckhouse.io/deckhouse/fe/install-standalone:v1.73.0"},
		{"security", reg.Security().Image("trivy-db").Ref("2"), "registry.deckhouse.io/deckhouse/fe/security/trivy-db:2"},
		{"module catalog entry", reg.Modules().Ref("stronghold"), "registry.deckhouse.io/deckhouse/fe/modules:stronghold"},
		{"module version", reg.Modules().Module("stronghold").Ref("v1.0.1"), "registry.deckhouse.io/deckhouse/fe/modules/stronghold:v1.0.1"},
		{"module release", reg.Modules().Module("stronghold").Releases().Ref("alpha"), "registry.deckhouse.io/deckhouse/fe/modules/stronghold/release:alpha"},
		{"module extra", reg.Modules().Module("neuvector").Extra().Image("scanner").Ref("3"), "registry.deckhouse.io/deckhouse/fe/modules/neuvector/extra/scanner:3"},
		{"package catalog entry", reg.Packages().Ref("stronghold"), "registry.deckhouse.io/deckhouse/fe/packages:stronghold"},
		{"package version", reg.Packages().Package("elma").Ref("v1.0.1"), "registry.deckhouse.io/deckhouse/fe/packages/elma:v1.0.1"},
		{"package release", reg.Packages().Package("elma").Versions().Ref("alpha"), "registry.deckhouse.io/deckhouse/fe/packages/elma/version:alpha"},
		{"installer", reg.Installer().Ref("v0.1.3"), "registry.deckhouse.io/deckhouse/installer:v0.1.3"},
		{"cli", reg.CLI().Ref("v1.0.1"), "registry.deckhouse.io/deckhouse/fe/deckhouse-cli:v1.0.1"},
		{"cli release", reg.CLI().Versions().Ref("v1.0.1"), "registry.deckhouse.io/deckhouse/fe/deckhouse-cli/version:v1.0.1"},
		{"plugin catalog entry", reg.CLI().Plugins().Ref("package"), "registry.deckhouse.io/deckhouse/fe/deckhouse-cli/plugins:package"},
		{"plugin", reg.CLI().Plugins().Plugin("package").Ref("v1.0.1"), "registry.deckhouse.io/deckhouse/fe/deckhouse-cli/plugins/package:v1.0.1"},
		{"plugin release", reg.CLI().Plugins().Plugin("package").Versions().Ref("v1.0.1"), "registry.deckhouse.io/deckhouse/fe/deckhouse-cli/plugins/package/version:v1.0.1"},

		// Digest references, with and without the leading "@".
		{"digest bare", reg.Deckhouse().Ref(digest), "registry.deckhouse.io/deckhouse/fe@" + digest},
		{"digest prefixed", reg.Deckhouse().Ref("@" + digest), "registry.deckhouse.io/deckhouse/fe@" + digest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.got)
		})
	}
}

// TestAllEditionRoots covers the documented root paths for every edition.
func TestAllEditionRoots(t *testing.T) {
	want := map[dhregistry.Edition]string{
		dhregistry.FEEdition:     "registry.deckhouse.io/deckhouse/fe",
		dhregistry.BEEdition:     "registry.deckhouse.io/deckhouse/be",
		dhregistry.CEEdition:     "registry.deckhouse.io/deckhouse/ce",
		dhregistry.EEEdition:     "registry.deckhouse.io/deckhouse/ee",
		dhregistry.SEEdition:     "registry.deckhouse.io/deckhouse/se",
		dhregistry.SEPlusEdition: "registry.deckhouse.io/deckhouse/se-plus",
	}

	for _, edition := range dhregistry.Editions() {
		t.Run(edition.String(), func(t *testing.T) {
			root := client.New("registry.deckhouse.io").WithSegment("deckhouse")
			reg := dhregistry.New(root, dhregistry.WithEdition(edition))

			assert.Equal(t, want[edition], reg.EditionRoot())
			assert.Equal(t, "registry.deckhouse.io/deckhouse", reg.Root())
		})
	}
}

// TestNoEdition covers a custom dev root, where the edition sub-path is absent
// and everything edition-scoped hangs directly off the root.
func TestNoEdition(t *testing.T) {
	root := client.New("dev-registry.deckhouse.io").WithSegment("sys", "deckhouse-oss")
	reg := dhregistry.New(root)

	assert.Equal(t, dhregistry.NoEdition, reg.Edition())
	assert.Equal(t, "dev-registry.deckhouse.io/sys/deckhouse-oss", reg.Root())
	assert.Equal(t, "dev-registry.deckhouse.io/sys/deckhouse-oss", reg.EditionRoot())
	assert.Equal(t, "dev-registry.deckhouse.io/sys/deckhouse-oss/release-channel", reg.Deckhouse().Releases().Path())
	assert.Equal(t, "dev-registry.deckhouse.io/sys/deckhouse-oss/modules/stronghold", reg.Modules().Module("stronghold").Path())
	assert.Equal(t, "dev-registry.deckhouse.io/sys/deckhouse-oss/installer", reg.Installer().Path())
}

// TestNewDoesNotDoubleAppendEdition covers passing an already edition-scoped
// client together with WithEdition.
func TestNewDoesNotDoubleAppendEdition(t *testing.T) {
	scoped := client.New("registry.deckhouse.io").WithSegment("deckhouse", "fe")
	reg := dhregistry.New(scoped, dhregistry.WithEdition(dhregistry.FEEdition))

	assert.Equal(t, "registry.deckhouse.io/deckhouse/fe", reg.EditionRoot())
	assert.Equal(t, "registry.deckhouse.io/deckhouse/fe/modules", reg.Modules().Path())
}

// TestNewForPath covers edition auto-detection from the client path.
func TestNewForPath(t *testing.T) {
	t.Run("edition detected", func(t *testing.T) {
		scoped := client.New("registry.deckhouse.io").WithSegment("deckhouse", "se-plus")
		reg := dhregistry.NewForPath(scoped)

		assert.Equal(t, dhregistry.SEPlusEdition, reg.Edition())
		assert.Equal(t, "registry.deckhouse.io/deckhouse/se-plus", reg.EditionRoot())
	})

	t.Run("no edition in path", func(t *testing.T) {
		scoped := client.New("dev-registry.deckhouse.io").WithSegment("sys", "deckhouse-oss")
		reg := dhregistry.NewForPath(scoped)

		assert.Equal(t, dhregistry.NoEdition, reg.Edition())
		assert.Equal(t, "dev-registry.deckhouse.io/sys/deckhouse-oss", reg.EditionRoot())
	})
}

// TestServicesAreStable checks that repeated lookups of the same dynamic name
// return the same service, so callers can hold on to them.
func TestServicesAreStable(t *testing.T) {
	reg := newFE(t)

	assert.Same(t, reg.Modules().Module("stronghold"), reg.Modules().Module("stronghold"))
	assert.Same(t, reg.Packages().Package("elma"), reg.Packages().Package("elma"))
	assert.Same(t, reg.Security().Image("trivy-db"), reg.Security().Image("trivy-db"))
	assert.Same(t, reg.CLI().Plugins().Plugin("package"), reg.CLI().Plugins().Plugin("package"))
	assert.NotSame(t, reg.Modules().Module("stronghold"), reg.Modules().Module("neuvector"))
}
