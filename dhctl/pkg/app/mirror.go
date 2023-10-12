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
	"os"
	"path/filepath"

	"github.com/Masterminds/semver/v3"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	enterpriseEditionRepo = "registry.deckhouse.io/deckhouse/ee"
	flantEditionRepo      = "registry.deckhouse.io/deckhouse/fe"
)

const (
	mirrorNoValidation   = "off"
	mirrorFastValidation = "fast"
	mirrorFullValidation = "full"
)

var (
	MirrorRegistryHost     = ""
	MirrorRegistryUsername = ""
	MirrorRegistryPassword = ""
	MirrorInsecure         = false
	MirrorDHLicenseToken   = ""
	MirrorImagesPath       = ""

	mirrorMinVersionString                 = ""
	MirrorMinVersion       *semver.Version = nil

	mirrorFlantEdition          = false
	MirrorDeckhouseRegistryRepo = enterpriseEditionRepo

	MirrorValidationMode = ""
)

func DefineMirrorFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("license", "Pull Deckhouse images to local machine using license key. Conflicts with --registry.").
		Short('l').
		Envar(configEnvName("MIRROR_LICENSE")).
		StringVar(&MirrorDHLicenseToken)
	cmd.Flag("registry", "Push Deckhouse images to your private registry, specified as registry-host[:port]. Conflicts with --license.").
		Short('r').
		Envar(configEnvName("MIRROR_PRIVATE_REGISTRY")).
		StringVar(&MirrorRegistryHost)
	cmd.Flag("registry-login", "Username to log into your registry.").
		Short('u').
		Envar(configEnvName("MIRROR_USER")).
		StringVar(&MirrorRegistryUsername)
	cmd.Flag("registry-password", "Password to log into your registry.").
		Short('p').
		Envar(configEnvName("MIRROR_PASS")).
		StringVar(&MirrorRegistryPassword)
	cmd.Flag("fe", "Copy Flant Edition images instead of Enterprise Edition.").
		Envar(configEnvName("MIRROR_FE")).
		BoolVar(&mirrorFlantEdition)
	cmd.Flag("min-version", "Minimal Deckhouse release to copy. Cannot be above current Rock Solid release.").
		Short('v').
		Envar(configEnvName("MIRROR_MIN_VERSION")).
		StringVar(&mirrorMinVersionString)
	cmd.Flag("validation", "Validation of mirrored indexes and images. "+
		`Defaults to "fast" validation, which only checks if manifests and indexes are compliant with OCI specs, `+
		`"full" validation also checks images contents for corruption`).
		Default(mirrorFastValidation).
		Envar(configEnvName("MIRROR_VALIDATION")).
		EnumVar(&MirrorValidationMode, mirrorNoValidation, mirrorFastValidation, mirrorFullValidation)
	cmd.Flag("images", "Directory for pulled images.").
		Short('i').
		Default("/tmp/d8-images").
		Envar(configEnvName("MIRROR_IMAGES_PATH")).
		StringVar(&MirrorImagesPath)
	cmd.Flag("insecure", "Skip TLS checks.").
		BoolVar(&MirrorInsecure)

	cmd.PreAction(func(c *kingpin.ParseContext) error {
		var err error

		if MirrorRegistryHost == "" && MirrorDHLicenseToken == "" {
			return errors.New("One of --license or --registry is required.")
		}

		if MirrorRegistryHost != "" && MirrorDHLicenseToken != "" {
			return errors.New("You have specified both --license and --registry flags. This is not how it works.\n\n" +
				"Leave only --license if you want to pull Deckhouse from public registry.\n" +
				"Leave only --registry if you already pulled Deckhouse and want to push it to your private registry.")
		}

		if mirrorMinVersionString != "" {
			MirrorMinVersion, err = semver.NewVersion(mirrorMinVersionString)
			if err != nil {
				return fmt.Errorf("Minimal deckhouse version: %w", err)
			}
		}

		if MirrorRegistryPassword != "" && MirrorRegistryUsername == "" {
			return errors.New("Registry username not specified")
		}

		if mirrorFlantEdition {
			MirrorDeckhouseRegistryRepo = flantEditionRepo
		}

		MirrorImagesPath = filepath.Clean(MirrorImagesPath)
		stats, err := os.Stat(MirrorImagesPath)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			break
		case err != nil && !errors.Is(err, fs.ErrNotExist):
			return fmt.Errorf("stat %s: %w", MirrorImagesPath, err)
		case !stats.IsDir():
			return fmt.Errorf("%s should be a directory", MirrorImagesPath)
		}

		return nil
	})
}
