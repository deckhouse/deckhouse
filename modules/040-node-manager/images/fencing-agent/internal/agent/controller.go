/*
Copyright 2024 Flant JSC

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
package agent

import (
	"context"
	"net/http"
	"time"

	"fecning-controller/internal/watchdog"

	"go.uber.org/zap"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	FecningNodeLabel = "node-manager.deckhouse.io/fencing-enabled"
)

var maintanenceAnnotations = [...]string{
	`update.node.deckhouse.io/disruption-approved`,
	`update.node.deckhouse.io/approved`,
	`node-manager.deckhouse.io/fencing-disable`,
}

type FencingAgent struct {
	logger     *zap.Logger
	config     Config
	kubeClient kubernetes.Interface
	watchDog   watchdog.WatchDog
}

func NewFencingAgent(logger *zap.Logger, config Config, kubeClient kubernetes.Interface, wd watchdog.WatchDog) *FencingAgent {
	l := logger.With(zap.String("node", config.NodeName))
	return &FencingAgent{
		logger:     l,
		config:     config,
		kubeClient: kubeClient,
		watchDog:   wd,
	}
}

func (fa *FencingAgent) setNodeLabel(ctx context.Context) error {
	node, err := fa.kubeClient.CoreV1().Nodes().Get(ctx, fa.config.NodeName, v1.GetOptions{})
	if err != nil {
		return err
	}
	node.Labels[FecningNodeLabel] = ""
	_, err = fa.kubeClient.CoreV1().Nodes().Update(ctx, node, v1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (fa *FencingAgent) removeNodeLabel() error {
	node, err := fa.kubeClient.CoreV1().Nodes().Get(context.TODO(), fa.config.NodeName, v1.GetOptions{})
	if err != nil {
		return err
	}
	delete(node.Labels, FecningNodeLabel)
	_, err = fa.kubeClient.CoreV1().Nodes().Update(context.TODO(), node, v1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (fa *FencingAgent) startWatchdog(ctx context.Context) error {
	var err error
	fa.logger.Info("Arm the watchdog")
	err = fa.watchDog.Start()
	if err != nil {
		return err
	}
	fa.logger.Info("Set fencing node label", zap.String("label", FecningNodeLabel))
	err = fa.setNodeLabel(ctx)
	if err != nil {
		// We must stop watchdog if we can't set nodelabel
		fa.logger.Error("Unable to set node label, so disarming watchdog...")
		_ = fa.watchDog.Stop()
		return err
	}
	return nil
}

func (fa *FencingAgent) startLiveness() {
	fa.logger.Info("Starting the healthz server")
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	_ = http.ListenAndServe(fa.config.HealthProbeBindAddress, nil)
}

func (fa *FencingAgent) stopWatchdog() error {
	var err error
	fa.logger.Info("Remove fencing node label", zap.String("label", FecningNodeLabel))
	err = fa.removeNodeLabel()
	if err != nil {
		return err
	}
	fa.logger.Info("Disarm the watchdog")
	err = fa.watchDog.Stop()
	if err != nil {
		return err
	}
	return nil
}

func (fa *FencingAgent) Run(ctx context.Context) error {
	ticker := time.NewTicker(fa.config.KubernetesAPICheckInterval)
	var APIIsAvailable bool
	var err error
	// for parallel tests
	if fa.config.HealthProbeBindAddress != "" {
		go fa.startLiveness()
	}

	for {
		select {
		case <-ticker.C:
			// check kubernets API
			node, err := fa.kubeClient.CoreV1().Nodes().Get(context.TODO(), fa.config.NodeName, v1.GetOptions{})
			if err != nil {
				fa.logger.Error("Unable to reach the API", zap.Error(err))
				APIIsAvailable = false
			} else {
				fa.logger.Debug("The API is available")
				APIIsAvailable = true
			}

			// check if node is in maintenance mode
			MaintenanceMode := false
			for _, annotation := range maintanenceAnnotations {
				_, annotationExists := node.Annotations[annotation]
				if annotationExists {
					fa.logger.Info("Maintenance annotation found", zap.String("annotation", annotation))
					MaintenanceMode = true
				}
			}

			// Watchdog activation lifecycle
			if MaintenanceMode && fa.watchDog.IsArmed() {
				err = fa.stopWatchdog()
				if err != nil {
					fa.logger.Error("Unable to disarm the watchdog", zap.Error(err))
				}
			}
			if !MaintenanceMode && !fa.watchDog.IsArmed() {
				err = fa.startWatchdog(ctx)
				if err != nil {
					fa.logger.Error("Unable to arm the watchdog", zap.Error(err))
					return err
				}
			}

			fa.logger.Debug("Watchdog status", zap.Bool("is armed", fa.watchDog.IsArmed()))

			// Checking API
			if APIIsAvailable && !MaintenanceMode {
				fa.logger.Debug("Feeding the watchdog")
				err = fa.watchDog.Feed()
				if err != nil {
					fa.logger.Error("Unable to feed watchdog", zap.Error(err))
				}
			}

		case <-ctx.Done():
			fa.logger.Debug("Finishing the API check")
			if fa.watchDog.IsArmed() {
				err = fa.stopWatchdog()
				if err != nil {
					fa.logger.Error("Unable to disarm watchdog", zap.Error(err))
				}
			}
			return nil
		}
	}
}
