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

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-dhctl/pkg/logger"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	ottrace "go.opentelemetry.io/otel/trace"

	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands"
	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands/bootstrap"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/configcmd"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/tofu"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
	"github.com/deckhouse/deckhouse/dhctl/pkg/telemetry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

const (
	oneShotDhctlServerCmd = "_server"
	grpcServerCmd         = "server"
	autoConvergeCmd       = "converge-periodical"
	terraformGroupCmd     = "terraform"
	exporterCmd           = "converge-exporter"
	phaseCatalogCmd       = "phase-catalog"

	appHelp = "A tool to create a Kubernetes cluster and infrastructure."
)

var commandList = []Command{
	{Name: grpcServerCmd, Help: "Start dhctl as GRPC server.", DefineFunc: commands.DefineServerCommand},
	{Name: oneShotDhctlServerCmd, Help: "Start dhctl as GRPC server. Single threaded version.", DefineFunc: commands.DefineSingleThreadedServerCommand},
	{Name: "bootstrap", Help: "Bootstrap a cluster.", DefineFunc: bootstrap.DefineBootstrapCommand},
	{Name: "bootstrap-phase", Help: "Commands to run a single phase of the bootstrap process.", Subcommands: []Command{
		{Name: "create-resources", Help: "Create resources in a Kubernetes cluster.", DefineFunc: bootstrap.DefineCreateResourcesCommand},
		{Name: "install-deckhouse", Help: "Install deckhouse and wait for its readiness.", DefineFunc: bootstrap.DefineBootstrapInstallDeckhouseCommand},
		{Name: "abort", Help: "Delete every node created during the bootstrap process.", DefineFunc: bootstrap.DefineBootstrapAbortCommand},
		{Name: "base-infra", Help: "Create base infrastructure for a cloud Kubernetes cluster.", DefineFunc: bootstrap.DefineBaseInfrastructureCommand},
	}},
	// Top level rather than under bootstrap: converge, destroy and check run the node
	// checks too, so the list is no longer about bootstrap alone.
	{Name: "preflight", Help: "Preflight checks.", Subcommands: []Command{
		{Name: "list", Help: "List the preflight checks, what each applies to and what it asserts.", DefineFunc: bootstrap.DefinePreflightListCommand},
	}},
	{Name: "converge", Help: "Converge a Kubernetes cluster.", DefineFunc: commands.DefineConvergeCommand},
	{Name: autoConvergeCmd, Help: "Start a service that runs converge periodically.", DefineFunc: commands.DefineAutoConvergeCommand},
	{Name: "converge-migration", Help: "Migrate state from terraform to opentofu. Start converge if the cluster has no infrastructure changes.", DefineFunc: commands.DefineConvergeMigrationCommand},
	{Name: "lock", Help: "Converge cluster lock", Subcommands: []Command{
		{Name: "release", Help: "Release the converge lock completely. This removes the converge lease lock from the cluster regardless of owner. Be careful.", DefineFunc: commands.DefineReleaseConvergeLockCommand},
	}},
	{Name: "destroy", Help: "Destroy Kubernetes cluster.", DefineFunc: commands.DefineDestroyCommand},
	{Name: phaseCatalogCmd, Help: "Print the phase and subphase title catalog as JSON.", DefineFunc: commands.DefinePhaseCatalogCommand},
	{Name: "session", Help: "SSH tunnel proxy to Kubernetes cluster and save local kubeconfig for kubectl.", DefineFunc: commands.DefineSessionCommand},
	{Name: terraformGroupCmd, Help: "Infrastructure commands.", Subcommands: []Command{
		{Name: exporterCmd, Help: "Run infrastructure converge exporter.", DefineFunc: commands.DefineInfrastructureConvergeExporterCommand},
		{Name: "check", Help: "Check differences between state of Kubernetes cluster and infrastructure state.", DefineFunc: commands.DefineInfrastructureCheckCommand},
	}},
	{Name: "config", Help: "Load, edit and save various dhctl configurations.", Subcommands: []Command{
		{Name: "parse", Help: "Parse, validate and output configurations.", Subcommands: []Command{
			{Name: "cluster-configuration", Help: "Parse configuration and print it.", DefineFunc: configcmd.DefineCommandParseClusterConfiguration},
			{Name: "cloud-discovery-data", Help: "Parse cloud discovery data and print it.", DefineFunc: configcmd.DefineCommandParseCloudDiscoveryData},
		}},
		{Name: "render", Help: "Render transitional configurations.", Subcommands: []Command{
			{Name: "bashible-bundle", Help: "Render bashible bundle.", DefineFunc: commands.DefineRenderBashibleBundle},
			{Name: "control-plane-manifests", Help: "Render control-plane manifests and PKI.", DefineFunc: commands.DefineRenderControlPlaneAndPKI},
			{Name: "master-bootstrap-scripts", Help: "Render master bootstrap scripts.", DefineFunc: commands.DefineRenderMasterBootstrap},
		}},
		{Name: "edit", Help: "Change configuration files in Kubernetes cluster conveniently and safely.", Subcommands: []Command{
			{Name: "cluster-configuration", Help: "Edit cluster-configuration in the Kubernetes cluster.", DefineFunc: configcmd.DefineEditCommand},
			{Name: "provider-cluster-configuration", Help: "Edit provider-cluster-configuration in the Kubernetes cluster.", DefineFunc: configcmd.DefineEditCommand},
			{Name: "static-cluster-configuration", Help: "Edit static-cluster-configuration in the Kubernetes cluster.", DefineFunc: configcmd.DefineEditCommand},
		}},
	}},
	{Name: "test", Help: "Commands to test the parts of bootstrap and converge process.", Subcommands: []Command{
		{Name: "ssh-connection", Help: "Test connection via SSH.", DefineFunc: commands.DefineTestSSHConnectionCommand},
		{Name: "kubernetes-api-connection", Help: "Test connection to the Kubernetes API via SSH or directly.", DefineFunc: commands.DefineTestKubernetesAPIConnectionCommand},
		{Name: "scp", Help: "Test scp file operations.", DefineFunc: commands.DefineTestSCPCommand},
		{Name: "upload-exec", Help: "Test scp upload and ssh execution of the uploaded script.", DefineFunc: commands.DefineTestUploadExecCommand},
		{Name: "bashible-bundle", Help: "Test upload and execute a bundle.", DefineFunc: commands.DefineTestBundle},
		{Name: "control-plane", Help: "Commands to test control plane nodes.", Subcommands: []Command{
			{Name: "manager", Help: "Test that the control plane manager is ready.", DefineFunc: commands.DefineTestControlPlaneManagerReadyCommand},
			{Name: "node", Help: "Test that the control plane node is ready.", DefineFunc: commands.DefineTestControlPlaneNodeReadyCommand},
		}},
		{Name: "deckhouse", Help: "Install and uninstall deckhouse.", Subcommands: []Command{
			{Name: "create-deployment", Help: "Install deckhouse after the infrastructure is applied successfully.", DefineFunc: commands.DefineDeckhouseCreateDeployment},
			{Name: "remove-deployment", Help: "Delete deckhouse deployment.", DefineFunc: commands.DefineDeckhouseRemoveDeployment},
			{Name: "deployment-ready", Help: "Wait until the deployment is ready.", DefineFunc: commands.DefineWaitDeploymentReadyCommand},
		}},
	}},
}

