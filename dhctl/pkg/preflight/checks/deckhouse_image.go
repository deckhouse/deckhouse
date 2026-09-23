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
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	cfgregistry "github.com/deckhouse/deckhouse/dhctl/pkg/config/registry"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/registryutil"
)

// DeckhouseImage is the image config fetched once by deckhouse-image-available and read by the
// checks that only need its labels. Sharing it is what lets those checks run without a network
// call of their own, so a mismatch in a label is reported as a mismatch and not as a registry
// error, and a registry error is reported once.
type DeckhouseImage struct {
	mu     sync.Mutex
	ref    string
	config *v1.ConfigFile
	// fromDevBranch records that the tag came from InitConfiguration.deckhouse.devBranch rather
	// than from the version this installer was built for. A development image is built from a
	// branch and carries whatever labels that branch happened to have, so the checks that
	// compare labels have nothing to compare against.
	fromDevBranch bool
	// registryMode is the mode the image was fetched in, carried here so the checks that read
	// the image afterwards can name the ModuleConfig section its fields live under without a
	// configuration of their own.
	registryMode string
}

func NewDeckhouseImage() *DeckhouseImage { return &DeckhouseImage{} }

func (i *DeckhouseImage) set(ref string, config *v1.ConfigFile, fromDevBranch bool, registryMode string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.ref, i.config, i.fromDevBranch, i.registryMode = ref, config, fromDevBranch, registryMode
}

// Get returns the image reference and its config, and whether they have been fetched.
func (i *DeckhouseImage) Get() (string, *v1.ConfigFile, bool) {
	if i == nil {
		return "", nil, false
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.ref, i.config, i.config != nil
}

// FromDevBranch reports whether the image was pulled by a development branch name.
func (i *DeckhouseImage) FromDevBranch() bool {
	if i == nil {
		return false
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.fromDevBranch
}

// RegistryMode is the registry mode the image was fetched in, or "" when it was never fetched.
func (i *DeckhouseImage) RegistryMode() string {
	if i == nil {
		return ""
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.registryMode
}

// DeckhouseImageAvailableCheck answers the question that used to be reported under the name
// "dhctl-edition": is the image this installer is about to pull actually in the registry?
//
// MANIFEST_UNKNOWN — the tag was never mirrored — is the most common air-gapped failure there is,
// and it arrived as "your edition installer image does not match", with a Go map dump appended.
type DeckhouseImageAvailableCheck struct {
	MetaConfig *config.MetaConfig
	Installer  *config.DeckhouseInstaller
	Image      *DeckhouseImage

	descriptor imageDescriptorProvider
}

const DeckhouseImageAvailableCheckName preflight.CheckName = "deckhouse-image-available"

func (DeckhouseImageAvailableCheck) Description() string {
	return "the Deckhouse image of this version is present in the registry"
}

func (DeckhouseImageAvailableCheck) Phase() preflight.Phase {
	return preflight.PhasePreInfra
}

func (DeckhouseImageAvailableCheck) RetryPolicy() preflight.RetryPolicy {
	return preflight.NetworkRetry
}

func (c DeckhouseImageAvailableCheck) Run(ctx context.Context) (string, error) {
	if c.MetaConfig == nil || c.Installer == nil {
		return "", fmt.Errorf("no cluster configuration was loaded")
	}

	registry := c.MetaConfig.Registry.Settings.RemoteData
	image, err := c.Installer.GetRemoteImage(ctx, true)
	if err != nil {
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  "the Deckhouse version this installer asks for",
			Observed: err.Error(),
			Expected: "a version this installer can pull (a version file in the installer image, or devBranch)",
			Fix:      "set InitConfiguration.deckhouse.devBranch for a development build, or use a release installer image",
		})
	}

	ref, err := parseImageReference(image, string(registry.Scheme))
	if err != nil {
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  registryImagesRepoField(c.registryMode()),
			Observed: fmt.Sprintf("%q is not a valid image reference: %s", image, err),
			Expected: "a registry address and a repository path",
			Fix:      "correct " + registryImagesRepoField(c.registryMode()),
		})
	}

	client, err := registryutil.NewRegistryClient(ctx, string(registry.Scheme), registry.CA)
	if err != nil {
		return "", preflight.Permanent(&preflight.Failure{
			Checked:  registryCAField(c.registryMode()),
			Observed: err.Error(),
			Expected: "a PEM-encoded CA bundle dhctl can parse",
			Fix:      "correct " + registryCAField(c.registryMode()),
		})
	}

	imageConfig, err := c.provider().ConfigFile(
		ref,
		remote.WithContext(ctx),
		remote.WithAuth(registryAuth(registry)),
		remote.WithTransport(client.Transport),
	)
	if err != nil {
		return "", imageFetchFailure(image, c.registryMode(), err)
	}

	// The tag is the version this installer was built for, unless there is none embedded in it —
	// then GetImageTag falls back to devBranch, and what was pulled is a development build.
	fromDevBranch := c.Installer.DevBranch != "" && strings.HasSuffix(image, ":"+c.Installer.DevBranch)

	c.Image.set(image, imageConfig, fromDevBranch, c.registryMode())
	if fromDevBranch {
		return fmt.Sprintf("%s is present in the registry (a development build of branch %s)", image, c.Installer.DevBranch), nil
	}
	return fmt.Sprintf("%s is present in the registry", image), nil
}

