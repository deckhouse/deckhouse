// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package modulesync

import (
	"context"
	"fmt"

	"github.com/jonboulle/clockwork"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// Syncer converges the v1alpha2 Module resources once, at startup, before
// controllers run.
type Syncer struct {
	// writer updates the v1alpha2 Module resources and marks duplicate
	// deployed releases superseded.
	writer client.Client

	// reader reads the source resources (overrides, releases, configs) and
	// the current v1alpha2 Modules; it must bypass the manager cache - see New.
	reader client.Reader

	// clock supplies the "now" for the only timestamp the sync writes:
	//  - written to ModuleRelease status.transitionTime;
	//  - stamped when a duplicate deployed release is demoted to Superseded;
	//  - Module resources get no timestamps;
	//  - only origin_module_releases.go uses it
	clock  clockwork.Clock
	logger *log.Logger

	// embeddedDir is where the running Deckhouse image ships its built-in
	// modules (/deckhouse/modules by default); the sync lists this directory
	// to learn which modules are embedded. Tests point it at a fixture.
	embeddedDir string
}

// Option tunes a Syncer.
type Option func(*Syncer)

// WithEmbeddedModulesDir overrides the directory embedded modules are read from.
func WithEmbeddedModulesDir(dir string) Option {
	return func(s *Syncer) {
		s.embeddedDir = dir
	}
}

// New builds a Syncer. All reads go through reader: callers pass a direct
// (uncached) one, so the sync sees its own writes and works before the
// manager's cache starts. Writes go through writer.
func New(writer client.Client, reader client.Reader, clock clockwork.Clock, logger *log.Logger, opts ...Option) *Syncer {
	s := &Syncer{
		writer:      writer,
		reader:      reader,
		clock:       clock,
		logger:      logger,
		embeddedDir: app.EmbeddedModulesDir,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Sync makes the v1alpha2 Module resources match where every module's package
// actually comes from. The returned Modules carry what was written: the
// caller hands them straight to the package runtime without re-reading
// the cluster.
func (s *Syncer) Sync(ctx context.Context) ([]v1alpha2.Module, error) {
	// where does every module's package come from?
	fromImage, err := s.originsFromImage(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve origins from the image: %w", err)
	}

	fromPullOverrides, err := s.originsFromModulePullOverrides(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve origins from module pull overrides: %w", err)
	}

	fromModuleReleases, err := s.originsFromModuleReleases(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve origins from module releases: %w", err)
	}

	// one origin per module; an earlier source beats a later one
	origins := mergeOrigins(fromImage, fromPullOverrides, fromModuleReleases)

	// how are the modules configured?
	configs, err := s.liveModuleConfigs(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve module configs: %w", err)
	}

	// write it all into the Module resources
	modulesV2, err := s.writeModulesV2(ctx, origins, configs)
	if err != nil {
		return nil, fmt.Errorf("apply modules: %w", err)
	}

	return modulesV2, nil
}
