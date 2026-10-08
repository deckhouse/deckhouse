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
	"path"
	"path/filepath"
	"strings"
	"time"

	crv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/iancoleman/strcase"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/module"
	"github.com/deckhouse/deckhouse/pkg/deckhouse-registry/service"
	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/deckhouse/deckhouse/pkg/registry"
	"github.com/deckhouse/deckhouse/pkg/registry/client"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config/digests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/providerdir"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/image"
)

const (
	ModuleSourceKind       = "ModuleSource"
	ModulePullOverrideKind = "ModulePullOverride"

	// The controller's built-in update policy, applied when no ModuleUpdatePolicy exists.
	defaultModuleReleaseChannel = "stable"

	modulesRepoSuffix = "modules"

	// The ModuleSource helm creates in every cluster. It cannot be in config.yml, so a
	// ModuleConfig naming it falls through to the default repository it points at.
	defaultModuleSourceName = "deckhouse"

	// The controller rewrites properties.source to this on every moduleloader start for a module
	// shipped inside the deckhouse image, despite the CRD promising the field "will be blank".
	moduleSourceEmbedded = "Embedded"

	// v1alpha1.ModulePullOverrideMessageReady: the controller is actually following the override.
	modulePullOverrideReady = "Ready"
)

// ModulePullOverride is addressed as v1alpha2 because v1alpha1 is no longer served. A cluster
// predating v1alpha2 answers NoMatch, which reads here as "no override".
var (
	ModuleGVR             = schema.GroupVersionResource{Group: ModuleConfigGroup, Version: "v1alpha1", Resource: "modules"}
	ModuleSourceGVR       = schema.GroupVersionResource{Group: ModuleConfigGroup, Version: "v1alpha1", Resource: "modulesources"}
	ModulePullOverrideGVR = schema.GroupVersionResource{Group: ModuleConfigGroup, Version: "v1alpha2", Resource: "modulepulloverrides"}
)

// ModuleSource is a local copy of deckhouse.io/v1alpha1 ModuleSource carrying only the registry
// access. Importing the controller's type would pull the repository root module into dhctl.
type ModuleSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec ModuleSourceSpec `json:"spec"`

	// Overrides spec.registry when the object points at the in-cluster mirror. See useUpstreamRegistry.
	conf *image.RegistryConfig
}

type ModuleSourceSpec struct {
	Registry ModuleSourceRegistry `json:"registry"`
}

type ModuleSourceRegistry struct {
	Repo      string `json:"repo"`
	DockerCfg string `json:"dockerCfg,omitempty"`
	Scheme    string `json:"scheme,omitempty"`
	CA        string `json:"ca,omitempty"`
}

// ModuleDocs is where the provider module comes from and which image of it to read, filled
// either from the raw configuration documents or from the cluster's Module objects.
type ModuleDocs struct {
	Sources []*ModuleSource

	// Keyed by module name: a configuration can carry several cloud-provider ModuleConfigs, and
	// merging them would graft one provider's spec.source onto another's module.
	ProviderConfigs map[string]*ModuleConfig

	// Tags addressing the module image directly, with no release image in between.
	ImageTags map[string]string

	ReleaseChannel string

	// Empty when the cluster answered instead, where a ModuleSource object resolves the repository.
	docs []string
}

func ParseModuleDocs(docs []string) (*ModuleDocs, error) {
	md := &ModuleDocs{ImageTags: map[string]string{}, ProviderConfigs: map[string]*ModuleConfig{}, docs: docs}

	for _, doc := range docs {
		if strings.TrimSpace(doc) == "" {
			continue
		}

		var obj unstructured.Unstructured
		if err := yaml.Unmarshal([]byte(doc), &obj); err != nil {
			// parseDocument sees the same documents right after and reports this with a
			// numbered line dump.
			continue
		}

		if err := md.readDocument(doc, &obj); err != nil {
			return nil, err
		}
	}

	return md, nil
}

func (md *ModuleDocs) readDocument(doc string, obj *unstructured.Unstructured) error {
	switch obj.GetKind() {
	case ModuleSourceKind:
		source := new(ModuleSource)
		if err := yaml.Unmarshal([]byte(doc), source); err != nil {
			return fmt.Errorf("unmarshal ModuleSource %q: %w", obj.GetName(), err)
		}

		md.Sources = append(md.Sources, source)

	case ModulePullOverrideKind:
		tag, _, err := unstructured.NestedString(obj.Object, "spec", "imageTag")
		if err != nil {
			return fmt.Errorf("read ModulePullOverride %q spec.imageTag: %w", obj.GetName(), err)
		}

		if tag != "" {
			md.ImageTags[obj.GetName()] = tag
		}

	case ModuleConfigKind:
		return md.readModuleConfig(doc, obj)
	}

	return nil
}

