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

package moduleconfig

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1beta1"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	// controllerName is the name the controller is registered under in the manager.
	controllerName = "d8-module-config-controller"

	// cacheSyncTimeout bounds the initial informer cache sync.
	cacheSyncTimeout = 3 * time.Minute

	// moduleNameDeckhouse is the module whose config carries the Deckhouse release channel.
	moduleNameDeckhouse = "deckhouse"
)

// RegisterController registers the controller that mirrors each Module's spec into its ModuleConfig.
func RegisterController(sync *sync.WaitGroup, runtime ctrlmanager.Manager, dc dependency.Container, logger *log.Logger) error {
	r := &reconciler{
		init:     sync,
		client:   runtime.GetClient(),
		registry: registry.NewService(dc, logger),
		logger:   logger.Named(controllerName),
	}

	// A config edit is watched too, so a change by a system account is reverted.
	if err := ctrl.NewControllerManagedBy(runtime).
		Named(controllerName).
		For(&v1beta1.Module{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&v1alpha1.ModuleConfig{}, &handler.EnqueueRequestForObject{}, builder.WithPredicates(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{}))).
		WithOptions(controller.Options{CacheSyncTimeout: cacheSyncTimeout}).
		Complete(r); err != nil {
		return fmt.Errorf("complete: %w", err)
	}

	return nil
}

// reconciler mirrors a Module onto the ModuleConfig of the same name.
type reconciler struct {
	init     *sync.WaitGroup
	client   client.Client
	registry *registry.Service
	logger   *log.Logger
}

// Reconcile writes the module spec into its config; a module with no config gets none.
func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// pkgsync migrates the configs into the modules before the mirror starts
	r.init.Wait()

	config := new(v1alpha1.ModuleConfig)
	if err := r.client.Get(ctx, client.ObjectKey{Name: req.Name}, config); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("get module config '%s': %w", req.Name, err)
	}

	if !config.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	module := new(v1beta1.Module)
	if err := r.client.Get(ctx, client.ObjectKey{Name: req.Name}, module); err != nil {
		if apierrors.IsNotFound(err) {
			// a migrated config lost its module on purpose, so the module is not brought back
			if isMigrated(config) {
				return ctrl.Result{}, nil
			}

			return r.createModule(ctx, config)
		}

		return ctrl.Result{}, fmt.Errorf("get module '%s': %w", req.Name, err)
	}

	if !module.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	patch := client.MergeFrom(config.DeepCopy())
	applyModule(config, module)

	patchData, err := patch.Data(config)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("build the patch for the '%s' module config: %w", req.Name, err)
	}

	if string(patchData) == "{}" {
		return ctrl.Result{}, nil
	}

	if err = r.client.Patch(ctx, config, client.RawPatch(patch.Type(), patchData)); err != nil {
		return ctrl.Result{}, fmt.Errorf("patch the '%s' module config: %w", req.Name, err)
	}

	r.logger.Debug("module config synced with its module", slog.String("name", req.Name))

	return ctrl.Result{}, nil
}

// applyModule copies the module fields into the config spec and marks the config migrated, which locks it.
func applyModule(config *v1alpha1.ModuleConfig, module *v1beta1.Module) {
	config.Spec.Enabled = module.Spec.Enabled
	config.Spec.Settings = module.Spec.Settings
	config.Spec.Version = module.Spec.SettingsVersion
	config.Spec.Maintenance = module.Spec.Maintenance

	metav1.SetMetaDataAnnotation(&config.ObjectMeta, v1alpha1.ModuleConfigAnnotationMigrated, "true")
}

