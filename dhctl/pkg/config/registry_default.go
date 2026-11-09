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
	"encoding/json"
	"errors"
	"fmt"

	dhlog "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-dhctl/pkg/logger"

	validatev1 "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/api/validate/v1"
	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config/registry"
)

const registryModuleName = "registry"

// addDefaultRegistryModuleConfig gives a configuration that names no registry anywhere the registry
// ModuleConfig of a CE installation: mode Managed, with the public CE registry as the upstream.
//
// The default used to be the deckhouse ModuleConfig's Direct mode, which belongs to the previous
// implementation of the module. The current one does not read it: it takes such a cluster for an
// unmanaged one, withdraws the node configuration the installer wrote, and leaves containerd pointed
// at the in-cluster registry address, which nothing serves after that. So the default is now stated
// in the current module's own terms, and the cluster gets exactly what that ModuleConfig written by
// the user would have given it: prepareRegistry reads it like any other.
//
// Only for a cluster dhctl creates the control plane of, on a container runtime the module can
// configure. A managed cluster such as EKS keeps the legacy Unmanaged CE default, which is
// prepareRegistry's own answer there. Converge and destroy read a cluster that already exists, and
// whatever it was installed with is not the installer's to restate.
func (m *MetaConfig) addDefaultRegistryModuleConfig(ctx context.Context, schemaStore *SchemaStore) error {
	needed, err := m.needsDefaultRegistry()
	if err != nil || !needed {
		return err
	}

	mc, err := buildModuleConfig(ctx, schemaStore, registryModuleName, true, registry.DefaultModuleSettings())
	if errors.Is(err, ErrSchemaNotFound) {
		// The modules here have no registry module, so neither would a ModuleConfig of the user's
		// configure anything. The legacy default is the answer for such a set of modules, and the
		// parse must not fail on it: the d8-cluster-configuration webhook runs it on every edit.
		dhlog.FromContext(ctx).DebugContext(ctx, "No registry module schema, keeping the legacy registry default")
		return nil
	}
	if err != nil {
		return fmt.Errorf("build the default registry ModuleConfig: %w", err)
	}

	m.ModuleConfigs = append(m.ModuleConfigs, mc)
	m.registryDefaulted = true

	return nil
}

// needsDefaultRegistry reports whether the configuration names no registry anywhere: neither the
// registry fields of InitConfiguration.deckhouse, nor the registry section of the deckhouse
// ModuleConfig, nor a registry ModuleConfig.
func (m *MetaConfig) needsDefaultRegistry() (bool, error) {
	if !m.installerDecidesRegistry() || m.FindModuleConfig(registryModuleName) != nil {
		return false, nil
	}

	named, err := m.registryNamedOutsideModule()
	if err != nil {
		return false, err
	}

	return !named, nil
}

// ErrNoRegistryToPullFrom is the reason refuseRegistryModuleWithoutRegistry gives.
var ErrNoRegistryToPullFrom = errors.New("no registry is given anywhere else, so there is nowhere to pull images from")

// ErrRegistrySettingsWithoutManaged is the reason refuseRegistryModuleWithoutRegistry gives for
// settings the module's schema accepts only together with mode: Managed.
var ErrRegistrySettingsWithoutManaged = errors.New("the registry ModuleConfig sets primary or storage without mode: Managed")

// refuseRegistryModuleWithoutRegistry refuses a registry ModuleConfig that leaves the pull path to
// someone else, Unmanaged or disabled, when nobody else names a registry: neither
// InitConfiguration.deckhouse nor the deckhouse ModuleConfig. Unmanaged is also what a ModuleConfig
// with no mode means.
//
// It also refuses primary or storage without mode: Managed, which the module's schema rejects. The
// installer would otherwise read such a ModuleConfig as Managed and pull through it, and then create
// it in a cluster that refuses it.
//
// Such a configuration used to get the legacy default, the deckhouse ModuleConfig's Direct mode on the
// CE registry, for the same reason and with the same result as the one addDefaultRegistryModuleConfig
// replaces: the current module does not read that section, and the nodes are left pointed at the
// in-cluster registry address, which nothing serves. Unlike that case, the default cannot be stated in
// the module's terms here, because the user turned the module's management off. So the installer asks
// which of the two the user meant.
//
// The same scope as the default: a cluster dhctl creates the control plane of, on a container runtime
// the module can configure, and not converge or destroy. Those read a cluster that exists, and refusing
// whatever it was installed with would leave it without them. Narrower than the default in one way: a
// commander parse with no operation is not refused either. Those are commander's ValidateResources and
// ConfigExtender calls and its cache-path lookups, which may be about a cluster that exists, and where
// the default only adds a ModuleConfig the refusal would fail them. A commander installation is still
// refused, by the bootstrap's own parse, which gives the operation.
func (m *MetaConfig) refuseRegistryModuleWithoutRegistry(commanderMode bool) error {
	if !m.installerDecidesRegistry() || (commanderMode && m.Operation == "") {
		return nil
	}

	mc := m.FindModuleConfig(registryModuleName)
	if mc == nil {
		return nil
	}

	if settingsNeedManaged(mc.Spec.Settings) {
		return fmt.Errorf(
			"%w, and the module's schema refuses that: in the default Unmanaged mode the module manages "+
				"nothing and these settings would have no effect. Set mode: Managed, or remove primary and storage",
			ErrRegistrySettingsWithoutManaged,
		)
	}

	var state string
	switch {
	case mc.Spec.Enabled != nil && !*mc.Spec.Enabled:
		state = "disabled"
	case moduleUnmanaged(mc.Spec.Settings):
		state = registryModeUnmanaged
	default:
		return nil
	}

	named, err := m.registryNamedOutsideModule()
	if err != nil || named {
		return err
	}

	return fmt.Errorf(
		"the registry module is %s in its ModuleConfig and %w. Either name the registry the nodes pull "+
			"from directly in InitConfiguration.deckhouse: imagesRepo, with registryDockerCfg for its "+
			"credentials; or set the registry ModuleConfig to enabled: true and mode: Managed with "+
			"primary.upstream, the registry to pull from (a private or mirror registry the images were "+
			"pushed to with d8 mirror push will do). To install an air-gapped cluster from an image "+
			"bundle instead, set the registry ModuleConfig to enabled: true and mode: Managed with "+
			"storage.cache: true and storage.source and no primary.upstream, and pass the bundle with "+
			"--img-bundle-path",
		state, ErrNoRegistryToPullFrom,
	)
}

