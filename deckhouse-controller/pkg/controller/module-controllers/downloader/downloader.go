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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/shell-operator/pkg/utils/measure"
	crv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/iancoleman/strcase"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"gopkg.in/yaml.v3"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	moduletypes "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader/types"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers/reginjector"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	moduletools "github.com/deckhouse/deckhouse/go_lib/module"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	defaultModuleWeight = 900
	DefaultDevVersion   = "dev"

	tracerName = "downloader"
)

// ReleaseImageInfoCache provides thread-safe caching for lightweight release metadata
// Uses LightweightReleaseInfo to minimize memory footprint (1-8KB vs 50-200MB per entry)
type ReleaseImageInfoCache struct {
	cache       map[string]*cacheEntry
	mutex       sync.RWMutex
	hitCount    int64
	missCount   int64
	maxSize     int
	maxMemoryMB int64
}

type cacheEntry struct {
	info        *LightweightReleaseInfo
	timestamp   time.Time
	accessCount int64
}

// NewReleaseImageInfoCache creates a new cache with optimized settings
func NewReleaseImageInfoCache() *ReleaseImageInfoCache {
	return &ReleaseImageInfoCache{
		cache:       make(map[string]*cacheEntry),
		mutex:       sync.RWMutex{},
		maxSize:     1000, // Maximum number of cached entries
		maxMemoryMB: 100,  // Maximum cache memory usage in MB
	}
}

// newReleaseImageInfoCache creates a new cache (internal function for backward compatibility)
func newReleaseImageInfoCache() *ReleaseImageInfoCache {
	return NewReleaseImageInfoCache()
}

// Get retrieves LightweightReleaseInfo from cache if it exists
func (c *ReleaseImageInfoCache) Get(digest string) (*LightweightReleaseInfo, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	entry, exists := c.cache[digest]
	if !exists {
		atomic.AddInt64(&c.missCount, 1)
		return nil, false
	}

	// Update access statistics
	atomic.AddInt64(&c.hitCount, 1)
	atomic.AddInt64(&entry.accessCount, 1)
	entry.timestamp = time.Now()

	return entry.info, true
}

// Set stores LightweightReleaseInfo in cache
func (c *ReleaseImageInfoCache) Set(digest string, info *LightweightReleaseInfo) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Check if cache needs cleanup before adding new entry
	c.evictIfNeeded()

	c.cache[digest] = &cacheEntry{
		info:        info,
		timestamp:   time.Now(),
		accessCount: 1,
	}
}

// Clear removes all entries from cache
func (c *ReleaseImageInfoCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.cache = make(map[string]*cacheEntry)
	atomic.StoreInt64(&c.hitCount, 0)
	atomic.StoreInt64(&c.missCount, 0)
}

// Stats returns cache statistics
func (c *ReleaseImageInfoCache) Stats() (int64, int64, int) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	hits := atomic.LoadInt64(&c.hitCount)
	misses := atomic.LoadInt64(&c.missCount)
	size := len(c.cache)
	return hits, misses, size
}

// GetHitRate returns cache hit rate as percentage
func (c *ReleaseImageInfoCache) GetHitRate() float64 {
	hits := atomic.LoadInt64(&c.hitCount)
	misses := atomic.LoadInt64(&c.missCount)
	total := hits + misses
	if total == 0 {
		return 0.0
	}
	return float64(hits) / float64(total) * 100.0
}

