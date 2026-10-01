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

		if err := metrics.RegisterDeckhouseControllerMetrics(ms); err != nil {
			return fmt.Errorf("register deckhouse controller metrics: %w", err)
		}

		deckhouse, err := controller.Build(cmd.Context(), client, ms, logger)
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

		if !enabledHA(logger) {
			return runV2(cmd.Context(), server, deckhouse, client, logger)
		}

		logger.Info("deckhouse starts in HA mode")
		return runV2WithLeaderElection(cmd.Context(), server, deckhouse, client, logger)
	}
}

func runV2WithLeaderElection(ctx context.Context, server *apiserver.Server, deckhouse *controller.Controller, client *klient.Client, logger *log.Logger) error {
	identity, err := buildIdentity(logger)
	if err != nil {
		return fmt.Errorf("build identity: %w", err)
	}

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
			OnStartedLeading: func(ctx context.Context) {
				if err := runV2(ctx, server, deckhouse, client, logger); err != nil {
					logger.Error("run", log.Err(err))
					os.Exit(1)
				}
			},
			OnStoppedLeading: func() {
				deckhouse.Stop()
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

	return nil
}

func runV2(ctx context.Context, server *apiserver.Server, deckhouse *controller.Controller, client *klient.Client, logger *log.Logger) error {
	if err := d8apis.EnsureCRDs(ctx, client); err != nil {
		return fmt.Errorf("ensure crds: %w", err)
	}

	// we have to lock the controller run if dhctl lock configmap exists
	if err := lockUntilClusterBootstraped(ctx, client, logger); err != nil {
		return fmt.Errorf("lock until cluster bootstraped: %w", err)
	}

	if err := deckhouse.Start(ctx); err != nil {
		return fmt.Errorf("start deckhouse controller: %w", err)
	}

	signalHandlerV2(ctx, deckhouse, server, logger)

	return nil
}

func signalHandlerV2(ctx context.Context, deckhouse *controller.Controller, server *apiserver.Server, logger *log.Logger) {
	telemetryShutdown := registerTelemetry(ctx, logger.Named("otlp-tracing"))

	interruptCh := make(chan os.Signal, 5)
	signal.Notify(interruptCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGCHLD)
	rm := reaperMutex{}
	for {
		select {
		case <-ctx.Done():
			logger.Info("context canceled - exiting")
			return

		case sig := <-interruptCh:
			switch sig {
			case syscall.SIGCHLD:
				rm.Lock()
				if !rm.scheduled {
					rm.scheduled = true
					rm.Unlock()
					go func() {
						defer rm.Release()
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

							if slices.Contains(status, process.Zombie) {
								ppid, err := ps.Ppid()
								if err != nil {
									logger.Debug("get parent process id", log.Err(err))
									continue
								}

								if ppid == 1 && !executor.Tracker().IsActive(int(ps.Pid)) {
									var status syscall.WaitStatus
									_, err := syscall.Wait4(int(ps.Pid), &status, syscall.WNOHANG, nil)
									if err != nil {
										// ignore if a child has already been reaped
										if !errors.Is(err, syscall.ECHILD) && !errors.Is(err, syscall.ESRCH) {
											logger.Error("process SIGCHLD signal", log.Err(err))
										}
									}
								}
							}
						}
					}()
				} else {
					rm.Unlock()
				}

			case syscall.SIGINT, syscall.SIGTERM:
				logger.Info(fmt.Sprintf("signal %q was received, deckhouse is shutting down...", sig.String()))
				if err := telemetryShutdown(ctx); err != nil {
					logger.Error("telemetry shutdown", log.Err(err))
				}

				deckhouse.Stop()
				if err := server.Stop(); err != nil {
					logger.Error("server stop", log.Err(err))
				}

				if err := syscall.Kill(-1, syscall.SIGKILL); err != nil {
					if !errors.Is(err, syscall.ECHILD) && !errors.Is(err, syscall.ESRCH) {
						logger.Error("couldn't kill child processes", log.Err(err))
					}
				}
				signum := 0
				if v, ok := sig.(syscall.Signal); ok {
					signum = int(v)
				}
				os.Exit(128 + signum)
			}
		}
	}
}
