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

package downloader

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/iancoleman/strcase"
	"gopkg.in/yaml.v3"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/models"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
)

const (
	defaultModuleWeight = 900
)

type ModuleDownloader struct {
	externalModulesDir string

	ms              *v1alpha1.ModuleSource
	registryOptions []cr.Option
}

func NewModuleDownloader(externalModulesDir string, ms *v1alpha1.ModuleSource, registryOptions []cr.Option) *ModuleDownloader {
	return &ModuleDownloader{
		externalModulesDir: externalModulesDir,
		ms:                 ms,
		registryOptions:    registryOptions,
	}
}

type ModuleDownloadResult struct {
	Checksum      string
	ModuleVersion string
	ModuleWeight  uint32

	ModuleDefinition *models.DeckhouseModuleDefinition
}

func (md *ModuleDownloader) DownloadByModuleVersion(moduleName, moduleVersion string) error {
	moduleVersionPath := path.Join(md.externalModulesDir, moduleName, moduleVersion)

	return md.fetchAndCopyModuleByVersion(moduleName, moduleVersion, moduleVersionPath)
}

func (md *ModuleDownloader) DownloadFromReleaseChannel(moduleName, releaseChannel, moduleChecksum string) (ModuleDownloadResult, error) {
	res := ModuleDownloadResult{}
	moduleVersion, checksum, err := md.fetchModuleVersionFromReleaseChannel(moduleName, releaseChannel, moduleChecksum)
	if err != nil {
		return res, err
	}

	res.Checksum = checksum
	res.ModuleVersion = moduleVersion

	// module was not updated
	if moduleVersion == "" {
		return res, nil
	}

	moduleVersionPath := path.Join(md.externalModulesDir, moduleName, moduleVersion)

	err = md.fetchAndCopyModuleByVersion(moduleName, moduleVersion, moduleVersionPath)
	if err != nil {
		return res, err
	}

	moduleDef := md.fetchModuleDefinition(moduleVersionPath)
	if moduleDef == nil {
		moduleDef = &models.DeckhouseModuleDefinition{
			Name:   moduleName,
			Weight: defaultModuleWeight,
			Path:   moduleVersionPath,
		}
	}

	if moduleDef.Weight == 0 {
		moduleDef.Weight = defaultModuleWeight
	}

	res.ModuleWeight = moduleDef.Weight
	res.ModuleDefinition = moduleDef

	return res, nil
}

func (md *ModuleDownloader) fetchAndCopyModuleByVersion(moduleName, moduleVersion, moduleVersionPath string) error {
	// TODO: if module exists on fs - skip this step

	regCli, err := cr.NewClient(path.Join(md.ms.Spec.Registry.Repo, moduleName), md.registryOptions...)
	if err != nil {
		return fmt.Errorf("fetch module error: %v", err)
	}

	img, err := regCli.Image(moduleVersion)
	if err != nil {
		return fmt.Errorf("fetch module version error: %v", err)
	}

	_ = os.RemoveAll(moduleVersionPath)

	err = md.copyModuleToFS(moduleVersionPath, img)
	if err != nil {
		return fmt.Errorf("copy module error: %v", err)
	}

	// inject registry to values
	err = injectRegistryToModuleValues(moduleVersionPath, md.ms)
	if err != nil {
		return fmt.Errorf("inject registry error: %v", err)
	}

	return nil
}

func (md *ModuleDownloader) copyModuleToFS(rootPath string, img v1.Image) error {
	rc := mutate.Extract(img)
	defer rc.Close()

	err := md.copyLayersToFS(rootPath, rc)
	if err != nil {
		return fmt.Errorf("copy tar to fs: %w", err)
	}

	return nil
}

