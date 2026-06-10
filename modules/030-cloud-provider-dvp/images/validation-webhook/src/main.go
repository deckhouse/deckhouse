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

	cpwebhook "github.com/deckhouse/deckhouse/go_lib/cloud-provider/webhook"
	cpwebhookstate "github.com/deckhouse/deckhouse/go_lib/cloud-provider/webhook/state"
	dvpval "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/validation"

	"cloud-provider-dvp-validation-webhook/webhooks"
)

var (
	instanceClassGVK = schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: dvpval.InstanceClassKind}
	moduleConfigGVK  = schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "ModuleConfig"}
	nodeGroupGVK     = schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1", Kind: "NodeGroup"}
)

func main() {
	kpApp := kingpin.New("dvp validation webhook", "Admission webhook for cloud-provider-dvp")
	kpApp.HelpFlag.Short('h')

	serverConfig := cpwebhook.DefaultServerConfig()
	cpwebhook.InitFlags(kpApp, &serverConfig)

	kpApp.Action(func(_ *kingpin.ParseContext) error {
		scheme := clientgoscheme.Scheme
		utilruntime.Must(clientgoscheme.AddToScheme(scheme))

		cfg := ctrl.GetConfigOrDie()

		server, err := cpwebhook.NewServer(cfg, scheme, serverConfig)
		if err != nil {
			return fmt.Errorf("init webhook server: %w", err)
		}
		stateBuilderConfig := cpwebhookstate.Config{
			ModuleName:        dvpval.ModuleName,
			NamespaceName:     dvpval.Namespace,
			InstanceClassKind: dvpval.InstanceClassKind,
		}
		builder := cpwebhookstate.NewRuntimeStateBuilder(server.Client(), stateBuilderConfig)

		registrars := []cpwebhook.Registrar{
			webhooks.NewCredentialSecretValidator(builder, &corev1.Secret{}),
			webhooks.NewModuleConfigValidator(builder, newWebhookObject(moduleConfigGVK)),
			webhooks.NewNodeGroupValidator(builder, newWebhookObject(nodeGroupGVK)),
			webhooks.NewDVPInstanceClassValidator(builder, newWebhookObject(instanceClassGVK)),
		}

		for _, registrar := range registrars {
			if err := server.Register(registrar); err != nil {
				return fmt.Errorf("register validation webhook: %w", err)
			}
		}

		if err := server.Start(ctrl.SetupSignalHandler()); err != nil {
			return fmt.Errorf("start webhook server: %w", err)
		}

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
