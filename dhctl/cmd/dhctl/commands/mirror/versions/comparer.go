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

package versions

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/containers/image/v5/signature"
	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands/mirror/image"
	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands/mirror/util"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

const (
	VersionRE = `(v[0-9]+\.[0-9]+)\.[0-9]+`
)

var (
	versionsRegexp  = regexp.MustCompile(`^` + VersionRE + `$`)
	releaseChannels = []string{"alpha", "beta", "early-access", "stable", "rock-solid"}
)

type VersionsComparer struct {
	source *image.RegistryConfig
	dest   *image.RegistryConfig

	policyContext *signature.PolicyContext

	destListOpts   []image.ListOption
	sourceListOpts []image.ListOption

	sourceCopyOpts []image.CopyOption

	logger log.Logger
}

func NewVersionsComparer(source, dest *image.RegistryConfig, destListOpts, sourceListOpts []image.ListOption, sourceCopyOpts []image.CopyOption, policyContext *signature.PolicyContext, logger log.Logger) *VersionsComparer {
	sourceCopyOpts = append(sourceCopyOpts, image.WithOutput(logger))
	return &VersionsComparer{
		source:         source,
		dest:           dest,
		sourceListOpts: sourceListOpts,
		destListOpts:   destListOpts,
		sourceCopyOpts: sourceCopyOpts,
		policyContext:  policyContext,
		logger:         logger,
	}
}

func (v *VersionsComparer) ImagesToCopy(ctx context.Context, minVersion string) ([]*image.ImageConfig, error) {
	diffLogger := v.logger.ProcessLogger()
	diffLogger.LogProcessStart("Calculating versions diff")
	diff, err := v.calculateDiff(ctx, minVersion)
	if err != nil {
		diffLogger.LogProcessFail()
		return nil, err
	}
	diffLogger.LogProcessEnd()

	modulesImgsLogger := v.logger.ProcessLogger()
	modulesImgsLogger.LogProcessStart("Retrieving modules images")
	modulesImages, err := v.modulesImages(ctx, diff)
	if err != nil {
		modulesImgsLogger.LogProcessFail()
		return nil, err
	}
	modulesImgsLogger.LogProcessEnd()

	images := make([]*image.ImageConfig, 0, 1+len(modulesImages)+len(releaseChannels)*3+len(diff)*2)

	images = append(images, image.NewImageConfig(v.source, "2", "", "security", "trivy-db"))
	images = append(images, modulesImages...)

	for _, release := range releaseChannels {
		images = append(
			images,
			image.NewImageConfig(v.source, release, ""),
			image.NewImageConfig(v.source, release, "", "install"),
			image.NewImageConfig(v.source, release, "", "release-channel"),
		)
	}

	// This allows us to be sure that all versions are copied
	// and if something goes wrong - deckhouse image versions would be pushed last.
	for _, p := range []string{"install", ""} {
		for _, tag := range diff {
			images = append(images, image.NewImageConfig(v.source, versionToTag(tag), "", p))
		}
	}

	return images, nil
}

func (v *VersionsComparer) modulesImages(ctx context.Context, diff []semver.Version) ([]*image.ImageConfig, error) {
	modulesImagesFile := make(map[semver.Version]map[string]string)
	for _, tag := range diff {
		versionImages, err := v.modulesImagesForVersion(ctx, tag)
		if err != nil {
			return nil, err
		}
		modulesImagesFile[tag] = versionImages
	}

	type imageSpec struct {
		imageName string
		d8Version semver.Version
	}
	uniqueImages := make(map[string]imageSpec)
	for version, versionImages := range modulesImagesFile {
		for imageName, identifier := range versionImages {
			if old, ok := uniqueImages[identifier]; ok && version.GreaterThan(&old.d8Version) {
				continue
			}
			uniqueImages[identifier] = imageSpec{
				imageName: imageName,
				d8Version: version,
			}
		}
	}

	modulesImages := make([]*image.ImageConfig, 0, len(uniqueImages))
	for identifier, imgSpec := range uniqueImages {
		tag, digest := identifier, ""
		if strings.HasPrefix(identifier, "sha256:") {
			tag, digest = versionToTag(imgSpec.d8Version)+"-"+imgSpec.imageName, identifier
		}
		modulesImages = append(modulesImages, image.NewImageConfig(v.source, tag, digest))
	}

	sort.Slice(modulesImages, func(i, j int) bool {
		return strings.Compare(modulesImages[i].Tag(), modulesImages[j].Tag()) < 0
	})

	return modulesImages, nil
}