// createModule writes a module the cluster does not carry yet.
func (r *reconciler) createModule(ctx context.Context, config *v1alpha1.ModuleConfig) (ctrl.Result, error) {
	repository, err := r.getModuleRepository(ctx, config.Name)
	if err != nil {
		r.logger.Error("failed to get the module repository",
			slog.String("name", config.Name),
			log.Err(err))
		return ctrl.Result{}, fmt.Errorf("get module repository: %w", err)
	}

	channel, update, err := r.getModuleUpdate(ctx, config)
	if err != nil {
		r.logger.Error("failed to get the module update settings",
			slog.String("name", config.Name),
			slog.String("repository", repository),
			log.Err(err))
		return ctrl.Result{}, fmt.Errorf("get module update: %w", err)
	}

	module := &v1beta1.Module{
		ObjectMeta: metav1.ObjectMeta{
			Name: config.Name,
		},
		Spec: v1beta1.ModuleSpec{
			PackageRepositoryName: repository,
			ReleaseChannel:        channel,
			Update:                update,
			Enabled:               config.Spec.Enabled,
			SettingsVersion:       config.Spec.Version,
			Settings:              config.Spec.Settings,
			Maintenance:           config.Spec.Maintenance,
		},
	}

	if err := r.setModuleVersion(ctx, module); err != nil {
		r.logger.Error("failed to get the module version from the registry",
			slog.String("name", module.Name),
			slog.String("repository", repository),
			slog.String("channel", channel),
			log.Err(err))
		return ctrl.Result{}, fmt.Errorf("get module version: %w", err)
	}

	r.logger.Debug("create the module for its config",
		slog.String("name", module.Name),
		slog.String("repository", repository),
		slog.String("channel", channel),
		slog.String("version", module.Spec.PackageVersion))

	if err := r.client.Create(ctx, module); err != nil {
		r.logger.Error("failed to create the module for its config",
			slog.String("name", module.Name),
			slog.String("repository", repository),
			slog.String("channel", channel),
			slog.String("version", module.Spec.PackageVersion),
			log.Err(err))
		return ctrl.Result{}, fmt.Errorf("create module: %w", err)
	}

	if err := r.markMigrated(ctx, config); err != nil {
		return ctrl.Result{}, fmt.Errorf("mark the '%s' module config migrated: %w", config.Name, err)
	}

	return ctrl.Result{}, nil
}

// getModuleRepository returns the only repository the module package is available in.
func (r *reconciler) getModuleRepository(ctx context.Context, name string) (string, error) {
	pkg := new(v1alpha1.ModulePackage)
	if err := r.client.Get(ctx, client.ObjectKey{Name: name}, pkg); err != nil {
		return "", fmt.Errorf("get module package '%s': %w", name, err)
	}

	r.logger.Debug("module package repositories",
		slog.String("name", name),
		slog.Any("repositories", pkg.Status.AvailableRepositories))

	switch len(pkg.Status.AvailableRepositories) {
	case 0:
		return "", errors.New("the module package is available in no repository")
	case 1:
		return pkg.Status.AvailableRepositories[0], nil
	default:
		return "", fmt.Errorf("the module package is available in several repositories: %v", pkg.Status.AvailableRepositories)
	}
}

// markMigrated sets the migrated annotation on the config, so its module is never created again.
func (r *reconciler) markMigrated(ctx context.Context, config *v1alpha1.ModuleConfig) error {
	patch := client.MergeFrom(config.DeepCopy())
	metav1.SetMetaDataAnnotation(&config.ObjectMeta, v1alpha1.ModuleConfigAnnotationMigrated, "true")

	return r.client.Patch(ctx, config, patch)
}

// isMigrated reports whether the config was migrated into its module.
func isMigrated(config *v1alpha1.ModuleConfig) bool {
	_, ok := config.Annotations[v1alpha1.ModuleConfigAnnotationMigrated]
	return ok
}