func main() {
	appContext := context.Background()

	opts := options.New()

	initGlobalVars()

	tomb.RegisterOnShutdown("Restore terminal if needed", restoreTerminal())

	tomb.RegisterOnShutdown("Leave alternate screen if needed", logger.RestoreTerminal)
	defer logger.RestoreTerminal()

	tomb.RegisterOnShutdown("Stop kubernetes provider daemon", tofu.StopProviderDaemon)

	go tomb.WaitForProcessInterruption(tomb.BeforeInterrupted{
		disableCleanupOnInterrupted,
	})

	runApplication(appContext, opts)
}

func runApplication(ctx context.Context, opts *options.Options) {
	r := &run{initer: newActionIniter(opts)}
	root := buildRoot(commandList, opts, allowedCommandsFromEnv(), r.prepare)
	version := &cobra.Command{
		Use:               "version",
		Short:             "Show version.",
		Args:              cobra.NoArgs,
		PersistentPreRunE: applyFlagsHook(opts),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s %s", app.AppName, opts.BuildInfo.AppVersion); err != nil {
				return fmt.Errorf("print version: %w", err)
			}

			return nil
		},
	}
	app.AfterApply(version, r.prepare)
	root.AddCommand(version)

	go func() {
		cmd, err := execute(ctx, root, os.Args[1:])
		errorCode := 0
		if err != nil {
			l := r.initer.Logger()
			l.Debug(commandName(cmd))

			msg := err.Error()

			if r.initer.logFile != "" {
				msg = fmt.Sprintf("%s\nDebug log file: %s", msg, r.initer.logFile)
			}

			l.Error(msg)
			errorCode = 1
		}

		endCommandSpan(r.span, err)
		tomb.Shutdown(errorCode)
	}()

	// Block "main" function until teardown callbacks are finished.
	exitCode := tomb.WaitShutdown()
	os.Exit(exitCode)
}

