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

// TODO structure these functions into classes
// TODO move states saving to operations/bootstrap/state.go

package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/name212/govalue"

	libcon "github.com/deckhouse/lib-connection/pkg"
	dhctllog "github.com/deckhouse/lib-dhctl/pkg/log"
	"github.com/deckhouse/lib-dhctl/pkg/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	registry_config "github.com/deckhouse/deckhouse/dhctl/pkg/config/registry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/entity"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/module/controlplane"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations"
	dhbashible "github.com/deckhouse/deckhouse/dhctl/pkg/operations/bootstrap/bashible"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/bootstrap/deps"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/bootstrap/registry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/bootstrap/rpp"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/telemetry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

const (
	BastionHostCacheKey = "bastion-hosts"
)

type ModulePreparator interface {
	PrepareModule(ctx context.Context) error
	Module() string
}

type BashiblePipelineParams struct {
	Node           libcon.Interface
	NodeIP         string
	MetaConfig     *config.MetaConfig
	DevicePath     string
	CommanderMode  bool
	IsDebug        bool
	GlobalOpts     *options.GlobalOptions
	LoggerProvider dhctllog.LoggerProvider
}

func (p *BashiblePipelineParams) Validate() error {
	if govalue.IsNil(p.Node) {
		return p.errIsNil("Node")
	}

	if govalue.IsNil(p.MetaConfig) {
		return p.errIsNil("MetaConfig")
	}

	if govalue.IsNil(p.GlobalOpts) {
		return p.errIsNil("GlobalOpts")
	}

	if govalue.IsNil(p.LoggerProvider) {
		return p.errIsNil("LoggerProvider")
	}

	return nil
}

func (p *BashiblePipelineParams) err(msg string) error {
	return fmt.Errorf("Internal error: BashiblePipelineParams %s", msg)
}

func (p *BashiblePipelineParams) errIsNil(c string) error {
	return p.err(fmt.Sprintf("%s is nil", c))
}

func RunBashiblePipeline(ctx context.Context, params *BashiblePipelineParams) error {
	ctx, span := telemetry.StartSpan(ctx, "RunBashiblePipeline")
	defer span.End()

	if err := params.Validate(); err != nil {
		return err
	}

	cfg := params.MetaConfig
	nodeInterface := params.Node
	nodeIP := params.NodeIP
	loggerProvider := params.LoggerProvider
	devicePath := params.DevicePath

	logger := loggerProvider()

	depsChecker := deps.NewDependenciesChecker(params.Node, loggerProvider)
	if err := depsChecker.Check(ctx); err != nil {
		return err
	}

	templateController := template.NewTemplateController("")
	bashible := dhbashible.NewRunner(nodeInterface, loggerProvider)

	err := log.ProcessCtx(ctx, "bootstrap", "Preparing bootstrap", func(ctx context.Context) error {
		log.DebugF("Rendered templates directory %s\n", templateController.TmpDir)

		if err := template.PrepareBootstrap(ctx, templateController, nodeIP, cfg, params.GlobalOpts); err != nil {
			return fmt.Errorf("prepare bootstrap: %v", err)
		}

		return bashible.Prepare(ctx)
	})
	if err != nil {
		return err
	}

	ready, err := bashible.AlreadyRun(ctx)
	if err != nil {
		return err
	}

	if ready {
		logger.Success("Bashible already run! Skip bashible install")
		return nil
	}

	// Bundle registry tunnel
	bundleRegistryTunnelStop, err := registry.InitTunnel(ctx, registry.TunnelParams{
		MetaConfig: cfg,
		Node:       params.Node,
		Logger:     params.LoggerProvider(),
		GlobalOpts: params.GlobalOpts,
	})
	if err != nil {
		return err
	}
	defer bundleRegistryTunnelStop()

	// RPP + RPP tunnel
	registryPackagesProxyCleanup, err := rpp.Init(ctx, rpp.InitParams{
		MetaConfig:     cfg,
		Node:           nodeInterface,
		LoggerProvider: params.LoggerProvider,
		SignCheck:      config.GetRPPSignCheck(),
		GlobalOpts:     params.GlobalOpts,
		Interactive:    input.IsTerminal(),
	})
	if err != nil {
		return err
	}

	defer registryPackagesProxyCleanup()

	if err = PrepareBashibleBundle(ctx, nodeIP, devicePath, cfg, templateController, params.GlobalOpts); err != nil {
		return err
	}
	tomb.RegisterOnShutdown("Delete templates temporary directory", func() {
		if !params.IsDebug {
			_ = os.RemoveAll(templateController.TmpDir)
		}
	})

	if err := prepareMasterNode(ctx, nodeInterface, templateController); err != nil {
		return err
	}

	nodeInfo, err := bashible.ReadNodeInfo(ctx)
	if err != nil {
		return fmt.Errorf("Cannot read node info: %w", err)
	}

	if err := PrepareControlPlaneArtifacts(nodeInfo, cfg, templateController, params.GlobalOpts); err != nil {
		return err
	}

	modulesPreparators := getModulesPreparators(params)
	for _, preparator := range modulesPreparators {
		logger.DebugF("Starting prepare module %s", preparator.Module())
		if err := preparator.PrepareModule(ctx); err != nil {
			return err
		}
	}

	return bashible.ExecuteBundle(ctx, dhbashible.ExecuteBundleParams{
		BundleDir:     templateController.TmpDir,
		CommanderMode: params.CommanderMode,
	})
}