func (md *ModuleDownloader) copyLayersToFS(rootPath string, rc io.ReadCloser) error {
	if err := os.MkdirAll(rootPath, 0700); err != nil {
		return fmt.Errorf("mkdir root path: %w", err)
	}

	tr := tar.NewReader(rc)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			// end of archive
			return nil
		}
		if err != nil {
			return fmt.Errorf("tar reader next: %w", err)
		}

		if strings.Contains(hdr.Name, "..") {
			// CWE-22 check, prevents path traversal
			return fmt.Errorf("path traversal detected in the module archive: malicious path %v", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path.Join(rootPath, hdr.Name), 0700); err != nil {
				return err
			}
		case tar.TypeReg:
			outFile, err := os.Create(path.Join(rootPath, hdr.Name))
			if err != nil {
				return fmt.Errorf("create file: %w", err)
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return fmt.Errorf("copy: %w", err)
			}
			outFile.Close()

			err = os.Chmod(outFile.Name(), os.FileMode(hdr.Mode)&0700) // remove only 'user' permission bit, E.x.: 644 => 600, 755 => 700
			if err != nil {
				return fmt.Errorf("chmod: %w", err)
			}
		case tar.TypeSymlink:
			link := path.Join(rootPath, hdr.Name)
			if isRel(hdr.Linkname, link) && isRel(hdr.Name, link) {
				if err := os.Symlink(hdr.Linkname, link); err != nil {
					return fmt.Errorf("create symlink: %w", err)
				}
			}

		case tar.TypeLink:
			err := os.Link(path.Join(rootPath, hdr.Linkname), path.Join(rootPath, hdr.Name))
			if err != nil {
				return fmt.Errorf("create hardlink: %w", err)
			}

		default:
			return errors.New("unknown tar type")
		}
	}
}

func (md *ModuleDownloader) fetchModuleVersionFromReleaseChannel(moduleName, releaseChannel, moduleChecksum string) ( /* moduleVersion */ string /*newChecksum*/, string, error) {
	regCli, err := cr.NewClient(path.Join(md.ms.Spec.Registry.Repo, moduleName, "release"), md.registryOptions...)
	if err != nil {
		return "", "", fmt.Errorf("fetch release image error: %v", err)
	}

	img, err := regCli.Image(strcase.ToKebab(releaseChannel))
	if err != nil {
		return "", "", fmt.Errorf("fetch image error: %v", err)
	}

	digest, err := img.Digest()
	if err != nil {
		return "", "", fmt.Errorf("fetch digest error: %v", err)
	}

	if moduleChecksum == digest.String() {
		return "", moduleChecksum, nil
	}

	moduleMetadata, err := md.fetchModuleReleaseMetadata(img)
	if err != nil {
		return "", digest.String(), fmt.Errorf("fetch release metadata error: %v", err)
	}

	return "v" + moduleMetadata.Version.String(), digest.String(), nil
}

func (md *ModuleDownloader) fetchModuleDefinition(moduleVersionPath string) *models.DeckhouseModuleDefinition {
	moduleDefFile := path.Join(moduleVersionPath, models.ModuleDefinitionFile)

	if _, err := os.Stat(moduleDefFile); err != nil {
		return nil
	}

	var def models.DeckhouseModuleDefinition

	f, err := os.Open(moduleDefFile)
	if err != nil {
		return nil
	}
	defer f.Close()

	err = yaml.NewDecoder(f).Decode(&def)
	if err != nil {
		return nil
	}

	return &def
}

func (md *ModuleDownloader) fetchModuleReleaseMetadata(img v1.Image) (moduleReleaseMetadata, error) {
	buf := bytes.NewBuffer(nil)
	var meta moduleReleaseMetadata

	layers, err := img.Layers()
	if err != nil {
		return meta, err
	}

	for _, layer := range layers {
		size, err := layer.Size()
		if err != nil {
			// dcr.logger.Warnf("couldn't calculate layer size")
			return meta, err
		}
		if size == 0 {
			// skip some empty werf layers
			continue
		}
		rc, err := layer.Uncompressed()
		if err != nil {
			return meta, err
		}

		err = untarMetadata(rc, buf)
		if err != nil {
			return meta, err
		}

		rc.Close()
	}

	err = json.Unmarshal(buf.Bytes(), &meta)

	return meta, err
}