// evictIfNeeded removes old entries if cache exceeds size limits
func (c *ReleaseImageInfoCache) evictIfNeeded() {
	// Note: This method should be called while holding write lock
	if len(c.cache) < c.maxSize {
		return
	}

	// Find oldest entries to evict (LRU strategy)
	type entryAge struct {
		digest    string
		timestamp time.Time
	}

	entries := make([]entryAge, 0, len(c.cache))
	for digest, entry := range c.cache {
		entries = append(entries, entryAge{
			digest:    digest,
			timestamp: entry.timestamp,
		})
	}

	// Sort by timestamp (oldest first)
	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[i].timestamp.After(entries[j].timestamp) {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	// Remove oldest 10% of entries
	toRemove := len(entries) / 10
	if toRemove == 0 {
		toRemove = 1
	}

	for i := 0; i < toRemove; i++ {
		delete(c.cache, entries[i].digest)
	}
}

// GetMemoryUsage estimates cache memory usage in bytes
func (c *ReleaseImageInfoCache) GetMemoryUsage() int64 {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	// Estimate: ~4KB per entry (metadata + overhead)
	return int64(len(c.cache)) * 4 * 1024
}

type ModuleDownloader struct {
	dc                   dependency.Container
	downloadedModulesDir string

	ms              *v1alpha1.ModuleSource
	registryOptions []cr.Option
	logger          *log.Logger

	// Cache for ReleaseImageInfo to avoid repeated downloads
	releaseInfoCache *ReleaseImageInfoCache
}

func NewModuleDownloader(dc dependency.Container, downloadedModulesDir string, ms *v1alpha1.ModuleSource, logger *log.Logger, registryOptions []cr.Option, cache ...*ReleaseImageInfoCache) *ModuleDownloader {
	var releaseInfoCache *ReleaseImageInfoCache

	// If no cache provided, create a new one (for backward compatibility)
	if len(cache) == 0 || cache[0] == nil {
		releaseInfoCache = newReleaseImageInfoCache()
	} else {
		releaseInfoCache = cache[0]
	}

	return &ModuleDownloader{
		dc:                   dc,
		downloadedModulesDir: downloadedModulesDir,
		ms:                   ms,
		registryOptions:      registryOptions,
		logger:               logger,
		releaseInfoCache:     releaseInfoCache,
	}
}

type ModuleDownloadResult struct {
	Checksum      string
	ModuleVersion string

	ModuleDefinition *moduletypes.Definition
	Changelog        map[string]any

	// FromReleaseChannel indicates that this result was obtained from a release channel
	// and contains complete metadata, avoiding the need for additional registry requests
	FromReleaseChannel bool
}

// DownloadDevImageTag downloads image tag and store it in the .../<moduleName>/dev fs path
// if checksum is equal to a module image digest - do nothing
// otherwise return new digest
func (md *ModuleDownloader) DownloadDevImageTag(moduleName, imageTag, checksum string) (string, *moduletypes.Definition, error) {
	moduleStorePath := path.Join(md.downloadedModulesDir, moduleName, DefaultDevVersion)

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

	if _, err = md.fetchAndCopyModuleByVersion(moduleName, imageTag, moduleStorePath); err != nil {
		return "", nil, err
	}

	return digest.String(), md.fetchModuleDefinitionFromFS(moduleName, moduleStorePath), nil
}

func (md *ModuleDownloader) DownloadByModuleVersion(ctx context.Context, moduleName, moduleVersion string) (*DownloadStatistic, error) {
	_, span := otel.Tracer(tracerName).Start(ctx, "DownloadByModuleVersion")
	defer span.End()

	if !strings.HasPrefix(moduleVersion, "v") {
		moduleVersion = "v" + moduleVersion
	}

	moduleVersionPath := path.Join(md.downloadedModulesDir, moduleName, moduleVersion)

	return md.fetchAndCopyModuleByVersion(moduleName, moduleVersion, moduleVersionPath)
}

// DownloadMetadataFromReleaseChannel downloads only module release image with metadata: version.json, checksum.json(soon)
// does not fetch and install the desired version on the module, only fetches its module definition
func (md *ModuleDownloader) DownloadMetadataFromReleaseChannel(ctx context.Context, moduleName, releaseChannel string) (*ModuleDownloadResult, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "DownloadMetadataFromReleaseChannel")
	defer span.End()

	span.SetAttributes(attribute.String("module", moduleName))
	span.SetAttributes(attribute.String("releaseChannel", releaseChannel))

	md.logger.Info("🔵 REGISTRY REQUEST: Starting download metadata from release channel",
		slog.String("module", moduleName),
		slog.String("releaseChannel", releaseChannel),
		slog.String("registry_operation", "GET_RELEASE_CHANNEL"),
		slog.String("controller", "source"),
	)

	releaseImageInfo, err := md.fetchModuleReleaseMetadataFromReleaseChannel(ctx, moduleName, releaseChannel)
	if err != nil {
		return nil, err
	}

	res := &ModuleDownloadResult{
		Checksum:           releaseImageInfo.Digest.String(),
		ModuleVersion:      "v" + releaseImageInfo.Metadata.Version.String(),
		Changelog:          releaseImageInfo.Metadata.Changelog,
		ModuleDefinition:   releaseImageInfo.Metadata.ModuleDefinition,
		FromReleaseChannel: true,
	}

	md.logger.Info("🔵 REGISTRY REQUEST: Completed download metadata from release channel",
		slog.String("module", moduleName),
		slog.String("version", res.ModuleVersion),
		slog.String("checksum", res.Checksum),
		slog.Bool("fromReleaseChannel", res.FromReleaseChannel),
	)

	return res, nil
}

