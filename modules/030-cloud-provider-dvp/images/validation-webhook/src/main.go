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

package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kingpin"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"

	cpvaladmission "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/admission"
	cpwebhook "github.com/deckhouse/deckhouse/go_lib/cloud-provider/webhook"
	dvpmeta "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/validation/meta"

	"cloud-provider-dvp-validation-webhook/webhooks"
)

var (
	instanceClassGVK = schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: dvpmeta.InstanceClassKind}
	moduleConfigGVK  = schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "ModuleConfig"}
	nodeGroupGVK     = schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1", Kind: "NodeGroup"}
)

func main() {
	kpApp := kingpin.New("dvp validation webhook", "Admission webhook for cloud-provider-dvp")
	kpApp.HelpFlag.Short('h')

	serverConfig := cpwebhook.DefaultServerConfig()
	logConfig := cpwebhook.DefaultLogConfig()
	cpwebhook.InitFlags(kpApp, &serverConfig)
	cpwebhook.InitLogFlags(kpApp, &logConfig)

	kpApp.Action(func(_ *kingpin.ParseContext) error {
		if err := cpwebhook.SetupLogger(logConfig); err != nil {
			return fmt.Errorf("setup logger: %w", err)
		}

		setupLog := ctrl.Log.WithName("setup")
		setupLog.Info(
			"starting validation webhook",
			"webhookPort", serverConfig.WebhookPort,
			"webhookCertDir", serverConfig.WebhookCertDir,
			"metricsBindAddress", serverConfig.MetricsBindAddress,
			"healthProbeBindAddress", serverConfig.HealthProbeBindAddress,
		)

		scheme := clientgoscheme.Scheme
		utilruntime.Must(clientgoscheme.AddToScheme(scheme))
		utilruntime.Must(cpwebhook.RegisterUnstructuredGVKs(
			scheme,
			moduleConfigGVK,
			nodeGroupGVK,
			instanceClassGVK,
		))

		cfg := ctrl.GetConfigOrDie()

		server, err := cpwebhook.NewServer(cfg, scheme, serverConfig)
		if err != nil {
			setupLog.Error(err, "failed to initialize webhook server")
			return fmt.Errorf("init webhook server: %w", err)
		}

		builder := cpvaladmission.NewStateBuilder(
			server.Client(),
			cpvaladmission.StateBuilderConfig{
				ModuleName:        dvpmeta.ModuleName,
				NamespaceName:     dvpmeta.Namespace,
				InstanceClassKind: dvpmeta.InstanceClassKind,
			},
		)

		registrars := []cpwebhook.Registrar{
			webhooks.NewCredentialSecretValidator(builder, &corev1.Secret{}),
			webhooks.NewModuleConfigValidator(builder, newWebhookObject(moduleConfigGVK)),
			webhooks.NewNodeGroupValidator(builder, newWebhookObject(nodeGroupGVK)),
			webhooks.NewDVPInstanceClassValidator(builder, newWebhookObject(instanceClassGVK)),
		}

		for _, registrar := range registrars {
			if err := server.Register(registrar); err != nil {
				setupLog.Error(err, "failed to register validation webhook")
				return fmt.Errorf("register validation webhook: %w", err)
			}
		}

		setupLog.Info("validation webhook server is starting")

		if err := server.Start(ctrl.SetupSignalHandler()); err != nil {
			setupLog.Error(err, "validation webhook server stopped with error")
			return fmt.Errorf("start webhook server: %w", err)
		}

		setupLog.Info("validation webhook server stopped")

		return nil
	})

	if _, err := kpApp.Parse(os.Args[1:]); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func newWebhookObject(gvk schema.GroupVersionKind) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	return obj
}