func (md *ModuleDocs) readModuleConfig(doc string, obj *unstructured.Unstructured) error {
	if obj.GetName() == "deckhouse" {
		channel, _, err := unstructured.NestedString(obj.Object, "spec", "settings", "releaseChannel")
		if err != nil {
			return fmt.Errorf("read deckhouse ModuleConfig settings.releaseChannel: %w", err)
		}

		md.ReleaseChannel = channel

		return nil
	}

	if !IsCloudProviderModuleName(obj.GetName()) {
		return nil
	}

	mc := new(ModuleConfig)
	if err := yaml.Unmarshal([]byte(doc), mc); err != nil {
		return fmt.Errorf("unmarshal ModuleConfig %q: %w", obj.GetName(), err)
	}

	// A base "enabled: true" entry plus a settings overlay is a legal shape, and only one of
	// them names the source, so last-wins would drop it.
	if prev := md.ProviderConfigs[obj.GetName()]; prev != nil && prev.Spec.Source != "" && mc.Spec.Source == "" {
		mc.Spec.Source = prev.Spec.Source
	}

	md.ProviderConfigs[obj.GetName()] = mc

	return nil
}

func (md *ModuleDocs) source(name string) *ModuleSource {
	for _, s := range md.Sources {
		if s.GetName() == name {
			return s
		}
	}
	return nil
}

// Release images are tagged in kebab-case.
func (md *ModuleDocs) releaseChannelTag() string {
	if md.ReleaseChannel != "" {
		return strcase.ToKebab(md.ReleaseChannel)
	}
	return defaultModuleReleaseChannel
}

func (md *ModuleDocs) providerSource(moduleName string) string {
	mc := md.ProviderConfigs[moduleName]
	if mc == nil {
		return ""
	}
	return mc.Spec.Source
}

// Both opt-ins count even for a module this installer still ships. The cluster ignores an
// override on an embedded module (the override controller answers ModuleEmbedded), but which
// bundle dhctl unpacks is dhctl's own decision.
func (md *ModuleDocs) providerModulePinned(moduleName string) bool {
	return md.providerSource(moduleName) != "" || md.ImageTags[moduleName] != ""
}

// A bare ModuleConfig means external only for a module absent from this image: the admission
// webhook rejects a spec.source outside the Module's availableSources, and an embedded module has
// none. See dhctl/CLOUD_PROVIDER_MODULE_MIGRATION.md.
func (md *ModuleDocs) providerIsExternal(provider string, globalOptions *options.GlobalOptions) bool {
	moduleName := CloudProviderModuleName(provider)

	if md.providerModulePinned(moduleName) {
		return true
	}

	return md.ProviderConfigs[moduleName] != nil && !moduleShippedInImage(moduleName, globalOptions)
}

// An empty dockerCfg is anonymous access, not an error - the CRD says so explicitly.
func (s *ModuleSource) RegistryConfig() (*image.RegistryConfig, error) {
	if s.conf != nil {
		return s.conf, nil
	}

	reg := s.Spec.Registry
	if reg.Repo == "" {
		return nil, fmt.Errorf("ModuleSource %q has no spec.registry.repo", s.GetName())
	}

	// The CRD's default; these documents never went through the schema store.
	scheme := reg.Scheme
	if scheme == "" {
		scheme = "HTTPS"
	}

	if reg.DockerCfg == "" {
		conf, err := image.NewRegistryConfig(scheme, reg.Repo, "", "", reg.CA)
		if err != nil {
			return nil, fmt.Errorf("ModuleSource %q registry config: %w", s.GetName(), err)
		}
		return conf, nil
	}

	dc, err := image.DecodeDockerConfig(reg.DockerCfg)
	if err != nil {
		return nil, fmt.Errorf("ModuleSource %q dockerCfg: %w", s.GetName(), err)
	}

	conf, err := image.RegistryConfigFromDockerConfig(dc, scheme, reg.Repo)
	if err != nil {
		return nil, fmt.Errorf("ModuleSource %q registry config: %w", s.GetName(), err)
	}

	conf.SetCA(reg.CA)

	return conf, nil
}

// providerBundleRef locates the terraform-manager bundle. An OCI digest only resolves inside its
// own repository, so an externally published bundle needs the full reference, not just the digest.
type providerBundleRef struct {
	// "<repo>/<module>@<digest>". Empty on the in-tree path, where the caller composes it.
	Image string

	// Always filled: everything cached on disk keys on it.
	Digest string

	// nil means "keep the caller's registry".
	Registry *image.RegistryConfig
}

