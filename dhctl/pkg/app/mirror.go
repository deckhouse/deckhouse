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

	enterpriseEditionRepo = deckhouseRegistryHost + enterpriseEditionRepoPath
)

const (
	mirrorNoValidation   = "off"
	mirrorFastValidation = "fast"
	mirrorFullValidation = "full"
)

var (
	MirrorRegistry         string
	MirrorRegistryHost     string
	MirrorRegistryPath     string
	MirrorRegistryUsername string
	MirrorRegistryPassword string

	MirrorInsecure                bool
	MirrorTLSSkipVerify           bool
	MirrorDHLicenseToken          string
	MirrorImagesBundlePath        string
	MirrorImagesBundleChunkSizeGB int64

	mirrorSpecificVersion string
	MirrorSpecificVersion *semver.Version

	mirrorMinVersionString string
	MirrorMinVersion       *semver.Version

	MirrorSourceRegistryRepo     = enterpriseEditionRepo // Fallback to EE just in case.
	MirrorSourceRegistryLogin    string
	MirrorSourceRegistryPassword string

	MirrorValidationMode string

	MirrorDoGOSTDigest            bool
	MirrorDontContinuePartialPull bool

	MirrorWithoutModules bool
)

func DefineMirrorFlags(cmd *kingpin.CmdClause) {
	cmd.Flag("images-bundle-path", "Path to the directory with pulled images bundle.").
		Short('i').
		PlaceHolder("PATH").
		Required().
		Envar(configEnvName("MIRROR_IMAGES_BUNDLE")).
		StringVar(&MirrorImagesBundlePath)
	cmd.Flag("images-bundle-chunk-size", "Size of the chunks file in bundle in gigabytes.").
		Short('c').
		PlaceHolder("GB").
		Envar(configEnvName("MIRROR_IMAGES_BUNDLE_CHUNK_SIZE")).
		Int64Var(&MirrorImagesBundleChunkSizeGB)
	cmd.Flag("source", "Pull Deckhouse images from source registry. This is the default mode of operation.").
		Default(enterpriseEditionRepo).
		Envar(configEnvName("MIRROR_SOURCE")).
		StringVar(&MirrorSourceRegistryRepo)
	cmd.Flag("source-login", "Source registry login.").
		Envar(configEnvName("MIRROR_SOURCE_LOGIN")).
		PlaceHolder("LOGIN").
		StringVar(&MirrorSourceRegistryLogin)
	cmd.Flag("source-password", "Source registry password.").
		Envar(configEnvName("MIRROR_SOURCE_PASSWORD")).
		PlaceHolder("PASS").
		StringVar(&MirrorSourceRegistryPassword)
	cmd.Flag("license", "Pull Deckhouse images to local machine using license key. Shortcut for --source-login=license-token --source-password=<>.").
		Short('l').
		PlaceHolder("TOKEN").
		Envar(configEnvName("MIRROR_LICENSE")).
		StringVar(&MirrorDHLicenseToken)
	cmd.Flag("registry", "Push Deckhouse images to your private registry, specified as registry-host[:port]/path. Conflicts with --license.").
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
	cmd.Flag("min-version", "Minimal Deckhouse release to copy. Cannot be above current Rock Solid release.").
		Short('v').
		Envar(configEnvName("MIRROR_MIN_VERSION")).
		StringVar(&mirrorMinVersionString)
	cmd.Flag("release", "Specific Deckhouse release to copy. Conflicts with --min-version.").
		Envar(configEnvName("MIRROR_RELEASE")).
		StringVar(&mirrorSpecificVersion)
	cmd.Flag("validation", "Validate mirrored indexes and images after pull is complete. "+
		`Defaults to "fast" validation, which only checks if manifests, configs and indexes are compliant with OCI specs, `+
		`"full" validation also checks images contents for corruption.`).
		Hidden().
		Default(mirrorFastValidation).
		EnumVar(&MirrorValidationMode, mirrorNoValidation, mirrorFastValidation, mirrorFullValidation)
	cmd.Flag("gost-digest", "Calculate GOST R 34.11-2012 STREEBOG digest for downloaded bundle").
		Envar(configEnvName("MIRROR_DO_GOST_DIGESTS")).
		BoolVar(&MirrorDoGOSTDigest)
	cmd.Flag("no-pull-resume", "Do not continue last unfinished pull operation.").
		BoolVar(&MirrorDontContinuePartialPull)
	cmd.Flag("no-modules", "Do not pull Deckhouse modules into bundle.").
		BoolVar(&MirrorWithoutModules)
	cmd.Flag("tls-skip-verify", "Disable TLS certificate validation.").
		BoolVar(&MirrorTLSSkipVerify)
	cmd.Flag("insecure", "Interact with registries over HTTP.").
		BoolVar(&MirrorInsecure)

	cmd.PreAction(func(c *kingpin.ParseContext) error {
		var err error
		if err = parseAndValidateVersionFlags(); err != nil {
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
		if err = validateChunkSizeFlag(); err != nil {
			return err
		}

		return nil
	})
}

func validateImagesBundlePathFlag() error {
	MirrorImagesBundlePath = filepath.Clean(MirrorImagesBundlePath)
	if filepath.Ext(MirrorImagesBundlePath) != ".tar" {
		return errors.New("--images-bundle-path should be a path to tar archive (.tar)")
	}

	stats, err := os.Stat(MirrorImagesBundlePath)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		// If only the file is not there it is fine, it will be created, but if directories on the path are also missing, this is bad.
		tarBundleDir := filepath.Dir(MirrorImagesBundlePath)
		if _, err = os.Stat(tarBundleDir); err != nil {
			return err
		}
		break
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		return err
	case stats.IsDir() || filepath.Ext(MirrorImagesBundlePath) != ".tar":
		return fmt.Errorf("%s should be a tar archive", MirrorImagesBundlePath)
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
			return errors.New("--registry you provided contains no registry host. Please specify registry address correctly.")
		}
		if MirrorRegistryPath == "" {
			return errors.New("--registry you provided contains no path to repo. Please specify registry repo path correctly.")
		}
	}
	return nil
}

func parseAndValidateVersionFlags() error {
	if mirrorMinVersionString != "" && mirrorSpecificVersion != "" {
		return errors.New("Using both --release and --min-version at the same time is ambiguous.")
	}

	var err error
	if mirrorMinVersionString != "" {
		MirrorMinVersion, err = semver.NewVersion(mirrorMinVersionString)
		if err != nil {
			return fmt.Errorf("Parse minimal deckhouse version: %w", err)
		}
	}

	if mirrorSpecificVersion != "" {
		MirrorSpecificVersion, err = semver.NewVersion(mirrorSpecificVersion)
		if err != nil {
			return fmt.Errorf("Parse required deckhouse version: %w", err)
		}
	}
	return nil
}

func validateChunkSizeFlag() error {
	if MirrorImagesBundleChunkSizeGB < 0 {
		return errors.New("Chunk size cannot be less than zero GB")
	}

	return nil
}
