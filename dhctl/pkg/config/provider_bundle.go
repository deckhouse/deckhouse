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

package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sync/singleflight"
	"sigs.k8s.io/yaml"

	validatev1 "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/api/validate/v1"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config/digests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/providerdir"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/registrydata"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/image"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

// KubeClientGetter lazily provides a kube client for the target cluster: which module build a
// cluster runs is a property of that cluster, not of the bundle lying in the download dir.
type KubeClientGetter func(ctx context.Context) (*client.KubernetesClient, error)

// bundleRegistryGetter reads the registry to download the provider bundle from. A function because
// one caller parses documents for it and another calls the target cluster.
type bundleRegistryGetter func(ctx context.Context) (*image.RegistryConfig, error)

var (
	ensureProviderGroup singleflight.Group

	// Prefers the module chain, falls back to this installer's embedded digests. globalOptions
	// travels along because providerIsExternal reads the modules directory, whose path is not
	// fixed: ResolveAndApplyPaths roots it at the working directory, or under DownloadDir.
	resolveProviderBundleRef = func(ctx context.Context, provider string, lookup providerModuleLookup, globalOptions *options.GlobalOptions) (providerBundleRef, error) {
		ref, err := resolveModuleProviderBundle(ctx, provider, lookup, globalOptions)
		if err != nil {
			return providerBundleRef{}, err
		}
		if ref.Digest != "" {
			return ref, nil
		}

		digest, err := digests.GetImage(sectionForProvider(provider), terraformManagerImageName)
		if err != nil {
			return providerBundleRef{}, fmt.Errorf("get terraform-manager image digest for provider %s: %w", provider, err)
		}
		return providerBundleRef{Digest: digest}, nil
	}

	downloadProviderBundle = image.DownloadAndUnpackImage
)

// IsInternalCloudProviderBundle reports whether the provider needs no external bundle download. This is
// true when its schemas ship in the image's candi and for an empty provider (a static cluster).
func IsInternalCloudProviderBundle(provider string, globalOptions *options.GlobalOptions) bool {
	// Is static cluster
	if provider == "" {
		return true
	}

	candiDir := options.DefaultCandiDir
	if globalOptions != nil && globalOptions.CandiDir != "" {
		candiDir = globalOptions.CandiDir
	}

	// Is internal provider
	schemaPath := filepath.Join(candiDir, "cloud-providers", strings.ToLower(provider), "openapi", "cluster_configuration.yaml")
	_, err := os.Stat(schemaPath)
	return err == nil
}

// EnsureProviderBundleFromConfig downloads the external provider bundle and loads its
// schemas so the provider becomes validatable in this process. No-op for
// static clusters and providers whose schemas are bundled in candi. Empty
// provider is extracted from docs; docs also supply registry access (default
// public registry otherwise). Concurrent same-provider calls share one download.
func EnsureProviderBundleFromConfig(ctx context.Context, provider string, docs []string, globalOptions *options.GlobalOptions) error {
	globalOptions = withDownloadDir(globalOptions)

	provider, err := cloudProviderNameOrFromDocs(provider, docs)
	if err != nil {
		return err
	}

	// Is static cluster
	if provider == "" {
		return nil
	}

	// Read before the early return below: an explicitly pinned provider module must reach the
	// resolver even when this image carries the provider's candi schemas. Otherwise dhctl would
	// install the pinned build while validating against another build's schemas.
	md, err := ParseModuleDocs(docs)
	if err != nil {
		return err
	}

	if IsInternalCloudProviderBundle(provider, globalOptions) && !md.providerModulePinned(CloudProviderModuleName(provider)) {
		return nil
	}

	// Update external provider
	return updateProviderBundle(ctx, provider, configModuleDocs(docs), docsBundleRegistry(provider, docs), globalOptions, true)
}

