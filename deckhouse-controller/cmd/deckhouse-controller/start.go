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

	"github.com/flant/kube-client/client"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/util/retry"

	addon_operator "github.com/flant/addon-operator/pkg/addon-operator"
	sh_app "github.com/flant/shell-operator/pkg/app"
	utils_signal "github.com/flant/shell-operator/pkg/utils/signal"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addon-operator/kube-config/backend"
	d8Apis "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/validation"
	d8config "github.com/deckhouse/deckhouse/go_lib/deckhouse-config"
	"github.com/deckhouse/deckhouse/go_lib/module"
	"github.com/deckhouse/deckhouse/modules/002-deckhouse/hooks/pkg/apis"
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

	err = waitForMe(ctx, operator.KubeClient())
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	operator.SetupKubeConfigManager(backend.New(operator.KubeClient().RestConfig(), nil))

	// TODO: remove deckhouse-config purge after release 1.56
	operator.ExplicitlyPurgeModules = []string{"deckhouse-config"}
	validation.RegisterAdmissionHandlers(operator)
	// TODO: move this routes to the deckhouse-controller
	module.SetupAdmissionRoutes(operator.AdmissionServer)

	err = operator.Setup()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	operator.ModuleManager.SetupModuleProducer(apis.NewModuleProducer())

	err = operator.Start()
	if err != nil {
		os.Exit(1)
	}

	// Init deckhouse-config service with ModuleManager instance.
	d8config.InitService(operator.ModuleManager)

	// Block main thread by waiting signals from OS.
	utils_signal.WaitForProcessInterruption(func() {
		operator.Stop()
		os.Exit(1)
	})

	return nil
}

const (
	cmLockName  = "foobar"
	cmNamespace = "d8-system"
)

func waitForMe(ctx context.Context, client *client.Client) error {
	bk := wait.Backoff{
		Duration: 1 * time.Second,
		Factor:   1.2,
		Jitter:   1,
		Steps:    10,
		Cap:      5 * time.Minute,
	}
	fmt.Println("RUN RETRY")
	return retry.OnError(bk, func(err error) bool {
		return true
	}, func() error {
		fmt.Println("CHECKING FOR MAP")
		_, err := client.CoreV1().ConfigMaps(cmNamespace).Get(ctx, cmLockName, v1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				fmt.Println("MAP DOEST NOT EXIST")
				return nil
			}

			return err
		}

		fmt.Println("MAP EXISTS. LOCKING")

		listOpts := v1.ListOptions{
			FieldSelector: "metadata.name=" + cmLockName,
			Watch:         true,
		}
		wch, err := client.CoreV1().ConfigMaps(cmNamespace).Watch(ctx, listOpts)
		if err != nil {
			return err
		}
		fmt.Println("RUN WATCHER")

		for event := range wch.ResultChan() {
			if event.Type == watch.Deleted {
				fmt.Println("MAP WAS DELETED")
				break
			}
		}
		fmt.Println("MAP STOP")
		wch.Stop()

		return nil
	})
}
