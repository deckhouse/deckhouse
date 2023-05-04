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
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	dnsAddressServiceSnapName = "kube_dns_cluster_ip"
	dnsAddressByD8AppSnapName = "cluster_dns_address_by_d8s_app"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       dnsAddressServiceSnapName,
			ApiVersion: "v1",
			Kind:       "Service",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"kube-dns"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			FilterFunc: applyDNSServiceIPFilter,
		},

		{
			Name:       dnsAddressByD8AppSnapName,
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

func discoveryDNSAddress(input *go_hook.HookInput) error {
	dnsAddress := ""

	for _, sRaw := range input.Snapshots["dns_cluster_ip"] {
		s := sRaw.(ServiceAddr)

		if s.Name == "kube-dns" && s.ClusterIP != "None" && s.ClusterIP != "" {
			input.Values.Set("global.discovery.dnsAddress", s.ClusterIP)
			return nil
		}

		if s.ClusterIP == "None" || s.ClusterIP == "" {
			continue
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
