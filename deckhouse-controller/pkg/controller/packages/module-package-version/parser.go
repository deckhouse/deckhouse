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

// Package modulepackageversion reconciles ModulePackageVersion resources by
// fetching package metadata from the registry and promoting drafts to ready.
package modulepackageversion

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/dto"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	moduletypes "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader/types"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	// versionFile is the JSON file inside the image tar that carries the package version.
	versionFile = "version.json"

	// moduleDefinitionFile is the legacy module manifest still emitted by older packages.
	// dto.DefinitionFile is the v2 manifest emitted by the new packaging pipeline.
	moduleDefinitionFile = "module.yaml"

	// changelogFile is the YAML release-notes file optionally shipped with a package.
	changelogFile = "changelog.yaml"

	// changelogFileYML is the alternate extension used by some packages.
	changelogFileYML = "changelog.yml"

	// settingsSchemaFile is the OpenAPI schema of the user-facing package settings;
	// legacySettingsSchemaFile is its older name, used when the preferred one is absent.
	settingsSchemaFile       = "openapi/settings.yaml"
	legacySettingsSchemaFile = "openapi/config-values.yaml"

	// valuesSchemaFile is the OpenAPI schema of the effective package values.
	valuesSchemaFile = "openapi/values.yaml"

	// maxMetadataFileSize bounds each metadata file extracted from the tar archive.
	// This guards against OOM from malicious or corrupted images.
	maxMetadataFileSize = 1 << 20 // 1 MB
)

// moduleMetadata holds all metadata extracted from a module package image tar archive.
// Either packageDefinition (v2) or moduleDefinition (legacy fallback) is populated, not both.
type moduleMetadata struct {
	version           string
	changelog         packageChangelog
	packageDefinition *dto.ModuleDefinition
	moduleDefinition  *moduletypes.Definition
	schemas           *v1alpha1.PackageVersionStatusSchemas
}

// packageChangelog represents user-facing release notes for a package version.
type packageChangelog struct {
	Features []string `yaml:"features,omitempty" json:"features,omitempty"`
	Fixes    []string `yaml:"fixes,omitempty" json:"fixes,omitempty"`
}

// metadataReader buffers the raw content of each metadata file extracted from the tar.
// Each buffer may remain empty if the corresponding file is absent from the archive.
type metadataReader struct {
	versionReader        *bytes.Buffer
	changelogReader      *bytes.Buffer
	packageReader        *bytes.Buffer
	moduleReader         *bytes.Buffer
	settingsReader       *bytes.Buffer
	legacySettingsReader *bytes.Buffer
	valuesReader         *bytes.Buffer
}

// parseVersionMetadataByImage extracts module metadata from a tar-formatted image reader.
// It looks for: version.json, package.yaml (v2 definition), module.yaml (legacy
// definition), changelog.yaml, and the openapi schema files (settings.yaml with
// the config-values.yaml fallback, values.yaml). All files are optional — missing
// files result in zero-value fields in the returned metadata.
func (r *reconciler) parseVersionMetadataByImage(_ context.Context, img io.Reader) (*moduleMetadata, error) {
	meta := new(moduleMetadata)

	mr := &metadataReader{
		versionReader:        bytes.NewBuffer(nil),
		changelogReader:      bytes.NewBuffer(nil),
		packageReader:        bytes.NewBuffer(nil),
		moduleReader:         bytes.NewBuffer(nil),
		settingsReader:       bytes.NewBuffer(nil),
		legacySettingsReader: bytes.NewBuffer(nil),
		valuesReader:         bytes.NewBuffer(nil),
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

	// Prefer v2 package.yaml; fall back to legacy module.yaml if v2 is absent.
	switch {
	case mr.packageReader.Len() > 0:
		var def dto.ModuleDefinition
		if err := yaml.Unmarshal(mr.packageReader.Bytes(), &def); err != nil {
			return nil, fmt.Errorf("decode package.yaml: %w", err)
		}

		meta.packageDefinition = &def
	case mr.moduleReader.Len() > 0:
		var def moduletypes.Definition
		if err := yaml.Unmarshal(mr.moduleReader.Bytes(), &def); err != nil {
			return nil, fmt.Errorf("decode module.yaml: %w", err)
		}

		meta.moduleDefinition = &def
	}

	if mr.changelogReader.Len() > 0 {
		if err := yaml.Unmarshal(mr.changelogReader.Bytes(), &meta.changelog); err != nil {
			r.logger.Warn("unmarshal package changelog", log.Err(err))
		}
	}

	// Prefer settings.yaml; fall back to the legacy config-values.yaml name.
	settingsRaw := mr.settingsReader.Bytes()
	if len(settingsRaw) == 0 {
		settingsRaw = mr.legacySettingsReader.Bytes()
	}

	schemas, err := v1alpha1.ParsePackageSchemas(settingsRaw, mr.valuesReader.Bytes())
	if err != nil {
		return nil, fmt.Errorf("parse package schemas: %w", err)
	}

	meta.schemas = schemas

	return meta, nil
}

// untarMetadata iterates through the tar archive and copies the content of recognized
// metadata files (version.json, package.yaml, module.yaml, changelog.yaml/yml, the
// openapi schema files) into their respective buffers. Unrecognized entries are
// skipped. Each file read is bounded by maxMetadataFileSize.
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

		switch strings.ToLower(hdr.Name) {
		case versionFile:
			if _, err = io.Copy(r.versionReader, io.LimitReader(tr, maxMetadataFileSize)); err != nil {
				return err
			}
		case dto.DefinitionFile:
			if _, err = io.Copy(r.packageReader, io.LimitReader(tr, maxMetadataFileSize)); err != nil {
				return err
			}
		case moduleDefinitionFile:
			if _, err = io.Copy(r.moduleReader, io.LimitReader(tr, maxMetadataFileSize)); err != nil {
				return err
			}
		case changelogFile, changelogFileYML:
			if _, err = io.Copy(r.changelogReader, io.LimitReader(tr, maxMetadataFileSize)); err != nil {
				return err
			}
		case settingsSchemaFile:
			if _, err = io.Copy(r.settingsReader, io.LimitReader(tr, maxMetadataFileSize)); err != nil {
				return err
			}
		case legacySettingsSchemaFile:
			if _, err = io.Copy(r.legacySettingsReader, io.LimitReader(tr, maxMetadataFileSize)); err != nil {
				return err
			}
		case valuesSchemaFile:
			if _, err = io.Copy(r.valuesReader, io.LimitReader(tr, maxMetadataFileSize)); err != nil {
				return err
			}
		default:
			continue
		}

		// All known metadata files captured — skip remaining tar entries. The
		// legacy settings name never satisfies the exit: settings.yaml may still
		// come later in the tar and must win over it.
		if r.versionReader.Len() > 0 &&
			r.changelogReader.Len() > 0 &&
			(r.packageReader.Len() > 0 || r.moduleReader.Len() > 0) &&
			r.settingsReader.Len() > 0 &&
			r.valuesReader.Len() > 0 {
			return nil
		}
	}
}