func (v *VersionsComparer) modulesImagesForVersion(ctx context.Context, deckhouseVersion semver.Version) (map[string]string, error) {
	img := image.NewImageConfig(v.source, versionToTag(deckhouseVersion), "")
	contents, err := fileFromImage(ctx, img, "deckhouse/modules/images_digests.json", v.policyContext, v.sourceCopyOpts...)
	if err != nil {
		contents, err = fileFromImage(ctx, img, "deckhouse/modules/images_tags.json", v.policyContext, v.sourceCopyOpts...)
		if err != nil {
			return nil, err
		}
	}

	var modulesImages map[string]map[string]string
	if err := json.Unmarshal(contents, &modulesImages); err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for module, images := range modulesImages {
		for image, identifier := range images {
			result[module+"-"+image] = identifier
		}
	}
	return result, nil
}

func (v *VersionsComparer) calculateDiff(ctx context.Context, minVersion string) ([]semver.Version, error) {
	sourceVersions, err := v.sourceVersions(ctx)
	if err != nil {
		return nil, err
	}

	destVersions, err := v.destVersions(ctx, sourceVersions, minVersion)
	if err != nil {
		return nil, err
	}

	deckhouseVersions, err := compareVersions(sourceVersions, destVersions, minVersion)
	if err != nil {
		return nil, err
	}

	releaseMetaVersions, err := v.releaseMetadataVersions(ctx, destVersions)
	if err != nil {
		return nil, err
	}

	result := make([]semver.Version, 0, len(deckhouseVersions)+len(releaseMetaVersions))
	for _, v := range deckhouseVersions {
		if f := releaseMetaVersions[v]; !f {
			result = append(result, v)
		}
	}
	for v := range releaseMetaVersions {
		result = append(result, v)
	}

	sort.Slice(result, func(i, j int) bool {
		lv, rv := result[i], result[j]
		return lv.LessThan(&rv)
	})

	return result, nil
}

func (v *VersionsComparer) sourceVersions(ctx context.Context) (latestVersions, error) {
	sourceVersions, err := findDeckhouseVersions(ctx, v.source, v.sourceListOpts...)
	if err != nil {
		return nil, err
	}
	if len(sourceVersions) < 1 {
		return nil, fmt.Errorf("no deckhouse versions from source found")
	}
	return sourceVersions, nil
}

func (v *VersionsComparer) destVersions(ctx context.Context, sourceVersions latestVersions, minVersion string) (latestVersions, error) {
	if v.dest.Transport() == image.DockerTransport {
		return findDeckhouseVersions(ctx, v.dest, v.destListOpts...)
	}
	return make(latestVersions), nil
}

func (v *VersionsComparer) releaseMetadataVersions(ctx context.Context, destVersions latestVersions) (map[semver.Version]bool, error) {
	releaseMetaVersions := make(map[semver.Version]bool, len(releaseChannels))
	for _, release := range releaseChannels {
		releaseVersion, err := v.fetchReleaseMetadataDeckhouseVersion(ctx, release)
		if err != nil {
			return nil, err
		}

		dv, err := destVersions.Get(*releaseVersion)
		if err != nil && !errors.Is(err, ErrNoVersion) {
			return nil, err
		}

		if (errors.Is(err, ErrNoVersion) || !dv.Equal(releaseVersion)) && !releaseMetaVersions[*releaseVersion] {
			releaseMetaVersions[*releaseVersion] = true
		}
	}
	return releaseMetaVersions, nil
}

// fetchReleaseMetadataDeckhouseVersion copies image to local directory and untar it's layers to find version.json and returns "version" key found in it
func (v *VersionsComparer) fetchReleaseMetadataDeckhouseVersion(ctx context.Context, release string) (*semver.Version, error) {
	img := image.NewImageConfig(v.source, release, "", "release-channel")
	contents, err := fileFromImage(ctx, img, "version.json", v.policyContext, v.sourceCopyOpts...)
	if err != nil {
		return nil, err
	}

	var meta struct {
		Version string `json:"version"`
	}

	if err := json.Unmarshal(contents, &meta); err != nil {
		return nil, err
	}
	return parse(meta.Version)
}

