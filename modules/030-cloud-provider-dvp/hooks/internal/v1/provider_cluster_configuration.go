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

package v1

type DvpProviderClusterConfiguration struct {
	APIVersion      *string      `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	Kind            *string      `json:"kind,omitempty" yaml:"kind,omitempty"`
	Provider        *DvpProvider `json:"provider,omitempty" yaml:"provider,omitempty"`
	Layout          *string      `json:"layout,omitempty" yaml:"layout,omitempty"`
	MasterNodeGroup any          `json:"masterNodeGroup,omitempty" yaml:"masterNodeGroup,omitempty"`
	NodeGroups      []any        `json:"nodeGroups,omitempty" yaml:"nodeGroups,omitempty"`
	SSHPublicKey    *string      `json:"sshPublicKey,omitempty" yaml:"sshPublicKey,omitempty"`
	Region          *string      `json:"region,omitempty" yaml:"region,omitempty"`
	Zones           *[]string    `json:"zones,omitempty" yaml:"zones,omitempty"`
}

type DvpProvider struct {
	KubeconfigDataBase64 *string `json:"kubeconfigDataBase64,omitempty" yaml:"kubeconfigDataBase64,omitempty"`
	Namespace            *string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
}

type DvpModuleConfiguration struct {
	Provider *DvpProvider `json:"provider,omitempty" yaml:"provider,omitempty"`
	Zones    *[]string    `json:"zones,omitempty" yaml:"zones,omitempty"`
}
