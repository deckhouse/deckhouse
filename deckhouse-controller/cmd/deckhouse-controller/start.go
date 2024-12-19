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
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	addonoperator "github.com/flant/addon-operator/pkg/addon-operator"
	"github.com/flant/kube-client/client"
	shapp "github.com/flant/shell-operator/pkg/app"
	utilsignal "github.com/flant/shell-operator/pkg/utils/signal"
	"gopkg.in/alecthomas/kingpin.v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/util/retry"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller"
	debugserver "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/debug-server"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	leaseName        = "deckhouse-leader-election"
	defaultNamespace = "d8-system"
	leaseDuration    = 35
	renewalDeadline  = 30
	retryPeriod      = 10

	crdsGlob = "/deckhouse/deckhouse-controller/crds/*.yaml"

	configMapLock = "deckhouse-bootstrap-lock"
)

func start(logger *log.Logger) func(_ *kingpin.ParseContext) error {
	return func(_ *kingpin.ParseContext) error {
		shapp.AppStartMessage = app.Version()

		ctx := context.Background()

		operator := addonoperator.NewAddonOperator(ctx, addonoperator.WithLogger(logger.Named("addon-operator")))

		operator.StartAPIServer()

		if app.VarModeHA == "true" {
			logger.Info("deckhouse is starting in HA mode")
			runHAMode(ctx, operator, logger)
			return nil
		}

		if err := run(ctx, operator, logger); err != nil {
			logger.Error("run", log.Err(err))
			os.Exit(1)
		}

		return nil
	}
}

func runHAMode(ctx context.Context, operator *addonoperator.AddonOperator, logger *log.Logger) {
	var identity string
	podName := os.Getenv("DECKHOUSE_POD")
	if len(podName) == 0 {
		log.Fatal("DECKHOUSE_POD env not set or empty")
	}

	podIP := os.Getenv("ADDON_OPERATOR_LISTEN_ADDRESS")
	if len(podIP) == 0 {
		log.Fatal("ADDON_OPERATOR_LISTEN_ADDRESS env not set or empty")
	}

	podNs := os.Getenv("ADDON_OPERATOR_NAMESPACE")
	if len(podNs) == 0 {
		podNs = defaultNamespace
	}

	clusterDomain := os.Getenv("KUBERNETES_CLUSTER_DOMAIN")
	if len(clusterDomain) == 0 {
		log.Warn("KUBERNETES_CLUSTER_DOMAIN env not set or empty - it's value won't be used for the leader election")
		identity = fmt.Sprintf("%s.%s.%s.pod", podName, strings.ReplaceAll(podIP, ".", "-"), podNs)
	} else {
		identity = fmt.Sprintf("%s.%s.%s.pod.%s", podName, strings.ReplaceAll(podIP, ".", "-"), podNs, clusterDomain)
	}

	if err := operator.WithLeaderElector(&leaderelection.LeaderElectionConfig{
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
	}); err != nil {
		operator.Logger.Error("run", log.Err(err))
	}

	go func() {
		<-ctx.Done()
		log.Info("Context canceled received")
		if err := syscall.Kill(1, syscall.SIGUSR2); err != nil {
			log.Fatalf("Couldn't shutdown deckhouse: %s\n", err)
		}
	}()

	operator.LeaderElector.Run(ctx)
}

func run(ctx context.Context, operator *addonoperator.AddonOperator, logger *log.Logger) error {
	if err := apis.EnsureCRDs(ctx, operator.KubeClient(), crdsGlob); err != nil {
		return fmt.Errorf("ensure crds: %w", err)
	}

	// we have to lock the controller run if dhctl lock configmap exists
	if err := lockOnBootstrap(ctx, operator.KubeClient(), logger); err != nil {
		return fmt.Errorf("lock on bootstrap: %w", err)
	}

	// initialize and start controllers, load modules from FS, and run deckhouse config event loop
	if err := controller.Start(ctx, operator, logger); err != nil {
		return fmt.Errorf("create deckhouse controller: %w", err)
	}

	if err := operator.Start(ctx); err != nil {
		return fmt.Errorf("start operator: %w", err)
	}

	debugserver.RegisterRoutes(operator.DebugServer)

	// block main thread by waiting signals from OS.
	utilsignal.WaitForProcessInterruption(func() {
		operator.Stop()
		os.Exit(0)
	})

	return nil
}

func lockOnBootstrap(ctx context.Context, client *client.Client, logger *log.Logger) error {
	backoff := wait.Backoff{
		Duration: 1 * time.Second,
		Factor:   1.2,
		Jitter:   1,
		Steps:    10,
		Cap:      5 * time.Minute,
	}

	return retry.OnError(backoff, func(err error) bool {
		logger.Errorf("failed to look bootstrap: %v, retry it", err)
		// retry on any error
		return true
	}, func() error {
		if _, err := client.CoreV1().ConfigMaps(app.NamespaceDeckhouse).Get(ctx, configMapLock, v1.GetOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("get the '%s' configmap: %w", configMapLock, err)
		}

		logger.Info("the bootstrap lock config map exists, wait for bootstrap process to be done")

		listOpts := v1.ListOptions{
			FieldSelector: "metadata.name=" + configMapLock,
			Watch:         true,
		}
		wch, err := client.CoreV1().ConfigMaps(app.NamespaceDeckhouse).Watch(ctx, listOpts)
		if err != nil {
			return fmt.Errorf("watch configmaps: %w", err)
		}

		for event := range wch.ResultChan() {
			if event.Type == watch.Deleted {
				break
			}
		}
		wch.Stop()

		logger.Info("bootstrap lock has been released")

		return nil
	})
}
