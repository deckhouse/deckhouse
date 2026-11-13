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

package registry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// embeddedSource is the Module source of modules shipped inside the Deckhouse
// image. Their documentation comes with Deckhouse, not from a registry.
const embeddedSource = "Embedded"

var (
	moduleGVR       = schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1alpha1", Resource: "modules"}
	moduleSourceGVR = schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1alpha1", Resource: "modulesources"}
	overrideGVR     = schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1alpha2", Resource: "modulepulloverrides"}
	releaseGVR      = schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1alpha1", Resource: "modulereleases"}
)

// enabledCondition is the Module condition set when the module is enabled.
const enabledCondition = "EnabledByModuleManager"

// ErrNoSource means the module is not available from any registry source.
var ErrNoSource = errors.New("module has no registry source")

// errBadSource means a ModuleSource has no usable registry settings.
var errBadSource = errors.New("module source has no registry")

// Source is the registry of a ModuleSource (.spec.registry).
type Source struct {
	Name      string
	Repo      string
	Scheme    string // HTTP or HTTPS, HTTPS when empty
	CA        string // PEM
	DockerCfg string // docker config JSON, base64-encoded or as it is
	// Login and Password of a PackageRepository are taken over DockerCfg.
	// Without a login and a docker config the access is anonymous.
	Login    string
	Password string
	// Fallback, when set, is read where this registry fails: the upstream
	// behind the store of the registry module (reachable).
	Fallback *Source
}

// Sources finds the registries a module can be fetched from.
type Sources struct {
	client dynamic.Interface
	logger *log.Logger

	// The ModulePullOverride tags by module and the error of their last
	// list, kept for overrideTTL.
	mu          sync.Mutex
	overrides   map[string]string
	overrideErr error
	overridesAt time.Time

	// What the registry module says of the in-cluster registry and the
	// error of its last read, kept for pullPathTTL.
	pathMu  sync.Mutex
	path    pullPath
	pathErr error
	pathAt  time.Time
}

// overrideTTL keeps the tags of pull overrides for the renders of the next
// minute: a pod start restores every pinned version and asks for each.
// overrideTimeout bounds one list, so a slow apiserver does not hold them up.
const (
	overrideTTL     = time.Minute
	overrideTimeout = 30 * time.Second
)

func NewSources(client dynamic.Interface, logger *log.Logger) *Sources {
	return &Sources{client: client, logger: logger}
}

// ForModule returns the sources of the module in the order to try them: the
// source the module is installed from, then the other available sources. A
// requested version may exist in only one of them. A source that can not be
// read, e.g. a ModuleSource removed since, is skipped with a warning; with no
// source left the error wraps ErrNoSource. A source on the in-cluster registry
// of the registry module is read where the node agent would fetch it from
// instead (reachable).
func (s *Sources) ForModule(ctx context.Context, module string) ([]Source, error) {
	obj, err := s.client.Resource(moduleGVR).Get(ctx, module, metav1.GetOptions{})
	// A module of the package repositories has no Module with a source.
	if apierrors.IsNotFound(err) {
		return s.packageSources(ctx, module)
	}
	if err != nil {
		return nil, fmt.Errorf("get module %s: %w", module, err)
	}

	if runsEmbedded(obj) {
		// The versions of its sources can not be installed while Deckhouse
		// carries the module: none is offered.
		return nil, fmt.Errorf("module %s runs its embedded copy: %w", module, ErrNoSource)
	}
	names, err := sourceNames(obj)
	if errors.Is(err, ErrNoSource) {
		return s.packageSources(ctx, module)
	}
	if err != nil {
		return nil, fmt.Errorf("module %s: %w", module, err)
	}

	sources := make([]Source, 0, len(names))
	// gone are the sources removed or without a registry; unanswered, those
	// the API server did not answer for, which may be there.
	var gone, unanswered []error
	for _, name := range names {
		src, err := s.source(ctx, name)
		if err != nil {
			s.logger.Warn("skip module source", slog.String("module", module), slog.String("module_source", name), log.Err(err))
			if apierrors.IsNotFound(err) || errors.Is(err, errBadSource) {
				gone = append(gone, err)
			} else {
				unanswered = append(unanswered, err)
			}
			continue
		}

		sources = append(sources, src)
	}

	if len(sources) == 0 {
		// Only the unanswered errors: a NotFound among them would make the
		// module look gone.
		if len(unanswered) > 0 {
			return nil, fmt.Errorf("module %s: %w", module, errors.Join(unanswered...))
		}
		return nil, fmt.Errorf("module %s: %w: %w", module, ErrNoSource, errors.Join(gone...))
	}

	return sources, nil
}

