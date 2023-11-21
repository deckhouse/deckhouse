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
	"time"

	addon_operator "github.com/flant/addon-operator/pkg/addon-operator"
	"github.com/flant/kube-client/client"
	sh_app "github.com/flant/shell-operator/pkg/app"
	utils_signal "github.com/flant/shell-operator/pkg/utils/signal"
	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/util/retry"

	"github.com/deckhouse/deckhouse/deckhouse-controller/controller"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addon-operator/kube-config/backend"
	d8Apis "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/validation"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller"
	d8config "github.com/deckhouse/deckhouse/go_lib/deckhouse-config"
)

func start(_ *kingpin.ParseContext) error {
	sh_app.AppStartMessage = version()

	ctx := context.Background()

	operator := addon_operator.NewAddonOperator(ctx)

	err := d8Apis.EnsureCRDs(ctx, operator.KubeClient(), "/deckhouse/deckhouse-controller/crds/*.yaml")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// we have to lock the controller run if dhctl lock configmap exists
	err = lockOnBootstrap(ctx, operator.KubeClient())
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	operator.SetupKubeConfigManager(backend.New(operator.KubeClient().RestConfig(), log.StandardLogger().WithField("KubeConfigManagerBackend", "ModuleConfig")))
	validation.RegisterAdmissionHandlers(operator)

	err = operator.Setup()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	dController, err := controller.NewDeckhouseController(ctx, operator.KubeClient().RestConfig(), operator.ModuleManager)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	err = dController.Start(operator.ModuleManager.GetModuleEventsChannel())
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	operator.ModuleManager.SetModuleLoader(dController)

	err = operator.Start()
	if err != nil {
		os.Exit(1)
	}

	// Init deckhouse-config service with ModuleManager instance.
	d8config.InitService(operator.ModuleManager)

	moduleDocsSyncer, err := controller.NewModuleDocsSyncer()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	go moduleDocsSyncer.Run(ctx)

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