// A function rather than a value because the cluster-side answer costs an API call. A nil lookup
// is the embedded-digest path.
type providerModuleLookup func(ctx context.Context) (*ModuleDocs, error)

func configModuleDocs(docs []string) providerModuleLookup {
	return func(context.Context) (*ModuleDocs, error) {
		return ParseModuleDocs(docs)
	}
}

// Walks the module chain: release image gives the version, module image gives images_digests.json,
// its terraformManager entry gives the bundle. A zero ref means the provider is not external. An
// error never falls back to the embedded digests: the installer's build is not the one this cluster runs.
func resolveModuleProviderBundle(ctx context.Context, provider string, lookup providerModuleLookup, globalOptions *options.GlobalOptions) (providerBundleRef, error) {
	if lookup == nil {
		return providerBundleRef{}, nil
	}

	md, err := lookup(ctx)
	if err != nil {
		return providerBundleRef{}, err
	}

	if !md.providerIsExternal(provider, globalOptions) {
		return providerBundleRef{}, nil
	}

	moduleName := CloudProviderModuleName(provider)

	catalogRepo, conf, err := md.moduleRepo(moduleName, md.providerSource(moduleName))
	if err != nil {
		return providerBundleRef{}, err
	}
	moduleRepo := path.Join(catalogRepo, moduleName)

	svc := moduleCatalog(conf, catalogRepo).Module(moduleName)

	moduleTag, err := md.moduleTag(ctx, svc, moduleName, moduleRepo, provider)
	if err != nil {
		return providerBundleRef{}, err
	}

	digest, err := moduleBundleDigest(ctx, svc, moduleName, moduleRepo, moduleTag, globalOptions)
	if err != nil {
		return providerBundleRef{}, err
	}

	// Nothing else records which build the bundle came from, and properties.version only
	// refreshes on a deckhouse restart, so converge can legitimately resolve the previous one.
	dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf("Provider bundle resolved from module %s:%s", moduleRepo, moduleTag))

	return providerBundleRef{
		Image:    moduleRepo + "@" + digest,
		Digest:   digest,
		Registry: conf,
	}, nil
}

const providerBundleDigestFile = "provider-bundle-digest"

// moduleBundleDigest reads which bundle the module image at tag names. The image itself is read
// only the first time its digest is seen: the answer is kept beside the unpacked images, keyed by
// the image digest, so a tag that was pushed over is read again.
func moduleBundleDigest(ctx context.Context, svc *module.Service, moduleName, moduleRepo, tag string, globalOptions *options.GlobalOptions) (string, error) {
	imageDigest, err := svc.GetDigest(ctx, tag)
	if err != nil {
		return "", fmt.Errorf("get digest of %s:%s: %w", moduleRepo, tag, err)
	}

	downloadDir := withDownloadDir(globalOptions).DownloadDir
	dir := providerdir.DigestDir(downloadDir, moduleName, imageDigest.String())

	if digest, recorded := recordedBundleDigest(ctx, dir); recorded {
		return digest, nil
	}

	// By digest, not by tag: the tag may have been pushed over since the digest request.
	bundle, err := svc.Fetch(ctx, imageDigest.String())
	if err != nil {
		return "", fmt.Errorf("read images digests of %s:%s: %w", moduleRepo, tag, err)
	}

	// A module image ships a flat images_digests.json, so the module selector stays empty.
	digest, _ := bundle.Digests().Lookup("", digests.TerraformManagerImage)
	if digest == "" {
		return "", fmt.Errorf("module %s:%s ships no %q image digest, so there is no provider bundle to unpack", moduleRepo, tag, digests.TerraformManagerImage)
	}

	if err := recordBundleDigest(dir, digest); err != nil {
		dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf("Record provider bundle digest of %s:%s: %v", moduleRepo, tag, err))
	}

	return digest, nil
}

// recordedBundleDigest reads the record. One that is missing, unreadable or not a digest is no record.
func recordedBundleDigest(ctx context.Context, dir string) (string, bool) {
	path := filepath.Join(dir, providerBundleDigestFile)
	recorded, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}

	digest := strings.TrimSpace(string(recorded))
	if _, err := crv1.NewHash(digest); err != nil {
		dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Ignore provider bundle digest record %s: %v", path, err))
		return "", false
	}

	return digest, true
}

// recordBundleDigest writes the record aside and renames it into place, so a reader running at the
// same time finds the whole record or none.
func recordBundleDigest(dir, digest string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create provider bundle digest record dir %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, providerBundleDigestFile+".*")
	if err != nil {
		return fmt.Errorf("create provider bundle digest record: %w", err)
	}
	if _, err := tmp.WriteString(digest); err != nil {
		return errors.Join(fmt.Errorf("write provider bundle digest record: %w", err), tmp.Close(), os.Remove(tmp.Name()))
	}
	if err := tmp.Close(); err != nil {
		return errors.Join(fmt.Errorf("close provider bundle digest record: %w", err), os.Remove(tmp.Name()))
	}
	if err := os.Rename(tmp.Name(), filepath.Join(dir, providerBundleDigestFile)); err != nil {
		return errors.Join(fmt.Errorf("move provider bundle digest record into place: %w", err), os.Remove(tmp.Name()))
	}

	return nil
}