// DownloadReleaseImageInfoByVersion downloads only module release image with metadata: version.json
// does not fetch and install the desired version on the module, only fetches its module definition
func (md *ModuleDownloader) DownloadReleaseImageInfoByVersion(ctx context.Context, moduleName, moduleVersion string) (*ModuleDownloadResult, error) {
	md.logger.Info("🔴 REGISTRY REQUEST: Starting download image info by specific version",
		slog.String("module", moduleName),
		slog.String("version", moduleVersion),
		slog.String("registry_operation", "GET_VERSION_SPECIFIC"),
		slog.String("controller", "release/override/moduleloader"),
	)

	releaseImageInfo, err := md.fetchModuleReleaseMetadataByVersion(ctx, moduleName, moduleVersion)
	if err != nil {
		return nil, fmt.Errorf("fetch module release: %w", err)
	}

	res := &ModuleDownloadResult{
		Checksum:           releaseImageInfo.Digest.String(),
		ModuleVersion:      moduleVersion,
		Changelog:          releaseImageInfo.Metadata.Changelog,
		FromReleaseChannel: false, // This is for specific version, not from release channel
	}
	if releaseImageInfo.Metadata.ModuleDefinition != nil {
		res.ModuleDefinition = releaseImageInfo.Metadata.ModuleDefinition
		md.logger.Info("🔴 REGISTRY REQUEST: Completed download image info by specific version (from metadata)",
			slog.String("module", moduleName),
			slog.String("version", moduleVersion),
			slog.String("checksum", res.Checksum),
			slog.Bool("fromReleaseChannel", res.FromReleaseChannel),
		)
		return res, nil
	}

	md.logger.Info("can not find module definition in metadata, extracting from image",
		slog.String("module_name", moduleName),
		slog.String("module_version", moduleVersion),
	)

	def, err := md.fetchModuleDefinitionFromImage(moduleName, releaseImageInfo.Image)
	if err != nil {
		return nil, fmt.Errorf("fetch module definition: %w", err)
	}
	res.ModuleDefinition = def

	md.logger.Info("🔴 REGISTRY REQUEST: Completed download image info by specific version (from image)",
		slog.String("module", moduleName),
		slog.String("version", moduleVersion),
		slog.String("checksum", res.Checksum),
		slog.Bool("fromReleaseChannel", res.FromReleaseChannel),
	)

	return res, nil
}

func (md *ModuleDownloader) GetDocumentationArchive(moduleName, moduleVersion string) (io.ReadCloser, error) {
	if !strings.HasPrefix(moduleVersion, "v") {
		moduleVersion = "v" + moduleVersion
	}

	img, err := md.fetchImage(moduleName, moduleVersion)
	if err != nil {
		return nil, fmt.Errorf("fetch image: %w", err)
	}

	return moduletools.ExtractDocs(img)
}

