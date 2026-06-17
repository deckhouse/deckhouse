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

package webhook

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	ctrlwebhook "sigs.k8s.io/controller-runtime/pkg/webhook"
)

// Registrar registers a validating admission webhook on a controller-runtime Manager.
type Registrar interface {
	Register(ctrl.Manager) error
}

// Server wraps a controller-runtime manager configured for admission webhooks.
type Server struct {
	manager ctrl.Manager
}

// NewServer creates a webhook server with TLS, metrics, and health probes enabled.
func NewServer(cfg *rest.Config, scheme *runtime.Scheme, options ServerConfig) (*Server, error) {
	manager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme,
		WebhookServer: ctrlwebhook.NewServer(ctrlwebhook.Options{
			Port:    options.WebhookPort,
			CertDir: options.WebhookCertDir,
		}),
		Metrics: metricsserver.Options{
			BindAddress: options.MetricsBindAddress,
		},
		HealthProbeBindAddress: options.HealthProbeBindAddress,
	})
	if err != nil {
		return nil, err
	}

	if err := manager.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return nil, err
	}

	if err := manager.AddReadyzCheck("readyz", manager.GetWebhookServer().StartedChecker()); err != nil {
		return nil, err
	}

	return &Server{manager: manager}, nil
}

// Register attaches a validating admission webhook to the server.
func (s *Server) Register(registrar Registrar) error {
	return registrar.Register(s.manager)
}

// Client returns the Kubernetes API client backed by the webhook server manager.
func (s *Server) Client() client.Client {
	return s.manager.GetClient()
}

// Start runs the webhook server until the context is canceled.
func (s *Server) Start(ctx context.Context) error {
	return s.manager.Start(ctx)
}
