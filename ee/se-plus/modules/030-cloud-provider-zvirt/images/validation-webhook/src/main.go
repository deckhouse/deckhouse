/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"

	zicv1 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/api/instanceclass/v1"
	zmeta "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/meta"
	zval "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/validation"
	cpvaladmission "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/admission"
	cpwebhook "github.com/deckhouse/deckhouse/go_lib/cloud-provider/webhook"

	"cloud-provider-zvirt-validation-webhook/webhooks"
)

var (
	nodeGroupGVK = schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1", Kind: "NodeGroup"}
	// ModuleConfig is served as v1alpha1 by the deckhouse-controller, whatever the settings
	// version inside it.
	moduleConfigGVK = schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "ModuleConfig"}
)

func main() {
	serverConfig := cpwebhook.DefaultServerConfig()
	logConfig := cpwebhook.DefaultLogConfig()

	rootCmd := &cobra.Command{
		Use:   "validation-webhook",
		Short: "Admission webhook for cloud-provider-zvirt",
		RunE: func(_ *cobra.Command, _ []string) error {
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
				nodeGroupGVK,
				moduleConfigGVK,
				zicv1.GroupVersionKind,
			))

			cfg := ctrl.GetConfigOrDie()

			server, err := cpwebhook.NewServer(cfg, scheme, serverConfig)
			if err != nil {
				setupLog.Error(err, "failed to initialize webhook server")
				return fmt.Errorf("init webhook server: %w", err)
			}

			factory := zval.NewAdmissionStateBuilderFactory(
				server.Client(),
				cpvaladmission.StateBuilderConfig{
					ModuleName:       zmeta.ModuleName,
					NamespaceName:    zmeta.Namespace,
					InstanceClassGVK: zicv1.GroupVersionKind,
				},
			)

			registrars := []cpwebhook.Registrar{
				webhooks.NewCredentialSecretValidator(factory, &corev1.Secret{}),
				webhooks.NewModuleConfigValidator(factory, newWebhookObject(moduleConfigGVK)),
				webhooks.NewNodeGroupValidator(factory, newWebhookObject(nodeGroupGVK)),
				webhooks.NewZvirtInstanceClassValidator(factory, newWebhookObject(zicv1.GroupVersionKind)),
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
		},
	}

	cpwebhook.InitServerFlags(rootCmd.Flags(), &serverConfig)
	cpwebhook.InitLogFlags(rootCmd.Flags(), &logConfig)

	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newWebhookObject(gvk schema.GroupVersionKind) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)

	return obj
}
