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

package hooks

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "dns_cluster_ip",
			ApiVersion: "v1",
			Kind:       "Service",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			LabelSelector: &v1.LabelSelector{
				MatchExpressions: []v1.LabelSelectorRequirement{
					{
						Key:      "k8s-app",
						Operator: v1.LabelSelectorOpIn,
						Values:   []string{"kube-dns", "coredns"},
					},
				},
			},
			FilterFunc: applyDNSServiceIPFilter,
		},
	},
}, discoveryDNSAddress)

type ServiceAddr struct {
	Name      string
	ClusterIP string
}

func applyDNSServiceIPFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var service v1core.Service
	err := sdk.FromUnstructured(obj, &service)
	if err != nil {
		return "", err
	}

	return ServiceAddr{service.Name, service.Spec.ClusterIP}, nil
}

// Providers are deploying node-local-dns to cluster in different ways
// E.g.: services in 'kube-system' namespace for Deckhouse and GKE

//      | .metadata.Name     |  .spec.Type   | .metadata.labels
//  ----+--------------------+---------------+-------------------
//  GKE | kube-dns           |  ClusterIP    |  k8s-app=kube-dns
//      | kube-dns-upstream  |  ClusterIP    |  k8s-app=kube-dns
//  ----+--------------------+---------------+-------------------
//  D8  | kube-dns           |  ExternalName |  k8s-app=kube-dns
//      | d8-kube-dns        |  ClusterIP    |  k8s-app=kube-dns

// dnsAdress will be taken only from ClusterIP service in that order:
// - from 'kube-dns' service
// - from any other service selected by label 'k8s-app=kube-dns'
//   if there are no more ClusterIP services with same label in namespace

func discoveryDNSAddress(_ context.Context, input *go_hook.HookInput) error {
	services, err := sdkobjectpatch.UnmarshalToStruct[ServiceAddr](input.Snapshots, "dns_cluster_ip")
	if err != nil {
		return fmt.Errorf("failed to unmarshal dns_cluster_ip snapshot: %w", err)
	}

	dnsAddress := ""

	for _, s := range services {
		if s.ClusterIP == "None" || s.ClusterIP == "" {
			continue
		}

		if s.Name == "kube-dns" {
			dnsAddress = s.ClusterIP
			break
		}

		if dnsAddress != "" && dnsAddress != s.ClusterIP {
			return fmt.Errorf("ERROR: can't select a single service by 'k8s-app: kube-dns' label, found %s %s", dnsAddress, s.ClusterIP)
		}

		dnsAddress = s.ClusterIP
	}

	if dnsAddress == "" {
		return fmt.Errorf("DNS addresses not found")
	}

	input.Values.Set("global.discovery.clusterDNSAddress", dnsAddress)

	return nil
}
