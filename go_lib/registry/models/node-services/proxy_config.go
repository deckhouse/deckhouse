/*
Copyright 2025 Flant JSC

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

package nodeservices

import (
	validation "github.com/go-ozzo/ozzo-validation/v4"

	"github.com/deckhouse/deckhouse/go_lib/registry/helpers"
)

var (
	_ validation.Validatable = ProxyConfig{}
)

type ProxyConfig struct {
	HTTP    string `json:"http,omitempty"`
	HTTPS   string `json:"https,omitempty"`
	NoProxy string `json:"no_proxy,omitempty"`
}

// Validate constrains the proxy settings to the shape the static pod manifest
// can carry. The values become HTTP_PROXY / HTTPS_PROXY / NO_PROXY environment
// variables in a manifest that kubelet runs as root on a control plane node, and
// this configuration arrives from a Kubernetes secret, so an unconstrained value
// is untrusted input for a privileged component.
func (proxyConfig ProxyConfig) Validate() error {
	return validation.ValidateStruct(&proxyConfig,
		validation.Field(&proxyConfig.HTTP, validation.By(helpers.ProxyURL)),
		validation.Field(&proxyConfig.HTTPS, validation.By(helpers.ProxyURL)),
		validation.Field(&proxyConfig.NoProxy, validation.By(helpers.NoProxyList)),
	)
}
