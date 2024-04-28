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
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/shell-operator/pkg/utils/measure"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/iancoleman/strcase"
	"gopkg.in/yaml.v3"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/models"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	"github.com/deckhouse/deckhouse/go_lib/module"
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
	Changelog        map[string]any
}

// DownloadDevImageTag downloads image tag and store it in the .../<moduleName>/dev fs path
// if checksum is equal to a module image digest - do nothing
// otherwise return new digest
func (md *ModuleDownloader) DownloadDevImageTag(moduleName, imageTag, checksum string) (string, *models.DeckhouseModuleDefinition, error) {
	moduleStorePath := path.Join(md.externalModulesDir, moduleName, "dev")

	img, err := md.fetchImage(moduleName, imageTag)
	if err != nil {
		return "", nil, err
	}

	digest, err := img.Digest()
	if err != nil {
		return "", nil, err
	}

	if digest.String() == checksum {
		// module is up-to-date
		return "", nil, nil
	}

	_, err = md.fetchAndCopyModuleByVersion(moduleName, imageTag, moduleStorePath)
	if err != nil {
		return "", nil, err
	}

	def := md.fetchModuleDefinitionFromFS(moduleName, moduleStorePath)

	return digest.String(), def, nil
}

func (md *ModuleDownloader) DownloadByModuleVersion(moduleName, moduleVersion string) (*DownloadStatistic, error) {
	if !strings.HasPrefix(moduleVersion, "v") {
		moduleVersion = "v" + moduleVersion
	}

	moduleVersionPath := path.Join(md.externalModulesDir, moduleName, moduleVersion)

	return md.fetchAndCopyModuleByVersion(moduleName, moduleVersion, moduleVersionPath)
}

// DownloadMetadataFromReleaseChannel downloads only module release image with metadata: version.json, checksum.json(soon)
// does not fetch and install the desired version on the module, only fetches its module definition
func (md *ModuleDownloader) DownloadMetadataFromReleaseChannel(moduleName, releaseChannel, moduleChecksum string) (ModuleDownloadResult, error) {
	res := ModuleDownloadResult{}

	moduleVersion, checksum, changelog, err := md.fetchModuleReleaseMetadataFromReleaseChannel(moduleName, releaseChannel, moduleChecksum)
	if err != nil {
		return res, err
	}

	res.Checksum = checksum
	res.ModuleVersion = moduleVersion
	res.Changelog = changelog

	// module was not updated
	if moduleVersion == "" {
		return res, nil
	}

	img, err := md.fetchImage(moduleName, moduleVersion)
	if err != nil {
		return res, err
	}

	def, err := md.fetchModuleDefinitionFromImage(moduleName, img)
	if err != nil {
		return res, err
	}

	res.ModuleWeight = def.Weight
	res.ModuleDefinition = def

	return res, nil
}

// DownloadModuleDefinitionByVersion returns a module definition from the repo by the module's name and version(tag)
func (md *ModuleDownloader) DownloadModuleDefinitionByVersion(moduleName, moduleVersion string) (*models.DeckhouseModuleDefinition, error) {
	img, err := md.fetchImage(moduleName, moduleVersion)
	if err != nil {
		return nil, err
	}

	return md.fetchModuleDefinitionFromImage(moduleName, img)
}

func (md *ModuleDownloader) GetDocumentationArchive(moduleName, moduleVersion string) (io.ReadCloser, error) {
	if !strings.HasPrefix(moduleVersion, "v") {
		moduleVersion = "v" + moduleVersion
	}

	img, err := md.fetchImage(moduleName, moduleVersion)
	if err != nil {
		return nil, fmt.Errorf("fetch image: %w", err)
	}

	return module.ExtractDocs(img), nil
}

func (md *ModuleDownloader) fetchImage(moduleName, imageTag string) (v1.Image, error) {
	regCli, err := cr.NewClient(path.Join(md.ms.Spec.Registry.Repo, moduleName), md.registryOptions...)
	if err != nil {
		return nil, fmt.Errorf("fetch module error: %v", err)
	}

	return regCli.Image(imageTag)
}

