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
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
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
}, dependency.WithExternalDependencies(generateDeckhouseEndpoints))

func generateDeckhouseEndpoints(input *go_hook.HookInput, dc dependency.Container) error {
	// hostname := os.Getenv("HOSTNAME")
	// At this moment we don't use Hostname because of 2 reasons:
	// 1. According to the endpoint controller, it should be set only when:
	//		len(pod.Spec.Hostname) > 0 && pod.Spec.Subdomain == svc.Name && svc.Namespace == pod.Namespace
	//    https://github.com/kubernetes/kubernetes/blob/v1.27.5/pkg/controller/util/endpoint/controller_utils.go#L116
	//
	// 2. Deckhouse is a singleton now. Probably we will need it, when we will make a HA mode
	// 		Pay attention!! That hostname could be like 'ip-10-0-3-207.eu-central-1.compute.internal' on the EKS installations for example,
	// 		so it wouldn't validate via RFC1123 DNS Subdomain.
	//		We have to lowercase the value and cut it until the first dot or something like that

	nodeName := os.Getenv("DECKHOUSE_NODE_NAME")
	podName := os.Getenv("DECKHOUSE_POD")
	address := os.Getenv("ADDON_OPERATOR_LISTEN_ADDRESS")

	ep := &v1.Endpoints{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Endpoints",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      d8Name,
			Namespace: d8Namespace,
			Annotations: map[string]string{
				"created-by": podName,
			},
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
						IP:       address,
						NodeName: ptr.To(nodeName),
						TargetRef: &v1.ObjectReference{
							Kind:       "Pod",
							Namespace:  d8Namespace,
							Name:       podName,
							APIVersion: "v1",
						},
					},
				},
				Ports: []v1.EndpointPort{
					{
						Name:     "self",
						Port:     4222,
						Protocol: v1.ProtocolTCP,
					},
					{
						Name:     "webhook",
						Port:     4223,
						Protocol: v1.ProtocolTCP,
					},
					{
						Name:     "debug-server",
						Port:     9652,
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
			Annotations: map[string]string{
				"created-by": podName,
			},
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
				Addresses: []string{address},
				Conditions: discv1.EndpointConditions{
					Ready:       ptr.To(true),
					Serving:     ptr.To(true),
					Terminating: ptr.To(false),
				},
				TargetRef: &v1.ObjectReference{
					Kind:      "Pod",
					Namespace: d8Namespace,
					Name:      podName,
				},
				NodeName: ptr.To(nodeName),
				Zone:     nil,
				Hints:    nil,
			},
		},
		Ports: []discv1.EndpointPort{
			{
				Name: ptr.To("self"),
				Port: ptr.To(int32(4222)),
			},
			{
				Name: ptr.To("webhook"),
				Port: ptr.To(int32(4223)),
			},
			{
				Name: ptr.To("debug-server"),
				Port: ptr.To(int32(9652)),
			},
		},
	}

	input.PatchCollector.Create(ep, object_patch.UpdateIfExists())
	input.PatchCollector.Create(es, object_patch.UpdateIfExists())

	return nil
}
