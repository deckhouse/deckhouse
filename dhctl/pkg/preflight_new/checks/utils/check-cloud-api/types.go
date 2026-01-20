// Copyright 2024 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package checkcloudapi

import "net/url"

type OpenStackProvider struct {
	AuthURL    string `json:"authURL,omitempty" yaml:"authURL,omitempty"`
	CACert     string `json:"caCert,omitempty" yaml:"caCert,omitempty"`
	DomainName string `json:"domainName,omitempty" yaml:"domainName,omitempty"`
	TenantName string `json:"tenantName,omitempty" yaml:"tenantName,omitempty"`
	TenantID   string `json:"tenantID,omitempty" yaml:"tenantID,omitempty"`
	Username   string `json:"username,omitempty" yaml:"username,omitempty"`
	Password   string `json:"password,omitempty" yaml:"password,omitempty"`
	Region     string `json:"region,omitempty" yaml:"region,omitempty"`
}

type VSphereProvider struct {
	Server   string `json:"server"`
	Username string `json:"username"`
	Password string `json:"password"`
	Insecure bool   `json:"insecure"`
}

type CloudApiConfig struct {
	URL      *url.URL
	Insecure bool
	CACert   string
}
