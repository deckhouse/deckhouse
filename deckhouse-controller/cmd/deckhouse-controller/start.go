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

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	addonoperator "github.com/flant/addon-operator/pkg/addon-operator"
	aoapp "github.com/flant/addon-operator/pkg/app"
	admetrics "github.com/flant/addon-operator/pkg/metrics"
	"github.com/flant/kube-client/client"
	shapp "github.com/flant/shell-operator/pkg/app"
	shmetrics "github.com/flant/shell-operator/pkg/metrics"
	"github.com/shirou/gopsutil/v3/process"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.30.0"
	"gopkg.in/alecthomas/kingpin.v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/util/retry"

	d8Apis "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller"
	debugserver "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/debug-server"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

const (
	deckhouseControllerBinaryPath         = "/usr/bin/deckhouse-controller"
	deckhouseControllerWithCapsBinaryPath = "/usr/bin/caps-deckhouse-controller"

	deckhouseBundleEnv = "DECKHOUSE_BUNDLE"
	chrootDirEnv       = "ADDON_OPERATOR_SHELL_CHROOT_DIR"
	modulesDirEnv      = "MODULES_DIR"
	skipEntrypointEnv  = "SKIP_ENTRYPOINT_EXECUTION"

	leaseName        = "deckhouse-leader-election"
	defaultNamespace = "d8-system"
	leaseDuration    = 35
	renewalDeadline  = 30
	retryPeriod      = 10
)

type reaperMutex struct {
	sync.Mutex
	scheduled bool
}

func (r *reaperMutex) Release() {
	r.Lock()
	r.scheduled = false
	r.Unlock()
}

func start(logger *log.Logger) func(_ *kingpin.ParseContext) error {
	return func(_ *kingpin.ParseContext) error {
		if os.Getenv(skipEntrypointEnv) != "true" {
			if err := entrypoint(logger); err != nil {
				logger.Error("entrypoint run", log.Err(err))
				os.Exit(1)
			}
		}

		shapp.AppStartMessage = version()

		ctx := context.Background()

		metricsStorage := metricsstorage.NewMetricStorage(
			metricsstorage.WithLogger(logger.Named("metric-storage")),
		)

		hookMetricStorage := metricsstorage.NewMetricStorage(
			metricsstorage.WithNewRegistry(),
			metricsstorage.WithLogger(logger.Named("hook-metric-storage")),
		)

		// Initialize metric names with the configured prefix
		shmetrics.InitMetrics(shapp.PrometheusMetricsPrefix)
		// Initialize addon-operator specific metrics
		admetrics.InitMetrics(shapp.PrometheusMetricsPrefix)

		operator := addonoperator.NewAddonOperator(ctx, metricsStorage, hookMetricStorage, addonoperator.WithLogger(logger.Named("addon-operator")))

		operator.StartAPIServer()

		versionFile := "/deckhouse/version"

		version := "unknown"
		content, err := os.ReadFile(versionFile)
		if err != nil {
			logger.Warn("cannot get deckhouse version", log.Err(err))
		} else {
			version = strings.TrimSuffix(string(content), "\n")
		}

		if version == "dev" && os.Getenv("DECKHOUSE_HA") == "false" {
			if err := run(ctx, operator, logger); err != nil {
				logger.Error("run", log.Err(err))
				os.Exit(1)
			}
		}

		logger.Info("Deckhouse starts in HA mode")
		runWithLeaderElection(ctx, operator, logger)

		return nil
	}
}

func entrypoint(logger *log.Logger) error {
	var possibleBundles = []string{"Default", "Minimal", "Managed"}
	bundleEnvValue, found := os.LookupEnv(deckhouseBundleEnv)
	if !found || len(bundleEnvValue) == 0 {
		bundleEnvValue = "Default"
	}

	if !slices.Contains(possibleBundles, bundleEnvValue) {
		logger.Fatal(fmt.Sprintf("Deckhouse bundle %q doesn't exist! -- Possible bundles: %s", bundleEnvValue, strings.Join(possibleBundles, ", ")))
	}

	chrootDirEnvValue, found := os.LookupEnv(chrootDirEnv)
	if found && len(chrootDirEnvValue) > 0 {
		chrootedTmpDirPath := filepath.Join(chrootDirEnvValue, aoapp.DefaultTempDir)
		if err := os.MkdirAll(chrootedTmpDirPath, 0750); err != nil {
			return fmt.Errorf("create chroot dir: %w", err)
		}

		if _, err := os.Stat(aoapp.DefaultTempDir); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				if err := os.Symlink(chrootedTmpDirPath, aoapp.DefaultTempDir); err != nil {
					return fmt.Errorf("create tmp directory symlink: %w", err)
				}
			} else {
				return fmt.Errorf("stat tmp directory symlink: %w", err)
			}
		}
	}

	modulesDirEnvValue, found := os.LookupEnv(modulesDirEnv)
	if !found || len(modulesDirEnvValue) == 0 {
		return fmt.Errorf("%q env not set", modulesDirEnv)
	}

	coreModulesDir := strings.Split(modulesDirEnvValue, ":")[0]
	bundelValuesFilePath := filepath.Join(coreModulesDir, fmt.Sprintf("values-%s.yaml", strings.ToLower(bundleEnvValue)))
	bytes, err := os.ReadFile(bundelValuesFilePath)
	if err != nil {
		return fmt.Errorf("read bundle values file: %w", err)
	}

	if err := os.WriteFile("/tmp/values.yaml", bytes, 0644); err != nil {
		return fmt.Errorf("write values file: %w", err)
	}

	logger.Info(fmt.Sprintf("-- Starting Deckhouse using %q $bundle --", bundleEnvValue))

	return nil
}