// Which image of the module to read. The controller pulls exactly <repo>/<module>:<imageTag> for
// an override: no release image, no version.json, no "v" prefix. Without one the release channel
// names a release image, and its version.json names the module image.
func (md *ModuleDocs) moduleTag(ctx context.Context, svc *module.Service, moduleName, moduleRepo, provider string) (string, error) {
	if tag := md.ImageTags[moduleName]; tag != "" {
		return tag, nil
	}

	channel := md.releaseChannelTag()

	rel, err := svc.Releases().Fetch(ctx, channel)
	if err != nil {
		return "", fmt.Errorf("resolve %s/release:%s: %w%s", moduleRepo, channel, err, repoHint(err, provider, md.providerSource(moduleName)))
	}

	version, err := rel.Version()
	if err != nil {
		return "", fmt.Errorf("resolve %s/release:%s: %w", moduleRepo, channel, err)
	}

	return moduleImageTag(version), nil
}

// Nothing in an installer distinguishes "this edition does not carry the provider" from "the
// provider is published outside this repository", so a 404 gets both readings; a named source or a
// denial gets none. ErrRepositoryNotFound wraps ErrImageNotFound, so one errors.Is covers both.
func repoHint(err error, provider, sourceName string) string {
	if sourceName != "" || !errors.Is(err, registry.ErrImageNotFound) {
		return ""
	}

	return fmt.Sprintf("\nThis installer ships no %q module, so the cloud provider %q was looked up in the default module repository. "+
		"Either this Deckhouse edition does not include that provider, or the ModuleSource publishing it is missing from the configuration.",
		CloudProviderModuleName(provider), provider)
}

// A named but missing source is an error: falling back to the main registry would fetch a
// different module than the operator asked for.
func (md *ModuleDocs) moduleRepo(moduleName, sourceName string) (string, *image.RegistryConfig, error) {
	if source := md.source(sourceName); source != nil {
		conf, err := source.RegistryConfig()
		if err != nil {
			return "", nil, err
		}
		return source.Spec.Registry.Repo, conf, nil
	}
	if sourceName != "" && sourceName != defaultModuleSourceName {
		return "", nil, fmt.Errorf("ModuleConfig %q refers to ModuleSource %q, which is not in the configuration", moduleName, sourceName)
	}

	conf, err := buildRegistryConfig(md.docs)
	if err != nil {
		return "", nil, err
	}
	return path.Join(conf.GetRegistry(), modulesRepoSuffix), conf, nil
}

// The module image tag the controller builds from a release version: "v" plus the version. Not
// moduleVersionTag - that one guards a raw ModulePullOverride imageTag ("mr1") and must leave it
// alone, while a version read out of version.json is always prefixed.
func moduleImageTag(version string) string {
	if strings.HasPrefix(version, "v") {
		return version
	}

	return "v" + version
}

// Bounds one registry request: LoadConfigFromFile passes a context with no deadline, so a registry
// that stalls after accepting would hang dhctl at config load. A var so tests can shorten it.
var registryRequestTimeout = 120 * time.Second

// Splits "host/a/b" into "host" and "a/b"; a bare host yields an empty path.
func splitHostPath(repo string) (string, string) {
	host, rest, _ := strings.Cut(strings.Trim(repo, "/"), "/")

	return host, rest
}

// A var so tests can drive the chain without a registry. repo addresses the module catalog, whose
// tags are module names. log.Default() rather than the context logger on purpose: lib-dhctl holds
// that one at fatal level and opens it to debug, routed to the log file, under DHCTL_DEBUG.
var moduleCatalog = func(conf *image.RegistryConfig, repo string) *module.Catalog {
	host, rest := splitHostPath(repo)

	cli := registry.Client(client.New(host,
		client.WithLoginPassword(conf.GetUsername(), conf.GetPassword()),
		client.WithCA(conf.GetCA()),
		client.WithInsecure(strings.EqualFold(conf.GetScheme(), "HTTP")),
		client.WithTimeout(registryRequestTimeout),
		client.WithLogger(log.Default()),
	))
	if rest != "" {
		cli = cli.WithSegment(strings.Split(rest, "/")...)
	}

	return module.NewCatalog(service.NewBasicService(module.CatalogServiceName, cli, log.Default()))
}