// EnsureProviderBundleFromCluster downloads and unpacks the external provider's OCI bundle using the
// registry read from the target cluster. Commander operations receive no registry_config, so the
// bundle registry is unknown from the request and the cluster is the only source of truth.
func EnsureProviderBundleFromCluster(ctx context.Context, kubeClient KubeClientGetter, clusterConfigData string, globalOptions *options.GlobalOptions, operation string) error {
	globalOptions = withDownloadDir(globalOptions)

	provider, err := cloudProviderNameOrFromDocs(
		"",
		input.YAMLSplitRegexp.Split(strings.TrimSpace(clusterConfigData), -1),
	)
	if err != nil {
		return err
	}

	// Is static cluster
	if provider == "" {
		return nil
	}

	// Is internal provider
	if IsInternalCloudProviderBundle(provider, globalOptions) {
		return nil
	}

	lookup := clusterModuleDocs(kubeClient, provider, globalOptions.KubeInCluster)
	return updateProviderBundle(ctx, provider, lookup, clusterBundleRegistry(kubeClient), globalOptions, operationRequiresFreshBundle(operation))
}

// providerSchemasAvailable reports whether this process can already read the provider's own
// configuration: the schemas ship in candi, were loaded earlier, or sit in a bundle delivered under
// the alias — the last case is loaded into the store on the way.
func providerSchemasAvailable(provider string, globalOptions *options.GlobalOptions) (bool, error) {
	if IsInternalCloudProviderBundle(provider, globalOptions) {
		return true, nil
	}

	store := NewSchemaStore(globalOptions)
	if store.HasProviderSchemas(provider) {
		return true, nil
	}

	dir, digest, ok := deliveredBundle(provider, globalOptions)
	if !ok {
		return false, nil
	}
	if err := store.LoadProviderDir(provider, digest, dir); err != nil {
		return false, err
	}
	return store.HasProviderSchemas(provider), nil
}

// ensureProviderSchemas makes the provider's configuration readable, downloading the bundle only
// when nothing has been delivered yet. Without the schemas the provider's own ClusterConfiguration
// document is silently kept as an unrelated resource, so a bundle that never arrives is an error.
func ensureProviderSchemas(ctx context.Context, provider string, docs []string, globalOptions *options.GlobalOptions) error {
	available, err := providerSchemasAvailable(provider, globalOptions)
	if err != nil {
		return err
	}
	if available {
		return nil
	}

	if err := EnsureProviderBundleFromConfig(ctx, provider, docs, globalOptions); err != nil {
		return err
	}

	available, err = providerSchemasAvailable(provider, globalOptions)
	if err != nil {
		return err
	}
	if !available {
		return fmt.Errorf("provider bundle for %q carries no schemas to read its configuration with: %s",
			provider, providerdir.ProviderDir(globalOptions.DownloadDir, provider))
	}
	return nil
}

// operationRequiresFreshBundle reports whether the operation must fail rather than go on with the
// bundle already on disk. Only destroy may go on: interrupted halfway with cluster access gone, it
// needs that bundle to clean up the resources it would otherwise orphan.
func operationRequiresFreshBundle(operation string) bool {
	switch validatev1.Operation(operation) {
	case validatev1.OperationDestroy:
		return false
	default:
		return true
	}
}

// docsBundleRegistry derives the registry from the configuration documents.
func docsBundleRegistry(provider string, docs []string) bundleRegistryGetter {
	return func(context.Context) (*image.RegistryConfig, error) {
		conf, err := buildRegistryConfig(docs)
		if err != nil {
			return nil, fmt.Errorf("registry data to fetch provider bundle for %q: %w", provider, err)
		}
		return conf, nil
	}
}

// clusterBundleRegistry reads the registry from the target cluster.
func clusterBundleRegistry(kubeClient KubeClientGetter) bundleRegistryGetter {
	return func(ctx context.Context) (*image.RegistryConfig, error) {
		kubeCl, err := kubeClient(ctx)
		if err != nil {
			return nil, fmt.Errorf("get kube client for provider bundle: %w", err)
		}

		// deckhouse-registry points at the registry.d8-system.svc mirror on clusters with an
		// in-cluster registry, which an out-of-cluster caller cannot resolve.
		conf, found, err := registrydata.GetUpstreamRegistryData(ctx, kubeCl)
		if err != nil {
			return nil, fmt.Errorf("get upstream registry data from cluster: %w", err)
		}
		if found {
			return conf, nil
		}

		conf, _, err = registrydata.GetRegistryData(ctx, kubeCl)
		if err != nil {
			return nil, fmt.Errorf("get registry data from cluster: %w", err)
		}
		return conf, nil
	}
}

