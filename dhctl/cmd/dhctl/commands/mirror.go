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

package commands

import (
	"bufio"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/logs"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/mirror"
)

func DefineMirrorCommand(parent *kingpin.Application) *kingpin.CmdClause {
	if app.IsDebug {
		logs.Debug.SetOutput(os.Stderr)
	}

	cmd := parent.Command("mirror", "Copy Deckhouse images from Deckhouse registry to local filesystem and to third-party registry")
	app.DefineMirrorFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		if app.MirrorRegistry != "" {
			return log.Process("mirror", "Push mirrored Deckhouse images from local filesystem to private registry", func() error {
				return mirrorPushDeckhouseToPrivateRegistry()
			})
		}

		return log.Process("mirror", "Pull Deckhouse images from registry to local filesystem", func() error {
			return mirrorPullDeckhouseToLocalFilesystem()
		})
	})

	return cmd
}

func mirrorPushDeckhouseToPrivateRegistry() error {
	mirrorCtx := &mirror.Context{
		Insecure:              app.MirrorInsecure,
		SkipTLSVerification:   app.MirrorTLSSkipVerify,
		RegistryHost:          app.MirrorRegistryHost,
		RegistryPath:          app.MirrorRegistryPath,
		DeckhouseRegistryRepo: app.MirrorSourceRegistryRepo,
		BundlePath:            app.MirrorImagesBundlePath,
		UnpackedImagesPath:    filepath.Join(app.TmpDirName, time.Now().Format("mirror_tmp_02-01-2006_15-04-05")),
		ValidationMode:        mirror.ValidationMode(app.MirrorValidationMode),
	}

	if app.MirrorRegistryUsername != "" {
		mirrorCtx.RegistryAuth = authn.FromConfig(authn.AuthConfig{
			Username: app.MirrorRegistryUsername,
			Password: app.MirrorRegistryPassword,
		})
	}

	defer os.RemoveAll(mirrorCtx.UnpackedImagesPath)

	if err := mirror.ValidateWriteAccessForRepo(
		mirrorCtx.RegistryHost+mirrorCtx.RegistryPath,
		mirrorCtx.RegistryAuth,
		mirrorCtx.Insecure,
		mirrorCtx.SkipTLSVerification,
	); err != nil {
		return fmt.Errorf("Registry credentials validation failure: %w", err)
	}

	err := log.Process("mirror", "Unpacking Deckhouse bundle", func() error {
		return mirror.UnpackBundle(mirrorCtx)
	})
	if err != nil {
		return err
	}

	err = log.Process("mirror", "Push Deckhouse images to registry", func() error {
		return operations.PushDeckhouseToRegistry(mirrorCtx)
	})
	if err != nil {
		return err
	}

	return nil
}