func (s *Sources) source(ctx context.Context, name string) (Source, error) {
	ms, err := s.client.Resource(moduleSourceGVR).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return Source{}, fmt.Errorf("get module source %s: %w", name, err)
	}

	src, err := sourceFrom(ms)
	if err != nil {
		return Source{}, fmt.Errorf("module source %s: %w: %w", name, errBadSource, err)
	}

	src, err = s.reachable(ctx, src)
	if err != nil {
		return Source{}, fmt.Errorf("module source %s: %w", name, err)
	}

	return src, nil
}

// Module is a module available from a registry source, as the cluster sees it.
type Module struct {
	Name    string
	Enabled bool
	// Version installed in the cluster (properties.version); a development
	// branch for a module pulled by a ModulePullOverride, empty when the module
	// was never downloaded.
	Version string
	// Override is the development image an enabled module runs, if any.
	Override *Override
	// Stage is the lifecycle stage of the module (properties.stage), empty
	// when the cluster does not know it.
	Stage string
}

// Override is the image of a ModulePullOverride: the full module image tagged
// Tag, with Digest changing on every push to the branch.
type Override struct {
	Tag    string
	Digest string
}

// Modules lists the modules that have a registry source, i.e. all but the
// embedded ones: those of the ModuleSources and those of the package
// repositories. A module that runs its embedded copy is embedded, whatever
// sources also offer it: its documentation comes with Deckhouse.
func (s *Sources) Modules(ctx context.Context) ([]Module, error) {
	list, err := s.client.Resource(moduleGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list modules: %w", err)
	}

	overrides, err := s.client.Resource(overrideGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list module pull overrides: %w", err)
	}

	byName := make(map[string]*Override, len(overrides.Items))
	for _, o := range overrides.Items {
		tag, _, _ := unstructured.NestedString(o.Object, "spec", "imageTag")
		digest, _, _ := unstructured.NestedString(o.Object, "status", "imageDigest")
		if tag != "" {
			byName[o.GetName()] = &Override{Tag: tag, Digest: digest}
		}
	}

	modules := make([]Module, 0, len(list.Items))
	embedded := make(map[string]bool)
	for i := range list.Items {
		obj := &list.Items[i]
		if runsEmbedded(obj) {
			embedded[obj.GetName()] = true
			continue
		}
		if names, err := sourceNames(obj); err != nil || len(names) == 0 {
			continue
		}

		version, _, _ := unstructured.NestedString(obj.Object, "properties", "version")
		stage, _, _ := unstructured.NestedString(obj.Object, "properties", "stage")
		m := Module{Name: obj.GetName(), Enabled: isEnabled(obj), Version: version, Stage: stage}
		if m.Enabled {
			m.Override = byName[m.Name]
		}
		modules = append(modules, m)
	}

	packages, err := s.packageModules(ctx)
	if err != nil {
		return nil, err
	}
	for _, m := range packages {
		if !embedded[m.Name] && !slices.ContainsFunc(modules, func(o Module) bool { return o.Name == m.Name }) {
			modules = append(modules, m)
		}
	}

	return modules, nil
}

