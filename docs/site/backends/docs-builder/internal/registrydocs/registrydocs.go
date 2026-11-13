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

// Package registrydocs reads the modules of ondemand from their registries:
// the release tags of a module (Tags), the versions its release channels
// point to (ChannelVersion, Channels) and the documentation of a version
// (Render). What a registry answers is told to ondemand in its own terms
// (ondemand.ErrNoVersions, ondemand.ErrVersionNotFound and the others).
package registrydocs

import (
	"context"
	"errors"
	"fmt"

	"github.com/flant/docs-builder/internal/ondemand"
	"github.com/flant/docs-builder/internal/registry"
)

// ModuleSources finds the registries of a module: the cluster objects
// (registry.Sources) or the fixed list of the public site
// (registry.StaticSources). A module with no source is an error wrapping
// registry.ErrNoSource.
type ModuleSources interface {
	ForModule(ctx context.Context, module string) ([]registry.Source, error)
}

// moduleSources returns the sources of the module; a module with no source
// has no versions (ondemand.ErrNoVersions).
func moduleSources(ctx context.Context, sources ModuleSources, module string) ([]registry.Source, error) {
	srcs, err := sources.ForModule(ctx, module)
	if errors.Is(err, registry.ErrNoSource) {
		return nil, fmt.Errorf("%w: %w", ondemand.ErrNoVersions, err)
	}
	if err != nil {
		return nil, fmt.Errorf("find sources: %w", err)
	}

	return srcs, nil
}
