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

package service

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/deckhouse/deckhouse/go_lib/libapi"
	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/deckhouse/deckhouse/pkg/registry"
	"github.com/goccy/go-yaml"
)

const (
	deckhouseReleaseChannelsSegment = "release-channel"

	deckhouseServiceName                = "deckhouse"
	deckhouseReleaseChannelsServiceName = "deckhouse_release_channel"
)

// DeckhouseService provides high-level operations for Deckhouse platform management
type DeckhouseService struct {
	client registry.Client

	*BasicService
	deckhouseReleaseChannels *DeckhouseReleaseService

	logger *log.Logger
}

// NewDeckhouseService creates a new deckhouse service
func NewDeckhouseService(client registry.Client, logger *log.Logger) *DeckhouseService {
	return &DeckhouseService{
		client: client,

		BasicService:             NewBasicService(deckhouseServiceName, client, logger),
		deckhouseReleaseChannels: NewDeckhouseReleaseService(NewBasicService(deckhouseReleaseChannelsServiceName, client.WithSegment(deckhouseReleaseChannelsSegment), logger)),

		logger: logger,
	}
}

func (s *DeckhouseService) ReleaseChannels() *DeckhouseReleaseService {
	return s.deckhouseReleaseChannels
}

// GetRoot gets path of the registry root
func (s *DeckhouseService) GetRoot() string {
	return s.client.GetRegistry()
}

type DeckhouseReleaseService struct {
	*BasicService
}

func NewDeckhouseReleaseService(basicService *BasicService) *DeckhouseReleaseService {
	return &DeckhouseReleaseService{
		BasicService: basicService,
	}
}

type DeckhouseReleaseMetadata struct {
	Version string

	Canary       map[string]CanarySettings
	Requirements map[string]string
	Disruptions  map[string][]string
	Suspend      bool

	Changelog map[string]interface{}
}

type CanarySettings struct {
	Enabled  bool
	Waves    uint
	Interval time.Duration
}

func (s *DeckhouseReleaseService) GetMetadata(ctx context.Context, tag string) (*DeckhouseReleaseMetadata, error) {
	logger := s.logger.With(slog.String("service", s.name), slog.String("tag", tag))

	logger.Debug("Getting metadata")

	img, err := s.client.GetImage(ctx, tag)
	if err != nil {
		return nil, fmt.Errorf("failed to get image: %w", err)
	}

	meta, err := s.extractDeckhouseReleaseMetadata(img.Extract())
	if err != nil {
		return nil, fmt.Errorf("failed to extract metadata: %w", err)
	}

	return meta, nil
}

type versionStruct struct {
	Version      string                    `json:"version"`
	Canary       map[string]canarySettings `json:"canary"`
	Requirements map[string]string         `json:"requirements"`
	Disruptions  map[string][]string       `json:"disruptions"`
	Suspend      bool                      `json:"suspend"`
}

type canarySettings struct {
	Enabled  bool            `json:"enabled"`
	Waves    uint            `json:"waves"`
	Interval libapi.Duration `json:"interval"` // in minutes
}

func (s *DeckhouseReleaseService) extractDeckhouseReleaseMetadata(rc io.ReadCloser) (*DeckhouseReleaseMetadata, error) {
	var meta = new(DeckhouseReleaseMetadata)

	defer rc.Close()

	drr := &deckhouseReleaseReader{
		versionReader:   bytes.NewBuffer(nil),
		changelogReader: bytes.NewBuffer(nil),
	}

	err := drr.untarMetadata(rc)
	if err != nil {
		return nil, err
	}

	var version versionStruct
	if drr.versionReader.Len() > 0 {
		err = json.NewDecoder(drr.versionReader).Decode(&version)
		if err != nil {
			return nil, fmt.Errorf("metadata decode: %w", err)
		}

		meta.Version = version.Version
		meta.Requirements = version.Requirements
		meta.Disruptions = version.Disruptions
		meta.Suspend = version.Suspend

		// Convert canary settings
		if version.Canary != nil {
			meta.Canary = make(map[string]CanarySettings)
			for k, v := range version.Canary {
				meta.Canary[k] = CanarySettings{
					Enabled:  v.Enabled,
					Waves:    v.Waves,
					Interval: v.Interval.Duration,
				}
			}
		}
	}

	if drr.changelogReader.Len() > 0 {
		var changelog map[string]any

		err = yaml.NewDecoder(drr.changelogReader).Decode(&changelog)
		if err != nil {
			// if changelog build failed - warn about it but don't fail the release
			s.logger.Warn("Unmarshal CHANGELOG yaml failed", log.Err(err))

			changelog = make(map[string]any)
		}

		meta.Changelog = changelog
	}

	return meta, nil
}

type deckhouseReleaseReader struct {
	versionReader   *bytes.Buffer
	changelogReader *bytes.Buffer
}

func (rr *deckhouseReleaseReader) untarMetadata(rc io.Reader) error {
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

		default:
			continue
		}
	}
}
