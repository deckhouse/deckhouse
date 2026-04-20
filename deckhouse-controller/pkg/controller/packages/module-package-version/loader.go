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

package modulepackageversion

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	registryv1 "github.com/google/go-containerregistry/pkg/v1"
	"gopkg.in/yaml.v2"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	moduletypes "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader/types"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	"github.com/deckhouse/deckhouse/pkg/log"
)

type versionFile struct {
	Version string `json:"version"`
}

type moduleMetadata struct {
	Version           string
	Changelog         *v1alpha1.PackageChangelog
	PackageDefinition *PackageDefinition
	ModuleDefinition  *moduletypes.Definition
}

var errImageIsNil = errors.New("image is nil")

type moduleMetadataReader struct {
	versionReader   *bytes.Buffer
	changelogReader *bytes.Buffer
	packageReader   *bytes.Buffer
	moduleReader    *bytes.Buffer
}

func (rr *moduleMetadataReader) untarMetadata(rc io.Reader) error {
	tr := tar.NewReader(rc)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		name := strings.ToLower(hdr.Name)

		switch name {
		case "version.json":
			if _, err = io.Copy(rr.versionReader, tr); err != nil {
				return err
			}
		case "changelog.yaml", "changelog.yml":
			if _, err = io.Copy(rr.changelogReader, tr); err != nil {
				return err
			}
		case "package.yaml", "package.yml":
			if _, err = io.Copy(rr.packageReader, tr); err != nil {
				return err
			}
		case "module.yaml":
			if _, err = io.Copy(rr.moduleReader, tr); err != nil {
				return err
			}
		}
	}
}

func (r *reconciler) fetchModuleMetadata(_ context.Context, img registryv1.Image) (*moduleMetadata, error) {
	if img == nil {
		return nil, errImageIsNil
	}

	meta := new(moduleMetadata)

	rc, err := cr.Extract(img)
	if err != nil {
		return nil, fmt.Errorf("extract image: %w", err)
	}
	defer rc.Close()

	rr := &moduleMetadataReader{
		versionReader:   bytes.NewBuffer(nil),
		changelogReader: bytes.NewBuffer(nil),
		packageReader:   bytes.NewBuffer(nil),
		moduleReader:    bytes.NewBuffer(nil),
	}

	if err = rr.untarMetadata(rc); err != nil {
		return nil, fmt.Errorf("untar metadata: %w", err)
	}

	if rr.versionReader.Len() > 0 {
		var version versionFile
		if err = json.NewDecoder(rr.versionReader).Decode(&version); err != nil {
			return nil, fmt.Errorf("decode version.json: %w", err)
		}
		meta.Version = version.Version
	}

	// Try package.yaml first (v2 format), fall back to module.yaml (legacy)
	if rr.packageReader.Len() > 0 {
		var def PackageDefinition
		if err = yaml.NewDecoder(rr.packageReader).Decode(&def); err != nil {
			return nil, fmt.Errorf("decode package.yaml: %w", err)
		}
		meta.PackageDefinition = &def
	} else if rr.moduleReader.Len() > 0 {
		var def moduletypes.Definition
		if err = yaml.NewDecoder(rr.moduleReader).Decode(&def); err != nil {
			return nil, fmt.Errorf("decode module.yaml: %w", err)
		}
		meta.ModuleDefinition = &def
	}

	if rr.changelogReader.Len() > 0 {
		var changelog v1alpha1.PackageChangelog
		if err = yaml.NewDecoder(rr.changelogReader).Decode(&changelog); err != nil {
			r.logger.Warn("unmarshal changelog yaml failed", log.Err(err))
		} else if len(changelog.Features) > 0 || len(changelog.Fixes) > 0 {
			meta.Changelog = &changelog
		}
	}

	return meta, nil
}
