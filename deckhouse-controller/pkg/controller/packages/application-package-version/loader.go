// Copyright 2025 Flant JSC
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

package applicationpackageversion

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	registryv1 "github.com/google/go-containerregistry/pkg/v1"
	"gopkg.in/yaml.v2"

	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	"github.com/deckhouse/deckhouse/pkg/log"
)

type VersionFile struct {
	Version string `json:"version"`
}

type PackageMetadata struct {
	Version           string
	Changelog         map[string]interface{}
	PackageDefinition *PackageDefinition
}

var ErrImageIsNil = errors.New("image is nil")

type packageReader struct {
	versionReader   *bytes.Buffer
	changelogReader *bytes.Buffer
	packageReader   *bytes.Buffer
}

func (rr *packageReader) untarMetadata(rc io.Reader) error {
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

		switch hdr.Name {
		case "version.json":
			_, err = io.Copy(rr.versionReader, tr)
			if err != nil {
				return err
			}
		case "changelog.yaml", "changelog.yml":
			_, err = io.Copy(rr.changelogReader, tr)
			if err != nil {
				return err
			}
		case "package.yaml", "package.yml":
			_, err := io.Copy(rr.packageReader, tr)
			if err != nil {
				return err
			}

		default:
			continue
		}
	}
}

func (r *reconciler) fetchPackageMetadata(_ context.Context, img registryv1.Image) (*PackageMetadata, error) {
	if img == nil {
		return nil, ErrImageIsNil
	}

	meta := new(PackageMetadata)

	rc, err := cr.Extract(img)
	if err != nil {
		return nil, fmt.Errorf("extract image: %w", err)
	}
	defer rc.Close()

	rr := &packageReader{
		versionReader:   bytes.NewBuffer(nil),
		changelogReader: bytes.NewBuffer(nil),
		packageReader:   bytes.NewBuffer(nil),
	}

	err = rr.untarMetadata(rc)
	if err != nil {
		return nil, fmt.Errorf("untar metadata: %w", err)
	}

	if rr.versionReader.Len() > 0 {
		var version VersionFile
		err = json.NewDecoder(rr.versionReader).Decode(&version)
		if err != nil {
			return nil, fmt.Errorf("metadata decode: %w", err)
		}
		meta.Version = version.Version
	}

	if rr.packageReader.Len() > 0 {
		var PackageDefinition PackageDefinition
		err = yaml.NewDecoder(rr.packageReader).Decode(&PackageDefinition)
		if err != nil {
			return nil, fmt.Errorf("unmarshal package yaml failed: %w", err)
		}

		meta.PackageDefinition = &PackageDefinition
	}

	if rr.changelogReader.Len() > 0 {
		var changelog map[string]any

		err = yaml.NewDecoder(rr.changelogReader).Decode(&changelog)
		if err != nil {
			// if changelog build failed - warn about it but don't fail the release
			r.logger.Warn("Unmarshal CHANGELOG yaml failed", log.Err(err))

			changelog = make(map[string]any)
		}

		meta.Changelog = changelog
	}

	return meta, nil
}
