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
	"regexp"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/go_lib/filter"
)

const (
	clusterDomainCoreCMSnapName  = "cm"
	clusterDomainDNSPodsSnapName = "pod"
)

var (
	clusterDomainFromConfigMapRegexp = regexp.MustCompile(`\s+kubernetes\s+(\S+?)\.?\s+in-addr.arpa\s+ip6.arpa\s+\{`)
	clusterDomainFromPodRegexp       = regexp.MustCompile(`(^|\s+)--domain=(\S+?)\.?(\s+|$)`)
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       clusterDomainCoreCMSnapName,
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"coredns", "d8-kube-dns"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			FilterFunc: applyClusterDomainFromConfigMapFilter,
		},
		{
			Name:       clusterDomainDNSPodsSnapName,
			ApiVersion: "v1",
			Kind:       "Pod",
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"k8s-app": "kube-dns",
				},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			FilterFunc: applyClusterDomainFromDNSPodFilter,
		},
		{
			Name:       "clusterConfiguration",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-cluster-configuration"},
			},
			ExecuteHookOnSynchronization: pointer.Bool(false),
			ExecuteHookOnEvents:          pointer.Bool(false),
			FilterFunc:                   applyClusterConfigurationYamlFilter,
		},
	},
}, discoveryClusterDomain)

func applyClusterDomainFromConfigMapFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var cm v1core.ConfigMap
	err := sdk.FromUnstructured(obj, &cm)
	if err != nil {
		return "", err
	}

	coreFile, ok := cm.Data["Corefile"]
	if !ok {
		return "", fmt.Errorf("not found core file in secret")
	}

	domainMatches := clusterDomainFromConfigMapRegexp.FindStringSubmatch(coreFile)

	if len(domainMatches) < 2 {
		return "", nil
	}

	return domainMatches[1], nil
}

func applyClusterDomainFromDNSPodFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return filter.GetArgFromUnstructuredPodWithRegexp(obj, clusterDomainFromPodRegexp, 1, "")
}

func discoveryClusterDomain(input *go_hook.HookInput) error {
	// We have a hook for handling clusterConfiguration.
	// During the operation of this hook, there is a blocking check for filling in the clusterDomain field.
	// So now we just need to check that the `clusterConfiguration` is present in the secrets.
	// And if this is the case, then the `global.discovery.clusterDomain` will be filled.
	currentConfig, ok := input.Snapshots["clusterConfiguration"]
	if ok && len(currentConfig) > 0 {
		return nil
	}

	const clusterDomainPath = "global.discovery.clusterDomain"
	if input.Values.Exists(clusterDomainPath) {
		return nil
	}

	clusterDomain := "cluster.local"

	clusterDomainCoreCMSnap := input.Snapshots[clusterDomainCoreCMSnapName]
	clusterDomainDNSPodsSnap := input.Snapshots[clusterDomainDNSPodsSnapName]

	if len(clusterDomainCoreCMSnap) > 0 {
		domain := clusterDomainCoreCMSnap[0].(string)
		if domain != "" {
			clusterDomain = domain
		}
	} else if len(clusterDomainDNSPodsSnap) > 0 {
		for _, domainRaw := range clusterDomainDNSPodsSnap {
			domain := domainRaw.(string)
			if domain != "" {
				clusterDomain = domain
				break
			}
		}
	}

	input.Values.Set(clusterDomainPath, clusterDomain)

	return nil
}
