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

package checks

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

// editionLabel is where a Deckhouse release image records the edition it was built for.
const editionLabel = "io.deckhouse.edition"

// DhctlEditionCheck compares two labels and touches no network. The fetch it used to do — and
// every way that fetch can fail — now belongs to deckhouse-image-available, which this check
// depends on and reads the result of.
type DhctlEditionCheck struct {
	BuildInfo options.BuildInfo
	Image     *DeckhouseImage
}

type imageDescriptorProvider interface {
	ConfigFile(ref name.Reference, opts ...remote.Option) (*v1.ConfigFile, error)
}

type remoteDescriptorProvider struct{}

func (remoteDescriptorProvider) ConfigFile(ref name.Reference, opts ...remote.Option) (*v1.ConfigFile, error) {
	image, err := remote.Image(ref, opts...)
	if err != nil {
		return &v1.ConfigFile{}, err
	}
	return image.ConfigFile()
}

const DhctlEditionCheckName preflight.CheckName = "dhctl-edition"

func (DhctlEditionCheck) Description() string {
	return "the installer edition matches the edition of the Deckhouse image"
}

func (DhctlEditionCheck) Phase() preflight.Phase {
	return preflight.PhasePreInfra
}

func (DhctlEditionCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NoRetry
}

func (c DhctlEditionCheck) Run(_ context.Context) (string, error) {
	// Only werf stamps an edition into the binary, and that is the decision: a build made any
	// other way reports "local" and this check has nothing to compare. It is reported rather
	// than silently passed, and it used to be a Disable() at construction — which the report
	// showed as a check the operator had skipped with a flag they had never passed.
	if c.BuildInfo.AppEdition == "" || c.BuildInfo.AppEdition == "local" {
		return "", preflight.NotApplicable("this installer was not built by werf, so it carries no edition to compare")
	}

	ref, imageConfig, ok := c.Image.Get()
	if !ok {
		return "", preflight.NotApplicable("deckhouse-image-available did not read the image, so there is nothing to compare")
	}

	// This check is about a release: an installer built for version X must not install the image
	// of another edition tagged X. A development image is built from a branch, and its labels say
	// whatever that branch happened to carry — including nothing at all. Comparing against it
	// would refuse the ordinary way of testing a change: set devBranch, run the installer built
	// from the same branch.
	if c.Image.FromDevBranch() {
		return "", preflight.NotApplicable("%s is a development build, which carries no release edition to compare", ref)
	}

	labels := imageConfig.Config.Labels
	imageEdition, labelled := labels[editionLabel]

	if !labelled || imageEdition == "" {
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  fmt.Sprintf("the %s label of %s", editionLabel, ref),
			Observed: "the image carries no edition label",
			Expected: "a release image with the io.deckhouse.edition label",
			Fix:      fmt.Sprintf("check %s and the version tag. For a development build, set InitConfiguration.deckhouse.devBranch", registryImagesRepoField(c.Image.RegistryMode())),
		})
	}

	if imageEdition != c.BuildInfo.AppEdition {
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  fmt.Sprintf("the %s label of %s", editionLabel, ref),
			Observed: fmt.Sprintf("the installer is %s and the image is %s", c.BuildInfo.AppEdition, imageEdition),
			Expected: "the same edition on both sides",
			Fix: fmt.Sprintf("run the installer image of edition %s, or point imagesRepo at the %s repository",
				imageEdition, c.BuildInfo.AppEdition),
		})
	}

	return fmt.Sprintf("installer edition %s matches %s", c.BuildInfo.AppEdition, ref), nil
}

func DhctlEdition(buildInfo options.BuildInfo, image *DeckhouseImage) preflight.Check {
	check := DhctlEditionCheck{BuildInfo: buildInfo, Image: image}
	return preflight.Check{
		Name:        DhctlEditionCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Run:         check.Run,
	}
}