const (
	// registryModeUnmanaged is the registry ModuleConfig's mode that leaves the pull path to the user.
	registryModeUnmanaged = "Unmanaged"
	// registryModeManaged is the registry ModuleConfig's mode in which the module owns the pull path.
	registryModeManaged = "Managed"
)

// moduleUnmanaged reports whether the registry ModuleConfig's settings leave the pull path to the user:
// mode Unmanaged, or no mode at all, which the module's schema defaults to Unmanaged and dhctl does not
// fill in.
func moduleUnmanaged(settings SettingsValues) bool {
	mode, set := settings["mode"]

	return !set || mode == registryModeUnmanaged
}

// settingsNeedManaged reports whether the registry ModuleConfig's settings break the module schema's
// rule that primary and storage come only with mode: Managed. The schema checks it on the raw settings,
// before its default for mode applies, so a missing mode counts as Unmanaged here too.
func settingsNeedManaged(settings SettingsValues) bool {
	_, primary := settings["primary"]
	_, storage := settings["storage"]

	return (primary || storage) && settings["mode"] != registryModeManaged
}

// installerDecidesRegistry reports whether the registry of this configuration is the installer's to
// default or refuse: a new cluster dhctl creates the control plane of, on a container runtime the
// registry module can configure.
func (m *MetaConfig) installerDecidesRegistry() bool {
	switch m.Operation {
	case string(validatev1.OperationConverge), string(validatev1.OperationDestroy):
		return false
	}

	return m.HasClusterConfiguration() && registry_const.IsCRISupported(registry_const.CRIType(m.effectiveDefaultCRI()))
}

// registryNamedOutsideModule reports whether a registry is named outside the registry ModuleConfig:
// in the registry fields of InitConfiguration.deckhouse or in the registry section of the deckhouse
// ModuleConfig.
func (m *MetaConfig) registryNamedOutsideModule() (bool, error) {
	if len(m.InitClusterConfig) > 0 {
		rawJSON, err := json.Marshal(m.InitClusterConfig)
		if err != nil {
			return false, err
		}
		initConfig, err := registry.ParseJSONInitConfig(rawJSON)
		if err != nil || initConfig != nil {
			return initConfig != nil, err
		}
	}

	if mc := m.FindModuleConfig("deckhouse"); mc != nil {
		rawJSON, err := json.Marshal(mc)
		if err != nil {
			return false, err
		}
		deckhouseSettings, err := registry.ParseJSONDeckhouseMC(rawJSON)
		if err != nil || deckhouseSettings != nil {
			return deckhouseSettings != nil, err
		}
	}

	return false, nil
}

// RegistryDefaulted reports whether the registry ModuleConfig was added by the installer because the
// configuration named no registry.
func (m *MetaConfig) RegistryDefaulted() bool {
	return m != nil && m.registryDefaulted
}

// WarnAboutDefaultRegistry says out loud which registry and edition an installation with no registry
// settings goes to. Said by the bootstrap, where the ModuleConfigs become what is installed, rather
// than while parsing: the same parse, with no operation given, also serves commander's cache-path
// lookups for check, converge, destroy and detach, the d8-cluster-configuration webhook and dhctl
// config parse, none of which installs anything. Nor in PrepareDeckhouseInstallConfig: dhctl
// deckhouse create-deployment shares it and creates only the Deployment, not the ModuleConfig the
// warning announces.
func WarnAboutDefaultRegistry(ctx context.Context, m *MetaConfig) {
	if !m.RegistryDefaulted() {
		return
	}

	dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf(
		"No registry settings were given: installing Deckhouse CE from %s through the registry module "+
			"(mode: Managed). The registry ModuleConfig with this upstream is created in the cluster. "+
			"To install another edition or from another registry, add a registry ModuleConfig with "+
			"mode: Managed and primary.upstream set to that registry and its credentials.",
		registry_const.DefaultImagesRepo,
	))
}