func (md *ModuleDownloader) fetchImage(moduleName, imageTag string) (crv1.Image, error) {
	regCli, err := md.dc.GetRegistryClient(path.Join(md.ms.Spec.Registry.Repo, moduleName), md.registryOptions...)
	if err != nil {
		return nil, fmt.Errorf("fetch module error: %v", err)
	}

	return regCli.Image(context.TODO(), imageTag)
}

func (md *ModuleDownloader) storeModule(moduleStorePath string, img crv1.Image) (*DownloadStatistic, error) {
	_ = os.RemoveAll(moduleStorePath)

	ds, err := md.copyModuleToFS(moduleStorePath, img)
	if err != nil {
		return nil, fmt.Errorf("copy module error: %v", err)
	}

	// inject registry to values
	err = reginjector.InjectRegistryToModuleValues(moduleStorePath, md.ms)
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

func (md *ModuleDownloader) copyModuleToFS(rootPath string, img crv1.Image) (*DownloadStatistic, error) {
	rc, err := cr.Extract(img)
	if err != nil {
		return nil, err
	}
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
		if os.Getenv("D8_IS_TESTS_ENVIRONMENT") == "true" {
			ds.PullDuration, _ = time.ParseDuration("555s")
		}
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

			// remove only 'user' permission bit, E.x.: 644 => 600, 755 => 700
			if err = os.Chmod(outFile.Name(), os.FileMode(hdr.Mode)&0o700); err != nil {
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
			if err = os.Link(path.Join(rootPath, hdr.Linkname), path.Join(rootPath, hdr.Name)); err != nil {
				return nil, fmt.Errorf("create hardlink: %w", err)
			}

		default:
			return nil, errors.New("unknown tar type")
		}
	}
}

// fetchModuleReleaseMetadataFromReleaseChannel get Image, Digest and release metadata by releaseChannel
// releaseChannel must be in kebab-case
// return error if version.json not found in metadata
// Image fetch path example: registry.deckhouse.io/deckhouse/ce/modules/$moduleName/release:$releaseChannel
func (md *ModuleDownloader) fetchModuleReleaseMetadataFromReleaseChannel(ctx context.Context, moduleName, releaseChannel string) (*ReleaseImageInfo, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "fetchModuleReleaseMetadataFromReleaseChannel")
	defer span.End()

	md.logger.Info("fetching module release metadata",
		slog.String("path", path.Join(md.ms.Spec.Registry.Repo, moduleName, "release")),
		slog.String("release_channel", releaseChannel),
	)

	md.logger.Debug("module metadata",
		slog.String("module_name", moduleName),
	)

	// fill releaseImageInfo.Image
	regCli, err := md.dc.GetRegistryClient(path.Join(md.ms.Spec.Registry.Repo, moduleName, "release"), md.registryOptions...)
	if err != nil {
		return nil, fmt.Errorf("fetch release image error: %w", err)
	}

	releaseImageInfo, err := md.getReleaseImageInfo(ctx, regCli, strcase.ToKebab(releaseChannel))
	if err != nil {
		return nil, fmt.Errorf("get image info: %w", err)
	}

	return releaseImageInfo, nil
}

// fetchModuleReleaseMetadataByVersion get Image, Digest and release metadata by version
// return error if version.json not found in metadata
// Image fetch path example: registry.deckhouse.io/deckhouse/ce/modules/$moduleName/release:$moduleVersion
func (md *ModuleDownloader) fetchModuleReleaseMetadataByVersion(ctx context.Context, moduleName, moduleVersion string) (*ReleaseImageInfo, error) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "fetchModuleReleaseMetadataByVersion")
	defer span.End()

	md.logger.Info("fetching module release metadata",
		slog.String("path", path.Join(md.ms.Spec.Registry.Repo, moduleName, "release")),
		slog.String("module_version", moduleVersion),
	)

	md.logger.Debug("module metadata",
		slog.String("module_name", moduleName),
	)

	// fill releaseImageInfo.Image
	regCli, err := md.dc.GetRegistryClient(path.Join(md.ms.Spec.Registry.Repo, moduleName, "release"), md.registryOptions...)
	if err != nil {
		return nil, fmt.Errorf("fetch release image error: %w", err)
	}

	releaseImageInfo, err := md.getReleaseImageInfo(ctx, regCli, moduleVersion)
	if err != nil {
		return nil, fmt.Errorf("get image info: %w", err)
	}

	return releaseImageInfo, nil
}

