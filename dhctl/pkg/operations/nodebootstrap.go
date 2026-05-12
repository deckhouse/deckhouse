// Copyright 2021 Flant JSC
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

package operations

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/hashicorp/go-multierror"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/entity"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	infrastructurestate "github.com/deckhouse/deckhouse/dhctl/pkg/state/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

func IsSequentialNodesBootstrap(cfg *config.MetaConfig) bool {
	seqEnv := os.Getenv("DHCTL_PARALLEL_CLOUD_PERMANENT_NODES_BOOTSTRAP")
	// vcd doesn't support parallel creating nodes in same vapp
	// https://github.com/vmware/terraform-provider-vcd/issues/530
	return seqEnv == "false" || cfg.ProviderName == "vcd"
}

func NodeName(cfg *config.MetaConfig, nodeGroupName string, index int) string {
	return fmt.Sprintf("%s-%s-%d", cfg.ClusterPrefix, nodeGroupName, index)
}

func BootstrapAdditionalNode(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
	cfg *config.MetaConfig,
	index int,
	step infrastructure.Step,
	nodeGroupName, cloudConfig string,
	infrastructureContext *infrastructure.Context,
) error {
	nodeName := NodeName(cfg, nodeGroupName, index)

	err := checkNodeResourceExistsInClusterDuringBootstrap(ctx, checkNodeParams{
		node: infrastructurestate.HasNodeStateInClusterParams{
			NodeGroup: nodeGroupName,
			Name:      nodeName,
		},

		kubeCl: kubeCl,
		logger: log.GetDefaultLogger(),
	})

	if err != nil {
		return err
	}

	nodeGroupSettings := cfg.FindTerraNodeGroup(nodeGroupName)

	// TODO pass cache as argument or better refact func
	runner, err := infrastructureContext.GetBootstrapNodeRunner(ctx, cfg, cache.Global(), infrastructure.BootstrapNodeRunnerOptions{
		NodeName:        nodeName,
		NodeGroupStep:   step,
		NodeGroupName:   nodeGroupName,
		NodeIndex:       index,
		NodeCloudConfig: cloudConfig,
		AdditionalStateSaverDestinations: []infrastructure.SaverDestination{
			infrastructurestate.NewNodeStateSaver(kubernetes.NewSimpleKubeClientGetter(kubeCl), nodeName, nodeGroupName, nodeGroupSettings),
		},
		RunnerLogger: log.GetDefaultLogger(),
	})
	if err != nil {
		return err
	}

	outputs, err := infrastructure.ApplyPipeline(ctx, runner, nodeName, infrastructure.OnlyState)
	if err != nil {
		return err
	}

	if tomb.IsInterrupted() {
		return global.ErrConvergeInterrupted
	}

	err = infrastructurestate.SaveNodeInfrastructureState(ctx, kubeCl, nodeName, nodeGroupName, outputs.InfrastructureState, nodeGroupSettings, log.GetDefaultLogger())
	if err != nil {
		return err
	}

	return nil
}

func BootstrapSequentialTerraNodes(ctx context.Context, kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig, terraNodeGroups []config.TerraNodeGroupSpec, infrastructureContext *infrastructure.Context) error {
	for _, ng := range terraNodeGroups {
		err := log.ProcessCtx(ctx, "bootstrap", fmt.Sprintf("Create %s NodeGroup", ng.Name), func(ctx context.Context) error {
			err := entity.CreateNodeGroup(ctx, kubeCl, ng.Name, log.GetDefaultLogger(), metaConfig.NodeGroupManifest(ng))
			if err != nil {
				return err
			}

			cloudConfig, err := entity.GetCloudConfig(ctx, kubeCl, ng.Name, global.ShowDeckhouseLogs, log.GetDefaultLogger())
			if err != nil {
				return err
			}

			for i := 0; i < ng.Replicas; i++ {
				err = BootstrapAdditionalNode(ctx, kubeCl, metaConfig, i, infrastructure.StaticNodeStep, ng.Name, cloudConfig, infrastructureContext)
				if err != nil {
					return err
				}
			}
			return nil
		})

		if err != nil {
			return err
		}
	}
	return nil
}

