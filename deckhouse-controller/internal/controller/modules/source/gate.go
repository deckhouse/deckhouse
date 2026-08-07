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
	"log/slog"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/controller/modules/source/releases"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

// defaultModuleSourceName is the built-in OSS source that ships with Deckhouse. When a module is
// offered by it alongside mirrors such as the EE source, it is the canonical choice rather than an
// ambiguous one.
const defaultModuleSourceName = "deckhouse"

// releaseEnsureAllowed reports whether releases for the module may be ensured from this source at
// all. It is a pure policy predicate over already-fetched data - no cluster I/O - and does not
// decide whether there is anything to fetch; that diff is the caller's.
//
// The module config is the sole record of operator intent here. A module carries no resource of
// its own until a release deploys, so an absent config, or one leaving .spec.enabled unset, means
// "undecided" and hands the decision to the bundle.
//
// TODO: embedded modules are no longer pre-staged. The previous implementation resolved a target
// source for a module whose embedded copy is still shipped - the config's .spec.source, the only
// available source, or the canonical one - so its release was already on the filesystem when the
// copy was dropped on upgrade. That logic read v1alpha1.Module.Properties, which the bootstrap now
// deletes. Restore it over the package system when the release controller is ported.
func (r *reconciler) releaseEnsureAllowed(
	source *v1alpha1.ModuleSource,
	moduleName string,
	meta *releases.Metadata,
	config *v1alpha1.ModuleConfig,
	repositories []string,
) bool {
	if meta.Definition != nil && meta.Definition.IsExperimental() && !r.settings.ExperimentalModuleAllowed(moduleName) {
		r.logger.Debug("the experimental module is not allowed, skip the release ensure",
			slog.String("source_name", source.Name),
			slog.String("module_name", moduleName))

		return false
	}

	if chosen := configuredSource(config); chosen != "" && chosen != source.Name {
		r.logger.Debug("the module is configured to another source, skip it",
			slog.String("source_name", source.Name),
			slog.String("module_name", moduleName),
			slog.String("configured_source", chosen))

		return false
	}

	// an explicit intent overrides the bundle in both directions
	if config != nil && config.Spec.Enabled != nil {
		return *config.Spec.Enabled
	}

	if meta.Definition == nil {
		return false
	}

	running := r.manager.Edition()
	if !meta.Definition.Accessibility.IsEnabled(running.Name, running.Bundle) {
		return false
	}

	// several repositories offer the module and none was selected - leave the choice to the
	// operator rather than letting whichever source scans first claim it
	return len(repositories) <= 1 || source.Name == defaultModuleSourceName
}

// configuredSource returns the source the operator selected in the module config, or an empty
// string when there is no config, no selection, or the "Embedded" sentinel - which names the
// built-in copy rather than a real ModuleSource.
func configuredSource(config *v1alpha1.ModuleConfig) string {
	if config == nil || config.Spec.Source == v1alpha1.ModuleSourceEmbedded {
		return ""
	}

	return config.Spec.Source
}

// configWantsModuleFromSource reports whether the operator explicitly enabled the module and left
// this source free to serve it. It gates the alerting metric, not the fetch.
func configWantsModuleFromSource(config *v1alpha1.ModuleConfig, sourceName string) bool {
	if config == nil || config.Spec.Enabled == nil || !*config.Spec.Enabled {
		return false
	}

	chosen := configuredSource(config)

	return chosen == "" || chosen == sourceName
}

// getModuleConfig returns the module's config, or nil when the module has none.
func (r *reconciler) getModuleConfig(ctx context.Context, moduleName string) (*v1alpha1.ModuleConfig, error) {
	config := new(v1alpha1.ModuleConfig)
	if err := r.client.Get(ctx, client.ObjectKey{Name: moduleName}, config); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("get module config '%s': %w", moduleName, err)
	}

	return config, nil
}

// availableRepositories lists the repositories offering the module. It stays empty until something
// registers the module in the package system, which leaves the ambiguity check off rather than
// guessing at it.
func (r *reconciler) availableRepositories(ctx context.Context, moduleName string) ([]string, error) {
	pkg := new(v1alpha1.ModulePackage)
	if err := r.client.Get(ctx, client.ObjectKey{Name: moduleName}, pkg); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("get module package '%s': %w", moduleName, err)
	}

	return pkg.Status.AvailableRepositories, nil
}
