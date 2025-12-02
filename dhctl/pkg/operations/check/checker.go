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

package check

import (
	"context"
	"fmt"
	"reflect"

	"github.com/google/uuid"
	"github.com/name212/govalue"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/entity"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	dhctlstate "github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
)

type Params struct {
	SSHClient     node.SSHClient
	StateCache    dhctlstate.Cache
	CommanderMode bool
	CommanderUUID uuid.UUID
	*commander.CommanderModeParams

	InfrastructureContext *infrastructure.Context

	KubeClient *client.KubernetesClient // optional

	TmpDir  string
	Logger  log.Logger
	IsDebug bool
}

type Cleaner func() error

type Checker struct {
	*Params

	logger log.Logger
}

func NewChecker(params *Params) *Checker {
	logger := params.Logger
	if govalue.IsNil(logger) {
		logger = log.GetDefaultLogger()
	}

	if !params.CommanderMode {
		panic("check operation currently supported only in commander mode")
	}

	// FIXME(dhctl-for-commander): commander uuid currently optional, make it required later
	// if params.CommanderUUID == uuid.Nil {
	//	panic("CommanderUUID required for check operation in commander mode!")
	// }

	return &Checker{
		Params: params,
		logger: logger,
	}
}

func (c *Checker) Check(ctx context.Context) (*CheckResult, Cleaner, error) {
	cleaner := func() error {
		return nil
	}

	kubeCl, err := c.GetKubeClient(ctx)
	if err != nil {
		return nil, cleaner, err
	}

	metaConfig, err := commander.ParseMetaConfig(ctx, c.StateCache, c.Params.CommanderModeParams, c.logger)
	if err != nil {
		return nil, cleaner, fmt.Errorf("unable to parse meta configuration: %w", err)
	}

	if c.InfrastructureContext == nil {
		providerGetter := infrastructureprovider.CloudProviderGetter(infrastructureprovider.CloudProviderGetterParams{
			TmpDir:           c.TmpDir,
			AdditionalParams: cloud.ProviderAdditionalParams{},
			Logger:           c.logger,
			IsDebug:          c.IsDebug,
		})

		c.InfrastructureContext = infrastructure.NewContextWithProvider(providerGetter, c.logger)
	}

	provider, err := c.InfrastructureContext.CloudProviderGetter()(ctx, metaConfig)
	if err != nil {
		return nil, cleaner, err
	}

	cleaner = func() error {
		return provider.Cleanup()
	}

	res := &CheckResult{
		Status: CheckStatusInSync,
	}

	if c.CommanderMode {
		shouldUpdate, err := commander.CheckShouldUpdateCommanderUUID(ctx, kubeCl, c.CommanderUUID)
		if err != nil {
			return nil, cleaner, fmt.Errorf("uuid consistency check failed: %w", err)
		}
		if shouldUpdate {
			res.Status = res.Status.CombineStatus(CheckStatusOutOfSync)
		}
	}

	hasTerraformState := false

	if metaConfig.ClusterType == config.CloudClusterType {
		resInfra, err := c.checkInfra(ctx, kubeCl, metaConfig, c.InfrastructureContext)
		if err != nil {
			return nil, cleaner, fmt.Errorf("unable to check infra state: %w", err)
		}
		res.Status = res.Status.CombineStatus(resInfra.Status)

		if resInfra.Status == CheckStatusDestructiveOutOfSync {
			res.DestructiveChangeID, err = DestructiveChangeID(resInfra.Statistics)
			if err != nil {
				return nil, cleaner, fmt.Errorf("unable to generate destructive change id: %w", err)
			}
		}

		hasTerraformState = resInfra.HasTerraformState
		res.StatusDetails.Statistics = *resInfra.Statistics
		res.StatusDetails.OpentofuMigrationStatus = resInfra.MigrationOpentofuStatus
	}

	configurationStatus, err := c.checkConfiguration(ctx, kubeCl, metaConfig)
	if err != nil {
		return nil, cleaner, fmt.Errorf("unable to check configuration state: %w", err)
	}
	res.Status = res.Status.CombineStatus(configurationStatus)
	res.StatusDetails.ConfigurationStatus = configurationStatus
	res.HasTerraformState = hasTerraformState

	return res, cleaner, nil
}

const (
	clusterConfigKind = "cluster config"
)

