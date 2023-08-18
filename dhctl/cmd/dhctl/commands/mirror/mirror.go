//go:build !ce

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
	"os"
	"strings"

	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/types"
	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands/mirror/image"
	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands/mirror/util"
	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands/mirror/versions"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/yaml.v3"
)

const (
	eeEdition = "ee"
	feEdition = "fe"

	destinationHelp = `destination for images to write (archive file: "file:<file path>.tar.gz" or registry: "docker://<registry repository").`
	sourceHelp      = `source for deckhouse images (archive file: "file:<file path>.tar.gz" or registry: "docker://<registry repository").`

	registryRegexp = `^(file:.+\.tar\.gz|docker://.+)$`
)

var (
	ErrNotEE     = errors.New("dhctl mirror can be used only in deckhouse EE")
	ErrNoLicense = errors.New("license is required to download Deckhouse Enterprise Edition. Please provide it with CLI argument --source-password")

	versionLatestRE = fmt.Sprintf(`^(%s|latest)$`, versions.VersionRE)
)

func DefineMirrorCommand(kpApp *kingpin.Application) *kingpin.CmdClause {
	var (
		minVersion = app.NewStringWithRegexpValidation(versionLatestRE)

		source         = app.NewStringWithRegexpValidation(registryRegexp)
		sourceUser     string
		sourcePassword string
		sourceInsecure bool
		sourceCAFile   string

		destination         = app.NewStringWithRegexpValidation(registryRegexp)
		destinationUser     string
		destinationPassword string
		destinationInsecure bool
		destinationCAFile   string
		dryRun              bool

		outputReportFile string
		outputFormat     string
	)

	edition, err := deckhouseEdition()
	if err != nil {
		panic(err)
	}

	cmd := kpApp.Command("mirror", "Copy images from deckhouse registry or tar.gz file to specified registry or tar.gz file.")

	cmd.Arg("DESTINATION", destinationHelp).Required().SetValue(destination)
	cmd.Flag("source", sourceHelp).Default(fmt.Sprintf("docker://registry.deckhouse.io/deckhouse/%s", edition)).SetValue(source)

	cmd.Flag("dry-run", "Run without actually copying data.").BoolVar(&dryRun)
	cmd.Flag("min-version", `The oldest version of deckhouse from your clusters or "latest" for clean installation.`).SetValue(minVersion)
	cmd.Flag("output-file", "File to save report with updated in destination registry images references.").StringVar(&outputReportFile)
	cmd.Flag("output", "Format of the output report.").Default("json").EnumVar(&outputFormat, "yaml", "json")

	// Deckhouse registry flags
	cmd.Flag("source-username", "Username for the source registry.").Default("license-token").StringVar(&sourceUser)
	cmd.Flag("source-password", "Password for the source registry.").StringVar(&sourcePassword)
	cmd.Flag("source-insecure", "Use http instead of https while connecting to source registry.").BoolVar(&sourceInsecure)
	cmd.Flag("source-ca-file", "Path to source registry CA.").ExistingFileVar(&sourceCAFile)

	// Destination registry flags
	cmd.Flag("dest-username", "Username for the destination registry.").StringVar(&destinationUser)
	cmd.Flag("dest-password", "Password for the destination registry.").StringVar(&destinationPassword)
	cmd.Flag("dest-insecure", "Use http instead of https while connecting to destination registry.").BoolVar(&destinationInsecure)
	cmd.Flag("dest-ca-file", "Path to destination registry CA.").ExistingFileVar(&destinationCAFile)

	logger := log.NewPrettyLogger()
	runFunc := func() error {
		ctx := context.Background()
		sourceTempCertsDir, err := util.CreateCertsDir(sourceCAFile)
		if err != nil {
			return err
		}
		destTempCertsDir, err := util.CreateCertsDir(destinationCAFile)
		if err != nil {
			return err
		}

		logger.LogDebugLn("Initializing source registry...")
		if strings.HasPrefix(source.String(), "docker://registry.deckhouse.io") && sourcePassword == "" {
			return ErrNoLicense
		}
		source, err := image.NewRegistry(source.String(), registryAuth(sourceUser, sourcePassword))
		if err != nil {
			return err
		}
		defer source.Close()

		logger.LogDebugLn("Initializing destination registry...")
		dest, err := image.NewRegistry(destination.String(), registryAuth(destinationUser, destinationPassword))
		if err != nil {
			return err
		}

		destListOptions := []image.ListOption{image.WithCertsDir(destTempCertsDir)}
		if destinationInsecure {
			destListOptions = append(destListOptions, image.WithInsecure())
		}

		sourceListOptions := []image.ListOption{image.WithCertsDir(sourceTempCertsDir)}
		if destinationInsecure {
			sourceListOptions = append(sourceListOptions, image.WithInsecure())
		}

		policyContext, err := image.NewPolicyContext()
		if err != nil {
			return err
		}
		defer policyContext.Destroy()

		copyOpts := []image.CopyOption{
			image.WithOutput(logger),
			image.WithSourceCertsDir(sourceTempCertsDir),
			image.WithDestCertsDir(destTempCertsDir),
		}

		finder := versions.NewVersionsComparer(source, dest, destListOptions, sourceListOptions, copyOpts, policyContext, logger)
		allImagesToCopy, err := finder.ImagesToCopy(ctx, minVersion.String())
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

		updatedImages := make(imagesWithModules)
		copyLogger := logger.ProcessLogger()
		copyLogger.LogProcessStart("Mirror images")
		logger.LogDebugF("will push %d images to destination registry\n", len(allImagesToCopy))
		for _, src := range allImagesToCopy {
			var exists bool
			for errCount := 0; errCount < 3; errCount++ {
				exists, err = copyImage(ctx, src, dest, policyContext, logger, copyOpts...)
				if err == nil {
					break
				}
				logger.LogDebugF("error copying image %s with tag %s and digest %s to %s: %v", src.Path(), src.Tag(), src.Digest(), dest.Path(), err)
			}
			if err != nil {
				copyLogger.LogProcessFail()
				return err
			}

			if exists {
				continue
			}

			updatedImages.set(src.WithNewRegistry(dest).Path(), src.Tag(), src.Digest())
		}
		copyLogger.LogProcessEnd()

		if !dryRun {
			commitLogger := logger.ProcessLogger()
			commitLogger.LogProcessStart("Commit to registry")
			if err := dest.Commit(); err != nil {
				commitLogger.LogProcessFail()
				return err
			}
			commitLogger.LogProcessEnd()
		}
		defer dest.Close()

		reportLogger := logger.ProcessLogger()
		reportLogger.LogProcessStart("Updated images report")
		if err := saveReportToFile(updatedImages, outputReportFile, outputFormat, logger); err != nil {
			reportLogger.LogProcessFail()
			return err
		}
		reportLogger.LogProcessEnd()

		return nil
	}

	cmd.Action(func(c *kingpin.ParseContext) error {
		return logger.LogProcess("mirror", "Copy images", runFunc)
	})
	return cmd
}

