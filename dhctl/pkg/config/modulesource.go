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
	"fmt"
	"path"
	"strings"

	"github.com/iancoleman/strcase"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/image"
)

const (
	ModuleSourceKind       = "ModuleSource"
	ModulePullOverrideKind = "ModulePullOverride"

	terraformManagerImageName = "terraformManager"

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

// A bare ModuleConfig means external only for a module absent from this image, because a bare
// ModuleConfig is also the only legal shape for a module that is present: the admission webhook
// rejects a spec.source outside the Module's availableSources, and an embedded module has none.
// Anchoring on images_digests.json instead would ignore the ModuleConfig for the whole migration
// window, since a still-shipped module keeps its section there. See
// dhctl/CLOUD_PROVIDER_MODULE_MIGRATION.md.
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

// Resolves through the module chain: release image gives the version, module image gives
// images_digests.json, its terraformManager entry gives the bundle. Once found is true a registry
// failure must NOT fall back to the embedded digests - validating a configuration against this
// installer's stale provider schema is worse than refusing to run.
func resolveModuleProviderBundle(ctx context.Context, provider string, lookup providerModuleLookup, globalOptions *options.GlobalOptions) (providerBundleRef, bool, error) {
	if lookup == nil {
		return providerBundleRef{}, false, nil
	}

	md, err := lookup(ctx)
	if err != nil {
		return providerBundleRef{}, false, err
	}

	if !md.providerIsExternal(provider, globalOptions) {
		return providerBundleRef{}, false, nil
	}

	moduleName := CloudProviderModuleName(provider)

	repo, conf, err := md.moduleRepo(moduleName, md.providerSource(moduleName))
	if err != nil {
		return providerBundleRef{}, true, err
	}
	moduleRepo := path.Join(repo, moduleName)

	// The controller pulls exactly <repo>/<module>:<imageTag> for an override: no release image,
	// no version.json, no "v" prefix.
	moduleTag := md.ImageTags[moduleName]
	if moduleTag == "" {
		tag := md.releaseChannelTag()
		releaseClient, err := moduleRegistryClient(conf, path.Join(moduleRepo, "release"))
		if err != nil {
			return providerBundleRef{}, true, fmt.Errorf("registry client for %s/release: %w", moduleRepo, err)
		}
		release, err := cr.ResolveChannel(ctx, releaseClient, tag)
		if err != nil {
			return providerBundleRef{}, true, fmt.Errorf("resolve %s/release:%s: %w%s", moduleRepo, tag, err, guessedRepoHint(provider, md.providerSource(moduleName)))
		}
		moduleTag = cr.ModuleImageTag(release.Version)
	}

	moduleClient, err := moduleRegistryClient(conf, moduleRepo)
	if err != nil {
		return providerBundleRef{}, true, fmt.Errorf("registry client for %s: %w", moduleRepo, err)
	}
	imageDigests, err := cr.ImagesDigests(ctx, moduleClient, moduleTag)
	if err != nil {
		return providerBundleRef{}, true, fmt.Errorf("read images digests of %s:%s: %w", moduleRepo, moduleTag, err)
	}

	// Nothing else records which build the bundle came from, and properties.version only
	// refreshes on a deckhouse restart, so converge can legitimately resolve the previous one.
	dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf("Provider bundle resolved from module %s:%s", moduleRepo, moduleTag))

	digest := imageDigests[terraformManagerImageName]
	if digest == "" {
		return providerBundleRef{}, true, fmt.Errorf("module %s:%s ships no %q image digest, so there is no provider bundle to unpack", moduleRepo, moduleTag, terraformManagerImageName)
	}

	return providerBundleRef{
		Image:    moduleRepo + "@" + digest,
		Digest:   digest,
		Registry: conf,
	}, true, nil
}

// Nothing in an installer distinguishes "this edition does not carry the provider" from "the
// provider is published outside this repository", so a 404 gets both readings. A named source
// needs none - the operator wrote the address that failed.
func guessedRepoHint(provider, sourceName string) string {
	if sourceName != "" {
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

// A var so tests can drive the chain without a registry.
var moduleRegistryClient = func(conf *image.RegistryConfig, repo string) (cr.Client, error) {
	return cr.NewClient(repo,
		cr.WithUserPasswordAuth(conf.GetUsername(), conf.GetPassword()),
		cr.WithCA(conf.GetCA()),
		cr.WithInsecureSchema(strings.EqualFold(conf.GetScheme(), "HTTP")),
	)
}