func (md *ModuleDownloader) storeModule(moduleStorePath string, img v1.Image) (*DownloadStatistic, error) {
	_ = os.RemoveAll(moduleStorePath)

	ds, err := md.copyModuleToFS(moduleStorePath, img)
	if err != nil {
		return nil, fmt.Errorf("copy module error: %v", err)
	}

	// inject registry to values
	err = InjectRegistryToModuleValues(moduleStorePath, md.ms)
	if err != nil {
		return nil, fmt.Errorf("inject registry error: %v", err)
	}

	return ds, nil
}

func (md *ModuleDownloader) fetchAndCopyModuleByVersion(moduleName, moduleVersion, moduleVersionPath string) (*DownloadStatistic, error) {
	// TODO: if module exists on fs - skip this step

	img, err := md.fetchImage(moduleName, moduleVersion)
	if err != nil {
		return nil, err
	}

	return md.storeModule(moduleVersionPath, img)
}

func (md *ModuleDownloader) copyModuleToFS(rootPath string, img v1.Image) (*DownloadStatistic, error) {
	rc := mutate.Extract(img)
	defer rc.Close()

	ds, err := md.copyLayersToFS(rootPath, rc)
	if err != nil {
		return nil, fmt.Errorf("copy tar to fs: %w", err)
	}

	return ds, nil
}

func (md *ModuleDownloader) copyLayersToFS(rootPath string, rc io.ReadCloser) (*DownloadStatistic, error) {
	ds := new(DownloadStatistic)
	defer measure.Duration(func(d time.Duration) {
		ds.PullDuration = d
	})()

	if err := os.MkdirAll(rootPath, 0o700); err != nil {
		return nil, fmt.Errorf("mkdir root path: %w", err)
	}

	tr := tar.NewReader(rc)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			// end of archive
			return ds, nil
		}

		ds.Size += uint32(hdr.Size)

		if err != nil {
			return nil, fmt.Errorf("tar reader next: %w", err)
		}

		if strings.Contains(hdr.Name, "..") {
			// CWE-22 check, prevents path traversal
			return nil, fmt.Errorf("path traversal detected in the module archive: malicious path %v", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path.Join(rootPath, hdr.Name), 0o700); err != nil {
				return nil, err
			}
		case tar.TypeReg:
			outFile, err := os.Create(path.Join(rootPath, hdr.Name))
			if err != nil {
				return nil, fmt.Errorf("create file: %w", err)
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return nil, fmt.Errorf("copy: %w", err)
			}
			outFile.Close()

			err = os.Chmod(outFile.Name(), os.FileMode(hdr.Mode)&0o700) // remove only 'user' permission bit, E.x.: 644 => 600, 755 => 700
			if err != nil {
				return nil, fmt.Errorf("chmod: %w", err)
			}
		case tar.TypeSymlink:
			link := path.Join(rootPath, hdr.Name)
			if isRel(hdr.Linkname, link) && isRel(hdr.Name, link) {
				if err := os.Symlink(hdr.Linkname, link); err != nil {
					return nil, fmt.Errorf("create symlink: %w", err)
				}
			}

		case tar.TypeLink:
			err := os.Link(path.Join(rootPath, hdr.Linkname), path.Join(rootPath, hdr.Name))
			if err != nil {
				return nil, fmt.Errorf("create hardlink: %w", err)
			}

		default:
			return nil, errors.New("unknown tar type")
		}
	}
}

func (md *ModuleDownloader) fetchModuleReleaseMetadataFromReleaseChannel(moduleName, releaseChannel, moduleChecksum string) (
	/* moduleVersion */ string /*newChecksum*/, string /*changelog*/, map[string]any, error) {
	regCli, err := cr.NewClient(path.Join(md.ms.Spec.Registry.Repo, moduleName, "release"), md.registryOptions...)
	if err != nil {
		return "", "", nil, fmt.Errorf("fetch release image error: %v", err)
	}

	img, err := regCli.Image(strcase.ToKebab(releaseChannel))
	if err != nil {
		return "", "", nil, fmt.Errorf("fetch image error: %v", err)
	}

	digest, err := img.Digest()
	if err != nil {
		return "", "", nil, fmt.Errorf("fetch digest error: %v", err)
	}

	if moduleChecksum == digest.String() {
		return "", moduleChecksum, nil, nil
	}

	moduleMetadata, err := md.fetchModuleReleaseMetadata(img)
	if err != nil {
		return "", digest.String(), nil, fmt.Errorf("fetch release metadata error: %v", err)
	}

	return "v" + moduleMetadata.Version.String(), digest.String(), moduleMetadata.Changelog, nil
}