func mirrorPullDeckhouseToLocalFilesystem() error {
	mirrorCtx := &mirror.Context{
		Insecure:              app.MirrorInsecure,
		SkipTLSVerification:   app.MirrorTLSSkipVerify,
		DoGOSTDigests:         app.MirrorDoGOSTDigest,
		RegistryHost:          app.MirrorRegistryHost,
		DeckhouseRegistryRepo: app.MirrorSourceRegistryRepo,
		RegistryAuth:          getSourceRegistryAuthProvider(),
		BundlePath:            app.MirrorImagesBundlePath,
		BundleChunkSize:       app.MirrorImagesBundleChunkSizeGB * 1024 * 1024 * 1024,
		UnpackedImagesPath: filepath.Join(
			app.TmpDirName,
			"mirror_pull",
			fmt.Sprintf("%x", md5.Sum([]byte(app.MirrorSourceRegistryRepo))),
		),
		ValidationMode: mirror.ValidationMode(app.MirrorValidationMode),
		MinVersion:     app.MirrorMinVersion,
	}

	if app.MirrorDontContinuePartialPull || lastPullWasTooLongAgoToRetry(mirrorCtx) {
		if err := os.RemoveAll(mirrorCtx.UnpackedImagesPath); err != nil {
			return fmt.Errorf("Cleanup last unfinished pull data: %w", err)
		}
	}

	if err := mirror.ValidateReadAccessForImage(
		mirrorCtx.DeckhouseRegistryRepo+":rock-solid",
		mirrorCtx.RegistryAuth,
		mirrorCtx.Insecure,
		mirrorCtx.SkipTLSVerification,
	); err != nil {
		return fmt.Errorf("Source registry access validation failure: %w", err)
	}

	var versionsToMirror []semver.Version
	var err error
	err = log.Process("mirror", "Looking for required Deckhouse releases", func() error {
		versionsToMirror, err = mirror.VersionsToCopy(mirrorCtx)
		if err != nil {
			return fmt.Errorf("Find versions to mirror: %w", err)
		}
		log.InfoF("Deckhouse releases to pull: %+v\n", versionsToMirror)
		return nil
	})
	if err != nil {
		return err
	}

	err = log.Process("mirror", "Pull images", func() error {
		return operations.MirrorDeckhouseToLocalFS(mirrorCtx, versionsToMirror)
	})
	if err != nil {
		return err
	}

	err = log.Process("mirror", "Pack images", func() error {
		return mirror.PackBundle(mirrorCtx)
	})
	if err != nil {
		return err
	}

	if mirrorCtx.DoGOSTDigests {
		err = log.Process("mirror", "Compute GOST digest", func() error {
			if err = computeGOSTDigest(mirrorCtx); err != nil {
				return fmt.Errorf("Compute GOST digest: %w", err)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	if err = os.RemoveAll(app.TmpDirName); err != nil {
		return fmt.Errorf("Cleanup temporary data after mirroring: %w", err)
	}

	return nil
}

func computeGOSTDigest(mirrorCtx *mirror.Context) error {
	bundleDir := filepath.Dir(mirrorCtx.BundlePath)
	catalog, err := os.ReadDir(bundleDir)
	if err != nil {
		return fmt.Errorf("read tar bundle directory: %w", err)
	}
	streams := make([]io.Reader, 0)
	for _, entry := range catalog {
		fileName := entry.Name()
		if !entry.Type().IsRegular() || filepath.Ext(fileName) != ".chunk" {
			continue
		}
		chunkStream, err := os.Open(filepath.Join(bundleDir, fileName))
		if err != nil {
			return fmt.Errorf("open bundle chunk for reading: %w", err)
		}
		defer chunkStream.Close() // nolint // defer in a loop is valid here as we need those streams to survive until everything is calculated at the end of this function
		streams = append(streams, chunkStream)
	}

	bundleStream := io.NopCloser(io.MultiReader(streams...))
	if len(streams) == 0 {
		bundleStream, err = os.Open(mirrorCtx.BundlePath)
		if err != nil {
			return fmt.Errorf("read tar bundle: %w", err)
		}
	}
	defer bundleStream.Close()

	gostDigest, err := mirror.CalculateBlobGostDigest(bufio.NewReaderSize(bundleStream, 512*1024))
	if err != nil {
		return fmt.Errorf("Calculate GOST Checksum: %w", err)
	}
	if err = os.WriteFile(mirrorCtx.BundlePath+".gostsum", []byte(gostDigest), 0666); err != nil {
		return fmt.Errorf("Write GOST Checksum: %w", err)
	}
	log.InfoF("Digest: %s\nWritten to %s\n", gostDigest, mirrorCtx.BundlePath+".gostsum")
	return nil
}

func lastPullWasTooLongAgoToRetry(mirrorCtx *mirror.Context) bool {
	s, err := os.Lstat(mirrorCtx.UnpackedImagesPath)
	if err != nil {
		return false
	}

	return time.Since(s.ModTime()) > 24*time.Hour
}

func getSourceRegistryAuthProvider() authn.Authenticator {
	if app.MirrorSourceRegistryLogin != "" {
		return authn.FromConfig(authn.AuthConfig{
			Username: app.MirrorSourceRegistryLogin,
			Password: app.MirrorSourceRegistryPassword,
		})
	}

	if app.MirrorDHLicenseToken != "" {
		return authn.FromConfig(authn.AuthConfig{
			Username: "license-token",
			Password: app.MirrorDHLicenseToken,
		})
	}

	return authn.Anonymous
}
