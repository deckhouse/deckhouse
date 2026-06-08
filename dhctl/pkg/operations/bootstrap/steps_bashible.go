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
	"strings"
	"time"

	"github.com/name212/govalue"

	libcon "github.com/deckhouse/lib-connection/pkg"
	dhctllog "github.com/deckhouse/lib-dhctl/pkg/log"
	"github.com/deckhouse/lib-dhctl/pkg/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	dhlog "github.com/deckhouse/deckhouse/dhctl/pkg/logger"
	dhbashible "github.com/deckhouse/deckhouse/dhctl/pkg/operations/bootstrap/bashible"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/bootstrap/deps"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/bootstrap/rpp"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/telemetry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

type BashiblePipelineParams struct {
	Node                   libcon.Interface
	NodeIP                 string
	MetaConfig             *config.MetaConfig
	DevicePath             string
	CommanderMode          bool
	IsDebug                bool
	LoggerProvider         dhctllog.LoggerProvider
	PhasedExecutionContext phases.DefaultPhasedExecutionContext
	GlobalOpts             *options.GlobalOptions
}

func (p *BashiblePipelineParams) Validate() error {
	if govalue.IsNil(p.Node) {
		return p.errIsNil("Node")
	}

	if govalue.IsNil(p.MetaConfig) {
		return p.errIsNil("MetaConfig")
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
	globalOpts := params.GlobalOpts

	depsChecker := deps.NewDependenciesChecker(params.Node, loggerProvider)
	if err := depsChecker.Check(ctx); err != nil {
		return err
	}

	templateController := template.NewTemplateController("")
	bashible := dhbashible.NewRunner(nodeInterface, loggerProvider)

	err := dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Preparing bootstrap", func(ctx context.Context) error {
		dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Rendered templates directory %s", templateController.TmpDir))

		if err := template.PrepareBootstrap(ctx, templateController, nodeIP, cfg, globalOpts); err != nil {
			return fmt.Errorf("prepare bootstrap: %v", err)
		}

		return bashible.Prepare(ctx)
	})

	if err != nil {
		return err
	}
	params.PhasedExecutionContext.CompleteSubPhase(ctx, phases.InstallKubernetesSubPhaseBundlePreparation)

	ready, err := bashible.AlreadyRun(ctx)
	if err != nil {
		return err
	}

	if ready {
		dhlog.Success(ctx, dhlog.FromContext(ctx), "Bashible already run! Skip bashible install")
		return nil
	}

	registryPackagesProxyCleanup, err := rpp.Init(ctx, rpp.InitParams{
		MetaConfig:     cfg,
		Node:           nodeInterface,
		LoggerProvider: params.LoggerProvider,
		SignCheck:      config.GetRPPSignCheck(),
		GlobalOpts:     globalOpts,
		Interactive:    input.IsTerminal(),
	})

	if err != nil {
		return err
	}

	defer registryPackagesProxyCleanup()
	params.PhasedExecutionContext.CompleteSubPhase(ctx, phases.InstallKubernetesSubPhaseRegistryPackagesProxy)

	if err = PrepareBashibleBundle(ctx, nodeIP, devicePath, cfg, templateController, globalOpts); err != nil {
		return err
	}
	tomb.RegisterOnShutdown("Delete templates temporary directory", func() {
		if !params.IsDebug {
			if err := os.RemoveAll(templateController.TmpDir); err != nil {
				params.LoggerProvider().WarnF("failed to cleanup temporary directory: %v", err)
			}
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

	if err := PrepareControlPlaneArtifacts(ctx, nodeName, discoveredNodeIP, cfg, templateController, globalOpts); err != nil {
		return err
	}

	params.PhasedExecutionContext.CompleteSubPhase(ctx, phases.InstallKubernetesSubPhaseNodePreparation)

	if err := bashible.ExecuteBundle(ctx, dhbashible.ExecuteBundleParams{
		BundleDir:     templateController.TmpDir,
		CommanderMode: params.CommanderMode,
		GlobalOpts:    params.GlobalOpts,
	}); err != nil {
		return err
	}

	params.PhasedExecutionContext.CompleteSubPhase(ctx, phases.InstallKubernetesSubPhaseExecuteBashibleBundle)
	return nil
}

func prepareMasterNode(ctx context.Context, nodeInterface libcon.Interface, controller *template.Controller) error {
	ctx, span := telemetry.StartSpan(ctx, "prepareMasterNode")
	defer span.End()

	upload := func(ctx context.Context, scriptPath string) error {
		ctx, span := telemetry.StartSpan(ctx, "upload script")
		defer span.End()

		if _, err := os.Stat(scriptPath); err != nil {
			if os.IsNotExist(err) {
				dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf("Script %s wasn't found", scriptPath))
				return nil
			}
			return fmt.Errorf("script path: %v", err)
		}

		logs := make([]string, 0)

		cmd := nodeInterface.UploadScript(scriptPath)
		cmd.WithStdoutHandler(func(l string) {
			logs = append(logs, l)
			dhlog.FromContext(ctx).DebugContext(ctx, l)
		})
		cmd.Sudo()

		_, err := cmd.Execute(ctx)
		if err != nil {
			stderr := ""

			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				stderr = string(exitErr.Stderr)
			}

			dhlog.FromContext(ctx).ErrorContext(ctx, fmt.Sprintf("%s\nstderr:\n%s", strings.Join(logs, "\n"), stderr))

			return fmt.Errorf("run %s: %w", scriptPath, err)
		}

		return nil
	}

	return dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Initial bootstrap", func(ctx context.Context) error {
		for _, bootstrapScript := range []string{"01-bootstrap-prerequisites.sh"} {
			scriptPath := filepath.Join(controller.TmpDir, "bootstrap", bootstrapScript)

			name := fmt.Sprintf("Execute %s", bootstrapScript)
			p := retry.NewEmptyParams(
				retry.WithName("%s", name),
				retry.WithAttempts(30),
				retry.WithWait(5*time.Second),
				retry.WithLogger(dhlog.NewLibdhctlAdapter(ctx)),
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
	globalOpts *options.GlobalOptions,
) error {
	return dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Prepare Bashible", func(ctx context.Context) error {
		return template.PrepareBundle(ctx, controller, nodeIP, devicePath, metaConfig, globalOpts)
	})
}

func PrepareControlPlaneArtifacts(
	ctx context.Context,
	nodeName, nodeIP string,
	metaConfig *config.MetaConfig,
	controller *template.Controller,
	globalOpts *options.GlobalOptions,
) error {
	return dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Prepare control-plane manifests", func(ctx context.Context) error {
		dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf("Using node hostname %q and IP %q for control-plane manifests", nodeName, nodeIP))

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

		if err := template.PrepareControlPlaneManifests(ctx, controller, controlPlaneData, globalOpts); err != nil {
			return fmt.Errorf("prepare control plane manifests: %w", err)
		}

		return nil
	})
}
