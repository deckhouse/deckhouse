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

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

// versionLabel is where a Deckhouse release image records the version it was built as.
const versionLabel = "io.deckhouse.version"

// DhctlVersionCheck compares the installer's own version with the version of the image it is
// about to install.
//
// The documentation has long promised a "DKP version check", and there was none: the label was
// never read. The installer and the image are built together and are not interchangeable — the
// installer writes the manifests the image's controllers then read — so a v1.77.3 installer
// pointed at a v1.77.2 image produces a cluster whose control plane and configuration disagree,
// with nothing saying why.
//
// Like dhctl-edition it reads the image config deckhouse-image-available already fetched, so it
// touches no network of its own.
type DhctlVersionCheck struct {
	BuildInfo options.BuildInfo
	Image     *DeckhouseImage
}

const DhctlVersionCheckName preflight.CheckName = "dhctl-version"

func (DhctlVersionCheck) Description() string {
	return "the installer version matches the version of the Deckhouse image"
}

func (DhctlVersionCheck) Phase() preflight.Phase {
	return preflight.PhasePreInfra
}

func (DhctlVersionCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NoRetry
}

func (c DhctlVersionCheck) Run(_ context.Context) (string, error) {
	// The same gate dhctl-edition applies, for the same reason: only werf stamps a version in,
	// and a build made any other way has nothing to compare.
	if c.BuildInfo.AppVersion == "" || c.BuildInfo.AppVersion == "local" || c.BuildInfo.AppVersion == "dev" {
		return "", preflight.NotApplicable("this installer was not built by werf, so it carries no version to compare")
	}

	ref, imageConfig, ok := c.Image.Get()
	if !ok {
		return "", preflight.NotApplicable("the Deckhouse image was not read")
	}

	// A development image is built from a branch and labelled with whatever that branch carried.
	// Comparing against it would refuse the ordinary way of testing a change.
	if c.Image.FromDevBranch() {
		return "", preflight.NotApplicable("%s is a development build, which carries no release version to compare", ref)
	}

	imageVersion, labelled := imageConfig.Config.Labels[versionLabel]
	if !labelled || imageVersion == "" {
		// Not a failure: unlike the edition label, this one is recent enough that an image
		// from an older release legitimately does not carry it, and refusing those would
		// break installing them.
		return "", preflight.NotApplicable("%s carries no %s label", ref, versionLabel)
	}

	if imageVersion != c.BuildInfo.AppVersion {
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  fmt.Sprintf("the %s label of %s", versionLabel, ref),
			Observed: fmt.Sprintf("the installer is %s and the image is %s", c.BuildInfo.AppVersion, imageVersion),
			Expected: "the same version on both sides",
			Fix: fmt.Sprintf("run the installer image of version %s, or point the version tag at %s",
				imageVersion, c.BuildInfo.AppVersion),
		})
	}

	return fmt.Sprintf("installer version %s matches %s", c.BuildInfo.AppVersion, ref), nil
}

func DhctlVersion(buildInfo options.BuildInfo, image *DeckhouseImage) preflight.Check {
	check := DhctlVersionCheck{BuildInfo: buildInfo, Image: image}
	return preflight.Check{
		Name:        DhctlVersionCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Timeout:     preflight.DefaultPreflightCheckTimeout,
		Run:         check.Run,
	}
}