// imageFetchFailure separates the registry's answers from one another. go-containerregistry
// reports them all as one *transport.Error whose text ends in a dump of a Go map.
func imageFetchFailure(image, mode string, err error) error {
	failure := &preflight.Failure{
		Checked: fmt.Sprintf("the image %s", image),
		Err:     err,
	}

	var transportErr *transport.Error
	if !errors.As(err, &transportErr) {
		failure.Observed = classifyNetworkError(err)
		failure.Expected = "an answer from the registry"
		failure.Fix = fmt.Sprintf("check %s", registryImagesRepoField(mode))
		return failure
	}

	for _, diagnostic := range transportErr.Errors {
		switch diagnostic.Code {
		case transport.ManifestUnknownErrorCode:
			failure.Observed = fmt.Sprintf("the tag of %s is not in the registry", image)
			failure.Expected = "the tag of this version, present in the registry"
			failure.Fix = "mirror this Deckhouse version into the registry (`d8 mirror pull` / `d8 mirror push`), " +
				"or run the installer image of a version the registry holds"
			return preflight.Permanent(failure)

		case transport.NameUnknownErrorCode:
			failure.Observed = fmt.Sprintf("the repository of %s does not exist on the registry", image)
			failure.Expected = "the repository named by imagesRepo, present in the registry"
			failure.Fix = "correct " + registryImagesRepoField(mode)
			return preflight.Permanent(failure)

		case transport.UnauthorizedErrorCode, transport.DeniedErrorCode:
			failure.Observed = fmt.Sprintf("the registry refused to serve %s with the configured credentials", image)
			failure.Expected = "pull access to the repository"
			failure.Fix = registryCredentialsFix
			return preflight.Permanent(failure)
		}
	}

	if transportErr.StatusCode >= http.StatusInternalServerError {
		failure.Observed = fmt.Sprintf("the registry answered HTTP %d", transportErr.StatusCode)
		failure.Expected = "the manifest served by the registry"
		failure.Fix = "check that the registry (or the mirror in front of it) is healthy"
		return failure
	}

	failure.Observed = strings.TrimSpace(transportErr.Error())
	failure.Expected = "the manifest served by the registry"
	failure.Fix = fmt.Sprintf("check %s", registryImagesRepoField(mode))
	return failure
}

func (c DeckhouseImageAvailableCheck) provider() imageDescriptorProvider {
	if c.descriptor != nil {
		return c.descriptor
	}
	return remoteDescriptorProvider{}
}

func parseImageReference(image, scheme string) (name.Reference, error) {
	if strings.EqualFold(scheme, "http") {
		return name.ParseReference(image, name.Insecure)
	}
	return name.ParseReference(image)
}

func registryAuth(registry cfgregistry.Data) authn.Authenticator {
	if registry.Username != "" && registry.Password != "" {
		return authn.FromConfig(authn.AuthConfig{
			Username: registry.Username,
			Password: registry.Password,
		})
	}
	return authn.Anonymous
}

func DeckhouseImageAvailable(meta *config.MetaConfig, cfg *config.DeckhouseInstaller, image *DeckhouseImage) preflight.Check {
	check := DeckhouseImageAvailableCheck{MetaConfig: meta, Installer: cfg, Image: image}
	return preflight.Check{
		Name:        DeckhouseImageAvailableCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Run:         check.Run,
	}
}

// registryMode is the mode the registry is configured in, used to name the ModuleConfig section
// the fields live under. Empty when no configuration was loaded, which registrySection reports
// as a placeholder rather than guessing.
func (c DeckhouseImageAvailableCheck) registryMode() string {
	if c.MetaConfig == nil {
		return ""
	}
	return string(c.MetaConfig.Registry.Settings.Mode)
}
