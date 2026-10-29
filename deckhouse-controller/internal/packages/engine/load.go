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

// The load function of the core and the global module: the real loader with the managers every
// package object shares.

package engine

import (
	"context"
	"fmt"

	objectpatch "github.com/flant/shell-operator/pkg/kube/object_patch"
	kubeeventsmanager "github.com/flant/shell-operator/pkg/kube_events_manager"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/cron"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/apps"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/grants"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/loader"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/modules"
	globalmodule "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/modules/global"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

// sharedManagers are the process-wide managers every package object is given.
type sharedManagers struct {
	patcher           *objectpatch.ObjectPatcher
	scheduleManager   *cron.Manager
	kubeEventsManager kubeeventsmanager.KubeEventsManager
	metricStorage     metricsstorage.Storage
}

// packageLoader is the core's lifecycle.LoadFunc over the real loader. It reads only the image (the
// grant resolver is used later, by settings validation).
type packageLoader struct {
	shared        sharedManagers
	grantResolver grants.Resolver
	appGlobals    apps.GlobalValuesGetter // appGlobals are what applications render .Platform from.
	global        *globalmodule.Module    // global is nil without EnableModuleV2: no module is loaded.
	logger        *log.Logger
}

func (l *packageLoader) load(ctx context.Context, kind model.Kind, image model.Image) (model.Package, error) {
	switch kind {
	case model.KindApplication:
		return l.loadApplication(ctx, image)
	case model.KindModule:
		return l.loadModule(ctx, image)
	default:
		return nil, fmt.Errorf("load %s: unknown package kind %q", image.Name, kind)
	}
}

func (l *packageLoader) loadApplication(ctx context.Context, image model.Image) (model.Package, error) {
	conf, err := loader.LoadAppConf(ctx, image.Path, image.Name, l.logger)
	if err != nil {
		return nil, fmt.Errorf("load application conf: %w", err)
	}

	conf.Repository = image.Source.Repository
	conf.Patcher = l.shared.patcher
	conf.ScheduleManager = l.shared.scheduleManager
	conf.KubeEventsManager = l.shared.kubeEventsManager
	conf.MetricStorage = l.shared.metricStorage
	conf.GrantResolver = l.grantResolver
	conf.GlobalValuesGetter = l.appGlobals

	application, err := apps.NewAppByConfig(image.Name, conf, l.logger)
	if err != nil {
		return nil, fmt.Errorf("new application: %w", err)
	}

	return application, nil
}

// loadModule loads a downloaded or an embedded module. An embedded module gets image.Version (its
// definition carries none), so the object and its node agree. A dev image gets app.DevModuleVersion:
// its version (dev-<digest>) is no semver for the dependency checks.
func (l *packageLoader) loadModule(ctx context.Context, image model.Image) (model.Package, error) {
	var (
		conf *modules.Config
		err  error
	)

	if image.Embedded {
		if conf, err = loader.LoadEmbeddedConf(ctx, image.Path, image.Version, l.logger); err != nil {
			return nil, fmt.Errorf("load embedded module conf: %w", err)
		}
	} else {
		if conf, err = loader.LoadModuleConf(ctx, image.Path, image.Name, l.logger); err != nil {
			return nil, fmt.Errorf("load module conf: %w", err)
		}

		conf.Repository = image.Source.Repository

		if registry.IsDevVersion(image.Version) {
			conf.Definition.Version = app.DevModuleVersion
		}
	}

	conf.Patcher = l.shared.patcher
	conf.ScheduleManager = l.shared.scheduleManager
	conf.KubeEventsManager = l.shared.kubeEventsManager
	conf.MetricStorage = l.shared.metricStorage
	conf.GlobalValuesGetter = l.global.GetValues

	module, err := modules.NewModuleByConfig(image.Name, conf, l.logger)
	if err != nil {
		return nil, fmt.Errorf("new module: %w", err)
	}

	return module, nil
}

// appGlobalValues returns the global values applications render against: the engine's global
// module, else addon-operator's global, looked up on every call because it may not exist yet when
// the engine is built.
func appGlobalValues(moduleManager moduleManager, globalModule *globalmodule.Module) apps.GlobalValuesGetter {
	if globalModule != nil {
		return func(bool) addonutils.Values { return globalModule.GetValues() }
	}

	return func(withPrefix bool) addonutils.Values { return moduleManager.GetGlobal().GetValues(withPrefix) }
}

// loadGlobal loads the global module from the deckhouse image. onDynamicChange is called when a
// global hook changes a module's enable signal.
func loadGlobal(ctx context.Context, shared sharedManagers, onDynamicChange func(), logger *log.Logger) (*globalmodule.Module, error) {
	conf, err := loader.LoadGlobalConf(ctx, logger)
	if err != nil {
		return nil, fmt.Errorf("load global conf: %w", err)
	}

	conf.Version = app.EmbeddedPackageVersion()
	conf.Patcher = shared.patcher
	conf.ScheduleManager = shared.scheduleManager
	conf.KubeEventsManager = shared.kubeEventsManager
	conf.MetricStorage = shared.metricStorage
	conf.OnDynamicChange = onDynamicChange

	module, err := globalmodule.NewModuleByConfig(conf, logger)
	if err != nil {
		return nil, fmt.Errorf("new global module: %w", err)
	}

	return module, nil
}