type run struct {
	initer *actionIniter
	span   ottrace.Span
}

// prepare is the setup main used to do in app-level actions, once the flags are applied and their
// hooks passed: kube auth mode, telemetry, the command span, logger and tmp dir.
func (r *run) prepare(cmd *cobra.Command) error {
	opts := r.initer.opts
	// Retry loops read the auth mode to tell an apiserver that cannot answer yet from
	// credentials that will never be allowed.
	ctx := providerinitializer.WithKubeAuthMode(cmd.Context(), &opts.Kube)
	if err := telemetry.Bootstrap(ctx); err != nil {
		return err
	}

	ctx, r.span = startCommandSpan(ctx, commandName(cmd))
	cmd.SetContext(ctx)

	return r.initer.init(cmd)
}

// applyFlags makes the options whole before the flag hooks read them: the environment, then the
// values derived from the flags.
func applyFlags(cmd *cobra.Command, opts *options.Options) error {
	if err := applyEnvironment(cmd, &opts.Global); err != nil {
		return err
	}
	// Config parsing picks the reachable registry (in-cluster mirror or upstream) from this flag.
	opts.Global.KubeInCluster = opts.Kube.InCluster
	options.ResolveAndApplyPaths(&opts.Global)

	return nil
}

// applyEnvironment applies the DHCTL_CLI_* variables, then forces -v where the command asks for it.
func applyEnvironment(cmd *cobra.Command, global *options.GlobalOptions) error {
	if err := app.ApplyEnvironment(cmd); err != nil {
		return err
	}

	if slices.Contains(forceVerboseCommands, commandName(cmd)) {
		global.ShowProgress = true
	}

	return nil
}

func startCommandSpan(ctx context.Context, name string) (context.Context, ottrace.Span) {
	return telemetry.StartSpan(ctx, name, ottrace.WithAttributes(
		attribute.String("dhctl.command", name),
		attribute.String("process.command_line", strings.Join(os.Args, " ")),
	))
}

// endCommandSpan closes the span prepare opened; a run that failed before it has none.
func endCommandSpan(span ottrace.Span, err error) {
	if span == nil {
		return
	}

	if err != nil {
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	span.End()

	ctx := context.Background()
	logger.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("TraceID: %s", span.SpanContext().TraceID().String()))
}

func initGlobalVars() {
	dhctlPath := ""

	if val, ok := os.LookupEnv("DHCTL_SKIP_LOOKUP_EXEC_PATH"); !ok || val != "yes" {
		// get current location of called binary
		var err error
		dhctlPath, err = os.Readlink(fmt.Sprintf("/proc/%d/exe", os.Getpid()))
		if err != nil {
			panic(err)
		}
		dhctlPath = filepath.Dir(dhctlPath)
		if dhctlPath == "/" {
			dhctlPath = "" // All our paths are already absolute by themselves
		}

		// set path to ssh and terraform binaries
		if err = os.Setenv("PATH", fmt.Sprintf("%s/bin:%s", dhctlPath, os.Getenv("PATH"))); err != nil {
			panic(err)
		}
	}
}
