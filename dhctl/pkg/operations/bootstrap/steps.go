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

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config/directoryconfig"
	registry_config "github.com/deckhouse/deckhouse/dhctl/pkg/config/registry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/entity"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations"
	dhbashible "github.com/deckhouse/deckhouse/dhctl/pkg/operations/bootstrap/bashible"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/bootstrap/deps"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/bootstrap/registry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/bootstrap/rpp"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

const (
	BastionHostCacheKey = "bastion-hosts"
)

type BashiblePipelineParams struct {
	Node           libcon.Interface
	NodeIP         string
	MetaConfig     *config.MetaConfig
	DevicePath     string
	CommanderMode  bool
	IsDebug        bool
	DirsConfig     *directoryconfig.DirectoryConfig
	LoggerProvider dhctllog.LoggerProvider
}

func (p *BashiblePipelineParams) Validate() error {
	if govalue.IsNil(p.Node) {
		return p.errIsNil("Node")
	}

	if govalue.IsNil(p.MetaConfig) {
		return p.errIsNil("MetaConfig")
	}

	if govalue.IsNil(p.DirsConfig) {
		return p.errIsNil("DirsConfig")
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
	if err := params.Validate(); err != nil {
		return err
	}

	cfg := params.MetaConfig
	nodeInterface := params.Node
	dc := params.DirsConfig
	nodeIP := params.NodeIP
	loggerProvider := params.LoggerProvider
	devicePath := params.DevicePath

	depsChecker := deps.NewDependenciesChecker(params.Node, loggerProvider)
	if err := depsChecker.Check(ctx); err != nil {
		return err
	}

	templateController := template.NewTemplateController("")
	bashible := dhbashible.NewRunner(nodeInterface, loggerProvider)

	err := log.ProcessCtx(ctx, "bootstrap", "Preparing bootstrap", func(ctx context.Context) error {
		log.DebugF("Rendered templates directory %s\n", templateController.TmpDir)

		if err := template.PrepareBootstrap(ctx, templateController, nodeIP, cfg, dc); err != nil {
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
		log.Success("Bashible already run! Skip bashible install\n")
		return nil
	}

	// Bundle registry tunnel
	bundleRegistryTunnelStop, err := registry.InitTunnel(ctx, registry.TunnelParams{
		MetaConfig: cfg,
		Node:       params.Node,
		Logger:     params.LoggerProvider(),
		DirsConfig: dc,
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
		DirsConfig:     dc,
		Interactive:    input.IsTerminal(),
	})

	if err != nil {
		return err
	}

	defer registryPackagesProxyCleanup()

	if err = PrepareBashibleBundle(ctx, nodeIP, devicePath, cfg, templateController, dc); err != nil {
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

	nodeName, err := readRemoteFileWithRetry(ctx, nodeInterface, "/var/lib/bashible/discovered-node-name")
	if err != nil {
		return fmt.Errorf("read discovered node name: %w", err)
	}

	discoveredNodeIP, err := readRemoteFileWithRetry(ctx, nodeInterface, "/var/lib/bashible/discovered-node-ip")
	if err != nil {
		return fmt.Errorf("read discovered node IP: %w", err)
	}

	if err := PrepareControlPlaneArtifacts(nodeName, discoveredNodeIP, cfg, templateController, dc); err != nil {
		return err
	}

	return bashible.ExecuteBundle(ctx, dhbashible.ExecuteBundleParams{
		BundleDir:     templateController.TmpDir,
		CommanderMode: params.CommanderMode,
	})
}

func prepareMasterNode(ctx context.Context, nodeInterface libcon.Interface, controller *template.Controller) error {
	upload := func(ctx context.Context, scriptPath string) error {
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
	dc *directoryconfig.DirectoryConfig,
) error {
	return log.ProcessCtx(ctx, "bootstrap", "Prepare Bashible", func(ctx context.Context) error {
		return template.PrepareBundle(ctx, controller, nodeIP, devicePath, metaConfig, dc)
	})
}

// PrepareControlPlaneArtifacts renders the PKI bundle, kubeconfig files and
// control-plane static-pod manifests into the local template tmp dir for the
// node identified by (nodeName, nodeIP).
func PrepareControlPlaneArtifacts(
	nodeName, nodeIP string,
	metaConfig *config.MetaConfig,
	controller *template.Controller,
	dc *directoryconfig.DirectoryConfig,
) error {
	return log.Process("bootstrap", "Prepare control-plane manifests", func() error {
		log.InfoF("Using node hostname %q and IP %q for control-plane manifests\n", nodeName, nodeIP)

		controlPlaneData, err := metaConfig.ConfigForControlPlaneTemplates("")
		if err != nil {
			return fmt.Errorf("get control-plane template data: %w", err)
		}

		// For first-master bootstrap we use the node IP itself as the
		// control-plane endpoint that goes into the apiserver SAN list.
		// Multi-master installations re-issue certificates later via
		// control-plane-manager once additional master endpoints are known.
		if err := template.PreparePKI(controller, nodeName, nodeIP, nodeIP, controlPlaneData); err != nil {
			return fmt.Errorf("prepare PKI: %w", err)
		}

		if err := template.PrepareControlPlaneManifests(controller, controlPlaneData, dc); err != nil {
			return fmt.Errorf("prepare control plane manifests: %w", err)
		}

		return nil
	})
}

func readRemoteFile(ctx context.Context, nodeInterface libcon.Interface, path string) (string, error) {
	cmd := nodeInterface.Command("cat", path)
	cmd.Sudo(ctx)
	cmd.WithTimeout(10 * time.Second)

	stdout, stderr, err := cmd.Output(ctx)
	if err != nil {
		return "", fmt.Errorf("read remote file %s: %w; stderr: %s", path, err, string(stderr))
	}

	output := string(stdout)
	// Sudo-wrapped commands prefix their stdout with the SUDO-SUCCESS marker;
	// strip everything up to and including the last occurrence so we keep only
	// the actual file payload. For non-sudo paths the marker is absent and
	// output stays untouched.
	if idx := strings.LastIndex(output, "SUDO-SUCCESS"); idx >= 0 {
		output = output[idx+len("SUDO-SUCCESS"):]
	}

	return strings.TrimSpace(output), nil
}

// readRemoteFileWithRetry wraps readRemoteFile with a short retry loop
func readRemoteFileWithRetry(ctx context.Context, nodeInterface libcon.Interface, path string) (string, error) {
	const (
		attempts = 5
		wait     = 3 * time.Second
	)

	var value string
	err := retry.NewLoop(fmt.Sprintf("Read remote file %s", path), attempts, wait).
		RunContext(ctx, func() error {
			v, err := readRemoteFile(ctx, nodeInterface, path)
			if err != nil {
				return err
			}
			value = v
			return nil
		})
	if err != nil {
		return "", err
	}
	return value, nil
}

func WaitForSSHConnectionOnMaster(ctx context.Context, sshClient libcon.SSHClient) error {
	return log.ProcessCtx(ctx, "bootstrap", "Wait for SSH on Master become Ready", func(ctx context.Context) error {
		availabilityCheck := sshClient.Check()
		_ = log.ProcessCtx(ctx, "default", "Connection string", func(ctx context.Context) error {
			log.InfoLn(availabilityCheck.String())
			return nil
		})

		if err := availabilityCheck.WithDelaySeconds(1).AwaitAvailability(ctx, retry.NewEmptyParams(
			retry.WithWait(5*time.Second),
			retry.WithAttempts(50),
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
		err := CheckPreventBreakAnotherBootstrappedCluster(ctx, kubeCl, config)
		if err != nil {
			return err
		}

		resManifests, err := deckhouse.CreateDeckhouseManifests(ctx, kubeCl, config, params.BeforeDeckhouseTask)
		if err != nil {
			return fmt.Errorf("Deckhouse create manifests: %w", err)
		}

		res.ManifestResult = resManifests

		if err := params.State.SaveManifestsCreated(ctx); err != nil {
			return fmt.Errorf("Set manifests in cluster flag to cache: %w", err)
		}

		err = deckhouse.WaitForReadiness(ctx, kubeCl, params.DeckhouseTimeout)
		if err != nil {
			return fmt.Errorf("Deckhouse not ready: %w", err)
		}

		// Warning! This function must be called at the end of the Deckhouse installation phase.
		// At the end of this function, the registry-init secret is deleted,
		// which is used during DeckhouseInstall for certain registry operation modes.
		err = registry_config.WaitForRegistryInitialization(ctx, kubeCl, config.Registry)
		if err != nil {
			return fmt.Errorf("registry initialization: %v", err)
		}

		return nil
	})
}

func BootstrapTerraNodes(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
	metaConfig *config.MetaConfig,
	terraNodeGroups []config.TerraNodeGroupSpec,
	infrastructureContext *infrastructure.Context,
) error {
	return log.ProcessCtx(ctx, "bootstrap", "Create CloudPermanent NG", func(ctx context.Context) error {
		return operations.ParallelCreateNodeGroup(ctx, kubeCl, metaConfig, terraNodeGroups, infrastructureContext)
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

func BootstrapAdditionalMasterNodes(ctx context.Context, kubeCl *client.KubernetesClient, metaConfig *config.MetaConfig, addressTracker map[string]string, infrastructureContext *infrastructure.Context, stateCache state.Cache) error {
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
			outputs, err := operations.BootstrapAdditionalMasterNode(ctx, kubeCl, metaConfig, i, masterCloudConfig, infrastructureContext)
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
	ctx context.Context,
	kubeCl *client.KubernetesClient,
	tasks []actions.ModuleConfigTask,
) error {
	for _, task := range tasks {
		err := retry.NewLoop(task.Title, 15, 5*time.Second).
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
	if result == nil {
		log.DebugF("Skip post install tasks because result is nil\n")
		return nil
	}

	return log.ProcessCtx(ctx, "bootstrap", "Run post bootstrap actions", func(ctx context.Context) error {
		return applyPostBootstrapModuleConfigs(ctx, kubeCl, result.ManifestResult.PostBootstrapMCTasks)
	})
}
