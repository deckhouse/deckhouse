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

package hooks

import (
	"os"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	v1 "k8s.io/api/core/v1"
	discv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

// We will create the EndpointSlice manually, because Deckhouse only goes to the Ready state after the 'first converge' of modules.
// But Deckhouse itself has ValidationWebhooks that should be executed even when pod is not ready.
// Endpoints created via service do not go to ready state in this case and we cannot use validation.

const (
	d8Namespace = "d8-system"
	d8Name      = "deckhouse"
)

// should run before all hooks
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 1},
}, generateDeckhouseEndpoints)

func generateDeckhouseEndpoints(input *go_hook.HookInput) error {
	ep := &v1.Endpoints{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Endpoints",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      d8Name,
			Namespace: d8Namespace,
			Labels: map[string]string{
				"app":                        d8Name,
				"module":                     d8Name,
				"heritage":                   d8Name,
				"kubernetes.io/service-name": d8Name,
			},
		},
		Subsets: []v1.EndpointSubset{
			{
				Addresses: []v1.EndpointAddress{
					{
						IP:       os.Getenv("ADDON_OPERATOR_LISTEN_ADDRESS"),
						Hostname: os.Getenv("DECKHOUSE_NODE_NAME"),
						NodeName: pointer.String(os.Getenv("DECKHOUSE_NODE_NAME")),
						TargetRef: &v1.ObjectReference{
							Kind:       "Pod",
							Namespace:  d8Namespace,
							Name:       os.Getenv("DECKHOUSE_POD"),
							APIVersion: "v1",
						},
					},
				},
				Ports: []v1.EndpointPort{
					{
						Name:     "self",
						Port:     9650,
						Protocol: v1.ProtocolTCP,
					},
					{
						Name:     "webhook",
						Port:     9651,
						Protocol: v1.ProtocolTCP,
					},
				},
			},
		},
	}

	es := &discv1.EndpointSlice{
		TypeMeta: metav1.TypeMeta{
			Kind:       "EndpointSlice",
			APIVersion: "discovery.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      d8Name,
			Namespace: d8Namespace,
			Labels: map[string]string{
				"app":                        d8Name,
				"module":                     d8Name,
				"heritage":                   d8Name,
				"kubernetes.io/service-name": d8Name,
			},
		},
		AddressType: "IPv4",
		Endpoints: []discv1.Endpoint{
			{
				Addresses: []string{os.Getenv("ADDON_OPERATOR_LISTEN_ADDRESS")},
				Conditions: discv1.EndpointConditions{
					Ready:       pointer.Bool(true),
					Serving:     pointer.Bool(true),
					Terminating: pointer.Bool(false),
				},
				Hostname: pointer.String(os.Getenv("DECKHOUSE_NODE_NAME")),
				TargetRef: &v1.ObjectReference{
					Kind:      "Pod",
					Namespace: d8Namespace,
					Name:      os.Getenv("DECKHOUSE_POD"),
				},
				NodeName: pointer.String(os.Getenv("DECKHOUSE_NODE_NAME")),
				Zone:     nil,
				Hints:    nil,
			},
		},
		Ports: []discv1.EndpointPort{
			{
				Name: pointer.String("self"),
				Port: pointer.Int32(9650),
			},
			{
				Name: pointer.String("webhook"),
				Port: pointer.Int32(9651),
			},
		},
	}

	input.PatchCollector.Create(ep, object_patch.UpdateIfExists())
	input.PatchCollector.Create(es, object_patch.UpdateIfExists())

	return nil
}
