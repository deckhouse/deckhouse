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

	addon_operator "github.com/flant/addon-operator/pkg/addon-operator"
	"github.com/flant/addon-operator/pkg/utils"
	"github.com/flant/kube-client/client"
	sh_app "github.com/flant/shell-operator/pkg/app"
	utils_signal "github.com/flant/shell-operator/pkg/utils/signal"
	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/util/retry"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addon-operator/kube-config/backend"
	d8Apis "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/validation"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller"
	d8config "github.com/deckhouse/deckhouse/go_lib/deckhouse-config"
)

const (
	leaseName        = "deckhouse-leader-election"
	defaultNamespace = "d8-system"
	leaseDuration    = 35
	renewalDeadline  = 30
	retryPeriod      = 10
)

func start(_ *kingpin.ParseContext) error {
	sh_app.AppStartMessage = version()

	ctx := context.Background()

	operator := addon_operator.NewAddonOperator(ctx)

	operator.StartAPIServer()

	if os.Getenv("DECKHOUSE_HA") == "true" {
		log.Info("Desckhouse is starting in HA mode")
		runHAMode(ctx, operator)
		return nil
	}

	err := run(ctx, operator)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}

	return nil
}

func runHAMode(ctx context.Context, operator *addon_operator.AddonOperator) {
	podName := os.Getenv("DECKHOUSE_POD")
	if len(podName) == 0 {
		log.Info("DECKHOUSE_POD env not set or empty")
		os.Exit(1)
	}

	podIP := os.Getenv("ADDON_OPERATOR_LISTEN_ADDRESS")
	if len(podIP) == 0 {
		log.Info("ADDON_OPERATOR_LISTEN_ADDRESS env not set or empty")
		os.Exit(1)
	}

	podNs := os.Getenv("ADDON_OPERATOR_NAMESPACE")
	if len(podNs) == 0 {
		podNs = defaultNamespace
	}
	identity := fmt.Sprintf("%s.%s.%s.pod", podName, strings.ReplaceAll(podIP, ".", "-"), podNs)

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
				err := run(ctx, operator)
				if err != nil {
					log.Info(err)
					os.Exit(1)
				}
			},
			OnStoppedLeading: func() {
				log.Info("Restarting because the leadership was handed over")
				operator.Stop()
				os.Exit(1)
			},
		},
		ReleaseOnCancel: true,
	})
	if err != nil {
		log.Error(err)
	}

	go func() {
		<-ctx.Done()
		log.Info("Context canceled received")
		err := syscall.Kill(1, syscall.SIGUSR2)
		if err != nil {
			log.Infof("Couldn't shutdown deckhouse: %s\n", err)
			os.Exit(1)
		}
	}()

	operator.LeaderElector.Run(ctx)
}

func run(ctx context.Context, operator *addon_operator.AddonOperator) error {
	err := d8Apis.EnsureCRDs(ctx, operator.KubeClient(), "/deckhouse/deckhouse-controller/crds/*.yaml")
	if err != nil {
		return err
	}

	// we have to lock the controller run if dhctl lock configmap exists
	err = lockOnBootstrap(ctx, operator.KubeClient())
	if err != nil {
		return err
	}

	deckhouseConfigC := make(chan utils.Values, 1)

	operator.SetupKubeConfigManager(backend.New(operator.KubeClient().RestConfig(), deckhouseConfigC, log.StandardLogger().WithField("KubeConfigManagerBackend", "ModuleConfig")))
	validation.RegisterAdmissionHandlers(operator)

	err = operator.Setup()
	if err != nil {
		return err
	}

	dController, err := controller.NewDeckhouseController(ctx, operator.KubeClient().RestConfig(), operator.ModuleManager, operator.MetricStorage)
	if err != nil {
		return err
	}

	err = dController.Start(operator.ModuleManager.GetModuleEventsChannel(), deckhouseConfigC)
	if err != nil {
		return err
	}

	operator.ModuleManager.SetModuleLoader(dController)

	// Init deckhouse-config service with ModuleManager instance.
	d8config.InitService(operator.ModuleManager)

	err = operator.Start()
	if err != nil {
		return err
	}

	dController.RunControllers()

	// Block main thread by waiting signals from OS.
	utils_signal.WaitForProcessInterruption(func() {
		operator.Stop()
		os.Exit(1)
	})

	return nil
}

const (
	cmLockName  = "deckhouse-bootstrap-lock"
	cmNamespace = "d8-system"
)

func lockOnBootstrap(ctx context.Context, client *client.Client) error {
	bk := wait.Backoff{
		Duration: 1 * time.Second,
		Factor:   1.2,
		Jitter:   1,
		Steps:    10,
		Cap:      5 * time.Minute,
	}

	return retry.OnError(bk, func(err error) bool {
		log.Errorf("An error occurred during the bootstrap lock: %s. Retrying", err)
		// retry on any error
		return true
	}, func() error {
		_, err := client.CoreV1().ConfigMaps(cmNamespace).Get(ctx, cmLockName, v1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}

			return err
		}

		log.Info("Bootstrap lock ConfigMap exists. Waiting for bootstrap process to be done")

		listOpts := v1.ListOptions{
			FieldSelector: "metadata.name=" + cmLockName,
			Watch:         true,
		}
		wch, err := client.CoreV1().ConfigMaps(cmNamespace).Watch(ctx, listOpts)
		if err != nil {
			return err
		}

		for event := range wch.ResultChan() {
			if event.Type == watch.Deleted {
				break
			}
		}
		wch.Stop()

		log.Info("Bootstrap lock has been released")

		return nil
	})
}