func runWithLeaderElection(ctx context.Context, operator *addonoperator.AddonOperator, logger *log.Logger) {
	var identity string
	podName := os.Getenv("DECKHOUSE_POD")
	if len(podName) == 0 {
		logger.Fatal("DECKHOUSE_POD env not set or empty")
	}

	podIP := os.Getenv("ADDON_OPERATOR_LISTEN_ADDRESS")
	if len(podIP) == 0 {
		logger.Fatal("ADDON_OPERATOR_LISTEN_ADDRESS env not set or empty")
	}

	podNs := os.Getenv("ADDON_OPERATOR_NAMESPACE")
	if len(podNs) == 0 {
		podNs = defaultNamespace
	}

	clusterDomain := os.Getenv("KUBERNETES_CLUSTER_DOMAIN")
	if len(clusterDomain) == 0 {
		logger.Warn("KUBERNETES_CLUSTER_DOMAIN env not set or empty - its value won't be used for the leader election")
		identity = fmt.Sprintf("%s.%s.%s.pod", podName, strings.ReplaceAll(podIP, ".", "-"), podNs)
	} else {
		identity = fmt.Sprintf("%s.%s.%s.pod.%s", podName, strings.ReplaceAll(podIP, ".", "-"), podNs, clusterDomain)
	}

	err := operator.WithLeaderElector(&leaderelection.LeaderElectionConfig{
		// Create a leaderElectionConfig for leader election
		Lock: &resourcelock.LeaseLock{
			LeaseMeta: v1.ObjectMeta{
				Name:      leaseName,
				Namespace: podNs,
			},
			Client: operator.KubeClient().CoordinationV1(),
			LockConfig: resourcelock.ResourceLockConfig{
				Identity: identity,
			},
		},
		LeaseDuration: time.Duration(leaseDuration) * time.Second,
		RenewDeadline: time.Duration(renewalDeadline) * time.Second,
		RetryPeriod:   time.Duration(retryPeriod) * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				err := run(ctx, operator, logger)
				if err != nil {
					operator.Logger.Info("run", log.Err(err))
					os.Exit(1)
				}
			},
			OnStoppedLeading: func() {
				operator.Logger.Info("Restarting because the leadership was handed over")
				operator.Stop()
				os.Exit(0)
			},
		},
		ReleaseOnCancel: true,
	})
	if err != nil {
		operator.Logger.Error("run with leader elector", log.Err(err))
	}

	go func() {
		<-ctx.Done()
		logger.Info("Context canceled received")
		if err := syscall.Kill(1, syscall.SIGUSR2); err != nil {
			logger.Fatal("Couldn't shutdown deckhouse", log.Err(err))
		}
	}()

	operator.LeaderElector.Run(ctx)
}

func run(ctx context.Context, operator *addonoperator.AddonOperator, logger *log.Logger) error {
	exitCh := make(chan struct{})
	operatorStarted := false
	go signalHandler(ctx, exitCh, operator, &operatorStarted, logger)

	if err := d8Apis.EnsureCRDs(ctx, operator.KubeClient(), "/deckhouse/deckhouse-controller/crds/*.yaml"); err != nil {
		return fmt.Errorf("ensure crds: %w", err)
	}

	// we have to lock the controller run if dhctl lock configmap exists
	if err := lockOnBootstrap(ctx, operator.KubeClient(), logger); err != nil {
		return fmt.Errorf("lock on bootstrap: %w", err)
	}

	if DefaultReleaseChannel == "" {
		DefaultReleaseChannel = defaultReleaseChannel
	}

	deckhouseController, err := controller.NewDeckhouseController(ctx, DeckhouseVersion, DefaultReleaseChannel, operator, logger.Named("deckhouse-controller"))
	if err != nil {
		return fmt.Errorf("create deckhouse controller: %w", err)
	}

	// load modules from FS, start controllers and run deckhouse config event loop
	if err = deckhouseController.Start(ctx); err != nil {
		return fmt.Errorf("start deckhouse controller: %w", err)
	}

	if err = operator.Start(ctx); err != nil {
		return fmt.Errorf("start operator: %w", err)
	}

	operatorStarted = true

	debugserver.RegisterRoutes(operator.DebugServer)

	// block main thread by waiting signals from OS.
	<-exitCh

	return nil
}

