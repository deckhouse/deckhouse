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
	"strings"

	"github.com/Masterminds/semver/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/registrydata"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/image"
)

// The kube client is taken lazily, so the dial happens where the lookup is consulted. inCluster
// is the caller's own location and only steers useUpstreamRegistry.
func clusterModuleDocs(kubeClient KubeClientGetter, provider string, inCluster bool) providerModuleLookup {
	return func(ctx context.Context) (*ModuleDocs, error) {
		kubeCl, err := kubeClient(ctx)
		if err != nil {
			return nil, fmt.Errorf("get kube client to resolve the provider module: %w", err)
		}
		return moduleDocsFromCluster(ctx, kubeCl, provider, inCluster)
	}
}

// No object, no CRD, or an RBAC that does not let us look: all three mean the cluster has nothing
// to say and leave this installer's digests as the answer, which is where an un-upgraded
// terraform-manager ClusterRole lands. A transport failure is a question unanswered, not an answer.
func clusterSaysNothing(err error) bool {
	return apierrors.IsNotFound(err) || meta.IsNoMatchError(err) ||
		apierrors.IsForbidden(err) || apierrors.IsUnauthorized(err)
}

// Answers from the Module object, never from a ModuleConfig. While the module ships inside the
// deckhouse image the controller rewrites properties.source back to the Embedded sentinel on every
// moduleloader start and stages ModuleReleases without activating them, so Embedded is the truth
// about what the cluster runs, not a stale answer. It stops being forced in the same build that
// drops the chart - the build the bootstrap path reads too, so the two stay in step.
//
// The modules directory the bootstrap path reads says nothing here: an out-of-cluster caller holds
// the installer's own tree, not the cluster's.
func moduleDocsFromCluster(ctx context.Context, kubeCl *client.KubernetesClient, provider string, inCluster bool) (*ModuleDocs, error) {
	moduleName := CloudProviderModuleName(provider)
	md := &ModuleDocs{ImageTags: map[string]string{}, ProviderConfigs: map[string]*ModuleConfig{}}

	module, err := kubeCl.Dynamic().Resource(ModuleGVR).Get(ctx, moduleName, metav1.GetOptions{})
	if err != nil {
		if clusterSaysNothing(err) {
			return md, nil
		}

		return nil, fmt.Errorf("get Module %q: %w", moduleName, err)
	}

	source, _, err := unstructured.NestedString(module.Object, "properties", "source")
	if err != nil {
		return nil, fmt.Errorf("read Module %q properties.source: %w", moduleName, err)
	}

	if source == "" || source == moduleSourceEmbedded {
		return md, nil
	}

	src, err := clusterModuleSource(ctx, kubeCl, source, moduleName, inCluster)
	if err != nil {
		return nil, err
	}

	if src == nil {
		return md, nil
	}

	md.Sources = []*ModuleSource{src}

	md.ProviderConfigs[moduleName] = &ModuleConfig{
		ObjectMeta: metav1.ObjectMeta{Name: moduleName},
		Spec:       ModuleConfigSpec{Source: source},
	}

	tag, err := clusterModuleTag(ctx, kubeCl, module, moduleName)
	if err != nil {
		return nil, err
	}

	if tag != "" {
		md.ImageTags[moduleName] = tag

		return md, nil
	}

	// Known but not yet installed from a release; its channel is the chain bootstrap walks.
	md.ReleaseChannel, _, err = unstructured.NestedString(module.Object, "properties", "releaseChannel")
	if err != nil {
		return nil, fmt.Errorf("read Module %q properties.releaseChannel: %w", moduleName, err)
	}

	if md.ReleaseChannel == "" {
		// Guessing Stable here would validate the configuration against schemas from a release
		// this cluster is not on.
		return nil, fmt.Errorf("module %q was downloaded from ModuleSource %q but reports neither properties.version nor properties.releaseChannel, and no ModulePullOverride pins it: there is no way to tell which provider bundle this cluster runs", moduleName, source)
	}

	return md, nil
}

