/*
Copyright 2023 Flant JSC

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

package downloader

import (
	"context"
	"os"
	"testing"

	"github.com/gojuno/minimock/v3"
	crv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/fake"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/go_lib/cache"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
)

func TestDownloadMetadataFromReleaseChannelError(t *testing.T) {
	ms := &v1alpha1.ModuleSource{}

	// Mock Digest method first
	dependency.TestDC.CRClient.DigestMock.When(minimock.AnyContext, "stable").Then("sha256:1234567890abcdef", nil)

	dependency.TestDC.CRClient.ImageMock.When(minimock.AnyContext, "stable").Then(&fake.FakeImage{
		ManifestStub: func() (*crv1.Manifest, error) {
			return &crv1.Manifest{
				SchemaVersion: 2,
				Layers:        []crv1.Descriptor{},
			}, nil
		},
		LayersStub: func() ([]crv1.Layer, error) {
			return []crv1.Layer{&utils.FakeLayer{}}, nil
		},
		DigestStub: func() (crv1.Hash, error) {
			return crv1.Hash{Algorithm: "sha256", Hex: "1234567890abcdef"}, nil
		},
	}, nil)

	md := NewModuleDownloader(dependency.TestDC, os.TempDir(), ms, log.NewNop(), nil)
	_, err := md.DownloadMetadataFromReleaseChannel(context.Background(), "commander", "stable")
	require.Error(t, err)
	require.Contains(t, err.Error(), "no version found")
}

func TestDownloadMetadataByVersion(t *testing.T) {
	ms := &v1alpha1.ModuleSource{}

	// Mock Digest method first
	dependency.TestDC.CRClient.DigestMock.When(minimock.AnyContext, "v1.2.3").Then("sha256:1234567890abcdef", nil)

	dependency.TestDC.CRClient.ImageMock.When(minimock.AnyContext, "v1.2.3").Then(&fake.FakeImage{
		ManifestStub: func() (*crv1.Manifest, error) {
			return &crv1.Manifest{
				SchemaVersion: 2,
				Layers:        []crv1.Descriptor{},
			}, nil
		},
		LayersStub: func() ([]crv1.Layer, error) {
			return []crv1.Layer{
				&utils.FakeLayer{},
				&utils.FakeLayer{
					FilesContent: map[string]string{
						"version.json":   `{"version": "v1.2.3"}`,
						"changelog.yaml": "feat:\n- Added new feature\n",
					}}}, nil
		},
		DigestStub: func() (crv1.Hash, error) {
			return crv1.Hash{Algorithm: "sha256", Hex: "1234567890abcdef"}, nil
		},
	}, nil)

	md := NewModuleDownloader(dependency.TestDC, os.TempDir(), ms, log.NewNop(), nil)
	meta, err := md.DownloadReleaseImageInfoByVersion(context.TODO(), "commander", "v1.2.3")
	require.NoError(t, err)
	require.Equal(t, "v1.2.3", meta.ModuleVersion)
	require.Equal(t, map[string]any{"feat": []any{"Added new feature"}}, meta.Changelog)
}

func TestReleaseImageInfoCache(t *testing.T) {
	// Test cache creation
	cacheInstance := cache.NewReleaseImageInfoCache()

	// Test setting and getting
	info1 := &cache.LightweightReleaseInfo{}
	cacheInstance.Set("digest1", info1)

	if cached, found := cacheInstance.Get("digest1"); !found {
		t.Error("Expected to find digest1 in cache")
	} else if cached != info1 {
		t.Error("Expected to get the same info from cache")
	}

	// Test cache growth without size limit
	info2 := &cache.LightweightReleaseInfo{}
	info3 := &cache.LightweightReleaseInfo{}

	cacheInstance.Set("digest2", info2)
	cacheInstance.Set("digest3", info3)
}

func TestModuleDownloaderCache(t *testing.T) {
	// Test that ModuleDownloader is created with cache
	ms := &v1alpha1.ModuleSource{}
	downloader := NewModuleDownloader(dependency.TestDC, os.TempDir(), ms, log.NewNop(), nil)
	if downloader.releaseInfoCache == nil {
		t.Error("Expected ModuleDownloader to have a cache")
	}
}

func TestModuleDownloaderWithGlobalCache(t *testing.T) {
	// Test that multiple ModuleDownloader instances share the same global cache
	ms := &v1alpha1.ModuleSource{}

	downloader1 := NewModuleDownloader(dependency.TestDC, os.TempDir(), ms, log.NewNop(), nil)
	downloader2 := NewModuleDownloader(dependency.TestDC, os.TempDir(), ms, log.NewNop(), nil)

	// Verify both downloaders share the same cache instance (global cache)
	if downloader1.releaseInfoCache != downloader2.releaseInfoCache {
		t.Error("Expected both downloaders to share the same global cache instance")
	}
}
