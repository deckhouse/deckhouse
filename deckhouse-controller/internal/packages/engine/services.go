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

// The cluster-facing services Build creates: the scheduler with its rule getters, the disk
// backends and the dedicated Kubernetes client of each service.

package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/Masterminds/semver/v3"
	klient "github.com/flant/kube-client/client"
	objectpatch "github.com/flant/shell-operator/pkg/kube/object_patch"
	kubeeventsmanager "github.com/flant/shell-operator/pkg/kube_events_manager"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/crd"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/disk/erofs"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/disk/symlink"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/health"
	globalmodule "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/modules/global"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/nelm"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/observed"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/orphans"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/tools/verity"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/edition"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

const (
	// The global values the scheduler's gate rules read (discovery fills them).
	bootstrappedGlobalValue = "clusterIsBootstrapped"
	kubernetesVersionValue  = "kubernetesVersion"
	deckhouseVersionValue   = "deckhouseVersion"

	// nelmMonitorRequestTimeout bounds discovery and metadata requests made by the release drift
	// monitor's client.
	nelmMonitorRequestTimeout = 30 * time.Second
)

// buildDisks returns the application and module disk backends: erofs with dm-verity when the node
// supports it, symlinks otherwise.
func buildDisks(reg *registry.Service, logger *log.Logger) (diskBackend, diskBackend) {
	downloaded := app.DownloadedModulesDir()
	appsDir := filepath.Join(downloaded, "apps")
	modulesDir := filepath.Join(downloaded, "modules")

	if verity.IsSupported() {
		logger.Info("erofs supported")

		return erofs.NewBackend(reg, appsDir, logger), erofs.NewBackend(reg, modulesDir, logger)
	}

	return symlink.NewBackend(reg, appsDir, logger), symlink.NewBackend(reg, modulesDir, logger)
}

// buildScheduler creates the paused scheduler with its rule getters. The gate inputs come from the
// global values (the engine's global module, else addon-operator's); the hooks' enable signal
// exists only with the engine's global module; dependencies are asked from addon-operator only
// while modules live there.
func buildScheduler(cli kclient.Client, moduleManager moduleManager, globalModule *globalmodule.Module, edit *edition.Edition, logger *log.Logger) *schedule.Scheduler {
	var globalValues func() addonutils.Values
	if globalModule != nil {
		globalValues = globalModule.GetValues
	} else {
		globalValues = func() addonutils.Values { return moduleManager.GetGlobal().GetValues(false) }
	}

	deckhouseVersion := func() (*semver.Version, error) {
		value, ok := globalValues()[deckhouseVersionValue]
		if !ok {
			return nil, errors.New("deckhouse version not found in global values")
		}

		version, ok := value.(string)
		if !ok {
			return nil, errors.New("invalid deckhouse version")
		}

		if version == "dev" {
			version = "v2.0.0"
		}

		return semver.NewVersion(version)
	}

	kubernetesVersion := func() (*semver.Version, error) {
		discovery := globalValues().GetKeySection("discovery")
		if len(discovery) == 0 {
			return nil, errors.New("discovery section not found in global values")
		}

		value, ok := discovery[kubernetesVersionValue]
		if !ok {
			return nil, errors.New("kubernetes version not found in global values")
		}

		version, ok := value.(string)
		if !ok {
			return nil, errors.New("invalid kubernetes version")
		}

		return semver.NewVersion(version)
	}

	bootstrapped := func() bool {
		value, ok := globalValues()[bootstrappedGlobalValue]
		if !ok {
			return false
		}

		ready, ok := value.(bool)

		return ok && ready
	}

	opts := []schedule.Option{
		schedule.WithBundleChecker(edit.IsEnabled),
		schedule.WithBootstrapCondition(bootstrapped),
		schedule.WithDeckhouseVersionGetter(deckhouseVersion),
		schedule.WithKubeVersionGetter(kubernetesVersion),
	}

	if globalModule != nil {
		opts = append(opts, schedule.WithDynamicGetter(globalModule.IsEnabled))
	}

	// without a dependency getter the scheduler answers dependency rules from its own graph
	if moduleManager != nil {
		opts = append(opts, schedule.WithDependencyGetter(addonDependencyGetter(cli, moduleManager, logger)))
	}

	return schedule.NewScheduler(logger, opts...)
}

// addonDependencyGetter returns the version of a module addon-operator has enabled, read from its
// Module resource; nil otherwise. A read failure counts as absent and is logged, because nothing
// else reports it.
func addonDependencyGetter(cli kclient.Client, moduleManager moduleManager, logger *log.Logger) func(name string) *semver.Version {
	return func(name string) *semver.Version {
		if !moduleManager.IsModuleEnabled(name) {
			return nil
		}

		module := new(v1alpha1.Module)
		err := retry.OnError(retry.DefaultRetry, apierrors.IsServiceUnavailable, func() error {
			return cli.Get(context.Background(), kclient.ObjectKey{Name: name}, module)
		})
		if err != nil {
			logger.Warn("read the module of a dependency", slog.String("module", name), log.Err(err))

			return nil
		}

		// set a default version for modules overridden by ModulePullOverride (MPOS)
		if module.IsCondition(v1alpha1.ModuleConditionIsOverridden, corev1.ConditionTrue) {
			return semver.MustParse("v2.0.0")
		}

		version, err := semver.NewVersion(module.GetVersion())
		if err != nil {
			// Debug: a module may carry no version (embedded), and this runs on every computation.
			logger.Debug("parse the version of a dependency", slog.String("module", name), log.Err(err))

			return nil
		}

		return version
	}
}