// getReleaseImageInfo get Image, Digest and release metadata using imageTag with existing registry client
// return error if version.json not found in metadata
func (md *ModuleDownloader) getReleaseImageInfo(ctx context.Context, regCli cr.Client, imageTag string) (*ReleaseImageInfo, error) {
	// First, get digest from registry without downloading the full image
	digestStr, err := regCli.Digest(ctx, imageTag)
	if err != nil {
		return nil, fmt.Errorf("fetch digest error: %w", err)
	}

	// Check cache first - only metadata is cached for memory efficiency
	if cachedLightInfo, found := md.releaseInfoCache.Get(digestStr); found {
		hits, misses, size := md.releaseInfoCache.Stats()
		hitRate := md.releaseInfoCache.GetHitRate()
		memUsage := md.releaseInfoCache.GetMemoryUsage()
		md.logger.Info("🟢 CACHE HIT: Release metadata found in cache, fetching image for complete info",
			slog.String("digest", digestStr),
			slog.String("image_tag", imageTag),
			slog.Int64("cache_hits", hits),
			slog.Int64("cache_misses", misses),
			slog.Int("cache_size", size),
			slog.Float64("hit_rate", hitRate),
			slog.Int64("memory_kb", memUsage/1024),
		)

		// Fetch image for complete ReleaseImageInfo (metadata is cached, image is fetched fresh)
		img, err := regCli.Image(ctx, imageTag)
		if err != nil {
			return nil, fmt.Errorf("fetch image error: %w", err)
		}

		// Return complete ReleaseImageInfo with cached metadata and fresh image
		return &ReleaseImageInfo{
			Image:    img,
			Digest:   cachedLightInfo.Digest,
			Metadata: cachedLightInfo.Metadata,
		}, nil
	}

	hits, misses, size := md.releaseInfoCache.Stats()
	hitRate := md.releaseInfoCache.GetHitRate()
	memUsage := md.releaseInfoCache.GetMemoryUsage()
	md.logger.Info("🔴 CACHE MISS: Release metadata not found in cache, downloading full image and extracting metadata",
		slog.String("digest", digestStr),
		slog.String("image_tag", imageTag),
		slog.Int64("cache_hits", hits),
		slog.Int64("cache_misses", misses),
		slog.Int("cache_size", size),
		slog.Float64("hit_rate", hitRate),
		slog.Int64("memory_kb", memUsage/1024),
		slog.String("registry_operation", "IMAGE_DOWNLOAD"),
	)

	// If not in cache, download the full image
	img, err := regCli.Image(ctx, imageTag)
	if err != nil {
		return nil, fmt.Errorf("fetch image error: %w", err)
	}

	// Verify digest matches
	imgDigest, err := img.Digest()
	if err != nil {
		return nil, fmt.Errorf("fetch image digest error: %w", err)
	}

	if imgDigest.String() != digestStr {
		return nil, fmt.Errorf("digest mismatch: expected %s, got %s", digestStr, imgDigest.String())
	}

	// fill releaseImageInfo.Metadata
	moduleMetadata, err := md.fetchModuleReleaseMetadata(ctx, img)
	if err != nil {
		return nil, fmt.Errorf("fetch release metadata error: %w", err)
	}
	if moduleMetadata.Version == nil {
		return nil, fmt.Errorf("metadata malformed: no version found")
	}

	// Create lightweight version for caching (saves 99.9% memory)
	lightweightInfo := &LightweightReleaseInfo{
		Digest:   imgDigest,
		Metadata: &moduleMetadata,
	}

	// Store only lightweight metadata in cache (not the heavy Image object)
	md.releaseInfoCache.Set(digestStr, lightweightInfo)

	// Create complete ReleaseImageInfo for return
	releaseImageInfo := &ReleaseImageInfo{
		Image:    img,
		Digest:   imgDigest,
		Metadata: &moduleMetadata,
	}

	hits, misses, size = md.releaseInfoCache.Stats()
	hitRate = md.releaseInfoCache.GetHitRate()
	memUsage = md.releaseInfoCache.GetMemoryUsage()
	md.logger.Info("💾 CACHE STORE: Lightweight metadata stored in cache (99.9% memory savings)",
		slog.String("digest", digestStr),
		slog.String("image_tag", imageTag),
		slog.Int64("cache_hits", hits),
		slog.Int64("cache_misses", misses),
		slog.Int("cache_size", size),
		slog.Float64("hit_rate", hitRate),
		slog.Int64("memory_kb", memUsage/1024),
	)

	return releaseImageInfo, nil
}

