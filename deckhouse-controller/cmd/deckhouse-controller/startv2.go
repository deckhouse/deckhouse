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
	"errors"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"sync/atomic"
	"syscall"
	"time"

	klient "github.com/flant/kube-client/client"
	"github.com/flant/shell-operator/pkg/executor"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/controller"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
	apiv1 "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/api/handlers/root/v1"
	apiserver "github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/api/server"
	d8apis "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/envconfig"
	d8requirements "github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

// shutdownTimeout bounds flushing telemetry on the way out.
const shutdownTimeout = 5 * time.Second

// signalError is the cause a caught SIGINT or SIGTERM cancels the run with.
type signalError struct {
	sig syscall.Signal
}

// Error names the signal.
func (e signalError) Error() string {
	return fmt.Sprintf("signal %q received", e.sig.String())
}

func startV2(logger *log.Logger) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		// flags are parsed by now; a start failure is not a usage error
		cmd.SilenceUsage = true

		if os.Getenv(envconfig.EnvSkipEntrypoint) != "true" {
			if err := entrypoint(logger); err != nil {
				logger.Error("entrypoint run", log.Err(err))
				os.Exit(1)
			}
		}

		// a signal cancels the startup as well as the run, and the elector releases the lease on it
		ctx, cancel := context.WithCancelCause(cmd.Context())
		defer cancel(nil)

		signals := make(chan os.Signal, 1)
		signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
		go cancelOnSignal(ctx, cancel, signals, logger)
		go reapZombies(ctx, logger)

		telemetryShutdown := registerTelemetry(ctx, logger.Named("otlp-tracing"))

		// addon-operator prints its own startup banner via its AppStartMessage.
		app.SetAppStartMessage(version())
		app.SetKubeClientFieldManager("deckhouse-hook")

		client := klient.New()
		client.WithContextName(app.KubeContext())
		client.WithConfigPath(app.KubeConfig())

		if err := client.Init(); err != nil {
			return fmt.Errorf("init client: %w", err)
		}

		ms := metricsstorage.NewMetricStorage(
			metricsstorage.WithLogger(logger.Named("metric-storage")),
		)

		// Register addon-operator specific metrics
		if err := metrics.RegisterAddonHookMetrics(ms); err != nil {
			return fmt.Errorf("register hook metrics: %w", err)
		}

		if err := metrics.RegisterShellOperatorMetrics(ms, []string{}); err != nil {
			return fmt.Errorf("register shell operator metrics: %w", err)
		}

		if err := metrics.RegisterDeckhouseControllerMetrics(ms); err != nil {
			return fmt.Errorf("register deckhouse controller metrics: %w", err)
		}

		if err := d8apis.EnsureCRDs(ctx, client); err != nil {
			return ignoreCanceled(ctx, fmt.Errorf("ensure crds: %w", err))
		}

		deckhouse, err := controller.Build(ctx, client, ms, logger)
		if err != nil {
			return fmt.Errorf("create deckhouse controller: %w", err)
		}

		manager := deckhouse.GetRuntime()

		deps := apiv1.Deps{
			Packages:     manager,
			Queues:       manager,
			Scheduler:    manager,
			Requirements: d8requirements.DumpValues,
		}

		server := apiserver.New(ms, manager, deps, logger)
		if err := server.Start(); err != nil {
			return fmt.Errorf("start server: %w", err)
		}

		shutdown := func() {
			stopV2(deckhouse, server, telemetryShutdown, logger)
		}

		if enabledHA(logger) {
			logger.Info("deckhouse starts in HA mode")
			err = runV2WithLeaderElection(ctx, shutdown, server, deckhouse, client, logger)
		} else {
			err = runV2(ctx, deckhouse, client, logger)
		}

		shutdown()

		var sigErr signalError
		if errors.As(context.Cause(ctx), &sigErr) {
			os.Exit(128 + int(sigErr.sig))
		}

		return err
	}
}

// cancelOnSignal cancels ctx with the first SIGINT or SIGTERM.
func cancelOnSignal(ctx context.Context, cancel context.CancelCauseFunc, signals <-chan os.Signal, logger *log.Logger) {
	select {
	case <-ctx.Done():
	case sig := <-signals:
		logger.Info(fmt.Sprintf("signal %q was received, deckhouse is shutting down...", sig.String()))

		if s, ok := sig.(syscall.Signal); ok {
			cancel(signalError{sig: s})
			return
		}

		cancel(nil)
	}
}

