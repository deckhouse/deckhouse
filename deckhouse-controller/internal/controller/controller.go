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

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/flant/addon-operator/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	coordv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/labels"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrlruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
	"github.com/deckhouse/module-sdk/pkg/utils/ptr"
)

const (
	// docsLeaseLabel marks the leases the ModuleDocumentation controller watches.
	docsLeaseLabel = "deckhouse.io/documentation-builder-sync"

	// gracefulShutdownTimeout bounds the controller-runtime manager shutdown.
	gracefulShutdownTimeout = 10 * time.Second
)

// Controller owns the controller-runtime manager and the state shared between the
// deckhouse.io controllers: the package runtime, the deckhouse settings and the
// embedded update policy.
type Controller struct {
	ctrl    ctrlmanager.Manager
	runtime *runtime.Runtime

	settingsCh     <-chan utils.Values
	settings       *helpers.DeckhouseSettingsContainer
	embeddedPolicy *helpers.ModuleUpdatePolicySpecContainer

	synced *sync.WaitGroup

	dc dependency.Container

	logger *log.Logger
}

// Build assembles the controller and its controller-runtime manager. It does not
// start anything, see Start.
func Build(ctx context.Context, restConfig *rest.Config, metricStorage metricsstorage.Storage, logger *log.Logger) (*Controller, error) {
	scheme, err := addSchemas()
	if err != nil {
		return nil, fmt.Errorf("add schemas: %w", err)
	}

	ctrl, err := ctrlruntime.NewManager(restConfig, buildControllerOpts(ctx, scheme))
	if err != nil {
		return nil, fmt.Errorf("create controller runtime manager: %w", err)
	}

	// create a default policy, it'll be filled in with relevant settings from the deckhouse moduleConfig
	embeddedPolicy := helpers.NewModuleUpdatePolicySpecContainer(&v1alpha2.ModuleUpdatePolicySpec{
		Update: v1alpha2.ModuleUpdatePolicySpecUpdate{
			Mode: v1alpha2.UpdateModeAuto.String(),
		},
		ReleaseChannel: app.DefaultReleaseChannel,
	})

	settings := helpers.NewDeckhouseSettingsContainer(nil, metricStorage)
	synced := new(sync.WaitGroup)

	dc := dependency.NewDependencyContainer()

	// edition, err := d8edition.Parse(app.Version)
	// if err != nil {
	// return nil, fmt.Errorf("parse edition: %w", err)
	// }

	// runtime, err := runtime.Build(runtimeManager.GetClient(), edition, operator.ModuleManager, dc, metricStorage, logger)
	// if err != nil {
	// return nil, fmt.Errorf("create package operator: %w", err)
	// }

	return &Controller{
		ctrl:           ctrl,
		dc:             dc,
		synced:         synced,
		settings:       settings,
		embeddedPolicy: embeddedPolicy,
		// runtime:           runtime,
		settingsCh: make(chan utils.Values, 1),
		logger:     logger,
	}, nil
}

// addSchemas builds the scheme with every API group the controllers work with.
func addSchemas() (*apiruntime.Scheme, error) {
	addToScheme := []func(s *apiruntime.Scheme) error{
		corev1.AddToScheme,
		coordv1.AddToScheme,
		v1alpha1.AddToScheme,
		v1alpha2.AddToScheme,
		appsv1.AddToScheme,
		discoveryv1.AddToScheme,
	}

	scheme := apiruntime.NewScheme()
	for _, add := range addToScheme {
		if err := add(scheme); err != nil {
			return nil, fmt.Errorf("add to scheme: %w", err)
		}
	}

	return scheme, nil
}

