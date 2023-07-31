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

package mirror

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/types"
	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands/mirror/image"
	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands/mirror/versions"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/yaml.v3"
)

const (
	eeEdition = "ee"
	feEdition = "fe"

	destinationHelp = `destination for images to write (archive file: "file:<file path>.tar.gz" or registry: "docker://<registry repositroy").`
	sourceHelp      = `source for deckhouse images (archive file: "file:<file path>.tar.gz" or registry: "docker://<registry repositroy").`

	registryRegexp = `^(file:.+\.tar\.gz|docker://.+)$`
)

var (
	ErrNotEE     = errors.New("dhctl mirror can be used only in deckhouse EE")
	ErrNoLicense = errors.New("license is required to download Deckhouse Enterprise Edition. Please provide it with CLI argument --license")

	versionLatestRE = fmt.Sprintf(`^(%s|latest)$`, versions.VersionRE)
)

func DefineMirrorCommand(kpApp *kingpin.Application) *kingpin.CmdClause {
	var (
		minVersion = app.NewStringWithRegexpValidation(versionLatestRE)

		source       = app.NewStringWithRegexpValidation(registryRegexp)
		licenseToken string

		destination         = app.NewStringWithRegexpValidation(registryRegexp)
		destinationUser     string
		destinationPassword string
		destinationInsecure bool
		dryRun              bool

		outputReportFile string
		outputFormat     string
	)

	cmd := kpApp.Command("mirror", "Copy images from deckhouse registry or tar.gz file to specified registry or tar.gz file.")

	cmd.Arg("DESTINATION", destinationHelp).Required().SetValue(destination)
	cmd.Flag("from", sourceHelp).Default("docker://registry.deckhouse.io/deckhouse").SetValue(source)

	cmd.Flag("dry-run", "Run without actually copying data.").BoolVar(&dryRun)
	cmd.Flag("min-version", `The oldest version of deckhouse from your clusters or "latest" for clean installation.`).SetValue(minVersion)
	cmd.Flag("output-file", "File to save report with updated in destination registry images references.").StringVar(&outputReportFile)
	cmd.Flag("output", "Format of the output report.").Default("json").EnumVar(&outputFormat, "yaml", "json")

	// Deckhouse registry flags
	cmd.Flag("license", "License key for Deckhouse registry.").Required().StringVar(&licenseToken)

	// Destination registry flags
	cmd.Flag("username", "Username for the destination registry.").StringVar(&destinationUser)
	cmd.Flag("password", "Password for the destination registry.").StringVar(&destinationPassword)
	cmd.Flag("insecure", "Use http instead of https while connecting to destination registry.").BoolVar(&destinationInsecure)

	logger := log.NewPrettyLogger()
	runFunc := func() error {
		ctx := context.Background()

		edition, err := deckhouseEdition()
		if err != nil {
			return err
		}

		source, err := deckhouseRegistry(source.String(), edition, licenseToken)
		if err != nil {
			return err
		}
		defer source.Close()

		dest, err := image.NewRegistry(destination.String(), registryAuth(destinationUser, destinationPassword), false)
		if err != nil {
			return err
		}

		destListOptions := make([]image.ListOption, 0)
		if destinationInsecure {
			destListOptions = append(destListOptions, image.WithInsecure())
		}

		policyContext, err := image.NewPolicyContext()
		if err != nil {
			return err
		}
		defer policyContext.Destroy()

		copyOpts := []image.CopyOption{
			image.WithOutput(logger),
		}

		finder := versions.NewVersionsComparer(source, dest, destListOptions, nil, copyOpts, policyContext, logger)
		modulesImages, err := finder.ImagesToCopy(ctx, minVersion.String())
		if err != nil {
			return err
		}

		copyOpts = append(copyOpts, image.WithCopyAllImages(), image.WithPreserveDigests())
		if destinationInsecure {
			copyOpts = append(copyOpts, image.WithDestInsecure())
		}

		if dryRun {
			copyOpts = append(copyOpts, image.WithDryRun())
		}

		updatedImages := make(map[string]map[string]map[string]string /* map[<d8 version>]map[<module name>]map[<image name>]<digest or tag> */)
		copyLogger := logger.ProcessLogger()
		copyLogger.LogProcessStart("Mirror images")
		for _, src := range modulesImages {
			exists, err := copyImage(ctx, src, dest, policyContext, copyOpts...)
			if err != nil {
				copyLogger.LogProcessFail()
				return err
			}

			splitted := strings.Split(src.Tag(), versions.Delimiter)
			if exists || len(splitted) != 3 {
				continue
			}

			d8Version, moduleName, moduleImage := splitted[0], splitted[1], splitted[2]
			if _, f := updatedImages[d8Version]; !f {
				updatedImages[d8Version] = make(map[string]map[string]string)
			}
			if _, f := updatedImages[d8Version][moduleName]; !f {
				updatedImages[d8Version][moduleName] = make(map[string]string)
			}
			updatedImages[d8Version][moduleName][moduleImage] = src.Digest()
		}

		if err := dest.Commit(); err != nil {
			copyLogger.LogProcessFail()
			return err
		}
		defer copyLogger.LogProcessEnd()

		return saveReportToFile(updatedImages, outputReportFile, outputFormat, logger)
	}

	cmd.Action(func(c *kingpin.ParseContext) error {
		return logger.LogProcess("mirror", "Copy images", runFunc)
	})
	return cmd
}

