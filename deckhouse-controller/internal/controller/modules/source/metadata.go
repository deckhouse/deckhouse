// Copyright 2026 Flant JSC
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

package source

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/controller/modules/source/releases"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/downloader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// metadataLoader reads what a module published, for one version or for a release channel.
//
// It is the single place the legacy downloader survives in this controller. The downloader is kept
// only because reading a release image's metadata has no equivalent on the registry service yet;
// nothing else here depends on it, so swapping it out is a change to this file alone. It writes
// nothing to disk: the two calls below never touch the downloader's store directory.
type metadataLoader struct {
	downloader *downloader.ModuleDownloader
}

// newMetadataLoader builds a loader against one source's registry.
//
// The cluster UUID travels as the registry User-Agent and is read per pass rather than cached, so a
// cluster that learns its identity after start stops sending the placeholder without a restart. The
// downloader's store directory is empty on purpose: neither call below writes to it.
func newMetadataLoader(
	ctx context.Context,
	dc dependency.Container,
	cli client.Client,
	source *v1alpha1.ModuleSource,
	logger *log.Logger,
) *metadataLoader {
	opts := utils.GenerateRegistryOptionsFromModuleSource(source, utils.GetClusterUUID(ctx, cli), logger)

	return &metadataLoader{
		downloader: downloader.NewModuleDownloader(dc, "", source, logger.Named("downloader"), opts),
	}
}

// ReleaseMetadata loads the metadata a module published for one version.
func (l *metadataLoader) ReleaseMetadata(ctx context.Context, moduleName, version string) (*releases.Metadata, error) {
	result, err := l.downloader.DownloadReleaseImageInfoByVersion(ctx, moduleName, version)
	if err != nil {
		return nil, fmt.Errorf("download the release image info of '%s': %w", version, err)
	}

	return toMetadata(result), nil
}

// channelMetadata loads the metadata of the version a release channel currently points at.
func (l *metadataLoader) channelMetadata(ctx context.Context, moduleName, releaseChannel string) (*releases.Metadata, error) {
	result, err := l.downloader.DownloadMetadataFromReleaseChannel(ctx, moduleName, releaseChannel)
	if err != nil {
		return nil, fmt.Errorf("download the metadata from the '%s' release channel: %w", releaseChannel, err)
	}

	return toMetadata(result), nil
}

// toMetadata restates a downloader result in the vocabulary the releases package works in.
func toMetadata(result *downloader.ModuleDownloadResult) *releases.Metadata {
	return &releases.Metadata{
		Version:    result.ModuleVersion,
		Checksum:   result.Checksum,
		Changelog:  result.Changelog,
		Definition: result.ModuleDefinition,
	}
}