// kubeClientOptions configure the dedicated client of one service; zero values keep the defaults.
type kubeClientOptions struct {
	name         string // name is the logger name.
	metricPrefix string
	qps          float32
	burst        int
	timeout      time.Duration
}

func newKubeClient(opts kubeClientOptions, logger *log.Logger) (*klient.Client, error) {
	client := klient.New(klient.WithLogger(logger.Named(opts.name)))
	client.WithContextName(app.KubeContext())
	client.WithConfigPath(app.KubeConfig())

	if opts.qps != 0 || opts.burst != 0 {
		client.WithRateLimiterSettings(opts.qps, opts.burst)
	}

	if opts.timeout != 0 {
		client.WithTimeout(opts.timeout)
	}

	client.WithMetricPrefix(opts.metricPrefix)

	if err := client.Init(); err != nil {
		return nil, fmt.Errorf("initialize %s: %w", opts.name, err)
	}

	return client, nil
}

// buildObjectPatcher creates the patcher of the hooks' patches; its timeout keeps a slow API call
// from hanging a hook.
func buildObjectPatcher(logger *log.Logger) (*objectpatch.ObjectPatcher, error) {
	client, err := newKubeClient(kubeClientOptions{
		name:         "object-patcher-client",
		metricPrefix: "packages_object_patcher_",
		qps:          app.ObjectPatcherKubeClientQPS(),
		burst:        app.ObjectPatcherKubeClientBurst(),
		timeout:      app.ObjectPatcherKubeClientTimeout(),
	}, logger)
	if err != nil {
		return nil, err
	}

	return objectpatch.NewObjectPatcher(client, logger.Named("object-patcher")), nil
}

// buildKubeEventsManager creates the manager of the hooks' watches, with the metric storage hook
// initialization needs.
func buildKubeEventsManager(logger *log.Logger) (kubeeventsmanager.KubeEventsManager, error) {
	client, err := newKubeClient(kubeClientOptions{
		name:         "kube-events-manager-client",
		metricPrefix: "packages_kube_events_manager_",
		qps:          app.KubeClientQPS(),
		burst:        app.KubeClientBurst(),
	}, logger)
	if err != nil {
		return nil, err
	}

	manager := kubeeventsmanager.NewKubeEventsManager(context.Background(), client, logger.Named("kube-events-manager"))
	manager.WithMetricStorage(metricsstorage.NewMetricStorage(
		metricsstorage.WithLogger(logger.Named("kube-events-metrics")),
		metricsstorage.WithNewRegistry(),
	))

	return manager, nil
}

func buildNelmService(observations *observed.Store, logger *log.Logger) (*nelm.Service, error) {
	client, err := newKubeClient(kubeClientOptions{
		name:         "nelm-monitor-client",
		metricPrefix: "packages_nelm_monitor_",
		qps:          app.HelmMonitorKubeClientQPS(),
		burst:        app.HelmMonitorKubeClientBurst(),
		timeout:      nelmMonitorRequestTimeout,
	}, logger)
	if err != nil {
		return nil, err
	}

	return nelm.NewService(client, observations, logger), nil
}

// buildOrphanService: its client is shaped like the patcher's.
func buildOrphanService(logger *log.Logger) (*orphans.Service, error) {
	client, err := newKubeClient(kubeClientOptions{
		name:         "orphans-client",
		metricPrefix: "packages_orphans_",
		qps:          app.ObjectPatcherKubeClientQPS(),
		burst:        app.ObjectPatcherKubeClientBurst(),
		timeout:      app.ObjectPatcherKubeClientTimeout(),
	}, logger)
	if err != nil {
		return nil, err
	}

	return orphans.NewService(client, logger), nil
}

func buildCRDService(logger *log.Logger) (*crd.Service, error) {
	client, err := newKubeClient(kubeClientOptions{
		name:         "crd-installer-client",
		metricPrefix: "packages_crd_installer_",
		qps:          app.KubeClientQPS(),
		burst:        app.KubeClientBurst(),
	}, logger)
	if err != nil {
		return nil, err
	}

	return crd.NewService(client, logger), nil
}

func buildHealthService(onChange health.Callback, logger *log.Logger) (*health.Service, error) {
	client, err := newKubeClient(kubeClientOptions{
		name:         "health-monitor-client",
		metricPrefix: "packages_health_monitor_",
		qps:          app.KubeClientQPS(),
		burst:        app.KubeClientBurst(),
	}, logger)
	if err != nil {
		return nil, err
	}

	service, err := health.NewService(client, onChange, logger)
	if err != nil {
		return nil, fmt.Errorf("create health service: %w", err)
	}

	return service, nil
}