func BootstrapAdditionalNodeForParallelRun(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
	cfg *config.MetaConfig,
	index int,
	step infrastructure.Step,
	nodeGroupName, cloudConfig string,
	infrastructureContext *infrastructure.Context,
	runnerLogger log.Logger,
) error {
	nodeName := NodeName(cfg, nodeGroupName, index)
	nodeGroupSettings := cfg.FindTerraNodeGroup(nodeGroupName)
	// TODO pass cache as argument or better refact func
	runner, err := infrastructureContext.GetBootstrapNodeRunner(ctx, cfg, cache.Global(), infrastructure.BootstrapNodeRunnerOptions{
		NodeName:        nodeName,
		NodeGroupStep:   step,
		NodeGroupName:   nodeGroupName,
		NodeIndex:       index,
		NodeCloudConfig: cloudConfig,
		AdditionalStateSaverDestinations: []infrastructure.SaverDestination{
			infrastructurestate.NewNodeStateSaver(kubernetes.NewSimpleKubeClientGetter(kubeCl), nodeName, nodeGroupName, nodeGroupSettings),
		},
		RunnerLogger: runnerLogger,
		// allow use state cache because in parallel run we cannot get correct output from user
		AllowUseStateCache: true,
	})

	if err != nil {
		return err
	}

	outputs, err := infrastructure.ApplyPipeline(ctx, runner, nodeName, infrastructure.OnlyState)
	if err != nil {
		return err
	}

	if tomb.IsInterrupted() {
		return global.ErrConvergeInterrupted
	}

	err = infrastructurestate.SaveNodeInfrastructureState(ctx, kubeCl, nodeName, nodeGroupName, outputs.InfrastructureState, nodeGroupSettings, runnerLogger)
	if err != nil {
		return err
	}

	return nil
}

func ParallelBootstrapAdditionalNodes(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
	cfg *config.MetaConfig,
	nodesIndexToCreate []int,
	step infrastructure.Step,
	nodeGroupName, cloudConfig string,
	infrastructureContext *infrastructure.Context,
	ngLogger log.Logger,
	saveLogToBuffer bool,
) ([]string, error) {
	var (
		nodesToWait []string
		wg          sync.WaitGroup
		mu          sync.Mutex
	)

	type checkResult struct {
		name        string
		buffNodeLog *bytes.Buffer
		err         error
	}

	var nodesCheckErrors *multierror.Error

	for _, indexCandidate := range nodesIndexToCreate {
		candidateName := NodeName(cfg, nodeGroupName, indexCandidate)

		err := checkNodeResourceExistsInClusterDuringBootstrap(ctx, checkNodeParams{
			node: infrastructurestate.HasNodeStateInClusterParams{
				NodeGroup: nodeGroupName,
				Name:      candidateName,
			},

			kubeCl: kubeCl,
			logger: ngLogger,
		})

		if err != nil {
			nodesCheckErrors = multierror.Append(nodesCheckErrors, err)
		}
	}

	if err := nodesCheckErrors.ErrorOrNil(); err != nil {
		return nil, fmt.Errorf("Check existing nodes in cluster error: %w", err)
	}

	if len(nodesIndexToCreate) > 1 && !saveLogToBuffer {
		ngLogger.LogWarnF("Many pipelines will run in parallel, infrastructure utility output for nodes %s-%v will be displayed after main execution.\n\n", nodeGroupName, nodesIndexToCreate[1:])
	}

	resultsChan := make(chan checkResult, len(nodesIndexToCreate))
	for i, indexCandidate := range nodesIndexToCreate {
		candidateName := fmt.Sprintf("%s-%s-%v", cfg.ClusterPrefix, nodeGroupName, indexCandidate)
		wg.Add(1)
		go func(i, indexCandidate int, candidateName string, logger log.Logger, saveLogToBuffer bool) {
			defer wg.Done()
			var buffNodeLog bytes.Buffer
			var nodeLogger log.Logger

			nodeLogger = logger.CreateBufferLogger(&buffNodeLog)
			if i == 0 && !saveLogToBuffer {
				nodeLogger = logger
			}
			err := BootstrapAdditionalNodeForParallelRun(
				ctx,
				kubeCl,
				cfg,
				indexCandidate,
				step,
				nodeGroupName,
				cloudConfig,
				infrastructureContext,
				nodeLogger,
			)

			resultsChan <- checkResult{
				name:        candidateName,
				buffNodeLog: &buffNodeLog,
				err:         err,
			}
			mu.Lock()
			nodesToWait = append(nodesToWait, candidateName)
			mu.Unlock()
		}(i, indexCandidate, candidateName, ngLogger, saveLogToBuffer)
	}

	wg.Wait()
	close(resultsChan)

	var bootstrapErrors *multierror.Error

	for candidate := range resultsChan {
		if candidate.err != nil {
			bootstrapErrors = multierror.Append(
				bootstrapErrors,
				fmt.Errorf("Node %s error: %w", candidate.name, candidate.err),
			)
			// always output from logger
		}

		if candidate.buffNodeLog.Len() == 0 {
			continue
		}

		ngLogger.LogInfoF("Output for node %s:\n", candidate.name)

		scanner := bufio.NewScanner(candidate.buffNodeLog)
		for scanner.Scan() {
			ngLogger.LogInfoLn((scanner.Text()))
		}
	}

	if err := bootstrapErrors.ErrorOrNil(); err != nil {
		return nodesToWait, err
	}

	return nodesToWait, nil
}

