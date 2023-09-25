/*
Copyright 2023 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"fmt"
	lclient "github.com/LINBIT/golinstor/client"
	"go.uber.org/zap/zapcore"
	v1 "k8s.io/api/core/v1"
	sv1 "k8s.io/api/storage/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"os"
	goruntime "runtime"
	"sds-drbd-operator/api/v1alpha1"
	"sds-drbd-operator/config"
	"sds-drbd-operator/pkg/controller"
	kubutils "sds-drbd-operator/pkg/kubeutils"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	OperatorNamespace = "d8-linstor"
)

var (
	resourcesSchemeFuncs = []func(*apiruntime.Scheme) error{
		v1alpha1.AddToScheme,
		clientgoscheme.AddToScheme,
		extv1.AddToScheme,
		v1.AddToScheme,
		sv1.AddToScheme,
	}
)

func main() {
	log := zap.New(zap.Level(zapcore.Level(-1)), zap.UseDevMode(true))
	ctx, _ := context.WithCancel(context.Background())

	log.Info(fmt.Sprintf("Go Version:%s ", goruntime.Version()))
	log.Info(fmt.Sprintf("OS/Arch:Go OS/Arch:%s/%s ", goruntime.GOOS, goruntime.GOARCH))

	cfgParams, err := config.NewConfig()
	if err != nil {
		log.Error(err, "error read configuration")
	}

	// Create default config Kubernetes client
	kConfig, err := kubutils.KubernetesDefaultConfigCreate()
	if err != nil {
		log.Error(err, "error read kubernetes configuration")
	}
	log.Info("read Kubernetes config")

	// Setup scheme for all resources
	scheme := runtime.NewScheme()
	for _, f := range resourcesSchemeFuncs {
		err := f(scheme)
		if err != nil {
			log.Error(err, "failed to add to scheme")
			os.Exit(1)
		}
	}
	log.Info("read scheme CR")

	managerOpts := manager.Options{
		Scheme:             scheme,
		MetricsBindAddress: cfgParams.MetricsPort,
		Logger:             log,
		Namespace:          OperatorNamespace, // TODO: change to cache options
	}

	mgr, err := manager.New(kConfig, managerOpts)
	if err != nil {
		log.Error(err, "failed create manager")
		os.Exit(1)
	}

	log.Info("create kubernetes manager")

	lc, err := lclient.NewClient()

	if _, err := controller.NewLinstorNode(ctx, mgr, lc, cfgParams.ConfigSecretName, cfgParams.ScanInterval); err != nil {
		log.Error(err, "failed create controller NewLinstorNode", err)
		os.Exit(1)
	}
	log.Info("controller NewLinstorNode start")

	if _, err := controller.NewLinstorStorageClass(ctx, mgr); err != nil {
		log.Error(err, "failed create controller NewLinstorStorageClass")
		os.Exit(1)
	}
	log.Info("controller NewLinstorStorageClass start")

	if _, err := controller.NewLinstorStoragePool(ctx, mgr, lc); err != nil {
		log.Error(err, "failed create controller NewLinstorStoragePool", err)
		os.Exit(1)
	}
	log.Info("controller NewLinstorStoragePool start")

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	err = mgr.Start(ctx)
	if err != nil {
		log.Error(err, "error start manager")
		os.Exit(1)
	}

	log.Info("starting the manager")
}