func getModulesPreparators(params *BashiblePipelineParams) []ModulePreparator {
	controlPlaneSettings := controlplane.NewSettingsExtractor(
		params.MetaConfig,
		config.NewSchemaStore(params.GlobalOpts),
		config.GetEdition(),
		params.LoggerProvider,
	)

	return []ModulePreparator{
		controlplane.NewBootstrapPreparator(
			controlPlaneSettings,
			params.Node,
			params.LoggerProvider,
		),
	}
}

func prepareMasterNode(ctx context.Context, nodeInterface libcon.Interface, controller *template.Controller) error {
	ctx, span := telemetry.StartSpan(ctx, "prepareMasterNode")
	defer span.End()

	upload := func(ctx context.Context, scriptPath string) error {
		ctx, span := telemetry.StartSpan(ctx, "upload script")
		defer span.End()

		if _, err := os.Stat(scriptPath); err != nil {
			if os.IsNotExist(err) {
				log.InfoF("Script %s wasn't found\n", scriptPath)
				return nil
			}
			return fmt.Errorf("script path: %v", err)
		}

		logs := make([]string, 0)

		cmd := nodeInterface.UploadScript(scriptPath)
		cmd.WithStdoutHandler(func(l string) {
			logs = append(logs, l)
			log.DebugLn(l)
		})
		cmd.Sudo()

		_, err := cmd.Execute(ctx)
		if err != nil {
			stderr := ""

			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				stderr = string(exitErr.Stderr)
			}

			log.ErrorF("%s\nstderr:\n%s\n", strings.Join(logs, "\n"), stderr)

			return fmt.Errorf("run %s: %w", scriptPath, err)
		}

		return nil
	}

	return log.ProcessCtx(ctx, "bootstrap", "Initial bootstrap", func(ctx context.Context) error {
		for _, bootstrapScript := range []string{"01-bootstrap-prerequisites.sh"} {
			scriptPath := filepath.Join(controller.TmpDir, "bootstrap", bootstrapScript)

			name := fmt.Sprintf("Execute %s", bootstrapScript)
			extLogger := log.ExternalLoggerProvider(log.GetDefaultLogger())
			p := retry.NewEmptyParams(
				retry.WithName("%s", name),
				retry.WithAttempts(30),
				retry.WithWait(5*time.Second),
				retry.WithLogger(extLogger()),
			)
			err := retry.NewLoopWithParams(p).RunContext(ctx, func() error {
				return upload(ctx, scriptPath)
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func PrepareBashibleBundle(
	ctx context.Context,
	nodeIP, devicePath string,
	metaConfig *config.MetaConfig,
	controller *template.Controller,
	globalOptions *options.GlobalOptions,
) error {
	return log.ProcessCtx(ctx, "bootstrap", "Prepare Bashible", func(ctx context.Context) error {
		return template.PrepareBundle(ctx, controller, nodeIP, devicePath, metaConfig, globalOptions)
	})
}

// PrepareControlPlaneArtifacts renders the PKI bundle, kubeconfig files and
// control-plane static-pod manifests into the local template tmp dir for the
// node identified by (nodeName, nodeIP).
func PrepareControlPlaneArtifacts(
	nodeInfo *dhbashible.NodeInfo,
	metaConfig *config.MetaConfig,
	controller *template.Controller,
	globalOptions *options.GlobalOptions,
) error {
	return log.Process("bootstrap", "Prepare control-plane manifests", func() error {
		nodeName := nodeInfo.NodeName
		nodeIP := nodeInfo.NodeIP

		log.InfoF("Using node hostname %q and IP %q for control-plane manifests\n", nodeName, nodeIP)

		controlPlaneConfig, err := metaConfig.ConfigForControlPlaneTemplates("")
		if err != nil {
			return fmt.Errorf("get control-plane template data: %w", err)
		}

		// For first-master bootstrap we use the node IP itself as the
		// control-plane endpoint that goes into the apiserver SAN list.
		// Multi-master installations re-issue certificates later via
		// control-plane-manager once additional master endpoints are known.
		if err := template.PreparePKI(controller, nodeName, nodeIP, nodeIP, controlPlaneConfig); err != nil {
			return fmt.Errorf("prepare PKI: %w", err)
		}

		if err := template.PrepareControlPlaneManifests(controller, controlPlaneConfig, globalOptions); err != nil {
			return fmt.Errorf("prepare control plane manifests: %w", err)
		}

		return nil
	})
}

func WaitForSSHConnectionOnMaster(ctx context.Context, sshClient libcon.SSHClient) error {
	return log.ProcessCtx(ctx, "bootstrap", "Wait for SSH on Master become Ready", func(ctx context.Context) error {
		availabilityCheck := sshClient.Check()
		_ = log.ProcessCtx(ctx, "default", "Connection string", func(ctx context.Context) error {
			log.InfoLn(availabilityCheck.String())
			return nil
		})

		extLogger := log.ExternalLoggerProvider(log.GetDefaultLogger())

		// Poll every 2s instead of 5s — VM SSH typically comes up within ~10s after
		// cloud-init finishes. Total timeout preserved at ~250s via larger attempt count.
		if err := availabilityCheck.WithDelaySeconds(1).AwaitAvailability(ctx, retry.NewEmptyParams(
			retry.WithWait(2*time.Second),
			retry.WithAttempts(125),
			retry.WithLogger(extLogger()),
		)); err != nil {
			return fmt.Errorf("await master to become available: %v", err)
		}
		return nil
	})
}

type InstallDeckhouseResult struct {
	ManifestResult *deckhouse.ManifestsResult
}

type InstallDeckhouseParams struct {
	BeforeDeckhouseTask func() error
	State               *State
	DeckhouseTimeout    time.Duration
}

func InstallDeckhouse(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
	config *config.DeckhouseInstaller,
	params InstallDeckhouseParams,
) (*InstallDeckhouseResult, error) {
	res := &InstallDeckhouseResult{}

	return res, log.ProcessCtx(ctx, "bootstrap", "Install Deckhouse", func(ctx context.Context) error {
		ctx, span := telemetry.StartSpan(ctx, "InstallDeckhouse")
		defer span.End()

		err := CheckPreventBreakAnotherBootstrappedCluster(ctx, kubeCl, config)
		if err != nil {
			return err
		}

		resManifests, err := withSpan(ctx, "InstallDeckhouse.CreateManifests", func(ctx context.Context) (*deckhouse.ManifestsResult, error) {
			return deckhouse.CreateDeckhouseManifests(ctx, kubeCl, config, params.BeforeDeckhouseTask)
		})
		if err != nil {
			return fmt.Errorf("Deckhouse create manifests: %w", err)
		}

		res.ManifestResult = resManifests

		if err := params.State.SaveManifestsCreated(ctx); err != nil {
			return fmt.Errorf("Set manifests in cluster flag to cache: %w", err)
		}

		if err := withSpanErr(ctx, "InstallDeckhouse.WaitDeckhouseReady", func(ctx context.Context) error {
			return deckhouse.WaitForReadiness(ctx, kubeCl, params.DeckhouseTimeout)
		}); err != nil {
			return fmt.Errorf("Deckhouse not ready: %w", err)
		}

		// Warning! This function must be called at the end of the Deckhouse installation phase.
		// At the end of this function, the registry-init secret is deleted,
		// which is used during DeckhouseInstall for certain registry operation modes.
		if err := withSpanErr(ctx, "InstallDeckhouse.WaitRegistryReady", func(ctx context.Context) error {
			return registry_config.WaitForRegistryInitialization(ctx, kubeCl, config.Registry)
		}); err != nil {
			return fmt.Errorf("registry initialization: %v", err)
		}

		return nil
	})
}

func withSpan[T any](ctx context.Context, name string, fn func(ctx context.Context) (T, error)) (T, error) {
	ctx, span := telemetry.StartSpan(ctx, name)
	defer span.End()
	return fn(ctx)
}

func withSpanErr(ctx context.Context, name string, fn func(ctx context.Context) error) error {
	ctx, span := telemetry.StartSpan(ctx, name)
	defer span.End()
	return fn(ctx)
}

func BootstrapTerraNodes(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
	metaConfig *config.MetaConfig,
	terraNodeGroups []config.TerraNodeGroupSpec,
	infrastructureContext *infrastructure.Context,
	globalOptions *options.GlobalOptions,
) error {
	return log.ProcessCtx(ctx, "bootstrap", "Create CloudPermanent NG", func(ctx context.Context) error {
		return operations.ParallelCreateNodeGroup(ctx, kubeCl, metaConfig, terraNodeGroups, infrastructureContext, globalOptions)
	})
}

func SaveBastionHostToCache(ctx context.Context, host string) {
	if err := cache.Global().Save(ctx, BastionHostCacheKey, []byte(host)); err != nil {
		log.ErrorF("Cannot save ssh hosts: %v\n", err)
	}
}

func GetBastionHostFromCache(ctx context.Context) (string, error) {
	exists, err := cache.Global().InCache(ctx, BastionHostCacheKey)
	if err != nil {
		return "", err
	}

	if !exists {
		return "", nil
	}

	host, err := cache.Global().Load(ctx, BastionHostCacheKey)
	if err != nil {
		return "", err
	}

	return string(host), nil
}

func BootstrapAdditionalMasterNodes(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
	metaConfig *config.MetaConfig,
	addressTracker map[string]string,
	infrastructureContext *infrastructure.Context,
	stateCache state.Cache,
	globalOptions *options.GlobalOptions,
) error {
	if metaConfig.MasterNodeGroupSpec.Replicas == 1 {
		log.DebugF("Skip bootstrap additional master nodes because replicas == 1")
		return nil
	}

	return log.ProcessCtx(ctx, "bootstrap", "Bootstrap additional master nodes", func(ctx context.Context) error {
		masterCloudConfig, err := entity.GetCloudConfig(ctx, kubeCl, global.MasterNodeGroupName, global.ShowDeckhouseLogs, log.GetDefaultLogger())
		if err != nil {
			return err
		}

		for i := 1; i < metaConfig.MasterNodeGroupSpec.Replicas; i++ {
			outputs, err := operations.BootstrapAdditionalMasterNode(ctx, kubeCl, metaConfig, i, masterCloudConfig, infrastructureContext, globalOptions)
			if err != nil {
				return err
			}
			addressTracker[fmt.Sprintf("%s-master-%d", metaConfig.ClusterPrefix, i)] = outputs.MasterIPForSSH

			state.SaveMasterHostsToCache(ctx, stateCache, addressTracker)
		}

		return nil
	})
}

func BootstrapGetNodesFromCache(
	ctx context.Context,
	metaConfig *config.MetaConfig,
	stateCache state.Cache,
) (map[string]map[int]string, error) {
	nodeGroupRegex := fmt.Sprintf("^%s-(.*)-([0-9]+)\\.tfstate$", metaConfig.ClusterPrefix)
	groupsReg, _ := regexp.Compile(nodeGroupRegex)

	nodesFromCache := make(map[string]map[int]string)

	err := stateCache.Iterate(ctx, func(name string, content []byte) error {
		switch {
		case strings.HasSuffix(name, ".backup"):
			fallthrough
		case strings.HasPrefix(name, string(infrastructure.BaseInfraStep)):
			fallthrough
		case strings.HasPrefix(name, "uuid"):
			fallthrough
		case !groupsReg.MatchString(name):
			return nil
		}

		nodeGroupNameAndNodeIndex := groupsReg.FindStringSubmatch(name)

		nodeGroupName := nodeGroupNameAndNodeIndex[1]
		rawIndex := nodeGroupNameAndNodeIndex[2]

		index, convErr := strconv.Atoi(rawIndex)
		if convErr != nil {
			return fmt.Errorf("can't convert %q to integer: %v", rawIndex, convErr)
		}

		if _, ok := nodesFromCache[nodeGroupName]; !ok {
			nodesFromCache[nodeGroupName] = make(map[int]string)
		}

		nodesFromCache[nodeGroupName][index] = strings.TrimSuffix(name, ".tfstate")
		return nil
	})
	return nodesFromCache, err
}

func applyPostBootstrapModuleConfigs(
	kubeCl *client.KubernetesClient,
	tasks []actions.ModuleConfigTask,
) error {
	for _, task := range tasks {
		extLogger := log.ExternalLoggerProvider(log.GetDefaultLogger())
		p := retry.NewEmptyParams(
			retry.WithName("%s", task.Title),
			retry.WithAttempts(15),
			retry.WithWait(5*time.Second),
			retry.WithLogger(extLogger()),
		)
		err := retry.NewLoopWithParams(p).
			Run(func() error {
				return task.Do(kubeCl)
			})
		if err != nil {
			return err
		}
	}

	return nil
}

func RunPostInstallTasks(ctx context.Context, kubeCl *client.KubernetesClient, result *InstallDeckhouseResult) error {
	ctx, span := telemetry.StartSpan(ctx, "RunPostInstallTasks")
	defer span.End()

	if result == nil {
		log.DebugF("Skip post install tasks because result is nil\n")
		return nil
	}

	return log.ProcessCtx(ctx, "bootstrap", "Run post bootstrap actions", func(ctx context.Context) error {
		return applyPostBootstrapModuleConfigs(kubeCl, result.ManifestResult.PostBootstrapMCTasks)
	})
}