func compareVersions(sourceVersions, destVersions latestVersions, minVersion string) (latestVersions, error) {
	var destOldestWithPatch *semver.Version
	switch len(destVersions) {
	case 0:
		var err error
		destOldestWithPatch, err = deckhouseMinVersion(sourceVersions, minVersion)
		if err != nil {
			return nil, fmt.Errorf("min version: %w", err)
		}
	default:
		destOldestWithPatch = destVersions.Oldest()
	}

	sourceLatestWithPatch := sourceVersions.Latest()
	resultVersions := make(latestVersions)
	for version := *parseFromInt(destOldestWithPatch.Major(), destOldestWithPatch.Minor(), 0); !version.GreaterThan(sourceLatestWithPatch); version = version.IncMinor() {
		sourceVersion, err := sourceVersions.Get(version)
		if err != nil {
			return nil, fmt.Errorf("version %s from source: %w", version, err)
		}

		destVersion, err := destVersions.Get(version)
		switch {
		case (err == nil && !destVersion.Equal(sourceVersion)) || errors.Is(err, ErrNoVersion):
			if _, err := resultVersions.Set(*sourceVersion); err != nil {
				return nil, err
			}
		case err != nil:
			return nil, fmt.Errorf("version %s from destination: %w", version, err)
		}
	}
	return resultVersions, nil
}

func findDeckhouseVersions(ctx context.Context, registry *image.RegistryConfig, opts ...image.ListOption) (latestVersions, error) {
	tags, err := registry.ListTags(ctx, opts...)
	if err != nil {
		return nil, err
	}

	versions := make(latestVersions)
	for _, tag := range tags {
		if !versionsRegexp.MatchString(tag) {
			continue
		}

		if _, err := versions.SetString(tag); err != nil {
			return nil, err
		}
	}
	return versions, nil
}

func deckhouseMinVersion(sourceVersions latestVersions, minVersion string) (*semver.Version, error) {
	latestWithPatch := sourceVersions.Latest()
	switch minVersion {
	case "":
		version := parseFromInt(latestWithPatch.Major(), latestWithPatch.Minor()-5, 0)
		return sourceVersions.Get(*version)
	case "latest":
		return latestWithPatch, nil
	}
	return sourceVersions.GetString(minVersion)
}

func fileFromImage(ctx context.Context, img *image.ImageConfig, filename string, policyContext *signature.PolicyContext, opts ...image.CopyOption) ([]byte, error) {
	dir, err := os.MkdirTemp("/tmp", "deckhouse_*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	destRegistry, err := image.NewRegistry("dir:"+dir, nil)
	if err != nil {
		return nil, err
	}

	dest := img.WithNewRegistry(destRegistry)
	if err := image.CopyImage(ctx, img, dest, policyContext, opts...); err != nil {
		return nil, err
	}

	imageDir := filepath.Join(dest.RegistryPath(), img.Tag())
	files, err := os.ReadDir(imageDir)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		contents, err := fileFromTarGz(filepath.Join(imageDir, file.Name()), filename)
		if errors.Is(err, io.EOF) || errors.Is(err, tar.ErrHeader) || errors.Is(err, gzip.ErrHeader) {
			continue
		}
		if err != nil {
			return nil, err
		}
		return contents, nil
	}
	return nil, fmt.Errorf(`"%s" file not found in image from "%s" dir`, filename, dir)
}

// fileFromTarGz finds finds "targetFile" in "archive" tar.gz file
func fileFromTarGz(archive, targetFile string) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	err := util.NewTarGzReader(archive, func(h *tar.Header, r *tar.Reader) (bool, error) {
		if h.Name != targetFile {
			return false, nil
		}
		_, err := io.Copy(buf, r)
		return true, err
	})
	return buf.Bytes(), err
}

func versionToTag(v semver.Version) string {
	return "v" + v.String()
}