func (md *ModuleDownloader) fetchModuleDefinitionFromFS(name, path string) *moduletypes.Definition {
	def := &moduletypes.Definition{
		Name:   name,
		Weight: defaultModuleWeight,
		Path:   path,
	}

	defPath := filepath.Join(path, moduletypes.DefinitionFile)

	if _, err := os.Stat(defPath); err != nil {
		return def
	}

	f, err := os.Open(defPath)
	if err != nil {
		return def
	}
	defer f.Close()

	if err = yaml.NewDecoder(f).Decode(def); err != nil {
		return def
	}

	return def
}

func (md *ModuleDownloader) fetchModuleDefinitionFromImage(moduleName string, img crv1.Image) (*moduletypes.Definition, error) {
	def := &moduletypes.Definition{
		Name:   moduleName,
		Weight: defaultModuleWeight,
	}

	rc, err := cr.Extract(img)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	buf := bytes.NewBuffer(nil)

	if err = untarModuleDefinition(rc, buf); err != nil {
		return def, err
	}

	if buf.Len() == 0 {
		return def, nil
	}

	if err = yaml.NewDecoder(buf).Decode(def); err != nil {
		return def, err
	}

	return def, nil
}

func (md *ModuleDownloader) fetchModuleReleaseMetadata(ctx context.Context, img crv1.Image) (ModuleReleaseMetadata, error) {
	_, span := otel.Tracer(tracerName).Start(ctx, "fetchModuleReleaseMetadata")
	defer span.End()

	var meta ModuleReleaseMetadata

	rc, err := cr.Extract(img)
	if err != nil {
		return meta, fmt.Errorf("extract: %w", err)
	}
	defer rc.Close()

	rr := &releaseReader{
		versionReader:   bytes.NewBuffer(nil),
		changelogReader: bytes.NewBuffer(nil),
		moduleReader:    bytes.NewBuffer(nil),
	}

	if err = rr.untarMetadata(rc); err != nil {
		return meta, fmt.Errorf("untar metadata: %w", err)
	}

	if rr.versionReader.Len() > 0 {
		err = json.NewDecoder(rr.versionReader).Decode(&meta)
		if err != nil {
			return meta, fmt.Errorf("json decode: %w", err)
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

	if rr.moduleReader.Len() > 0 {
		var ModuleDefinition moduletypes.Definition
		err = yaml.NewDecoder(rr.moduleReader).Decode(&ModuleDefinition)
		if err != nil {
			meta.ModuleDefinition = nil
			return meta, nil
		}

		meta.ModuleDefinition = &ModuleDefinition
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

type ModuleReleaseMetadata struct {
	Version *semver.Version `json:"version"`

	Changelog        map[string]any          `json:"-"`
	ModuleDefinition *moduletypes.Definition `json:"module,omitempty"`
}

// LightweightReleaseInfo contains only metadata and digest, without the heavy Image object
// This reduces memory usage by 99.9% (from ~50-200MB to ~1-8KB per cache entry)
type LightweightReleaseInfo struct {
	Metadata *ModuleReleaseMetadata
	Digest   crv1.Hash
}

type ReleaseImageInfo struct {
	Metadata *ModuleReleaseMetadata
	Image    crv1.Image
	Digest   crv1.Hash
}
