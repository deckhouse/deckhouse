// Copyright 2023 Flant JSC
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

package app

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"

	"github.com/Masterminds/semver/v3"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	deckhouseRegistryHost     = "registry.deckhouse.io"
	enterpriseEditionRepoPath = "/deckhouse/ee"
	flantEditionRepoPath      = "/deckhouse/fe"

	enterpriseEditionRepo = deckhouseRegistryHost + enterpriseEditionRepoPath
	flantEditionRepo      = deckhouseRegistryHost + flantEditionRepoPath
)

const (
	mirrorNoValidation   = "off"
	mirrorFastValidation = "fast"
	mirrorFullValidation = "full"
)

var (
	MirrorRegistry         = ""
	MirrorRegistryHost     = ""
	MirrorRegistryPath     = ""
	MirrorRegistryUsername = ""
	MirrorRegistryPassword = ""

	MirrorInsecure       = false
	MirrorDHLicenseToken = ""
	MirrorTarBundle      = ""

	mirrorMinVersionString                 = ""
	MirrorMinVersion       *semver.Version = nil

	MirrorFlantEdition          = false
	MirrorDeckhouseRegistryRepo = enterpriseEditionRepo

	MirrorValidationMode = ""

	MirrorSkipGOSTHashing         = false
	MirrorDontContinuePartialPull = false
)

func DefineMirrorFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("license", "Pull Deckhouse images to local machine using license key. Conflicts with --registry.").
		Short('l').
		PlaceHolder("TOKEN").
		Envar(configEnvName("MIRROR_LICENSE")).
		StringVar(&MirrorDHLicenseToken)
	cmd.Flag("registry", "Push Deckhouse images to your private registry, specified as registry-host[:port][/path]. Conflicts with --license.").
		Short('r').
		Envar(configEnvName("MIRROR_PRIVATE_REGISTRY")).
		StringVar(&MirrorRegistry)
	cmd.Flag("registry-login", "Username to log into your registry.").
		Short('u').
		PlaceHolder("LOGIN").
		Envar(configEnvName("MIRROR_USER")).
		StringVar(&MirrorRegistryUsername)
	cmd.Flag("registry-password", "Password to log into your registry.").
		Short('p').
		PlaceHolder("PASSWORD").
		Envar(configEnvName("MIRROR_PASS")).
		StringVar(&MirrorRegistryPassword)
	cmd.Flag("fe", "Copy Flant Edition images instead of Enterprise Edition.").
		Envar(configEnvName("MIRROR_FE")).
		BoolVar(&MirrorFlantEdition)
	cmd.Flag("min-version", "Minimal Deckhouse release to copy. Cannot be above current Rock Solid release.").
		Short('v').
		Envar(configEnvName("MIRROR_MIN_VERSION")).
		StringVar(&mirrorMinVersionString)
	cmd.Flag("validation", "Validation of mirrored indexes and images. "+
		`Defaults to "fast" validation, which only checks if manifests and indexes are compliant with OCI specs, `+
		`"full" validation also checks images contents for corruption`).
		Hidden().
		Default(mirrorFastValidation).
		Envar(configEnvName("MIRROR_VALIDATION")).
		EnumVar(&MirrorValidationMode, mirrorNoValidation, mirrorFastValidation, mirrorFullValidation)
	cmd.Flag("skip-gost-digest", "Do not calculate GOST R 34.11-2012 STREEBOG digest for downloaded bundle").
		Envar(configEnvName("MIRROR_SKIP_GOST_DIGESTS")).
		BoolVar(&MirrorSkipGOSTHashing)
	cmd.Flag("images-bundle-path", "Path of tar bundle with pulled images. Should be a path to tar archive (.tar)").
		Short('i').
		PlaceHolder("PATH").
		Required().
		Envar(configEnvName("MIRROR_IMAGES_BUNDLE")).
		StringVar(&MirrorTarBundle)
	cmd.Flag("no-pull-resume", "Do not continue last unfinished pull operation.").
		BoolVar(&MirrorDontContinuePartialPull)
	cmd.Flag("insecure", "Interact with registries over HTTP.").
		BoolVar(&MirrorInsecure)

	cmd.PreAction(func(c *kingpin.ParseContext) error {
		var err error
		if err = validateRegistryAndTokenFlagsUsage(); err != nil {
			return err
		}
		if err = parseAndValidateMinVersionFlag(); err != nil {
			return err
		}
		if err = parseAndValidateRegistryURLFlag(); err != nil {
			return err
		}
		if err = validateRegistryCredentials(); err != nil {
			return err
		}
		if err = validateImagesBundlePathFlag(); err != nil {
			return err
		}

		if MirrorFlantEdition {
			MirrorDeckhouseRegistryRepo = flantEditionRepo
		}

		return nil
	})
}

func validateImagesBundlePathFlag() error {
	MirrorTarBundle = filepath.Clean(MirrorTarBundle)
	if filepath.Ext(MirrorTarBundle) != ".tar" {
		return errors.New("--images-bundle-path should be a path to tar archive (.tar)")
	}

	stats, err := os.Stat(MirrorTarBundle)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		break
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("stat %s: %w", MirrorTarBundle, err)
	case stats.IsDir() || filepath.Ext(MirrorTarBundle) != ".tar":
		return fmt.Errorf("%s should be a tar archive", MirrorTarBundle)
	}
	return nil
}

func validateRegistryCredentials() error {
	if MirrorRegistryPassword != "" && MirrorRegistryUsername == "" {
		return errors.New("Registry username not specified")
	}
	return nil
}

func parseAndValidateRegistryURLFlag() error {
	if MirrorRegistry != "" {
		registryUrl, err := url.Parse("docker://" + MirrorRegistry)
		if err != nil {
			return fmt.Errorf("Validate registry address: %w", err)
		}
		MirrorRegistryHost = registryUrl.Host
		MirrorRegistryPath = registryUrl.Path
		if MirrorRegistryHost == "" {
			return errors.New("Please specify registry address correctly")
		}
		if MirrorRegistryPath == "" {
			MirrorRegistryPath = enterpriseEditionRepoPath
			if MirrorFlantEdition {
				MirrorRegistryPath = flantEditionRepoPath
			}
		}
	}
	return nil
}

func parseAndValidateMinVersionFlag() error {
	var err error
	if mirrorMinVersionString != "" {
		MirrorMinVersion, err = semver.NewVersion(mirrorMinVersionString)
		if err != nil {
			return fmt.Errorf("Minimal deckhouse version: %w", err)
		}
	}
	return nil
}

func validateRegistryAndTokenFlagsUsage() error {
	if MirrorRegistry == "" && MirrorDHLicenseToken == "" {
		return errors.New("One of --license or --registry is required.")
	}

	if MirrorRegistry != "" && MirrorDHLicenseToken != "" {
		return errors.New("You have specified both --license and --registry flags. This is not how it works.\n\n" +
			"Leave only --license if you want to pull Deckhouse images from public registry.\n" +
			"Leave only --registry if you already pulled Deckhouse images and want to push it to your private registry.")
	}
	return nil
}