func (c *Checker) checkConfiguration(ctx context.Context, kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig) (CheckStatus, error) {
	const (
		commanderSource = "commander"
		inClusterSource = "in-cluster"
	)

	clusterConfig, err := getClusterConfig(metaConfig, commanderSource)
	if err != nil {
		return "", err
	}

	// we use static or provider config because commander does not support managed cluster
	staticOrProviderClusterConfig, err := getClusterSpecificConfig(ctx, metaConfig, commanderSource)
	if err != nil {
		return "", fmt.Errorf("Unable to get static/provider cluster config: %w", err)
	}

	inClusterMetaConfig, err := entity.GetMetaConfig(ctx, kubeCl, c.logger)
	if err != nil {
		return "", fmt.Errorf("Unable to get in-cluster meta config: %w", err)
	}

	inClusterConfig, err := getClusterConfig(inClusterMetaConfig, inClusterSource)
	if err != nil {
		return "", err
	}

	// we use static or provider config because commander does not support managed cluster
	inClusterStaticOrProviderClusterConfig, err := getClusterSpecificConfig(ctx, inClusterMetaConfig, inClusterSource)
	if err != nil {
		return "", fmt.Errorf("Unable to get in-cluster static/provider cluster config yaml: %w", err)
	}

	checks := []checkFunc{
		equalByOperatorCheck(metaConfig.UUID, inClusterMetaConfig.UUID, "cluster UUID"),
		equalMapByDeepEqualFuncCheck(clusterConfig, inClusterConfig, clusterConfigKind),
		equalMapByDeepEqualFuncCheck(staticOrProviderClusterConfig, inClusterStaticOrProviderClusterConfig, "provider configuration"),
	}

	syncStatus := CheckStatusInSync

	for _, check := range checks {
		if err := check(); err != nil {
			syncStatus = CheckStatusOutOfSync
			c.logger.LogInfoLn(err.Error())
		}
	}

	return syncStatus, nil
}

type InfraResult struct {
	Status                  CheckStatus
	Statistics              *Statistics
	HasTerraformState       bool
	MigrationOpentofuStatus CheckStatus
}

func (c *Checker) checkInfra(ctx context.Context, kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig, infrastructureContext *infrastructure.Context) (*InfraResult, error) {
	stat, hasTerraformState, err := CheckState(
		ctx, kubeCl, metaConfig, infrastructureContext,
		CheckStateOptions{
			CommanderMode: c.CommanderMode,
			StateCache:    c.StateCache,
		},
		false,
	)
	if err != nil {
		return nil, err
	}

	checkStatus := CheckStatusInSync

	if stat != nil {
		for _, node := range stat.Node {
			checkStatus = checkStatus.CombineStatus(resolveStatisticsStatus(node.Status))
		}
		for _, nodeTempl := range stat.NodeTemplates {
			checkStatus = checkStatus.CombineStatus(resolveStatisticsStatus(nodeTempl.Status))
		}
		checkStatus = checkStatus.CombineStatus(resolveStatisticsStatus(stat.Cluster.Status))
	}

	migrateToTofuStatus := CheckStatusInSync

	providerGetter := infrastructureContext.CloudProviderGetter()
	if providerGetter == nil {
		return nil, fmt.Errorf("Infrastructure context does not have a provider getter")
	}

	provider, err := providerGetter(ctx, metaConfig)
	if err != nil {
		return nil, err
	}

	if provider.NeedToUseTofu() && hasTerraformState {
		checkStatus = checkStatus.CombineStatus(CheckStatusOutOfSync)
		migrateToTofuStatus = CheckStatusOutOfSync
	}

	return &InfraResult{
		Status:                  checkStatus,
		Statistics:              stat,
		HasTerraformState:       hasTerraformState,
		MigrationOpentofuStatus: migrateToTofuStatus,
	}, nil
}

func resolveStatisticsStatus(status string) CheckStatus {
	switch status {
	case OKStatus:
		return CheckStatusInSync
	case ChangedStatus:
		// NOTE: Regular out-of-sync state, which can be fixed by the converge run
		return CheckStatusOutOfSync
	case DestructiveStatus:
		// NOTE: Something will be destroyed by the converge run, such change should be approved
		return CheckStatusDestructiveOutOfSync
	case AbandonedStatus:
		// NOTE: Excess node — treat as destructive out-of-sync, because this node will be destroyed during converge run
		return CheckStatusDestructiveOutOfSync
	case AbsentStatus:
		// NOTE: Lost node — treat as out-of-sync for now
		return CheckStatusOutOfSync
	case ErrorStatus:
		// NOTE: Unknown error, probably can be healed by the retry
		return CheckStatusDestructiveOutOfSync
	}
	panic(fmt.Sprintf("unknown check infra status: %q", status))
}