// updateProviderBundle resolves which bundle the provider needs and unpacks it unless that digest
// is already in place. Every step that can leave the provider without a fresh bundle falls back to
// the one already downloaded.
func updateProviderBundle(
	ctx context.Context,
	provider string,
	lookup providerModuleLookup,
	registry bundleRegistryGetter,
	globalOptions *options.GlobalOptions,
	required bool,
) error {
	ref, err := resolveProviderBundleRef(ctx, provider, lookup, globalOptions)
	if err != nil {
		if required {
			return err
		}
		return fallBackToDeliveredBundle(ctx, provider, "digest resolution", err, globalOptions)
	}

	if bundleReadyAtDigest(provider, ref.Digest, globalOptions) {
		return nil
	}

	_, err, _ = ensureProviderGroup.Do(provider+"@"+ref.Digest, func() (interface{}, error) {
		if bundleReadyAtDigest(provider, ref.Digest, globalOptions) {
			return nil, nil
		}
		if err := unpackProviderBundle(ctx, provider, ref, registry, globalOptions); err != nil {
			return nil, err
		}
		// Validate and load from the real digest dir before activating it. The old alias must remain
		// usable until the replacement is known to carry readable provider schemas.
		digestDir := providerdir.ProviderDigestDir(globalOptions.DownloadDir, provider, ref.Digest)
		if _, err := os.Stat(providerdir.SchemaPath(digestDir)); err != nil {
			_ = os.RemoveAll(digestDir)
			return nil, fmt.Errorf("provider bundle %s carries no cluster configuration schema: %w", digestDir, err)
		}
		if err := NewSchemaStore(globalOptions).LoadProviderDir(provider, ref.Digest, digestDir); err != nil {
			_ = os.RemoveAll(digestDir)
			return nil, err
		}
		return nil, switchProviderSymlink(providerdir.ProviderDir(globalOptions.DownloadDir, provider), digestDir)
	})
	if err != nil {
		if required {
			return err
		}
		return fallBackToDeliveredBundle(ctx, provider, "download", err, globalOptions)
	}
	return nil
}

func unpackProviderBundle(ctx context.Context, provider string, ref providerBundleRef, registry bundleRegistryGetter, globalOptions *options.GlobalOptions) error {
	digest := ref.Digest
	digestDir := providerdir.ProviderDigestDir(globalOptions.DownloadDir, provider, digest)
	if _, err := os.Stat(digestDir); err != nil {
		// Asked for here and not earlier: a digest already unpacked needs no registry at all, and
		// reading one costs a call to the target cluster.
		conf := ref.Registry
		if conf == nil {
			if conf, err = registry(ctx); err != nil {
				return fmt.Errorf("registry data to download provider bundle for %q: %w", provider, err)
			}
		}

		// The module chain knows the repository; the in-tree path pins the flat images repo.
		imgName := ref.Image
		if imgName == "" {
			imgName = conf.GetRegistry() + "@" + digest
		}
		dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Downloading provider bundle for %s", provider))
		// Download into a temp dir and rename into place on success: the image
		// puller creates the destination before writing, so a failed or killed
		// download would otherwise leave a partial digestDir whose bare
		// existence short-circuits the re-download above (and the tmp cleaner
		// deliberately keeps bundle dirs), permanently poisoning the cache. An
		// orphaned .partial dir does not match the cleaner's bundle-dir pattern
		// and is swept on the next run.
		partialDir := digestDir + ".partial"
		if err := os.RemoveAll(partialDir); err != nil {
			return fmt.Errorf("clean partial provider bundle dir %s: %w", partialDir, err)
		}
		if err := downloadProviderBundle(ctx, imgName, partialDir, globalOptions.DownloadCacheDir, *conf, globalOptions.ShowProgress); err != nil {
			_ = os.RemoveAll(partialDir)
			// Runs before every preflight check, so this message is all the operator gets.
			return fmt.Errorf("download provider bundle for %q from %s: %w\n"+
				"It carries the provider schemas and the validator, so the configuration cannot be checked without it. "+
				"Make that reference reachable with the registry credentials from the configuration, "+
				"or unpack the bundle into %s yourself before dhctl starts.",
				provider, imgName, err, providerdir.ProviderDir(globalOptions.DownloadDir, provider))
		}
		// The image puller leaves the downloaded tarball next to the unpacked
		// tree. The digest-pinned directory itself is the cache (its presence
		// short-circuits the download above), so the tarball only duplicates
		// the bundle on disk — drop it.
		_ = os.Remove(filepath.Join(partialDir, digest))
		if err := os.Rename(partialDir, digestDir); err != nil {
			return fmt.Errorf("move provider bundle into place %s: %w", digestDir, err)
		}
	}
	return nil
}

