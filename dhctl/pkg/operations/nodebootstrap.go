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

func IsSequentialNodesBootstrap() bool {
	if os.Getenv("DHCTL_PARALLEL_CLOUD_PERMANENT_NODES_BOOTSTRAP") == "false" {
		return true
	}

	return false
}

func NodeName(cfg *config.MetaConfig, nodeGroupName string, index int) string {
	return fmt.Sprintf("%s-%s-%v", cfg.ClusterPrefix, nodeGroupName, index)
}

func BootstrapAdditionalNode(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
	cfg *config.MetaConfig,
	index int,
	step infrastructure.Step,
	nodeGroupName, cloudConfig string,
	isConverge bool,
	infrastructureContext *infrastructure.Context,
) error {
	nodeName := NodeName(cfg, nodeGroupName, index)

	if isConverge {
		nodeExists, err := entity.IsNodeExistsInCluster(ctx, kubeCl, nodeName, log.GetDefaultLogger())
		if err != nil {
			return err
		} else if nodeExists {
			return fmt.Errorf("node with name %s exists in cluster", nodeName)
		}
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
		err := log.Process("bootstrap", fmt.Sprintf("Create %s NodeGroup", ng.Name), func() error {
			err := entity.CreateNodeGroup(ctx, kubeCl, ng.Name, log.GetDefaultLogger(), metaConfig.NodeGroupManifest(ng))
			if err != nil {
				return err
			}

			cloudConfig, err := entity.GetCloudConfig(ctx, kubeCl, ng.Name, global.ShowDeckhouseLogs, log.GetDefaultLogger())
			if err != nil {
				return err
			}

			for i := 0; i < ng.Replicas; i++ {
				err = BootstrapAdditionalNode(ctx, kubeCl, metaConfig, i, infrastructure.StaticNodeStep, ng.Name, cloudConfig, false, infrastructureContext)
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
	isConverge bool,
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
	isConverge bool,
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

	for _, indexCandidate := range nodesIndexToCreate {
		candidateName := fmt.Sprintf("%s-%s-%v", cfg.ClusterPrefix, nodeGroupName, indexCandidate)
		nodeExists, err := entity.IsNodeExistsInCluster(ctx, kubeCl, candidateName, ngLogger)
		if err != nil {
			return nil, err
		} else if nodeExists {
			return nil, fmt.Errorf("node with name %s exists in cluster", candidateName)
		}
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

			nodeLogger = ngLogger.CreateBufferLogger(&buffNodeLog)
			if i == 0 && !saveLogToBuffer {
				nodeLogger = ngLogger
			}
			err := BootstrapAdditionalNodeForParallelRun(
				ctx,
				kubeCl,
				cfg,
				indexCandidate,
				step,
				nodeGroupName,
				cloudConfig,
				true,
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

	for candidate := range resultsChan {
		if candidate.err != nil {
			return nodesToWait, candidate.err
		}
		if candidate.buffNodeLog.Len() == 0 {
			continue
		}

		scanner := bufio.NewScanner(candidate.buffNodeLog)
		for scanner.Scan() {
			ngLogger.LogInfoLn((scanner.Text()))
		}
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

	return log.Process("converge", msg, func() error {
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

				_, err = ParallelBootstrapAdditionalNodes(ctx, kubeCl, metaConfig, nodesIndexToCreate, infrastructure.StaticNodeStep, group.Name, nodeCloudConfig, true, infrastructureContext, ngLogger, saveLogToBuffer)

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

		for ng := range resultsChan {
			if ng.err != nil {
				return ng.err
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

		return entity.WaitForNodesBecomeReady(ctx, kubeCl, ngWaitMap)
	})
}

func BootstrapAdditionalMasterNode(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
	cfg *config.MetaConfig,
	index int,
	cloudConfig string,
	isConverge bool,
	infrastructureContext *infrastructure.Context,
) (*infrastructure.PipelineOutputs, error) {
	nodeName := NodeName(cfg, global.MasterNodeGroupName, index)

	if isConverge {
		nodeExists, existsErr := entity.IsNodeExistsInCluster(ctx, kubeCl, nodeName, log.GetDefaultLogger())
		if existsErr != nil {
			return nil, existsErr
		} else if nodeExists {
			return nil, fmt.Errorf("node with name %s exists in cluster", nodeName)
		}
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
