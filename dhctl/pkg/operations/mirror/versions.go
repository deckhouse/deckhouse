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
	"encoding/json"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
)

func VersionsToCopy(mirrorCtx *Context) ([]*semver.Version, error) {
	rockSolidVersion, err := getRockSolidVersionFromRegistry(mirrorCtx)
	if err != nil {
		return nil, fmt.Errorf("get rock-solid release version from registry: %w", err)
	}
	mirrorFromVersion := *rockSolidVersion
	if mirrorCtx.MinVersion != nil {
		mirrorFromVersion = *mirrorCtx.MinVersion
		if rockSolidVersion.LessThan(mirrorCtx.MinVersion) {
			mirrorFromVersion = *rockSolidVersion
		}
	}

	tags, err := getTagsFromRegistry(mirrorCtx)
	if err != nil {
		return nil, fmt.Errorf("get releases from github: %w", err)
	}

	versionsAboveMinimal := parseAndFilterVersionsAboveMinimal(&mirrorFromVersion, tags)
	versionsAboveMinimal = filterOnlyLatestPatches(versionsAboveMinimal)

	log.InfoF("Deckhouse releases to pull: %+v\n", versionsAboveMinimal)

	return versionsAboveMinimal, nil
}

func getTagsFromRegistry(mirrorCtx *Context) ([]string, error) {
	nameOpts := []name.Option{}
	remoteOpts := []remote.Option{}
	if mirrorCtx.Insecure {
		nameOpts = append(nameOpts, name.Insecure)
	}
	if mirrorCtx.RegistryAuth != nil {
		remoteOpts = append(remoteOpts, remote.WithAuth(mirrorCtx.RegistryAuth))
	}

	repo, err := name.NewRepository(mirrorCtx.RegistryRepo+"/release-channel", nameOpts...)
	if err != nil {
		return nil, fmt.Errorf("parsing repo: %v", err)
	}
	tags, err := remote.List(repo, remoteOpts...)
	if err != nil {
		return nil, fmt.Errorf("get tags from Deckhouse registry: %w", err)
	}

	return tags, nil
}

func parseAndFilterVersionsAboveMinimal(minVersion *semver.Version, tags []string) []*semver.Version {
	versionsAboveMinimal := make([]*semver.Version, 0)
	for _, tag := range tags {
		version, err := semver.NewVersion(tag)
		if err != nil || minVersion.GreaterThan(version) {
			continue
		}
		versionsAboveMinimal = append(versionsAboveMinimal, version)
	}
	return versionsAboveMinimal
}

func filterOnlyLatestPatches(versions []*semver.Version) []*semver.Version {
	type majorMinor [2]uint64
	patches := map[majorMinor]uint64{}
	for _, version := range versions {
		release := majorMinor{version.Major(), version.Minor()}
		if patch := patches[release]; patch <= version.Patch() {
			patches[release] = version.Patch()
		}
	}

	topPatches := make([]*semver.Version, 0, len(patches))
	for majMin, patch := range patches {
		topPatches = append(topPatches, semver.MustParse(fmt.Sprintf("v%d.%d.%d", majMin[0], majMin[1], patch)))
	}
	return topPatches
}

func getRockSolidVersionFromRegistry(mirrorCtx *Context) (*semver.Version, error) {
	refOpts := []name.Option{name.StrictValidation}
	if mirrorCtx.Insecure {
		refOpts = append(refOpts, name.Insecure)
	}

	ref, err := name.ParseReference(mirrorCtx.RegistryRepo+"/release-channel:rock-solid", refOpts...)
	if err != nil {
		return nil, fmt.Errorf("parse rock solid release ref: %w", err)
	}

	rockSolidReleaseImage, err := remote.Image(ref, remote.WithAuth(mirrorCtx.RegistryAuth))
	if err != nil {
		return nil, fmt.Errorf("get rock-solid release channel data: %w", err)
	}

	versionJSON, err := readFileFromImage(rockSolidReleaseImage, "version.json")
	if err != nil {
		return nil, fmt.Errorf("cannot get rock-solid release channel version: %w", err)
	}

	tmp := &struct {
		Version string `json:"version"`
	}{}
	if err = json.Unmarshal(versionJSON.Bytes(), tmp); err != nil {
		return nil, fmt.Errorf("cannot find release channel version: %w", err)
	}

	ver, err := semver.NewVersion(tmp.Version)
	if err != nil {
		return nil, fmt.Errorf("cannot find release channel version: %w", err)
	}
	return ver, nil

}
