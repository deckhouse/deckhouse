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

	"sigs.k8s.io/yaml"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
	libdhctlyaml "github.com/deckhouse/lib-dhctl/pkg/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config/digests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/providerdir"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/registrydata"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/image"
)

// KubeClientGetter lazily provides a kube client for the target cluster: which module build a
// cluster runs is a property of that cluster, not of the bundle lying in the download dir.
type KubeClientGetter func(ctx context.Context) (*client.KubernetesClient, error)

// bundleRegistryGetter reads the registry to download the provider bundle from. A function because
// one caller parses documents for it and another calls the target cluster.
type bundleRegistryGetter func(ctx context.Context) (*image.RegistryConfig, error)

var (
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

		digest, err := digests.GetImage(digests.ProviderImagesSection(provider), digests.TerraformManagerImage)
		if err != nil {
			return providerBundleRef{}, fmt.Errorf("get terraform-manager image digest for provider %s: %w", provider, err)
		}
		return providerBundleRef{Digest: digest}, nil
	}

	// Replaces the registry pull in tests.
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

// ensureProviderBundleFromConfig downloads the external provider bundle named by the
// ClusterConfiguration in docs and loads its schemas. No-op for static clusters and providers
// whose schemas are bundled in candi. Docs also supply registry access (default public registry otherwise).
func ensureProviderBundleFromConfig(ctx context.Context, docs []string, globalOptions *options.GlobalOptions) (string, error) {
	globalOptions = withDownloadDir(globalOptions)

	provider, err := cloudProviderNameFromDocs(docs)
	if err != nil {
		// Both callers parse the same documents next, and that parse reports this one with the rest.
		dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Skip provider bundle, a document is unreadable: %v", err))
		return "", nil
	}

	// Is static cluster
	if provider == "" {
		return "", nil
	}

	// Read before the early return below: an explicitly pinned provider module must reach the
	// resolver even when this image carries the provider's candi schemas. Otherwise dhctl would
	// install the pinned build while validating against another build's schemas.
	md, err := ParseModuleDocs(docs)
	if err != nil {
		return "", err
	}

	if IsInternalCloudProviderBundle(provider, globalOptions) && !md.providerModulePinned(CloudProviderModuleName(provider)) {
		return "", nil
	}

	// Update external provider
	dir, _, err := updateProviderBundle(ctx, provider, configModuleDocs(docs), docsBundleRegistry(provider, docs), globalOptions)
	return dir, err
}

// EnsureProviderBundleFromCluster downloads and unpacks the external provider's OCI bundle using the
// registry read from the target cluster and returns its directory and digest. Commander operations
// receive no registry_config, so the cluster is the only source of truth for the bundle registry.
func EnsureProviderBundleFromCluster(ctx context.Context, kubeClient KubeClientGetter, clusterConfigData string, globalOptions *options.GlobalOptions) (string, string, error) {
	globalOptions = withDownloadDir(globalOptions)

	provider, err := cloudProviderNameFromDocs(libdhctlyaml.SplitYAML(clusterConfigData))
	if err != nil {
		return "", "", err
	}

	// Is static cluster
	if provider == "" {
		return "", "", nil
	}

	// Is internal provider
	if IsInternalCloudProviderBundle(provider, globalOptions) {
		return "", "", nil
	}

	lookup := clusterModuleDocs(kubeClient, provider, globalOptions.KubeInCluster)
	return updateProviderBundle(ctx, provider, lookup, clusterBundleRegistry(kubeClient), globalOptions)
}

// UseUnpackedProviderBundle makes the bundle of the given digest usable in this process without
// asking anyone which bundle the cluster needs. The digest comes from the state of that cluster.
func UseUnpackedProviderBundle(ctx context.Context, clusterConfigData, digest string, globalOptions *options.GlobalOptions) (string, error) {
	globalOptions = withDownloadDir(globalOptions)

	provider, err := cloudProviderNameFromDocs(libdhctlyaml.SplitYAML(clusterConfigData))
	if err != nil {
		return "", fmt.Errorf("read provider from cluster configuration: %w", err)
	}
	if provider == "" || digest == "" {
		return "", fmt.Errorf("use unpacked provider bundle: provider and digest are required, got %q and %q", provider, digest)
	}

	dir := providerdir.DigestDir(globalOptions.DownloadDir, provider, digest)
	if err := loadBundleSchemas(provider, digest, dir, globalOptions); err != nil {
		return "", err
	}

	return dir, nil
}