// fallBackToDeliveredBundle keeps the operation running on the bundle this download dir already
// holds after stage failed with cause. Without a usable bundle there is nothing to fall back to and
// cause is returned untouched.
func fallBackToDeliveredBundle(
	ctx context.Context,
	provider, stage string,
	cause error,
	globalOptions *options.GlobalOptions,
) error {
	dir, digest, ok := deliveredBundle(provider, globalOptions)
	if !ok {
		return cause
	}

	dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf(
		"Provider bundle update for %q failed at %s: %v. Using the bundle already downloaded at %s.",
		provider, stage, cause, dir,
	))

	if err := NewSchemaStore(globalOptions).LoadProviderDir(provider, digest, dir); err != nil {
		return errors.Join(cause, fmt.Errorf("load provider bundle %s: %w", dir, err))
	}
	return nil
}

// bundleReadyAtDigest reports that this process already validates the provider against digest and
// that the default alias leads to that very bundle. Everything looked up later — the validator
// binary, the opentofu plugin — follows that alias, so schemas from one digest next to an alias
// pointing at another is not ready.
func bundleReadyAtDigest(provider, digest string, globalOptions *options.GlobalOptions) bool {
	if !NewSchemaStore(globalOptions).ProviderSchemasLoaded(provider, digest) {
		return false
	}

	_, delivered, ok := deliveredBundle(provider, globalOptions)
	return ok && delivered == digest
}

// deliveredBundle reports the usable bundle under the provider's default alias — its directory and
// its digest. Usable means the alias resolves to a digest dir and that dir carries the schema dhctl
// validates against.
func deliveredBundle(provider string, globalOptions *options.GlobalOptions) (string, string, bool) {
	dir, digest, ok := providerdir.Delivered(globalOptions.DownloadDir, provider)
	if !ok {
		return "", "", false
	}
	if _, err := os.Stat(providerdir.SchemaPath(dir)); err != nil {
		return "", "", false
	}
	return dir, digest, true
}

// switchProviderSymlink atomically points linkPath at target. A pre-symlink
// layout may have left a real directory at linkPath — it is replaced.
func switchProviderSymlink(linkPath, target string) error {
	if info, err := os.Lstat(linkPath); err == nil && info.Mode()&os.ModeSymlink == 0 {
		if err := os.RemoveAll(linkPath); err != nil {
			return fmt.Errorf("remove legacy provider dir %s: %w", linkPath, err)
		}
	}
	tmp := linkPath + ".tmp"
	_ = os.Remove(tmp)
	if err := os.Symlink(target, tmp); err != nil {
		return fmt.Errorf("create provider symlink: %w", err)
	}
	if err := os.Rename(tmp, linkPath); err != nil {
		return fmt.Errorf("activate provider symlink: %w", err)
	}
	return nil
}

// Configuration and reference helpers.
func cloudProviderNameOrFromDocs(provider string, docs []string) (string, error) {
	if provider != "" {
		return strings.ToLower(provider), nil
	}

	provider, err := cloudProviderNameFromDocs(docs)
	if err != nil {
		return "", err
	}

	return strings.ToLower(provider), nil
}

func cloudProviderNameFromDocs(docs []string) (string, error) {
	for _, doc := range docs {
		if err := detectMergedDocuments(doc); err != nil {
			return "", fmt.Errorf("config validation failed: %w\ndata:\n%s\n", err, numerateManifestLines([]byte(doc)))
		}

		var config struct {
			Kind  string `yaml:"kind"`
			Cloud struct {
				Provider string `yaml:"provider"`
			} `yaml:"cloud"`
		}
		if err := yaml.Unmarshal([]byte(doc), &config); err != nil {
			return "", err
		}
		if config.Kind == "ClusterConfiguration" {
			return strings.ToLower(config.Cloud.Provider), nil
		}
	}
	return "", nil
}

// The images_digests.json section a provider's images live under.
func sectionForProvider(provider string) string {
	return "cloudProvider" + strings.ToUpper(provider[:1]) + provider[1:]
}
