// Copyright 2022 Flant JSC
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

package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/ettle/strcase"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	regTransport "github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dhRelease "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/deckhouse-release"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
)

type DeckhouseService struct {
	dc dependency.Container

	registry        string
	registryOptions []cr.Option
}

func NewDeckhouseService(registryAddress string, registryConfig *utils.RegistryConfig) *DeckhouseService {
	return &DeckhouseService{
		dc:              dependency.NewDependencyContainer(),
		registry:        registryAddress,
		registryOptions: utils.GenerateRegistryOptions(registryConfig),
	}
}

func (svc *DeckhouseService) GetDeckhouseRelease(ctx context.Context, releaseChannel string) (*dhRelease.ReleaseMetadata, error) {
	regCli, err := svc.dc.GetRegistryClient(path.Join(svc.registry, "release-channel"), svc.registryOptions...)
	if err != nil {
		return nil, fmt.Errorf("get registry client: %w", err)
	}

	img, err := regCli.Image(ctx, strcase.ToKebab(releaseChannel))
	if err != nil {
		if strings.Contains(err.Error(), string(regTransport.ManifestUnknownErrorCode)) {
			err = errors.Join(err, ErrChannelIsNotFound)
		}

		return nil, fmt.Errorf("fetch image: %w", err)
	}

	releaseMetadata, err := svc.fetchReleaseMetadata(img)
	if err != nil {
		return nil, fmt.Errorf("fetch release metadata: %w", err)
	}

	if releaseMetadata.Version == "" {
		return nil, fmt.Errorf("release metadata malformed: no version found")
	}

	return releaseMetadata, nil
}

func (svc *DeckhouseService) ListDeckhouseReleases(ctx context.Context) ([]string, error) {
	regCli, err := svc.dc.GetRegistryClient(svc.registry, svc.registryOptions...)
	if err != nil {
		return nil, fmt.Errorf("get registry client: %w", err)
	}

	ls, err := regCli.ListTags(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}

	return ls, nil
}

func (svc *DeckhouseService) fetchReleaseMetadata(img v1.Image) (*dhRelease.ReleaseMetadata, error) {
	var meta = new(dhRelease.ReleaseMetadata)

	rc := mutate.Extract(img)
	defer rc.Close()

	rr := &releaseReader{
		versionReader:   bytes.NewBuffer(nil),
		changelogReader: bytes.NewBuffer(nil),
	}

	err := rr.untarDeckhouseLayer(rc)
	if err != nil {
		return nil, err
	}

	if rr.versionReader.Len() > 0 {
		err = json.NewDecoder(rr.versionReader).Decode(&meta)
		if err != nil {
			return nil, err
		}
	}

	if rr.changelogReader.Len() > 0 {
		var changelog map[string]any

		err = yaml.NewDecoder(rr.changelogReader).Decode(&changelog)
		if err != nil {
			// if changelog build failed - warn about it but don't fail the release
			fmt.Printf("Unmarshal CHANGELOG yaml failed: %s\n", err)

			meta.Changelog = make(map[string]any)

			return meta, nil
		}

		meta.Changelog = changelog
	}

	cooldown := svc.fetchCooldown(img)
	if cooldown != nil {
		meta.Cooldown = cooldown
	}

	return meta, nil
}

func (svc *DeckhouseService) fetchCooldown(image v1.Image) *metav1.Time {
	cfg, err := image.ConfigFile()
	if err != nil {
		fmt.Printf("image config error: %s\n", err)

		return nil
	}

	if cfg == nil {
		return nil
	}

	if len(cfg.Config.Labels) == 0 {
		return nil
	}

	if v, ok := cfg.Config.Labels["cooldown"]; ok {
		t, err := parseTime(v)
		if err != nil {
			fmt.Printf("parse cooldown(%s) error: %s\n", v, err)
			return nil
		}

		mt := metav1.NewTime(t)

		return &mt
	}

	return nil
}

func parseTime(s string) (time.Time, error) {
	t, err := time.Parse("2006-01-02 15:04", s)
	if err == nil {
		return t, nil
	}

	t, err = time.Parse("2006-01-02 15:04:05", s)
	if err == nil {
		return t, nil
	}

	return time.Parse(time.RFC3339, s)
}
