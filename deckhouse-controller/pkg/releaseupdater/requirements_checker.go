/*
Copyright 2024 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package releaseupdater

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"go.opentelemetry.io/otel"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders/kubernetesversion"
	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

const (
	deckhouseClusterConfigurationConfig = "d8-cluster-configuration"
	systemNamespace                     = "kube-system"
	k8sAutomaticVersion                 = "Automatic"
	reqCheckerServiceName               = "requirements-checker"
	MigratedModulesRequirementFieldName = "migratedModules"
)

type RequirementsChecker[T any] interface {
	MetRequirements(ctx context.Context, v *T) []NotMetReason
}

type Check[T any] interface {
	GetName() string
	Verify(ctx context.Context, v *T) error
}

type NotMetReason struct {
	Reason  string
	Message string
}

// CheckerOption represents an option for configuring a Checker
type CheckerOption func(*CheckerConfig)

// CheckerConfig holds configuration for a Checker
type CheckerConfig struct {
	logger *log.Logger
}

// WithLogger sets the logger for the checker
func WithLogger(logger *log.Logger) CheckerOption {
	return func(config *CheckerConfig) {
		config.logger = logger
	}
}

// applyOptions applies the given options to the config
func applyOptions(options []CheckerOption) *CheckerConfig {
	config := &CheckerConfig{}
	for _, option := range options {
		option(config)
	}
	return config
}

var _ RequirementsChecker[v1alpha1.DeckhouseRelease] = (*Checker[v1alpha1.DeckhouseRelease])(nil)

type Checker[T any] struct {
	fns []Check[T]

	logger *log.Logger
}

func (c *Checker[T]) MetRequirements(ctx context.Context, v *T) []NotMetReason {
	ctx, span := otel.Tracer(reqCheckerServiceName).Start(ctx, "met-requirements")
	defer span.End()

	reasons := make([]NotMetReason, 0)

	for _, fn := range c.fns {
		err := fn.Verify(ctx, v)
		if err != nil {
			reasons = append(reasons, NotMetReason{
				Reason:  fn.GetName(),
				Message: err.Error(),
			})
		}
	}

	return reasons
}

// NewDeckhouseReleaseRequirementsChecker returns DeckhouseRelease checker with this checks:
//
// 1) deckhouse version check
// 2) deckhouse requirements check
// 3) deckhouse kubernetes version check
// 4) migrated modules check
//
// for more checks information - look at extenders
func NewDeckhouseReleaseRequirementsChecker(k8sclient client.Client, enabledModules []string, exts *extenders.ExtendersStack, metricStorage metricsstorage.Storage, options ...CheckerOption) (*Checker[v1alpha1.DeckhouseRelease], error) {
	config := applyOptions(options)

	if config.logger == nil {
		config.logger = log.NewLogger().Named("deckhouse-release-requirements-checker")
	}

	k8sCheck, err := newKubernetesVersionCheck(k8sclient, enabledModules)
	if err != nil {
		return nil, err
	}

	return &Checker[v1alpha1.DeckhouseRelease]{
		fns: []Check[v1alpha1.DeckhouseRelease]{
			newDeckhouseVersionCheck(enabledModules, exts),
			newDeckhouseRequirementsCheck(enabledModules, exts),
			k8sCheck,
			newMigratedModulesCheck(k8sclient, metricStorage, config.logger),
		},
		logger: config.logger,
	}, nil
}

type deckhouseVersionCheck struct {
	name string
	exts *extenders.ExtendersStack

	enabledModules set.Set
}

func newDeckhouseVersionCheck(enabledModules []string, exts *extenders.ExtendersStack) *deckhouseVersionCheck {
	return &deckhouseVersionCheck{
		name:           "deckhouse version check",
		enabledModules: set.New(enabledModules...),
		exts:           exts,
	}
}

func (c *deckhouseVersionCheck) GetName() string {
	return c.name
}

func (c *deckhouseVersionCheck) Verify(_ context.Context, dr *v1alpha1.DeckhouseRelease) error {
	releaseName, err := c.exts.DeckhouseVersion.ValidateBaseVersion(dr.GetVersion().String())
	if err != nil {
		// invalid deckhouse version in deckhouse release
		// or an enabled module has requirements
		// prevent deckhouse release from becoming predicted
		if releaseName == "" || c.enabledModules.Has(releaseName) {
			return err
		}
	}

	return nil
}

type kubernetesVersionCheck struct {
	name string

	enabledModules           set.Set
	clusterKubernetesVersion string

	k8sclient client.Client
}

func newKubernetesVersionCheck(k8sclient client.Client, enabledModules []string) (*kubernetesVersionCheck, error) {
	c := &kubernetesVersionCheck{
		name:           "kubernetes version check",
		enabledModules: set.New(enabledModules...),
		k8sclient:      k8sclient,
	}

	err := c.initClusterKubernetesVersion(context.TODO())
	// if discovery failed, we musn't suspend the release
	if err != nil {
		return nil, fmt.Errorf("getting cluster kubernetes version: %w", err)
	}

	return c, nil
}

func (c *kubernetesVersionCheck) GetName() string {
	return c.name
}

func (c *kubernetesVersionCheck) Verify(_ context.Context, dr *v1alpha1.DeckhouseRelease) error {
	if c.isKubernetesVersionAutomatic() && len(dr.GetRequirements()["autoK8sVersion"]) > 0 {
		if moduleName, err := kubernetesversion.Instance().ValidateBaseVersion(dr.GetRequirements()["autoK8sVersion"]); err != nil {
			// invalid auto kubernetes version in deckhouse release
			// or an enabled module has requirements
			// prevent deckhouse release from becoming predicted
			if moduleName == "" || c.enabledModules.Has(moduleName) {
				return err
			}
		}
	}

	return nil
}

func (c *kubernetesVersionCheck) isKubernetesVersionAutomatic() bool {
	return c.clusterKubernetesVersion == k8sAutomaticVersion
}

type clusterConf struct {
	KubernetesVersion string `json:"kubernetesVersion"`
}

func (c *kubernetesVersionCheck) initClusterKubernetesVersion(ctx context.Context) error {
	key := client.ObjectKey{Namespace: systemNamespace, Name: deckhouseClusterConfigurationConfig}
	secret := new(corev1.Secret)
	if err := c.k8sclient.Get(ctx, key, secret); err != nil {
		// the secret does not exist in managed cluster
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to get the 'd8-cluster-configuration' secret: %w", err)
	}

	clusterConfigurationRaw, ok := secret.Data["cluster-configuration.yaml"]
	if !ok {
		return fmt.Errorf("expected field 'cluster-configuration.yaml' not found in secret %s", secret.Name)
	}

	conf := new(clusterConf)
	if err := yaml.Unmarshal(clusterConfigurationRaw, conf); err != nil {
		return fmt.Errorf("failed to unmarshal cluster configuration: %w", err)
	}

	c.clusterKubernetesVersion = conf.KubernetesVersion

	return nil
}

type deckhouseRequirementsCheck struct {
	name string
	exts *extenders.ExtendersStack

	enabledModules set.Set
}

func newDeckhouseRequirementsCheck(enabledModules []string, exts *extenders.ExtendersStack) *deckhouseRequirementsCheck {
	return &deckhouseRequirementsCheck{
		name:           "deckhouse requirements check",
		exts:           exts,
		enabledModules: set.New(enabledModules...),
	}
}

func (c *deckhouseRequirementsCheck) GetName() string {
	return c.name
}

func (c *deckhouseRequirementsCheck) Verify(_ context.Context, dr *v1alpha1.DeckhouseRelease) error {
	for key, value := range dr.GetRequirements() {
		// these fields are checked by extenders in module release controller
		if c.exts.IsExtendersField(key) {
			continue
		}

		passed, err := requirements.CheckRequirement(key, value, c.enabledModules)
		if !passed {
			msg := fmt.Sprintf("%q requirement for DeckhouseRelease %q not met: %s", key, dr.GetVersion(), err)

			return errors.New(msg)
		}
	}

	return nil
}

// NewPreApplyChecker returns Release checker with this checks:
//
// 1) disruption check
func NewPreApplyChecker(settings *Settings, logger *log.Logger) *Checker[v1alpha1.Release] {
	return &Checker[v1alpha1.Release]{
		fns: []Check[v1alpha1.Release]{
			newDisruptionCheck(settings),
		},
		logger: logger,
	}
}

type disruptionCheck struct {
	name     string
	settings *Settings
}

// check: release disruptions (hard lock)
func newDisruptionCheck(settings *Settings) *disruptionCheck {
	return &disruptionCheck{
		name:     "release disruption check",
		settings: settings,
	}
}

func (c *disruptionCheck) GetName() string {
	return c.name
}

func (c *disruptionCheck) Verify(_ context.Context, pointer *v1alpha1.Release) error {
	release := *pointer

	if !c.settings.InDisruptionApprovalMode() {
		return nil
	}

	for _, key := range release.GetDisruptions() {
		hasDisruptionUpdate, reason := requirements.HasDisruption(key)
		if hasDisruptionUpdate && !release.GetDisruptionApproved() {
			return fmt.Errorf("(`kubectl annotate DeckhouseRelease %s release.deckhouse.io/disruption-approved=true`): %s", release.GetName(), reason)
		}
	}

	return nil
}

// NewModuleReleaseRequirementsChecker returns ModuleRelease checker with this checks:
//
// 1) module release requirements check
//
// for more checks information - look at extenders
func NewModuleReleaseRequirementsChecker(exts *extenders.ExtendersStack, options ...CheckerOption) (*Checker[v1alpha1.ModuleRelease], error) {
	config := applyOptions(options)

	if config.logger == nil {
		config.logger = log.NewLogger().Named("module-release-requirements-checker")
	}

	return &Checker[v1alpha1.ModuleRelease]{
		fns: []Check[v1alpha1.ModuleRelease]{
			newModuleRequirementsCheck(exts),
		},
		logger: config.logger,
	}, nil
}

type moduleRequirementsCheck struct {
	name string
	exts *extenders.ExtendersStack
}

func newModuleRequirementsCheck(exts *extenders.ExtendersStack) *moduleRequirementsCheck {
	return &moduleRequirementsCheck{
		name: "deckhouse requirements check",
		exts: exts,
	}
}

func (c *moduleRequirementsCheck) GetName() string {
	return c.name
}

func (c *moduleRequirementsCheck) Verify(_ context.Context, mr *v1alpha1.ModuleRelease) error {
	err := c.exts.CheckModuleReleaseRequirements(mr.GetModuleName(), mr.GetName(), mr.GetVersion(), mr.GetModuleReleaseRequirements())
	if err != nil {
		return err
	}

	return nil
}

type migratedModulesCheck struct {
	name string

	k8sclient     client.Client
	metricStorage metricsstorage.Storage
	logger        *log.Logger
}

func newMigratedModulesCheck(k8sclient client.Client, metricStorage metricsstorage.Storage, logger *log.Logger) *migratedModulesCheck {
	return &migratedModulesCheck{
		name:          "migrated modules check",
		k8sclient:     k8sclient,
		metricStorage: metricStorage,
		logger:        logger,
	}
}

func (c *migratedModulesCheck) GetName() string {
	return c.name
}

func (c *migratedModulesCheck) Verify(ctx context.Context, dr *v1alpha1.DeckhouseRelease) error {
	c.metricStorage.Grouped().ExpireGroupMetrics(metrics.MigratedModuleNotFoundGroup)
	requirements := dr.GetRequirements()
	migratedModules, exists := requirements[MigratedModulesRequirementFieldName]
	if !exists || migratedModules == "" {
		return nil
	}

	modules := strings.Split(migratedModules, ",")
	for i, module := range modules {
		modules[i] = strings.TrimSpace(module)
	}

	modules = slices.DeleteFunc(modules, func(module string) bool {
		return module == ""
	})

	if len(modules) == 0 {
		return nil
	}

	c.logger.Debug("checking migrated modules", slog.Any("modules", modules))

	// Fetch ModuleConfigs and ModuleSources
	moduleList := &v1alpha1.ModuleList{}
	if err := c.k8sclient.List(ctx, moduleList); err != nil {
		return fmt.Errorf("failed to list Modules: %w", err)
	}

	moduleSources := &v1alpha1.ModuleSourceList{}
	if err := c.k8sclient.List(ctx, moduleSources); err != nil {
		return fmt.Errorf("failed to list ModuleSources: %w", err)
	}

	for _, moduleName := range modules {
		foundMS := false
		foundModule := false
		// Check if module exists in ModuleList and is disabled
		for _, module := range moduleList.Items {
			if module.Name == moduleName &&
				module.IsCondition(v1alpha1.ModuleConditionEnabledByModuleManager, corev1.ConditionFalse) {
				c.logger.Debug("migrated module is disabled by module manager", slog.String("module", moduleName))
				foundModule = true
				break
			}
		}
		if foundModule {
			continue
		}

		// If module is not in ModuleConfig or is enabled, check ModuleSource
		for _, source := range moduleSources.Items {
			if c.isModuleAvailableInSource(moduleName, &source) {
				foundMS = true
				c.logger.Debug("migrated module found in source", slog.String("module", moduleName), slog.String("sourceName", source.Name))
				break
			}
		}

		if !foundMS {
			c.logger.Warn("migrated module not found in any ModuleSource registry", slog.String("module", moduleName))
			c.setMigratedModuleNotFoundAlert(moduleName)

			return fmt.Errorf(`migrated module '%s' not found in any ModuleSource registry`, moduleName)
		}
	}

	c.logger.Debug("all migrated modules validation passed")

	return nil
}

// isModuleAvailableInSource checks if a module is available in a specific ModuleSource
func (c *migratedModulesCheck) isModuleAvailableInSource(moduleName string, source *v1alpha1.ModuleSource) bool {
	// Check if module is in the available modules list
	for _, availableModule := range source.Status.AvailableModules {
		if availableModule.Name == moduleName {
			// If there's a pull error, the module is not actually available
			return availableModule.Error == ""
		}
	}
	return false
}

// setMigratedModuleNotFoundAlert generates a Prometheus alert for missing migrated module
func (c *migratedModulesCheck) setMigratedModuleNotFoundAlert(moduleName string) {
	// Set the metric value to 1 to trigger alert
	c.metricStorage.Grouped().GaugeSet(
		metrics.MigratedModuleNotFoundGroup,
		metrics.MigratedModuleNotFoundMetricName,
		1,
		map[string]string{
			"module_name": moduleName,
		})
}