// setModuleVersion reads the module version from the registry and writes it into the module spec.
func (r *reconciler) setModuleVersion(ctx context.Context, module *v1beta1.Module) error {
	repo := new(v1alpha1.PackageRepository)
	if err := r.client.Get(ctx, client.ObjectKey{Name: module.Spec.PackageRepositoryName}, repo); err != nil {
		return fmt.Errorf("get package repository: %w", err)
	}

	versionPath := fmt.Sprintf("%s/version", module.Name)

	err := r.registry.ImageExists(ctx, registry.BuildRemote(repo), versionPath, module.Spec.ReleaseChannel)
	if err != nil {
		versionPath = fmt.Sprintf("%s/release", module.Name)
	}

	releaseChannel := strings.ToLower(module.Spec.ReleaseChannel)

	raw, err := r.registry.GetFileContent(ctx, registry.BuildRemote(repo), versionPath, releaseChannel, "version.json")
	if err != nil {
		return fmt.Errorf("get module version from the registry: %w", err)
	}

	version := struct {
		Version string `json:"version"`
	}{}

	if err := json.Unmarshal(raw, &version); err != nil {
		return fmt.Errorf("unmarshal version file: %w", err)
	}

	module.Spec.PackageVersion = version.Version

	return nil
}

// getModuleUpdate reads the channel and the update settings from the update policy the config names;
// with none, or one the cluster has lost, the Deckhouse channel applies with no update settings.
func (r *reconciler) getModuleUpdate(ctx context.Context, config *v1alpha1.ModuleConfig) (string, *v1beta1.ModuleUpdate, error) {
	if config.Spec.UpdatePolicy == "" {
		channel, err := r.getDeckhouseReleaseChannel(ctx)
		return channel, nil, err
	}

	policy := new(v1alpha2.ModuleUpdatePolicy)
	if err := r.client.Get(ctx, client.ObjectKey{Name: config.Spec.UpdatePolicy}, policy); err != nil {
		if !apierrors.IsNotFound(err) {
			return "", nil, fmt.Errorf("get module update policy '%s': %w", config.Spec.UpdatePolicy, err)
		}

		channel, err := r.getDeckhouseReleaseChannel(ctx)
		return channel, nil, err
	}

	return policy.Spec.ReleaseChannel, moduleUpdateFromPolicy(policy), nil
}

// getDeckhouseReleaseChannel reads the channel from the deckhouse module config; with none set the build default applies.
func (r *reconciler) getDeckhouseReleaseChannel(ctx context.Context) (string, error) {
	config := new(v1alpha1.ModuleConfig)
	if err := r.client.Get(ctx, client.ObjectKey{Name: moduleNameDeckhouse}, config); err != nil {
		if apierrors.IsNotFound(err) {
			return app.DefaultReleaseChannel, nil
		}

		return "", fmt.Errorf("get module config '%s': %w", moduleNameDeckhouse, err)
	}

	if config.Spec.Settings == nil {
		return app.DefaultReleaseChannel, nil
	}

	settings := struct {
		ReleaseChannel string `json:"releaseChannel"`
	}{}

	if err := json.Unmarshal(config.Spec.Settings.Raw, &settings); err != nil {
		r.logger.Warn("the deckhouse module config settings do not parse, fall back to the build release channel", log.Err(err))

		return app.DefaultReleaseChannel, nil
	}

	if settings.ReleaseChannel == "" {
		return app.DefaultReleaseChannel, nil
	}

	return settings.ReleaseChannel, nil
}

// moduleUpdateFromPolicy converts the update settings of the policy into the module ones.
// The Ignore mode has no module counterpart, so it yields none and the Deckhouse settings apply.
func moduleUpdateFromPolicy(policy *v1alpha2.ModuleUpdatePolicy) *v1beta1.ModuleUpdate {
	if policy.Spec.Update.Mode == v1alpha2.ModuleUpdatePolicyModeIgnore {
		return nil
	}

	update := &v1beta1.ModuleUpdate{Mode: policy.Spec.Update.Mode}

	for _, window := range policy.Spec.Update.Windows {
		update.Windows = append(update.Windows, v1beta1.ModuleUpdateWindow{
			From: window.From,
			To:   window.To,
			Days: window.Days,
		})
	}

	return update
}