func (c *Checker) GetKubeClient(ctx context.Context) (*client.KubernetesClient, error) {
	if c.KubeClient != nil {
		return c.KubeClient, nil
	}

	kubeCl, err := kubernetes.ConnectToKubernetesAPI(ctx, ssh.NewNodeInterfaceWrapper(c.SSHClient))
	if err != nil {
		return nil, fmt.Errorf("unable to connect to kubernetes api over ssh: %w", err)
	}
	return kubeCl, nil
}

type configTypeForCompare map[string]any

type clusterSpecificConfigProvider struct {
	kind string
}

func (f *clusterSpecificConfigProvider) Cloud(_ context.Context, metaConfig *config.MetaConfig) (configTypeForCompare, error) {
	providerConfig, err := metaConfig.ProviderClusterConfigYAML()
	if err != nil {
		return nil, err
	}

	// provider cluster config should present
	return unmarshalToCompare(providerConfig, false, "provider cluster config", f.kind)
}

func (f *clusterSpecificConfigProvider) Static(_ context.Context, metaConfig *config.MetaConfig) (configTypeForCompare, error) {
	staticConfig, err := metaConfig.StaticClusterConfigYAML()
	if err != nil {
		return nil, err
	}

	// empty static configuration is ok because we have auto discovery
	return unmarshalToCompare(staticConfig, true, "static cluster config", f.kind)
}

func (f *clusterSpecificConfigProvider) Incorrect(_ context.Context, metaConfig *config.MetaConfig) (configTypeForCompare, error) {
	return nil, config.UnsupportedClusterTypeErr(metaConfig)
}

func getClusterConfig(metaConfig *config.MetaConfig, stateSource string) (configTypeForCompare, error) {
	clusterConfigData, err := metaConfig.ClusterConfigYAML()
	if err != nil {
		return nil, fmt.Errorf("Unable to get '%s' yaml from '%s': %w", clusterConfigKind, stateSource, err)
	}

	// commander does not support managed cluster, cluster config should present
	return unmarshalToCompare(clusterConfigData, false, clusterConfigKind, stateSource)
}

func getClusterSpecificConfig(ctx context.Context, metaConfig *config.MetaConfig, kind string) (configTypeForCompare, error) {
	return config.DoByClusterType(ctx, metaConfig, &clusterSpecificConfigProvider{kind: kind})
}

func handleEmpty(canBeEmpty bool, kind, stateSource string) (configTypeForCompare, error) {
	// always return nil to prevent compare empty map and nil
	if canBeEmpty {
		return nil, nil
	}

	return nil, fmt.Errorf("'%s' for '%s' cannot be empty", kind, stateSource)
}

// unmarshalToCompare
// we need marshal and unmarshal to map[string]any because json.RawMessage is []byte
// and it can ba marshal in random order in every time
// thus we need to compare two maps
func unmarshalToCompare(content []byte, canBeEmpty bool, kind, stateSource string) (configTypeForCompare, error) {
	if len(content) == 0 {
		return handleEmpty(canBeEmpty, kind, stateSource)
	}

	var res configTypeForCompare
	err := yaml.Unmarshal(content, &res)
	if err != nil {
		return nil, fmt.Errorf("Cannot unmarshal '%s' from '%s': %w", kind, stateSource, err)
	}

	if len(res) == 0 {
		return handleEmpty(canBeEmpty, kind, stateSource)
	}

	return res, nil
}

type checkFunc func() error

func checkError(kind string) error {
	return fmt.Errorf("Commander state meta config %s does not equal in-cluster meta config %s", kind, kind)
}

func equalMapByDeepEqualFuncCheck(expected, data configTypeForCompare, kind string) checkFunc {
	return func() error {
		if reflect.DeepEqual(expected, data) {
			return nil
		}

		return checkError(kind)
	}
}

func equalByOperatorCheck[T comparable](expected, val T, kind string) checkFunc {
	return func() error {
		if expected == val {
			return nil
		}

		return checkError(kind)
	}
}