func (md *ModuleDownloader) fetchModuleDefinitionFromFS(moduleName, moduleVersionPath string) *models.DeckhouseModuleDefinition {
	def := &models.DeckhouseModuleDefinition{
		Name:   moduleName,
		Weight: defaultModuleWeight,
		Path:   moduleVersionPath,
	}

	moduleDefFile := path.Join(moduleVersionPath, models.ModuleDefinitionFile)

	if _, err := os.Stat(moduleDefFile); err != nil {
		return def
	}

	f, err := os.Open(moduleDefFile)
	if err != nil {
		return def
	}
	defer f.Close()

	err = yaml.NewDecoder(f).Decode(&def)
	if err != nil {
		return def
	}

	return def
}

func (md *ModuleDownloader) fetchModuleDefinitionFromImage(moduleName string, img v1.Image) (*models.DeckhouseModuleDefinition, error) {
	def := &models.DeckhouseModuleDefinition{
		Name:   moduleName,
		Weight: defaultModuleWeight,
	}

	rc := mutate.Extract(img)
	defer rc.Close()

	buf := bytes.NewBuffer(nil)

	err := untarModuleDefinition(rc, buf)
	if err != nil {
		return def, err
	}

	if buf.Len() == 0 {
		return def, nil
	}

	err = yaml.NewDecoder(buf).Decode(&def)
	if err != nil {
		return def, err
	}

	return def, nil
}

func (md *ModuleDownloader) fetchModuleReleaseMetadata(img v1.Image) (moduleReleaseMetadata, error) {
	var meta moduleReleaseMetadata

	rc := mutate.Extract(img)
	defer rc.Close()

	rr := &releaseReader{
		versionReader:   bytes.NewBuffer(nil),
		changelogReader: bytes.NewBuffer(nil),
	}

	err := rr.untarMetadata(rc)
	if err != nil {
		return meta, err
	}

	if rr.versionReader.Len() > 0 {
		err = json.NewDecoder(rr.versionReader).Decode(&meta)
		if err != nil {
			return meta, err
		}
	}

	if rr.changelogReader.Len() > 0 {
		var changelog map[string]any
		err = yaml.NewDecoder(rr.changelogReader).Decode(&changelog)
		if err != nil {
			meta.Changelog = make(map[string]any)
			return meta, nil
		}
		meta.Changelog = changelog
	}

	return meta, err
}

func untarModuleDefinition(rc io.ReadCloser, rw io.Writer) error {
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
		case "module.yaml":
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

	Changelog map[string]any
}

// Inject registry to module values

func InjectRegistryToModuleValues(moduleVersionPath string, moduleSource *v1alpha1.ModuleSource) error {
	valuesFile := path.Join(moduleVersionPath, "openapi", "values.yaml")

	valuesData, err := os.ReadFile(valuesFile)
	if err != nil {
		return err
	}

	valuesData, err = mutateOpenapiSchema(valuesData, moduleSource)
	if err != nil {
		return err
	}

	return os.WriteFile(valuesFile, valuesData, 0o666)
}

func mutateOpenapiSchema(sourceValuesData []byte, moduleSource *v1alpha1.ModuleSource) ([]byte, error) {
	reg := new(registrySchemaForValues)
	reg.SetBase(moduleSource.Spec.Registry.Repo)
	reg.SetDockercfg(moduleSource.Spec.Registry.DockerCFG)
	reg.SetScheme(moduleSource.Spec.Registry.Scheme)
	reg.SetCA(moduleSource.Spec.Registry.CA)

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
		Scheme struct {
			Type    string `yaml:"type"`
			Default string `yaml:"default"`
		} `yaml:"scheme"`
		CA struct {
			Type    string `yaml:"type"`
			Default string `yaml:"default,omitempty"`
		} `yaml:"ca"`
	} `yaml:"properties"`
}

func (rsv *registrySchemaForValues) fillTypes() {
	rsv.Properties.Base.Type = "string"
	rsv.Properties.Dockercfg.Type = "string"
	rsv.Properties.Scheme.Type = "string"
	rsv.Properties.CA.Type = "string"
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

func (rsv *registrySchemaForValues) SetScheme(scheme string) {
	rsv.Properties.Scheme.Default = scheme
}

func (rsv *registrySchemaForValues) SetCA(ca string) {
	rsv.Properties.CA.Default = ca
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