func untarMetadata(rc io.ReadCloser, rw io.Writer) error {
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
		if strings.HasPrefix(hdr.Name, ".werf") {
			continue
		}

		switch hdr.Name {
		case "version.json":
			_, err = io.Copy(rw, tr)
			if err != nil {
				return err
			}
			return nil

		default:
			continue
		}
	}
}

func isRel(candidate, target string) bool {
	// GOOD: resolves all symbolic links before checking
	// that `candidate` does not escape from `target`
	if filepath.IsAbs(candidate) {
		return false
	}
	realpath, err := filepath.EvalSymlinks(filepath.Join(target, candidate))
	if err != nil {
		return false
	}
	relpath, err := filepath.Rel(target, realpath)
	return err == nil && !strings.HasPrefix(filepath.Clean(relpath), "..")
}

type moduleReleaseMetadata struct {
	Version *semver.Version `json:"version"`
}

// Inject registry to module values

func injectRegistryToModuleValues(moduleVersionPath string, moduleSource *v1alpha1.ModuleSource) error {
	valuesFile := path.Join(moduleVersionPath, "openapi", "values.yaml")

	valuesData, err := os.ReadFile(valuesFile)
	if err != nil {
		return err
	}

	valuesData, err = mutateOpenapiSchema(valuesData, moduleSource)
	if err != nil {
		return err
	}

	return os.WriteFile(valuesFile, valuesData, 0666)
}

func mutateOpenapiSchema(sourceValuesData []byte, moduleSource *v1alpha1.ModuleSource) ([]byte, error) {
	reg := new(registrySchemaForValues)
	reg.SetBase(moduleSource.Spec.Registry.Repo)
	reg.SetDockercfg(moduleSource.Spec.Registry.DockerCFG)

	var yamlData injectedValues

	err := yaml.Unmarshal(sourceValuesData, &yamlData)
	if err != nil {
		return nil, err
	}

	yamlData.Properties.Registry = reg

	buf := bytes.NewBuffer(nil)

	yamlEncoder := yaml.NewEncoder(buf)
	yamlEncoder.SetIndent(2)
	err = yamlEncoder.Encode(yamlData)

	return buf.Bytes(), err
}

// part of openapi schema for injecting registry values
type registrySchemaForValues struct {
	Type       string   `yaml:"type"`
	Default    struct{} `yaml:"default"`
	Properties struct {
		Base struct {
			Type    string `yaml:"type"`
			Default string `yaml:"default"`
		} `yaml:"base"`
		Dockercfg struct {
			Type    string `yaml:"type"`
			Default string `yaml:"default,omitempty"`
		} `yaml:"dockercfg"`
	} `yaml:"properties"`
}

func (rsv *registrySchemaForValues) fillTypes() {
	rsv.Properties.Base.Type = "string"
	rsv.Properties.Dockercfg.Type = "string"
	rsv.Type = "object"
}

func (rsv *registrySchemaForValues) SetBase(registryBase string) {
	rsv.fillTypes()

	rsv.Properties.Base.Default = registryBase
}

func (rsv *registrySchemaForValues) SetDockercfg(dockercfg string) {
	if len(dockercfg) == 0 {
		return
	}

	rsv.Properties.Dockercfg.Default = dockercfg
}

type injectedValues struct {
	Type       string                 `json:"type" yaml:"type"`
	Xextend    map[string]interface{} `json:"x-extend" yaml:"x-extend"`
	Properties struct {
		YYY      map[string]interface{}   `json:",inline" yaml:",inline"`
		Registry *registrySchemaForValues `json:"registry" yaml:"registry"`
	} `json:"properties" yaml:"properties"`
	XXX map[string]interface{} `json:",inline" yaml:",inline"`
}