// OverrideTag returns the image tag a ModulePullOverride pulls the module
// with, "" when there is none; the module may be disabled. It lists the
// overrides, as the module may only list them, and keeps the answer, an
// error too, for overrideTTL. The error of a caller that gave up, such as a
// leader that lost its Lease, says nothing of the cluster and is not kept:
// the callers of the next minute would take every version for a branch.
func (s *Sources) OverrideTag(ctx context.Context, module string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.overridesAt.IsZero() || time.Since(s.overridesAt) >= overrideTTL {
		overrides, err := s.listOverrideTags(ctx)
		if err != nil && ctx.Err() != nil {
			return "", err
		}
		s.overrides, s.overrideErr, s.overridesAt = overrides, err, time.Now()
	}

	return s.overrides[module], s.overrideErr
}

func (s *Sources) listOverrideTags(ctx context.Context) (map[string]string, error) {
	ctx, cancel := context.WithTimeout(ctx, overrideTimeout)
	defer cancel()

	list, err := s.client.Resource(overrideGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list module pull overrides: %w", err)
	}

	tags := make(map[string]string, len(list.Items))
	for _, o := range list.Items {
		tags[o.GetName()], _, _ = unstructured.NestedString(o.Object, "spec", "imageTag")
	}

	return tags, nil
}

// Release is a version of a module the cluster is about to update to.
type Release struct {
	Module  string
	Version string
}

// PendingReleases lists the ModuleReleases waiting to be deployed: the
// versions whose documentation an administrator reads before the update.
func (s *Sources) PendingReleases(ctx context.Context) ([]Release, error) {
	list, err := s.client.Resource(releaseGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list module releases: %w", err)
	}

	var releases []Release
	for _, obj := range list.Items {
		phase, _, _ := unstructured.NestedString(obj.Object, "status", "phase")
		module, _, _ := unstructured.NestedString(obj.Object, "spec", "moduleName")
		version, _, _ := unstructured.NestedString(obj.Object, "spec", "version")
		if phase != "Pending" || module == "" || version == "" {
			continue
		}
		// spec.version goes without the "v" of the image tags.
		if !strings.HasPrefix(version, "v") {
			version = "v" + version
		}
		releases = append(releases, Release{Module: module, Version: version})
	}

	return releases, nil
}

func isEnabled(module *unstructured.Unstructured) bool {
	conditions, _, _ := unstructured.NestedSlice(module.Object, "status", "conditions")
	for _, c := range conditions {
		cond, ok := c.(map[string]any)
		if ok && cond["type"] == enabledCondition && cond["status"] == "True" {
			return true
		}
	}

	return false
}

// runsEmbedded reports a module Deckhouse runs from its own image
// (properties.source Embedded). While the embedded copy is there, it runs
// even when a source offers the module too, e.g. a module moving out of
// Deckhouse into a source.
func runsEmbedded(module *unstructured.Unstructured) bool {
	source, _, _ := unstructured.NestedString(module.Object, "properties", "source")
	return source == embeddedSource
}

func sourceNames(module *unstructured.Unstructured) ([]string, error) {
	current, _, err := unstructured.NestedString(module.Object, "properties", "source")
	if err != nil {
		return nil, fmt.Errorf("read properties.source: %w", err)
	}

	available, _, err := unstructured.NestedStringSlice(module.Object, "properties", "availableSources")
	if err != nil {
		return nil, fmt.Errorf("read properties.availableSources: %w", err)
	}

	names := make([]string, 0, len(available)+1)
	for _, name := range append([]string{current}, available...) {
		if name == "" || name == embeddedSource || slices.Contains(names, name) {
			continue
		}

		names = append(names, name)
	}

	if len(names) == 0 {
		return nil, ErrNoSource
	}

	return names, nil
}

func sourceFrom(ms *unstructured.Unstructured) (Source, error) {
	registry, found, err := unstructured.NestedStringMap(ms.Object, "spec", "registry")
	if err != nil {
		return Source{}, fmt.Errorf("read spec.registry: %w", err)
	}

	if !found || registry["repo"] == "" {
		return Source{}, errors.New("spec.registry.repo is empty")
	}

	return Source{
		Name:      ms.GetName(),
		Repo:      registry["repo"],
		Scheme:    registry["scheme"],
		CA:        registry["ca"],
		DockerCfg: registry["dockerCfg"],
	}, nil
}
