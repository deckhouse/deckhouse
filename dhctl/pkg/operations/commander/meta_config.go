// Copyright 2023 Flant JSC
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

package commander

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/deckhouse/lib-dhctl/pkg/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
)

const clusterSettingsModuleConfigsCacheKey = "cluster-settings-module-configs"

// ParseMetaConfig parses commander-mode config. operation
// (infrastructureprovider.DhctlOperation*) reaches the provider validator,
// which skips bootstrap-only checks on other operations. globalOptions carries
// the server's download dir, so the external provider bundle is unpacked into
// the same directory the provider settings are later read from; passing nil
// would unpack it into the default dir and break a non-default --download-dir.
func ParseMetaConfig(ctx context.Context, stateCache state.Cache, params *CommanderModeParams, operation infrastructureprovider.DhctlOperation, kubeClient config.KubeClientGetter, globalOptions *options.GlobalOptions) (*config.MetaConfig, error) {
	clusterUUIDBytes, err := stateCache.Load(ctx, "uuid")
	if err != nil {
		return nil, fmt.Errorf("error loading cluster uuid from state cache: %w", err)
	}
	clusterUUID := string(clusterUUIDBytes)
	if clusterUUID == "" {
		return nil, fmt.Errorf("error loading cluster uuid from state cache: uuid is empty")
	}

	// Commander does not send registry access in the request; read it from the
	// target cluster and deliver the external provider bundle before parsing, so
	// the parse below finds it on disk and skips the registry-less download.
	if err := config.EnsureExternalProviderBundle(ctx, kubeClient, string(params.ClusterConfigurationData), globalOptions); err != nil {
		return nil, fmt.Errorf("ensure provider bundle from cluster: %w", err)
	}

	// Commander sends only ClusterConfiguration and the provider configuration. The cluster settings that
	// live in ModuleConfigs (prefix, network, Kubernetes version) are read from the cluster, the way the
	// CLI reads them.
	isDestroy := operation == infrastructureprovider.DhctlOperationDestroy
	var moduleConfigs []*config.ModuleConfig
	cached := false
	if isDestroy {
		moduleConfigs, cached, err = cachedClusterSettingsModuleConfigs(ctx, stateCache)
		if err != nil {
			return nil, err
		}
	}
	if !cached {
		moduleConfigs, err = readClusterSettingsModuleConfigs(ctx, kubeClient)
		if err != nil {
			return nil, err
		}
	}

	configData := fmt.Sprintf("%s\n---\n%s", params.ClusterConfigurationData, params.ProviderClusterConfigurationData)
	metaConfig, err := config.ParseConfigFromDataEnsureProvider(
		ctx,
		configData,
		infrastructureprovider.MetaConfigValidatorProvider(),
		globalOptions,
		config.ValidateOptionCommanderMode(true),
		config.ValidateOptionStrictUnmarshal(true),
		config.ValidateOptionValidateExtensions(true),
		config.ValidateOptionOperation(operation),
		config.ValidateOptionModuleConfigsFromCluster(moduleConfigs),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to parse config: %w", err)
	}
	metaConfig.UUID = clusterUUID

	// Destroy keeps them in the state: its retry after the masters are gone has no API to read them
	// from. Saved only once the parse accepted them, or a rejected ModuleConfig could not be corrected
	// in the cluster: the retry would read it from the state.
	if isDestroy && !cached {
		if err := stateCache.SaveStruct(ctx, clusterSettingsModuleConfigsCacheKey, moduleConfigs); err != nil {
			return nil, fmt.Errorf("save ModuleConfigs to state cache: %w", err)
		}
	}

	return metaConfig, nil
}

func cachedClusterSettingsModuleConfigs(ctx context.Context, stateCache state.Cache) ([]*config.ModuleConfig, bool, error) {
	cached, err := stateCache.InCache(ctx, clusterSettingsModuleConfigsCacheKey)
	if err != nil {
		return nil, false, fmt.Errorf("check ModuleConfigs in state cache: %w", err)
	}
	if !cached {
		return nil, false, nil
	}

	var mcs []*config.ModuleConfig
	if err := stateCache.LoadStruct(ctx, clusterSettingsModuleConfigsCacheKey, &mcs); err != nil {
		return nil, false, fmt.Errorf("load ModuleConfigs from state cache: %w", err)
	}
	return mcs, true, nil
}

// readClusterSettingsModuleConfigs reads them from the cluster, retrying transient API errors.
func readClusterSettingsModuleConfigs(ctx context.Context, kubeClient config.KubeClientGetter) ([]*config.ModuleConfig, error) {
	var mcs []*config.ModuleConfig
	loopParams := retry.NewEmptyParams(
		retry.WithName("Get ModuleConfigs %s from the cluster", strings.Join(config.ClusterSettingsModuleConfigs(), ", ")),
		retry.WithAttempts(50),
		retry.WithWait(time.Second),
		retry.WithWhitelist(config.ErrParseConfigTransient),
	)
	err := retry.NewLoopWithParams(loopParams).RunContext(ctx, func() error {
		kubeCl, err := kubeClient(ctx)
		if err != nil {
			return fmt.Errorf("get kube client: %w", err)
		}
		mcs, err = config.LoadClusterSettingsModuleConfigs(ctx, kubeCl)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("read ModuleConfigs from the cluster: %w", err)
	}
	return mcs, nil
}