// buildControllerOpts returns the manager options, restricting the cache to the
// namespaces and labels the controllers actually need.
func buildControllerOpts(ctx context.Context, scheme *apiruntime.Scheme) ctrlruntime.Options {
	return ctrlruntime.Options{
		Scheme: scheme,
		BaseContext: func() context.Context {
			return ctx
		},
		// disable manager's metrics for a while
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
		GracefulShutdownTimeout: ptr.To(gracefulShutdownTimeout),
		Cache: cache.Options{
			ByObject: map[client.Object]cache.ByObject{
				// for ModuleDocumentation controller
				&coordv1.Lease{}: {
					Namespaces: map[string]cache.Config{
						app.NamespaceDeckhouse: {
							LabelSelector: labels.SelectorFromSet(map[string]string{docsLeaseLabel: ""}),
						},
					},
				},
				// for ModuleRelease controller and DeckhouseRelease controller
				&corev1.Secret{}: {
					Namespaces: map[string]cache.Config{
						app.NamespaceDeckhouse: {
							LabelSelector: labels.SelectorFromSet(map[string]string{"heritage": "deckhouse", "module": "deckhouse"}),
						},
						app.NamespaceKubeSystem: {
							LabelSelector: labels.SelectorFromSet(map[string]string{"name": "d8-cluster-configuration"}),
						},
					},
				},
				// for DeckhouseRelease controller
				&corev1.Pod{}: {
					Namespaces: map[string]cache.Config{
						app.NamespaceDeckhouse: {
							LabelSelector: labels.SelectorFromSet(map[string]string{"app": "deckhouse"}),
						},
					},
				},
				// for DeckhouseRelease controller
				&corev1.ConfigMap{}: {
					Namespaces: map[string]cache.Config{
						app.NamespaceDeckhouse: {
							LabelSelector: labels.SelectorFromSet(map[string]string{"heritage": "deckhouse"}),
						},
					},
				},
				// for deckhouse.io apis
				&v1alpha1.Module{}:                     {},
				&v1alpha1.ModuleConfig{}:               {},
				&v1alpha1.ModuleDocumentation{}:        {},
				&v1alpha1.ModuleRelease{}:              {},
				&v1alpha1.ModuleSource{}:               {},
				&v1alpha2.ModuleUpdatePolicy{}:         {},
				&v1alpha2.ModulePullOverride{}:         {},
				&v1alpha1.DeckhouseRelease{}:           {},
				&v1alpha1.PackageRepository{}:          {},
				&v1alpha1.PackageRepositoryOperation{}: {},
				&v1alpha1.ApplicationPackageVersion{}:  {},
				&v1alpha1.ApplicationPackage{}:         {},
				&v1alpha1.Application{}:                {},
			},
		},
	}
}

// Start starts the controller-runtime manager, brings the on-disk state in sync
// with the cluster and then runs the background loops.
func (c *Controller) Start(ctx context.Context) error {
	c.synced.Add(1)

	// syncs the fs with the cluster state, starts the manager and various controllers
	go func() {
		if err := c.ctrl.Start(ctx); err != nil {
			c.logger.Fatal("start controller manager failed", log.Err(err))
		}
	}()

	if err := c.restoreOverrides(ctx); err != nil {
		return fmt.Errorf("restore overrides failed: %w", err)
	}

	if err := c.restoreReleases(ctx); err != nil {
		return fmt.Errorf("restore releases failed: %w", err)
	}

	if err := c.loadInitialConfiguration(ctx); err != nil {
		return fmt.Errorf("load initial configuration failed: %w", err)
	}

	c.synced.Done()
	c.runtime.ResumeScheduler()

	c.runDeleteStaleModuleReleasesLoop(ctx)
	c.runSyncDeckhouseSettings()

	return nil
}

// runSyncDeckhouseSettings updates embeddedPolicy and deckhouse settings by the deckhouse moduleConfig
func (c *Controller) runSyncDeckhouseSettings() {
	for deckhouseConfig := range c.settingsCh {
		configBytes, err := deckhouseConfig.AsBytes("json")
		if err != nil {
			c.logger.Error("failed to encode the deckhouse settings", log.Err(err))
			continue
		}

		settings := helpers.DefaultDeckhouseSettings()
		if err := json.Unmarshal(configBytes, settings); err != nil {
			c.logger.Error("failed to unmarshal the deckhouse setting", log.Err(err))
			continue
		}

		c.logger.Debug("update deckhouse settings")

		c.settings.Set(settings)

		// if deckhouse moduleConfig has releaseChannel unset, apply default releaseChannel Stable to the embedded policy
		if len(settings.ReleaseChannel) == 0 {
			settings.ReleaseChannel = app.DefaultReleaseChannel
			c.logger.Debug("the embedded deckhouse policy release channel set", slog.String("release_channel", settings.ReleaseChannel))
		}

		c.embeddedPolicy.Set(settings)
	}
}