func ParallelCreateNodeGroup(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
	metaConfig *config.MetaConfig,
	terraNodeGroups []config.TerraNodeGroupSpec,
	infrastructureContext *infrastructure.Context,
) error {
	msg := "Create NodeGroups "
	for _, group := range terraNodeGroups {
		msg += fmt.Sprintf("%s (replicas: %v)️; ", group.Name, group.Replicas)
	}

	return log.ProcessCtx(ctx, "converge", msg, func(ctx context.Context) error {
		var (
			mu sync.Mutex
			wg sync.WaitGroup
		)
		type checkResult struct {
			name    string
			buffLog *bytes.Buffer
			err     error
		}
		currentLogger := log.GetDefaultLogger()

		ngWaitMap := make(map[string]int)
		resultsChan := make(chan checkResult, len(terraNodeGroups))
		for i, group := range terraNodeGroups {
			wg.Add(1)
			go func(i int, group config.TerraNodeGroupSpec) {
				defer wg.Done()

				var (
					buffNGLog       bytes.Buffer
					ngLogger        log.Logger
					saveLogToBuffer bool
				)

				if i == 0 {
					saveLogToBuffer = false
					ngLogger = currentLogger
				} else {
					saveLogToBuffer = true
					ngLogger = currentLogger.CreateBufferLogger(&buffNGLog)
				}

				err := entity.CreateNodeGroup(ctx, kubeCl, group.Name, ngLogger, metaConfig.NodeGroupManifest(group))
				if err != nil {
					resultsChan <- checkResult{
						name:    group.Name,
						buffLog: &buffNGLog,
						err:     err,
					}
					return
				}

				nodeCloudConfig, err := entity.GetCloudConfig(ctx, kubeCl, group.Name, global.ShowDeckhouseLogs, ngLogger)
				if err != nil {
					resultsChan <- checkResult{
						name:    group.Name,
						buffLog: &buffNGLog,
						err:     err,
					}
					return
				}

				var nodesIndexToCreate []int
				for i := 0; i < group.Replicas; i++ {
					nodesIndexToCreate = append(nodesIndexToCreate, i)
				}

				_, err = ParallelBootstrapAdditionalNodes(ctx, kubeCl, metaConfig, nodesIndexToCreate, infrastructure.StaticNodeStep, group.Name, nodeCloudConfig, infrastructureContext, ngLogger, saveLogToBuffer)

				resultsChan <- checkResult{
					name:    group.Name,
					buffLog: &buffNGLog,
					err:     err,
				}
				mu.Lock()
				ngWaitMap[group.Name] = group.Replicas
				mu.Unlock()
			}(i, group)
		}

		wg.Wait()
		close(resultsChan)

		var bootstrapErrors *multierror.Error

		for ng := range resultsChan {
			if ng.err != nil {
				bootstrapErrors = multierror.Append(
					bootstrapErrors,
					fmt.Errorf("Node group %s errors:\n%w", ng.name, ng.err),
				)
				// always output from logger
			}

			if ng.buffLog.Len() == 0 {
				continue
			}
			currentPLogger := log.GetProcessLogger()
			currentPLogger.LogProcessStart(fmt.Sprintf("Output NG [%s] log", ng.name))
			scanner := bufio.NewScanner(ng.buffLog)
			for scanner.Scan() {
				log.InfoLn(scanner.Text())
			}
			currentPLogger.LogProcessEnd()
		}

		if err := bootstrapErrors.ErrorOrNil(); err != nil {
			return err
		}

		return entity.WaitForNodesBecomeReady(ctx, kubeCl, ngWaitMap)
	})
}