func signalHandler(ctx context.Context, exitCh chan struct{}, operator *addonoperator.AddonOperator, operatorStarted *bool, logger *log.Logger) {
	telemetryShutdown := registerTelemetry(ctx)

	interruptCh := make(chan os.Signal, 5)
	signal.Notify(interruptCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1, syscall.SIGUSR2, syscall.SIGCHLD)
	rm := reaperMutex{}
	for {
		select {
		case <-ctx.Done():
			logger.Info("Context canceled - exiting")

			exitCh <- struct{}{}
			return

		case sig := <-interruptCh:
			switch sig {
			case syscall.SIGUSR1, syscall.SIGUSR2:
				environ := os.Environ()
				skipEntrypointKeyValue := fmt.Sprintf("%s=true", skipEntrypointEnv)
				if !slices.Contains(environ, skipEntrypointKeyValue) {
					environ = append(environ, skipEntrypointKeyValue)
				}
				logger.Info(fmt.Sprintf("A %q signal was received, Deckhouse is restarting", sig.String()))
				if err := telemetryShutdown(ctx); err != nil {
					logger.Error("telemetry shutdown", log.Err(err))
				}

				if *operatorStarted {
					operator.Stop()
				}
				if err := syscall.Kill(-1, syscall.SIGKILL); err != nil {
					if !errors.Is(err, syscall.ECHILD) && !errors.Is(err, syscall.ESRCH) {
						logger.Error("Couldn't kill child processes", log.Err(err))
					}
				}
				deckhouseBinaryToRun := deckhouseControllerBinaryPath
				chrootDirEnvValue, found := os.LookupEnv(chrootDirEnv)
				if found && len(chrootDirEnvValue) > 0 {
					deckhouseBinaryToRun = deckhouseControllerWithCapsBinaryPath
				}
				if err := syscall.Exec(deckhouseBinaryToRun, []string{deckhouseBinaryToRun, "start"}, environ); err != nil {
					log.Error("Couldn't restart Deckhouse", log.Err(err))
					os.Exit(1)
				}

			case syscall.SIGCHLD:
				rm.Lock()
				if !rm.scheduled {
					rm.scheduled = true
					rm.Unlock()
					go func() {
						defer rm.Release()
						// give some time to real parent processes to reap their children if any
						time.Sleep(time.Second)

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

								if ppid == 1 {
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
				logger.Info(fmt.Sprintf("A %q signal was received, Deckhouse is shutting down", sig.String()))
				if err := telemetryShutdown(ctx); err != nil {
					logger.Error("telemetry shutdown", log.Err(err))
				}

				if *operatorStarted {
					operator.Stop()
				}
				if err := syscall.Kill(-1, syscall.SIGKILL); err != nil {
					if !errors.Is(err, syscall.ECHILD) && !errors.Is(err, syscall.ESRCH) {
						logger.Error("Couldn't kill child processes", log.Err(err))
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

const (
	cmLockName  = "deckhouse-bootstrap-lock"
	cmNamespace = "d8-system"
)

func lockOnBootstrap(ctx context.Context, client *client.Client, logger *log.Logger) error {
	bk := wait.Backoff{
		Duration: 1 * time.Second,
		Factor:   1.2,
		Jitter:   1,
		Steps:    10,
		Cap:      5 * time.Minute,
	}

	return retry.OnError(bk, func(err error) bool {
		logger.Error("An error occurred during the bootstrap lock. Retrying", log.Err(err))
		// retry on any error
		return true
	}, func() error {
		if _, err := client.CoreV1().ConfigMaps(cmNamespace).Get(ctx, cmLockName, v1.GetOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("get the '%s' configmap: %w", cmLockName, err)
		}

		logger.Info("Bootstrap lock ConfigMap exists. Waiting for bootstrap process to be done")

		listOpts := v1.ListOptions{
			FieldSelector: "metadata.name=" + cmLockName,
			Watch:         true,
		}
		wch, err := client.CoreV1().ConfigMaps(cmNamespace).Watch(ctx, listOpts)
		if err != nil {
			return fmt.Errorf("watch configmaps: %w", err)
		}

		for event := range wch.ResultChan() {
			if event.Type == watch.Deleted {
				break
			}
		}
		wch.Stop()

		logger.Info("Bootstrap lock has been released")

		return nil
	})
}

func registerTelemetry(ctx context.Context) func(ctx context.Context) error {
	endpoint := os.Getenv("TRACING_OTLP_ENDPOINT")
	authToken := os.Getenv("TRACING_OTLP_AUTH_TOKEN")

	if endpoint == "" {
		return func(_ context.Context) error {
			return nil
		}
	}

	opts := make([]otlptracegrpc.Option, 0, 1)

	opts = append(opts, otlptracegrpc.WithEndpoint(endpoint))
	opts = append(opts, otlptracegrpc.WithInsecure())

	if authToken != "" {
		opts = append(opts, otlptracegrpc.WithHeaders(map[string]string{
			"Authorization": "Bearer " + strings.TrimSpace(authToken),
		}))
	}

	exporter, _ := otlptracegrpc.New(ctx, opts...)

	resource := sdkresource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(AppName),
		semconv.ServiceVersionKey.String(DeckhouseVersion),
		semconv.TelemetrySDKLanguageKey.String("en"),
		semconv.K8SDeploymentName(AppName),
	)

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	otel.SetTracerProvider(provider)

	return provider.Shutdown
}
