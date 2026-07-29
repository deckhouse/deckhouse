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
	"context"
	"fmt"

	"github.com/deckhouse/deckhouse/pkg/registry/client"
	"github.com/deckhouse/deckhouse/pkg/registry/fake"

	dhregistry "github.com/deckhouse/deckhouse/pkg/deckhouse-registry"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/bundle"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/definition"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/release"
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

const exampleModuleYAML = `
name: stronghold
weight: 910
stage: General Availability
requirements:
  deckhouse: ">= 1.70"
  kubernetes: ">= 1.27"
  modules:
    user-authn: ">= 1.0.0"
`

const examplePackageYAML = `
apiVersion: deckhouse.io/v1alpha1
type: Module
name: elma
version: v1.0.1
requirements:
  kubernetes:
    constraint: ">= 1.27"
  modules:
    mandatory:
      - name: user-authn
        constraint: ">= 1.0.0"
`

// exampleRegistry builds a registry over an in-memory fake, so the examples
// below run without touching a real registry. Production code passes a client
// from pkg/registry/client instead — see Example.
func exampleRegistry() *dhregistry.Registry {
	reg := fake.NewRegistry("registry.deckhouse.io")

	// A Deckhouse release image, and the Deckhouse image it describes.
	reg.MustAddImage("deckhouse/fe/release-channel", "stable", fake.NewImageBuilder().
		WithFile(release.VersionFile, deckhouseVersionJSON).
		MustBuild())
	reg.MustAddImage("deckhouse/fe", "v1.73.0", fake.NewImageBuilder().
		WithFile(bundle.ModulesImagesDigestsPath, nestedDigests).
		MustBuild())

	// A module release image, and the module image it describes.
	reg.MustAddImage("deckhouse/fe/modules/stronghold/release", "alpha", fake.NewImageBuilder().
		WithFile(release.VersionFile, `{"version": "v1.0.1"}`).
		WithFile(definition.ModuleFile, exampleModuleYAML).
		MustBuild())
	reg.MustAddImage("deckhouse/fe/modules/stronghold", "v1.0.1", fake.NewImageBuilder().
		WithFile(bundle.RootPath, flatDigests).
		MustBuild())

	// A package release image, and the package image it describes.
	reg.MustAddImage("deckhouse/fe/packages/elma/version", "v1.0.1", fake.NewImageBuilder().
		WithFile(release.VersionFile, `{"version": "v1.0.1"}`).
		WithFile(definition.PackageFile, examplePackageYAML).
		MustBuild())
	reg.MustAddImage("deckhouse/fe/packages/elma", "v1.0.1", fake.NewImageBuilder().
		WithFile(bundle.RootPath, flatDigests).
		MustBuild())

	return dhregistry.New(
		fake.NewClient(reg).WithSegment("deckhouse"),
		dhregistry.WithEdition(dhregistry.FEEdition),
	)
}

// ExampleRegistry_Deckhouse reads the version.json of a Deckhouse release.
// Only a Deckhouse release carries the rollout controls below — the canary
// waves, disruption notices and environment requirements that stage a platform
// upgrade.
func ExampleRegistry_Deckhouse() {
	ctx := context.Background()

	meta, err := exampleRegistry().Deckhouse().Releases().Metadata(ctx, "stable")
	if err != nil {
		fmt.Println("error:", err)

		return
	}

	fmt.Println("version:    ", meta.Version)
	fmt.Println("suspend:    ", meta.Suspend)
	fmt.Println("k8s:        ", meta.Requirements["k8s"])
	fmt.Println("disruptions:", meta.Disruptions["1.73"])
	fmt.Println("canary:     ", meta.Canary["stable"].Waves, "waves every", meta.Canary["stable"].Interval)

	// Output:
	// version:     v1.73.0
	// suspend:     false
	// k8s:         >= 1.27
	// disruptions: [ingressNginx]
	// canary:      5 waves every 15m0s
}

