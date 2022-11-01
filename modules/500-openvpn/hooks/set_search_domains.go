/*
Copyright 2021 Flant JSC

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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
}, setSearchDomain)

const (
	clusterDomainGlobalPath          = "global.discovery.clusterDomain"
	clusterDomainsValuesPath         = "openvpn.pushToClientSearchDomains"
	clusterDomainsInternalValuesPath = "openvpn.internal.pushToClientSearchDomains"
)

func setSearchDomain(input *go_hook.HookInput) error {
	userDefinedDomains, ok := input.ConfigValues.GetOk(clusterDomainsValuesPath)
	if ok {
		domains := make([]string, 0)
		for _, domain := range userDefinedDomains.Array() {
			domains = append(domains, domain.String())
		}
		input.Values.Set(clusterDomainsInternalValuesPath, domains)
		return nil
	}

	// Fallback to global discovery.
	clusterDomain := input.Values.Get(clusterDomainGlobalPath).String()
	input.Values.Set(clusterDomainsInternalValuesPath, []string{clusterDomain})

	return nil
}
