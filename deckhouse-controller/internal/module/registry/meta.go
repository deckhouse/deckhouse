/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package registry

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/iancoleman/strcase"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"gopkg.in/yaml.v3"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	moduletypes "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader/types"
)

const (
	pathVersion    = "version.json"
	pathDefinition = "module.yaml"
)

type Meta struct {
	Checksum   string
	Version    *semver.Version
	Changelog  map[string]any
	Definition *moduletypes.Definition
}

// GetReleaseMeta downloads the release image and parses its meta
// <registry>/modules/<module>/release:<releaseChannel>
func (s *Service) GetReleaseMeta(ctx context.Context, ms *v1alpha1.ModuleSource, module, releaseChannel string) (*Meta, error) {
	module = fmt.Sprintf("%s/release", module)
	releaseChannel = strcase.ToKebab(releaseChannel)

	// <registry>/modules/<module>/release:<releaseChannel>
	return s.GetModuleMeta(ctx, ms, module, releaseChannel)
}

// GetModuleMeta downloads the module image and parses its meta
// <registry>/modules/<module>:<tag>
func (s *Service) GetModuleMeta(ctx context.Context, ms *v1alpha1.ModuleSource, module, tag string) (*Meta, error) {
	_, span := otel.Tracer(tracerName).Start(ctx, "GetImageMeta")
	defer span.End()

	span.SetAttributes(attribute.String("module", module))
	span.SetAttributes(attribute.String("tag", tag))
	span.SetAttributes(attribute.String("source", ms.GetName()))

	logger := s.logger.With("module", module, "tag", tag)

	logger.Debug("download module release image")

	// <registry>/modules/<module>
	cli, err := s.buildRegistryClient(ms, module)
	if err != nil {
		return nil, fmt.Errorf("build registry client: %w", err)
	}

	// get <registry>/modules/<module>:<tag>
	img, err := cli.Image(ctx, tag)
	if err != nil {
		return nil, fmt.Errorf("get release image: %w", err)
	}

	meta := new(Meta)

	digest, err := img.Digest()
	if err != nil {
		return nil, fmt.Errorf("get release image digest: %w", err)
	}

	meta.Checksum = digest.String()

	reader := &metaReader{
		versionReader:   bytes.NewBuffer(nil),
		changelogReader: bytes.NewBuffer(nil),
		moduleReader:    bytes.NewBuffer(nil),
	}

	rc := mutate.Extract(img)
	defer rc.Close()

	if err = reader.untarMetadata(rc); err != nil {
		return meta, fmt.Errorf("untar release image: %w", err)
	}

	if reader.versionReader.Len() > 0 {
		version := new(semver.Version)
		if err = json.NewDecoder(reader.versionReader).Decode(version); err != nil {
			return nil, fmt.Errorf("decode version: %w", err)
		}

		meta.Version = version
	}

	if reader.moduleReader.Len() > 0 {
		def := new(moduletypes.Definition)
		if err = yaml.NewDecoder(reader.moduleReader).Decode(def); err != nil {
			return nil, fmt.Errorf("decode module definition: %w", err)
		}

		meta.Definition = def
	}

	if reader.changelogReader.Len() > 0 {
		var changelog map[string]any
		if err = yaml.NewDecoder(reader.changelogReader).Decode(&changelog); err != nil {
			s.logger.Warn("failed to unmarshal changelog")
			changelog = make(map[string]any)
		}

		meta.Changelog = changelog
	}

	return meta, nil
}

type metaReader struct {
	versionReader   *bytes.Buffer
	changelogReader *bytes.Buffer
	moduleReader    *bytes.Buffer
}

func (r *metaReader) untarMetadata(rc io.ReadCloser) error {
	tr := tar.NewReader(rc)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			// end of archive
			return nil
		}
		if err != nil {
			return err
		}

		// skip werf files
		if strings.HasPrefix(hdr.Name, ".werf") {
			continue
		}

		switch strings.ToLower(hdr.Name) {
		case pathVersion:
			if _, err = io.Copy(r.versionReader, tr); err != nil {
				return err
			}

		case "changelog.yaml", "changelog.yml":
			if _, err = io.Copy(r.changelogReader, tr); err != nil {
				return err
			}

		case pathDefinition:
			if _, err = io.Copy(r.moduleReader, tr); err != nil {
				return err
			}

		default:
			continue
		}
	}
}
