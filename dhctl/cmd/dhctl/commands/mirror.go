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
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-containerregistry/pkg/authn"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/mirror"
)

func DefineMirrorCommand(parent *kingpin.Application) *kingpin.CmdClause {
	cmd := parent.Command("mirror", "Copy Deckhouse registry for air-gaped installation.")
	app.DefineMirrorFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		if app.MirrorRegistryHost != "" {
			return log.Process("mirror", "Push mirrored Deckhouse Registry images to private registry", func() error {
				return mirrorPushDeckhouseToPrivateRegistry()
			})
		}

		return log.Process("mirror", "Pull Deckhouse to local filesystem", func() error {
			return mirrorPullDeckhouseToLocalFilesystem()
		})
	})

	return cmd
}

func mirrorPushDeckhouseToPrivateRegistry() error {
	mirrorCtx := &mirror.Context{
		Insecure:       app.MirrorInsecure,
		RegistryHost:   app.MirrorRegistryHost,
		RegistryRepo:   app.MirrorDeckhouseRegistryRepo,
		ImagesPath:     app.MirrorImagesPath,
		ValidationMode: mirror.ValidationMode(app.MirrorValidationMode),
	}

	if app.MirrorRegistryUsername != "" {
		mirrorCtx.RegistryAuth = authn.FromConfig(authn.AuthConfig{
			Username: app.MirrorRegistryUsername,
			Password: app.MirrorRegistryPassword,
		})
	}

	return operations.PushMirrorToRegistry(mirrorCtx)
}

func mirrorPullDeckhouseToLocalFilesystem() error {
	mirrorCtx := &mirror.Context{
		Insecure:     app.MirrorInsecure,
		RegistryHost: app.MirrorRegistryHost,
		RegistryRepo: app.MirrorDeckhouseRegistryRepo,
		RegistryAuth: authn.FromConfig(authn.AuthConfig{
			Username: "license-token",
			Password: app.MirrorDHLicenseToken,
		}),
		ImagesPath:     app.MirrorImagesPath,
		ValidationMode: mirror.ValidationMode(app.MirrorValidationMode),
		MinVersion:     app.MirrorMinVersion,
	}

	var versionsToMirror []*semver.Version
	var err error
	err = log.Process("mirror", "Looking for required Deckhouse releases", func() error {
		versionsToMirror, err = mirror.VersionsToCopy(mirrorCtx)
		if err != nil {
			return fmt.Errorf("find versions to mirror: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	err = log.Process("mirror", "Pull images", func() error {
		return operations.MirrorRegistryToLocalFS(mirrorCtx, versionsToMirror)
	})
	if err != nil {
		return err
	}
	return nil
}
