// Copyright 2021 Flant JSC
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

package deckhouse

import (
	"context"

	addon_operator "github.com/flant/addon-operator/pkg/addon-operator"
	klient "github.com/flant/kube-client/client"
	sh_app "github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/metric_storage"
	log "github.com/sirupsen/logrus"
)

// Ignore error: type name will be used as deckhouse.DeckhouseController by other packages, and that stutters
//nolint:golint
type DeckhouseController struct {
	*addon_operator.AddonOperator

	ctx    context.Context
	cancel context.CancelFunc
}

func NewDeckhouseController() *DeckhouseController {
	return &DeckhouseController{
		AddonOperator: addon_operator.NewAddonOperator(),
	}
}

func (d *DeckhouseController) WithContext(ctx context.Context) *DeckhouseController {
	d.ctx, d.cancel = context.WithCancel(ctx)
	d.AddonOperator.WithContext(d.ctx)
	return d
}

func (d *DeckhouseController) Stop() {
	if d.cancel != nil {
		d.cancel()
	}
}

func (d *DeckhouseController) Shutdown() {
	d.AddonOperator.Shutdown()
}

// StartWatchRegistry initializes and starts a RegistryManager.
func (d *DeckhouseController) InitAndStartRegistryWatcher() error {
	// Initialize RegistryWatcher dependencies

	// Create and start a metric storage for the RegistryWatcher and the AddonOperator.
	metricStorage := metric_storage.NewMetricStorage()
	metricStorage.WithContext(d.ctx)
	metricStorage.WithPrefix(sh_app.PrometheusMetricsPrefix)
	metricStorage.Start()
	addon_operator.RegisterAddonOperatorMetrics(metricStorage)
	RegisterDeckhouseMetrics(metricStorage)
	// Set MetricStorage in addon-operator to use in a Kubernetes client initialization.
	d.AddonOperator.WithMetricStorage(metricStorage)

	// Create and initialize a Kubernetes client for the RegistryWatcher and the AddonOperator
	// Register metrics for client-go with custom labels.
	//nolint:staticcheck
	klient.RegisterKubernetesClientMetrics(metricStorage, d.AddonOperator.GetMainKubeClientMetricLabels())
	// Initialize a Kubernetes client with settings, metricStorage and custom metric labels.
	kubeClient, err := d.AddonOperator.InitMainKubeClient()
	if err != nil {
		log.Errorf("MAIN Fatal: initialize kube client: %s\n", err)
		return err
	}
	// Set KubeClient in AddonOperator.
	d.AddonOperator.WithKubernetesClient(kubeClient)

	return nil
}

func DefaultDeckhouse() *DeckhouseController {
	ctrl := NewDeckhouseController()
	ctrl.WithContext(context.Background())
	return ctrl
}

func InitAndStart(ctrl *DeckhouseController) error {
	err := addon_operator.InitAndStart(ctrl.AddonOperator)
	if err != nil {
		return err
	}

	return nil
}