func deckhouseEdition() (string, error) {
	content, err := os.ReadFile("/deckhouse/edition")
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrNotEE, err)
	}

	edition := strings.TrimSpace(string(content))
	if edition != eeEdition && edition != feEdition {
		return "", ErrNotEE
	}

	return edition, nil
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

func copyImage(ctx context.Context, srcImage *image.ImageConfig, destRegistry *image.RegistryConfig, policyContext *signature.PolicyContext, logger log.Logger, opts ...image.CopyOption) (bool, error) {
	srcImg := sourceImage(srcImage)
	destImage := destinationImage(destRegistry, srcImage)
	return image.CopyImage(ctx, srcImg, destImage, policyContext, logger, opts...)
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

type imagesWithModules map[string]map[string]map[string]string

func (u imagesWithModules) set(regPath, tag, digest string) {
	var d8Version, moduleName, moduleImage string
	if splitted := strings.Split(tag, versions.Delimiter); len(splitted) == 3 {
		d8Version, moduleName, moduleImage = splitted[0], splitted[1], splitted[2]
	} else {
		d8Version, moduleName, moduleImage = "otherImages", regPath, tag
	}

	if _, f := u[d8Version]; !f {
		u[d8Version] = make(map[string]map[string]string)
	}
	if _, f := u[d8Version][moduleName]; !f {
		u[d8Version][moduleName] = make(map[string]string)
	}
	u[d8Version][moduleName][moduleImage] = digest
}