// A nil source with no error means the cluster has nothing to say: a Module keeps
// properties.source after the object is deleted, a state the controller itself tolerates.
func clusterModuleSource(ctx context.Context, kubeCl *client.KubernetesClient, source, moduleName string, inCluster bool) (*ModuleSource, error) {
	ms, err := kubeCl.Dynamic().Resource(ModuleSourceGVR).Get(ctx, source, metav1.GetOptions{})
	if err != nil {
		if clusterSaysNothing(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("get ModuleSource %q the module %q was downloaded from: %w", source, moduleName, err)
	}

	// Only the spec: the converter cannot handle the unexported conf field on ModuleSource.
	spec, _, err := unstructured.NestedMap(ms.Object, "spec")
	if err != nil {
		return nil, fmt.Errorf("read ModuleSource %q spec: %w", source, err)
	}

	src := new(ModuleSource)
	src.SetName(source)

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(spec, &src.Spec); err != nil {
		return nil, fmt.Errorf("decode ModuleSource %q spec: %w", source, err)
	}

	if err := useUpstreamRegistry(ctx, kubeCl, src, inCluster); err != nil {
		return nil, err
	}

	return src, nil
}

// An override outranks properties.version: while one exists the controller keeps pulling its tag.
func clusterModuleTag(ctx context.Context, kubeCl *client.KubernetesClient, module *unstructured.Unstructured, moduleName string) (string, error) {
	tag, err := clusterPullOverrideTag(ctx, kubeCl, moduleName)
	if err != nil {
		return "", err
	}

	if tag != "" {
		return tag, nil
	}

	version, _, err := unstructured.NestedString(module.Object, "properties", "version")
	if err != nil {
		return "", fmt.Errorf("read Module %q properties.version: %w", moduleName, err)
	}

	return moduleVersionTag(version), nil
}

// A module restored from a ModulePullOverride carries that override's raw imageTag in
// properties.version (moduleloader/sync.go), and deleting the override never resets it. Gluing
// "v" onto "mr1" addresses an image nobody pushed, so only semver gets the prefix.
func moduleVersionTag(version string) string {
	if version == "" {
		return ""
	}
	if _, err := semver.NewVersion(version); err != nil {
		return version
	}
	return cr.ModuleImageTag(version)
}

// registry.d8-system.svc only resolves inside the cluster, yet the built-in "deckhouse"
// ModuleSource is templated from exactly that address in Direct/Proxy mode. The upstream registry
// serves the same content at a reachable host under the same path. Mirrors the preference
// GetRegistryDataPreferUpstream applies to the candi download.
func useUpstreamRegistry(ctx context.Context, kubeCl *client.KubernetesClient, src *ModuleSource, inCluster bool) error {
	repo := src.Spec.Registry.Repo
	if inCluster || !strings.HasPrefix(repo, registry_const.Host) {
		return nil
	}

	upstream, found, err := registrydata.GetUpstreamRegistryData(ctx, kubeCl)
	if err != nil {
		return fmt.Errorf("get upstream registry for ModuleSource %q: %w", src.GetName(), err)
	}
	if !found {
		// Local mode publishes nothing upstream; failing on the pull names the host better.
		return nil
	}

	repo = upstream.GetRegistry() + strings.TrimPrefix(strings.TrimPrefix(repo, registry_const.HostWithPath), registry_const.Host)
	conf, err := image.NewRegistryConfig(upstream.GetScheme(), repo, upstream.GetUsername(), upstream.GetPassword(), upstream.GetCA())
	if err != nil {
		return fmt.Errorf("upstream registry config for ModuleSource %q: %w", src.GetName(), err)
	}
	src.Spec.Registry.Repo = repo
	src.conf = conf
	return nil
}

// An override the controller is not following (deleted, or not Ready because its tag does not
// resolve) reads as empty: the cluster keeps running the version it had.
func clusterPullOverrideTag(ctx context.Context, kubeCl *client.KubernetesClient, moduleName string) (string, error) {
	mpo, err := kubeCl.Dynamic().Resource(ModulePullOverrideGVR).Get(ctx, moduleName, metav1.GetOptions{})
	if err != nil {
		if clusterSaysNothing(err) {
			return "", nil
		}
		return "", fmt.Errorf("get ModulePullOverride %q: %w", moduleName, err)
	}
	if mpo.GetDeletionTimestamp() != nil {
		return "", nil
	}
	message, _, err := unstructured.NestedString(mpo.Object, "status", "message")
	if err != nil {
		return "", fmt.Errorf("read ModulePullOverride %q status.message: %w", moduleName, err)
	}
	if message != modulePullOverrideReady {
		return "", nil
	}
	tag, _, err := unstructured.NestedString(mpo.Object, "spec", "imageTag")
	if err != nil {
		return "", fmt.Errorf("read ModulePullOverride %q spec.imageTag: %w", moduleName, err)
	}
	return tag, nil
}