// RestoreProviderBundle makes the bundle a MetaConfig from the state cache was built with usable
// in this process. False means that MetaConfig cannot be used as is: it names an external
// provider but was saved by a dhctl that recorded no bundle.
func RestoreProviderBundle(ctx context.Context, m *MetaConfig, globalOptions *options.GlobalOptions) (bool, error) {
	if m.ProviderBundleDir != "" {
		if err := loadBundleSchemas(m.ProviderName, providerdir.DigestFromDir(m.ProviderBundleDir), m.ProviderBundleDir, globalOptions); err != nil {
			return false, fmt.Errorf("restore provider bundle from the state cache: %w", err)
		}
		return true, nil
	}

	return IsInternalCloudProviderBundle(m.ProviderName, globalOptions), nil
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

// updateProviderBundle resolves which bundle the provider needs, unpacks it unless that digest is
// already on disk, loads its schemas and returns its directory and digest.
func updateProviderBundle(
	ctx context.Context,
	provider string,
	lookup providerModuleLookup,
	registry bundleRegistryGetter,
	globalOptions *options.GlobalOptions,
) (string, string, error) {
	ref, err := resolveProviderBundleRef(ctx, provider, lookup, globalOptions)
	if err != nil {
		return "", "", err
	}

	dir, err := unpackProviderBundle(ctx, provider, ref, registry, globalOptions)
	if err != nil {
		return "", "", err
	}

	return dir, ref.Digest, nil
}

func unpackProviderBundle(ctx context.Context, provider string, ref providerBundleRef, registry bundleRegistryGetter, globalOptions *options.GlobalOptions) (string, error) {
	dir, err := image.EnsureUnpacked(ctx, image.UnpackRequest{
		Root:   globalOptions.DownloadDir,
		Name:   provider,
		Digest: ref.Digest,
		Registry: func(ctx context.Context) (*image.RegistryConfig, error) {
			// The bundle lives in the ModuleSource's registry, not the one resolved for deckhouse.
			if ref.Registry != nil {
				return ref.Registry, nil
			}
			return registry(ctx)
		},
	}, image.EnsureUnpackedOptions{
		// The module chain knows the repository. The in-tree path pins the flat images repo.
		Image:        ref.Image,
		ShowProgress: globalOptions.ShowProgress,
		Download: func(ctx context.Context, imageRef, destDir string, registryConfig image.RegistryConfig, showProgress bool) error {
			if err := downloadProviderBundle(ctx, imageRef, destDir, registryConfig, showProgress); err != nil {
				// Runs before every preflight check, so this message is all the operator gets.
				return fmt.Errorf("%w\n"+
					"It carries the provider schemas and the validator, so the configuration cannot be checked without it. "+
					"Make that reference reachable with the registry credentials from the configuration, "+
					"or unpack the bundle into %s yourself before dhctl starts.",
					err, providerdir.DigestDir(globalOptions.DownloadDir, provider, ref.Digest))
			}
			return nil
		},
		Prepare: func(_ context.Context, stagingDir, finalDir string) error {
			// Named by the final dir too: the staging dir is removed before anyone reads this.
			if err := checkBundleSchema(stagingDir); err != nil {
				return fmt.Errorf("check provider bundle %s: %w", finalDir, err)
			}
			return nil
		},
	})
	if err != nil {
		return "", fmt.Errorf("unpack provider bundle for %q: %w", provider, err)
	}

	if err := loadBundleSchemas(provider, ref.Digest, dir, globalOptions); err != nil {
		return "", err
	}

	return dir, nil
}

// loadBundleSchemas loads the schemas of an unpacked bundle into this process.
func loadBundleSchemas(provider, digest, dir string, globalOptions *options.GlobalOptions) error {
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("provider bundle %s is missing: %w", dir, err)
	}
	if err := checkBundleSchema(dir); err != nil {
		return err
	}

	if err := NewSchemaStore(globalOptions).LoadProviderDir(provider, digest, dir); err != nil {
		return fmt.Errorf("load schemas of provider bundle %s: %w", dir, err)
	}
	return nil
}

// errBundleWithoutSchema marks a bundle that was pulled but has nothing to validate against, so no
// registry access would help.
var errBundleWithoutSchema = errors.New("provider bundle has no cluster configuration schema")

func checkBundleSchema(dir string) error {
	if _, err := os.Stat(providerdir.SchemaPath(dir)); err != nil {
		return fmt.Errorf("check cluster configuration schema in %s: %w: %w", dir, errBundleWithoutSchema, err)
	}
	return nil
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
