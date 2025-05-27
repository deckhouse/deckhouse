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
	validation "github.com/go-ozzo/ozzo-validation"
)

type ProxyConfig struct {
	HTTP    string `json:"http,omitempty" yaml:"http,omitempty"`
	HTTPS   string `json:"https,omitempty" yaml:"https,omitempty"`
	NoProxy string `json:"no_proxy,omitempty" yaml:"no_proxy,omitempty"`
}

func (proxyConfig ProxyConfig) Validate() error {
	return validation.ValidateStruct(&proxyConfig,
		validation.Field(&proxyConfig.HTTP, validation.Required),
		validation.Field(&proxyConfig.HTTPS, validation.Required),
		validation.Field(&proxyConfig.NoProxy, validation.Required),
	)
}