// ExampleRegistry_Deckhouse_digests reads images_digests.json out of the
// Deckhouse image. It bundles every module of its edition, so the file is keyed
// by module and the result is nested.
func ExampleRegistry_Deckhouse_digests() {
	ctx := context.Background()

	d, err := exampleRegistry().Deckhouse().Digests(ctx, "v1.73.0")
	if err != nil {
		fmt.Println("error:", err)

		return
	}

	digest, ok := d.Lookup("ingressNginx", "controller")

	fmt.Println("nested:", d.IsNested())
	fmt.Println("images:", d.Count())
	fmt.Println("ingressNginx/controller:", digest, ok)

	// Output:
	// nested: true
	// images: 3
	// ingressNginx/controller: sha256:111 true
}

// ExampleRegistry_Modules reads the module.yaml a module release publishes.
// module.yaml states its requirements as bare version ranges and a flat module
// map — compare ExampleRegistry_Packages.
func ExampleRegistry_Modules() {
	ctx := context.Background()

	releases := exampleRegistry().Modules().Module("stronghold").Releases()

	version, err := releases.Version(ctx, "alpha")
	if err != nil {
		fmt.Println("error:", err)

		return
	}

	def, err := releases.Definition(ctx, "alpha")
	if err != nil {
		fmt.Println("error:", err)

		return
	}

	fmt.Println("alpha resolves to:", version)
	fmt.Println("name:             ", def.Name)
	fmt.Println("weight:           ", def.Weight)
	fmt.Println("stage:            ", def.Stage)
	fmt.Println("needs deckhouse:  ", def.Requirements.Deckhouse)
	fmt.Println("needs user-authn: ", def.Requirements.ParentModules["user-authn"])

	// Output:
	// alpha resolves to: v1.0.1
	// name:              stronghold
	// weight:            910
	// stage:             General Availability
	// needs deckhouse:   >= 1.70
	// needs user-authn:  >= 1.0.0
}

// ExampleRegistry_Modules_digests reads images_digests.json out of a module
// image. A module bundles only its own images, so the file is flat.
func ExampleRegistry_Modules_digests() {
	ctx := context.Background()

	d, err := exampleRegistry().Modules().Module("stronghold").Digests(ctx, "v1.0.1")
	if err != nil {
		fmt.Println("error:", err)

		return
	}

	fmt.Println("nested:    ", d.IsNested())
	fmt.Println("controller:", d.Images["controller"])
	fmt.Println("webhook:   ", d.Images["webhook"])

	// Output:
	// nested:     false
	// controller: sha256:aaa
	// webhook:    sha256:bbb
}

// ExampleRegistry_Packages reads the package.yaml a package release publishes.
// One schema covers modules and applications, and unlike module.yaml it wraps
// each constraint in an object and splits dependencies into buckets.
func ExampleRegistry_Packages() {
	ctx := context.Background()

	pkg, err := exampleRegistry().Packages().Package("elma").Versions().Definition(ctx, "v1.0.1")
	if err != nil {
		fmt.Println("error:", err)

		return
	}

	fmt.Println("type:      ", pkg.Type)
	fmt.Println("is module: ", pkg.IsModule())
	fmt.Println("name:      ", pkg.Name)
	fmt.Println("version:   ", pkg.Version)
	fmt.Println("kubernetes:", pkg.Requirements.Kubernetes.Constraint)
	fmt.Println("mandatory: ", pkg.Requirements.Modules.Mandatory[0].Name,
		pkg.Requirements.Modules.Mandatory[0].Constraint)

	// Output:
	// type:       Module
	// is module:  true
	// name:       elma
	// version:    v1.0.1
	// kubernetes: >= 1.27
	// mandatory:  user-authn >= 1.0.0
}

// ExampleRegistry_Packages_digests reads images_digests.json out of a package
// image — the v2 counterpart of a module image, and flat for the same reason.
func ExampleRegistry_Packages_digests() {
	ctx := context.Background()

	d, err := exampleRegistry().Packages().Package("elma").Digests(ctx, "v1.0.1")
	if err != nil {
		fmt.Println("error:", err)

		return
	}

	digest, ok := d.Lookup("", "controller")

	fmt.Println("nested:", d.IsNested())
	fmt.Println("images:", d.Count())
	fmt.Println("controller:", digest, ok)

	// Output:
	// nested: false
	// images: 2
	// controller: sha256:aaa true
}
