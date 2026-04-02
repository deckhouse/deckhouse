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
	"fmt"
	"io"

	"gopkg.in/yaml.v3"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/dto"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	// versionFile is the name of the JSON file inside the image tar that contains the package version.
	versionFile = "version.json"

	// maxMetadataFileSize limits the size of individual metadata files extracted from tar archives.
	// This guards against OOM from malicious or corrupted images containing oversized entries.
	maxMetadataFileSize = 1 << 20 // 1 MB
)

// packageMetadata holds all metadata extracted from a package image tar archive.
type packageMetadata struct {
	version    string
	changelog  packageChangelog
	definition dto.ApplicationDefinition
}

// packageChangelog represents user-facing release notes for a package version.
type packageChangelog struct {
	Features []string `yaml:"features,omitempty"`
	Fixes    []string `yaml:"fixes,omitempty"`
}

// metadataReader buffers the raw content of each metadata file extracted from the tar.
// Each buffer may remain empty if the corresponding file is absent from the archive.
type metadataReader struct {
	definitionReader *bytes.Buffer
	versionReader    *bytes.Buffer
	changelogReader  *bytes.Buffer
}

// parseVersionMetadataByImage extracts package metadata from a tar-formatted image reader.
// It looks for three files: version.json, package.yaml (definition), and changelog.yaml.
// All files are optional — missing files result in zero-value fields in the returned metadata.
func (r *reconciler) parseVersionMetadataByImage(_ context.Context, img io.Reader) (*packageMetadata, error) {
	meta := new(packageMetadata)

	mr := &metadataReader{
		versionReader:    bytes.NewBuffer(nil),
		changelogReader:  bytes.NewBuffer(nil),
		definitionReader: bytes.NewBuffer(nil),
	}

	if err := mr.untarMetadata(img); err != nil {
		return nil, fmt.Errorf("untar metadata: %w", err)
	}

	if mr.versionReader.Len() > 0 {
		version := struct {
			Version string `json:"version"`
		}{}

		if err := json.NewDecoder(mr.versionReader).Decode(&version); err != nil {
			return nil, fmt.Errorf("unmarshal version file: %w", err)
		}

		meta.version = version.Version
	}

	if mr.definitionReader.Len() > 0 {
		if err := yaml.NewDecoder(mr.definitionReader).Decode(&meta.definition); err != nil {
			return nil, fmt.Errorf("unmarshal package definition: %w", err)
		}
	}

	if mr.changelogReader.Len() > 0 {
		if err := yaml.NewDecoder(mr.changelogReader).Decode(&meta.changelog); err != nil {
			r.logger.Warn("unmarshal package changelog", log.Err(err))
		}
	}

	return meta, nil
}

// untarMetadata iterates through the tar archive and copies the content of recognized
// metadata files (version.json, changelog.yaml/yml, package.yaml) into their respective buffers.
// Unrecognized entries are skipped. Each file read is bounded by maxMetadataFileSize.
func (r *metadataReader) untarMetadata(rc io.Reader) error {
	tr := tar.NewReader(rc)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		switch hdr.Name {
		case versionFile:
			if _, err = io.Copy(r.versionReader, io.LimitReader(tr, maxMetadataFileSize)); err != nil {
				return err
			}
		case "changelog.yaml", "changelog.yml":
			if _, err = io.Copy(r.changelogReader, io.LimitReader(tr, maxMetadataFileSize)); err != nil {
				return err
			}
		case dto.DefinitionFile:
			if _, err = io.Copy(r.definitionReader, io.LimitReader(tr, maxMetadataFileSize)); err != nil {
				return err
			}
		default:
			continue
		}

		// All metadata files found — skip remaining tar entries.
		if r.versionReader.Len() > 0 && r.changelogReader.Len() > 0 && r.definitionReader.Len() > 0 {
			return nil
		}
	}
}