func deckhouseEdition() (string, error) {
	content, err := os.ReadFile("/deckhouse/edition")
	if err != nil {
		return "", err
	}

	edition := strings.TrimSpace(string(content))
	if edition != eeEdition && edition != feEdition {
		return "", ErrNotEE
	}

	return edition, nil
}

func deckhouseRegistry(deckhouseRegistry, edtiton, licenseToken string) (*image.RegistryConfig, error) {
	registry, err := image.NewRegistry(deckhouseRegistry, nil, true)
	if err != nil {
		return nil, err
	}

	if registry.Transport() != image.DockerTransport {
		return registry, nil
	}

	auth, err := deckhouseRegistryAuth(edtiton, licenseToken)
	if err != nil {
		return nil, err
	}

	u, err := url.Parse(deckhouseRegistry)
	if err != nil {
		return nil, err
	}
	u.Path = filepath.Join(u.Path, edtiton)
	return image.NewRegistry(u.String(), auth, true)
}

func deckhouseRegistryAuth(edition, licenseToken string) (*types.DockerAuthConfig, error) {
	if licenseToken == "" {
		return nil, ErrNoLicense
	}
	return registryAuth("license-token", licenseToken), nil
}

func registryAuth(username, password string) *types.DockerAuthConfig {
	if username == "" || password == "" {
		return nil
	}

	return &types.DockerAuthConfig{
		Username: username,
		Password: password,
	}
}

func copyImage(ctx context.Context, srcImage *image.ImageConfig, destRegistry *image.RegistryConfig, policyContext *signature.PolicyContext, opts ...image.CopyOption) (bool, error) {
	srcImg := sourceImage(srcImage)
	destImage := destinationImage(destRegistry, srcImage)
	return image.CopyImage(ctx, srcImg, destImage, policyContext, opts...)
}

// sourceImage source destination image
func sourceImage(srcImage *image.ImageConfig) *image.ImageConfig {
	// https://github.com/containers/image/blob/v5.26.1/docker/docker_transport.go#L80
	// If image has both tag and digest we want to pull it with digest
	if srcImage.RegistryTransport() == image.DockerTransport && srcImage.Digest() != "" {
		return srcImage.WithTag("")
	}
	return srcImage
}

// destinationImage prepares destination image
func destinationImage(destRegistry *image.RegistryConfig, srcImage *image.ImageConfig) *image.ImageConfig {
	destImage := srcImage.WithNewRegistry(destRegistry)
	// https://github.com/containers/image/blob/v5.26.1/docker/docker_transport.go#L80
	// If image has both tag and digest we want to push it with tag (because digest will be saved)
	// (because when pushing with digest image becames dangling in the registry)
	if destRegistry.Transport() == image.DockerTransport && srcImage.Tag() != "" {
		return destImage.WithDigest("")
	}
	return destImage
}

func saveReportToFile(content interface{}, filename, outFormat string, logger *log.PrettyLogger) error {
	var (
		f   io.Writer
		err error
	)

	if filename == "" {
		logger.LogSuccess("updated images report:\n\n")
		f = logger
	} else {
		logger.LogSuccess("saved updated images report to file\n")
		f, err = os.Create(filename)
	}
	if err != nil {
		return err
	}

	var marshaledReport []byte
	switch outFormat {
	case "yaml":
		b := bytes.NewBuffer(nil)
		yamlEncoder := yaml.NewEncoder(b)
		yamlEncoder.SetIndent(2)
		yamlEncoder.Encode(content)
		marshaledReport = b.Bytes()
	case "json":
		marshaledReport, err = json.MarshalIndent(content, "", "  ")
	}
	if err != nil {
		return err
	}

	_, err = f.Write(append(marshaledReport, '\n'))
	return err
}
