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
	"errors"
	"net"
	"net/http"
	"time"

	"fencing-controller/internal/watchdog"

	"go.uber.org/zap"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	fencingNodeLabel = "node-manager.deckhouse.io/fencing-enabled"
)

var maintenanceAnnotations = [...]string{
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
	if node.Labels == nil {
		node.Labels = make(map[string]string)
	}
	node.Labels[fencingNodeLabel] = ""
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
	delete(node.Labels, fencingNodeLabel)
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
	fa.logger.Info("Set fencing node label", zap.String("label", fencingNodeLabel))
	err = fa.setNodeLabel(ctx)
	if err != nil {
		// We must stop watchdog if we can't set nodelabel
		fa.logger.Error("Unable to set node label, so disarming watchdog...")
		_ = fa.watchDog.Stop()
		return err
	}
	return nil
}

func (fa *FencingAgent) startLiveness(ctx context.Context) {
	fa.logger.Info("Starting the healthz server")
	srv := &http.Server{Addr: fa.config.HealthProbeBindAddress, Handler: nil}

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			fa.logger.Fatal("HTTP server ListenAndServe:", zap.Error(err))
		}
	}()

	// Graceful shutdown
	go func() {
		<-ctx.Done()
		fa.logger.Info("Shutting down the healthz server")
		if err := srv.Shutdown(context.Background()); err != nil {
			fa.logger.Fatal("HTTP server Shutdown:", zap.Error(err))
		}
	}()
}

func (fa *FencingAgent) stopWatchdog() error {
	var err error
	fa.logger.Info("Remove fencing node label", zap.String("label", fencingNodeLabel))
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
	var lastMessageTime time.Time
	var err error

	if fa.config.HealthProbeBindAddress != "" {
		fa.startLiveness(ctx)
	}

	for {
		select {
		case <-ticker.C:
			// check kubernets API
			node, err := fa.kubeClient.CoreV1().Nodes().Get(context.TODO(), fa.config.NodeName, v1.GetOptions{})
			if err != nil {
				var netErr net.Error

				if errors.As(err, &netErr) && netErr.Timeout() {
					// only API timeout is reasonable error
					fa.logger.Error("API request timed out", zap.Error(err))
					APIIsAvailable = false
				} else {
					// API is available but some error happened
					fa.logger.Error("Unable to reach the API due to an error", zap.Error(err))
					APIIsAvailable = true
				}
			} else {
				// show message just one time in an interval
				if time.Since(lastMessageTime) > fa.config.KubernetesAPICheckInterval {
					fa.logger.Info("The API is available")
					lastMessageTime = time.Now()
				}
				APIIsAvailable = true
			}
			// check if node is in maintenance mode
			MaintenanceMode := false
			for _, annotation := range maintenanceAnnotations {
				_, annotationExists := node.Annotations[annotation]
				if annotationExists {
					fa.logger.Info("Maintenance annotation found", zap.String("annotation", annotation))
					MaintenanceMode = true
				}
			}

			// Watchdog activation lifecycle
			if MaintenanceMode && fa.watchDog.IsArmed() {
				// disarm the watchdog if maintenance mode is on and the watchdog is armed
				err = fa.stopWatchdog()
				if err != nil {
					fa.logger.Error("Unable to disarm the watchdog", zap.Error(err))
				}
			}
			if !MaintenanceMode && !fa.watchDog.IsArmed() {
				// arm the watchdog if maintenance mode is off and the watchdog is not armed
				err = fa.startWatchdog(ctx)
				if err != nil {
					fa.logger.Error("Unable to arm the watchdog", zap.Error(err))
					return err
				}
			}

			fa.logger.Debug("Watchdog status", zap.Bool("is armed", fa.watchDog.IsArmed()))

			// Checking API
			if APIIsAvailable && !MaintenanceMode {
				// API is available and not in maintenance mode, so feed the watchdog
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