func runV2WithLeaderElection(ctx context.Context, shutdown func(), server *apiserver.Server, deckhouse *controller.Controller, client *klient.Client, logger *log.Logger) error {
	identity, err := buildIdentity(logger)
	if err != nil {
		return fmt.Errorf("build identity: %w", err)
	}

	// the run started on leading winds down with the lease; the caller stops only once it has
	var leading atomic.Bool
	runDone := make(chan struct{})

	elector, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		// Create a leaderElectionConfig for leader election
		Lock: &resourcelock.LeaseLock{
			LeaseMeta: metav1.ObjectMeta{
				Name:      leaseName,
				Namespace: app.NamespaceDeckhouse,
			},
			Client: client.CoordinationV1(),
			LockConfig: resourcelock.ResourceLockConfig{
				Identity: identity,
			},
		},
		LeaseDuration: time.Duration(leaseDuration) * time.Second,
		RenewDeadline: time.Duration(renewalDeadline) * time.Second,
		RetryPeriod:   time.Duration(retryPeriod) * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(leaderCtx context.Context) {
				leading.Store(true)
				defer close(runDone)

				if err := runV2(leaderCtx, deckhouse, client, logger); err != nil && leaderCtx.Err() == nil {
					logger.Error("run", log.Err(err))
					os.Exit(1)
				}
			},
			OnStoppedLeading: func() {
				// on a signal the caller shuts down; only a lost lease ends the process from here
				if ctx.Err() != nil {
					return
				}

				logger.Info("leadership lost, deckhouse is shutting down")
				shutdown()
				os.Exit(0)
			},
		},
		ReleaseOnCancel: true,
	})
	if err != nil {
		return fmt.Errorf("create leader elector: %w", err)
	}

	server.SetElector(elector)
	elector.Run(ctx)

	// a run that has not stored the flag yet meets a canceled ctx at its first call and does nothing
	if leading.Load() {
		<-runDone
	}

	return nil
}

// runV2 starts the controller and blocks until ctx is canceled; a cancel during startup is not an error.
func runV2(ctx context.Context, deckhouse *controller.Controller, client *klient.Client, logger *log.Logger) error {
	// we have to lock the controller run if dhctl lock configmap exists
	if err := lockUntilClusterBootstraped(ctx, client, logger); err != nil {
		return ignoreCanceled(ctx, fmt.Errorf("lock until cluster bootstraped: %w", err))
	}

	if err := deckhouse.Start(ctx); err != nil {
		return ignoreCanceled(ctx, fmt.Errorf("start deckhouse controller: %w", err))
	}

	<-ctx.Done()

	return nil
}

// ignoreCanceled drops an error caused by ctx being canceled, which is a shutdown and not a failure.
func ignoreCanceled(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return nil
	}

	return err
}

// stopV2 stops the controller and the API, flushes telemetry and kills the hook processes left behind.
func stopV2(deckhouse *controller.Controller, server *apiserver.Server, telemetryShutdown func(context.Context) error, logger *log.Logger) {
	deckhouse.Stop()
	if err := server.Stop(); err != nil {
		logger.Error("server stop", log.Err(err))
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := telemetryShutdown(ctx); err != nil {
		logger.Error("telemetry shutdown", log.Err(err))
	}

	if err := syscall.Kill(-1, syscall.SIGKILL); err != nil {
		if !errors.Is(err, syscall.ECHILD) && !errors.Is(err, syscall.ESRCH) {
			logger.Error("couldn't kill child processes", log.Err(err))
		}
	}
}

// reapZombies reaps orphaned zombies on SIGCHLD from the start, since deckhouse runs as PID 1; hook processes the executor tracks are left to it.
func reapZombies(ctx context.Context, logger *log.Logger) {
	children := make(chan os.Signal, 5)
	signal.Notify(children, syscall.SIGCHLD)
	defer signal.Stop(children)

	rm := reaperMutex{}
	for {
		select {
		case <-ctx.Done():
			return

		case <-children:
			rm.Lock()
			if rm.scheduled {
				rm.Unlock()
				continue
			}

			rm.scheduled = true
			rm.Unlock()

			go func() {
				defer rm.Release()
				reapOrphans(logger)
			}()
		}
	}
}

// reapOrphans waits on every zombie whose parent is PID 1 and that the executor does not track.
func reapOrphans(logger *log.Logger) {
	processes, err := process.Processes()
	if err != nil {
		logger.Debug("get processes", log.Err(err))
		return
	}

	for _, ps := range processes {
		status, err := ps.Status()
		if err != nil {
			logger.Debug("get process status", log.Err(err))
			continue
		}

		if !slices.Contains(status, process.Zombie) {
			continue
		}

		ppid, err := ps.Ppid()
		if err != nil {
			logger.Debug("get parent process id", log.Err(err))
			continue
		}

		if ppid != 1 || executor.Tracker().IsActive(int(ps.Pid)) {
			continue
		}

		var waitStatus syscall.WaitStatus
		if _, err := syscall.Wait4(int(ps.Pid), &waitStatus, syscall.WNOHANG, nil); err != nil {
			// ignore if a child has already been reaped
			if !errors.Is(err, syscall.ECHILD) && !errors.Is(err, syscall.ESRCH) {
				logger.Error("process SIGCHLD signal", log.Err(err))
			}
		}
	}
}