func BootstrapAdditionalMasterNode(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
	cfg *config.MetaConfig,
	index int,
	cloudConfig string,
	infrastructureContext *infrastructure.Context,
) (*infrastructure.PipelineOutputs, error) {
	nodeGroupName := global.MasterNodeGroupName
	nodeName := NodeName(cfg, nodeGroupName, index)

	err := checkNodeResourceExistsInClusterDuringBootstrap(ctx, checkNodeParams{
		node: infrastructurestate.HasNodeStateInClusterParams{
			NodeGroup: nodeGroupName,
			Name:      nodeName,
		},

		kubeCl: kubeCl,
		logger: log.GetDefaultLogger(),
	})

	if err != nil {
		return nil, err
	}

	// TODO pass cache as argument or better refact func
	runner, err := infrastructureContext.GetBootstrapNodeRunner(ctx, cfg, cache.Global(), infrastructure.BootstrapNodeRunnerOptions{
		NodeName:        nodeName,
		NodeGroupStep:   infrastructure.MasterNodeStep,
		NodeGroupName:   global.MasterNodeGroupName,
		NodeIndex:       index,
		NodeCloudConfig: cloudConfig,
		AdditionalStateSaverDestinations: []infrastructure.SaverDestination{
			infrastructurestate.NewNodeStateSaver(kubernetes.NewSimpleKubeClientGetter(kubeCl), nodeName, global.MasterNodeGroupName, nil),
		},
		RunnerLogger: log.GetDefaultLogger(),
	})
	if err != nil {
		return nil, err
	}

	outputs, err := infrastructure.ApplyPipeline(ctx, runner, nodeName, infrastructure.GetMasterNodeResult)
	if err != nil {
		return nil, err
	}

	if tomb.IsInterrupted() {
		return nil, global.ErrConvergeInterrupted
	}

	err = infrastructurestate.SaveMasterNodeInfrastructureState(ctx, kubeCl, nodeName, outputs.InfrastructureState, []byte(outputs.KubeDataDevicePath))
	if err != nil {
		return outputs, err
	}

	return outputs, err
}

type checkNodeParams struct {
	kubeCl *client.KubernetesClient
	node   infrastructurestate.HasNodeStateInClusterParams
	logger log.Logger
}

func checkNodeResourceExistsInClusterDuringBootstrap(ctx context.Context, params checkNodeParams) error {
	kubeCl := params.kubeCl
	nodeName := params.node.Name
	logger := params.logger

	hasState, err := infrastructurestate.HasNodeStateInCluster(ctx, kubeCl, params.node)
	if err != nil {
		return fmt.Errorf("Cannot check that state in cluster for %s: %w", nodeName, err)
	}

	if hasState {
		// we skip in because we need check node only when state not in cluster
		// during bootstrap we always call bootstrap additional nodes
		// and if client restart bootstrap we can get situation:
		// - infra utility creates partially vm
		// - but vm was registered in cluster
		// this case could happen in dvp:
		// - infra utility creates vm
		// - infra utility fail with wait timeout
		// - client fix cloud issue (like extend quota)
		// - vm started and registered
		// - client restart bootstrap
		logger.LogDebugF("Has node state in cluster for '%s'. Skip checking node resource in cluster\n", nodeName)
		return nil
	}

	nodeExists, err := entity.IsNodeExistsInCluster(ctx, kubeCl, nodeName, logger)
	if err != nil {
		return fmt.Errorf("Cannot check that node resource exists for %s: %w", nodeName, err)
	}

	if nodeExists {
		return fmt.Errorf("Node with name %s exists in cluster", nodeName)
	}

	return nil
}
