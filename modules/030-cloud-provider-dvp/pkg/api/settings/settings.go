// Copyright 2026 Flant JSC
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

package settings

// Describes the configuration of a cloud cluster in Deckhouse Virtualization Platform (DVP).
//
// Used by the cloud provider if a cluster's control plane is hosted in the DVP cloud.
//
// Run the following command to change the configuration in a running cluster:
//
// ```shell
// d8 k edit moduleconfig cloud-provider-dvp
// ```
//
// > Once you modify the node configuration, run `dhctl converge` for changes to take effect for permanent nodes.
// +deckhouse:ru:description:value="Описывает конфигурацию облачного кластера в Deckhouse Virtualization Platform (DVP)."
// +deckhouse:ru:description:value=
// +deckhouse:ru:description:value="Используется облачным провайдером, если управляющий слой (control plane) кластера размещён в облаке."
// +deckhouse:ru:description:value=
// +deckhouse:ru:description:value="Выполните следующую команду, чтобы изменить конфигурацию в работающем кластере:"
// +deckhouse:ru:description:value=
// +deckhouse:ru:description:value="```shell```"
// +deckhouse:ru:description:value="d8 k edit moduleconfig cloud-provider-dvp"
// +deckhouse:ru:description:value="```"
// +deckhouse:ru:description:value=
// +deckhouse:ru:description:value="> Чтобы изменения вступили в силу, после изменения параметров узлов выполните команду `dhctl converge`."
// +deckhouse:XDocSearch=ModuleConfig
// +deckhouse:XConfigVersion=2
// +deckhouse:DisableAdditionalProperties=true
type ModuleConfigSettings struct {
	Provider Provider `json:"provider"`
	// +optional
	Storage Storage `json:"storage,omitempty"`
	Nodes   Nodes   `json:"nodes"`
	// +optional
	CCM CCM `json:"ccm"`
}

// +deckhouse:DisableAdditionalProperties=true
type Provider struct {
	Parameters ProviderParameters `json:"parameters"`
}

// +deckhouse:DisableAdditionalProperties=true
type Storage struct {
	// +kubebuilder:default=false
	// +optional
	Disabled   bool              `json:"disabled,omitempty"`
	Parameters StorageParameters `json:"parameters"`
}

// +deckhouse:DisableAdditionalProperties=true
type Nodes struct {
	// +kubebuilder:default=false
	// +optional
	Disabled   bool            `json:"disabled,omitempty"`
	Parameters NodesParameters `json:"parameters"`
}

// +deckhouse:DisableAdditionalProperties=true
type CCM struct {
	// +kubebuilder:default=false
	// +optional
	Disabled bool `json:"disabled,omitempty"`
}

// Contains settings to connect to the Deckhouse Kubernetes Platform API.
// +deckhouse:ru:description:value="Содержит настройки для подключения к API Deckhouse Kubernetes Platform."
// +deckhouse:DisableAdditionalProperties=true
type ProviderParameters struct {
	// Namespace in which DKP cluster resources will be created.
	//
	// > If not explicitly specified, the default namespace for kubeconfig will be used.
	// +deckhouse:ru:description:value="Пространство имён, в котором будут созданы ресурсы кластера DKP."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="> Если не указано явно, будет использоваться пространство имён по умолчанию для kubeconfig."
	Namespace string `json:"namespace"`
	// Control rules for network traffic to and from workloads running in the Project resource.
	//
	// * `Isolated`: Applies a restrictive NetworkPolicy that allows only network traffic required for platform system components to function. All other traffic is denied.
	// * `None`: The cluster does not request any network policies. Existing project-level restrictions still apply.
	// +deckhouse:ru:description:value="Правила управления сетевым трафиком к нагрузкам и от нагрузок, запущенных в рамках ресурса Project."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="* `Isolated` — применяется ограничивающая allowlist-политика: разрешён только сетевой трафик, необходимый для работы платформенных компонентов; весь остальной трафик запрещён."
	// +deckhouse:ru:description:value="* `None` — кластер не заказывает сетевые политики. Ограничения, заданные на уровне Project, продолжают действовать."
	// +kubebuilder:validation:Enum=Isolated;None
	// +optional
	NetworkPolicy string `json:"networkPolicy,omitempty"`
}

// +deckhouse:DisableAdditionalProperties=true
type StorageParameters struct {
	// +optional
	ExcludedStorageClasses []string `json:"excludedStorageClasses,omitempty"`
}

// +deckhouse:DisableAdditionalProperties=true
type NodesParameters struct {
	// A public key for accessing nodes.
	// +deckhouse:ru:description:value="Публичный ключ для доступа на узлы."
	// +deckhouse:XRules=sshPublicKey
	SSHPublicKey string `json:"sshPublicKey"`
	// Layout name.
	//
	// [Read more about possible provider layouts](/modules/cloud-provider-dvp/layouts.html).
	// +deckhouse:ru:description:value="Название схемы размещения."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="[Подробнее о возможных схемах размещения провайдера](/modules/cloud-provider-dvp/layouts.html)."
	// +kubebuilder:validation:Enum=Standard
	Layout string `json:"layout"`
	// Region name.
	//
	// To use this setting, the `topology.kubernetes.io/region` label must be set on DVP nodes.
	// [Read more about topological labels](https://kubernetes.io/docs/reference/labels-annotations-taints/#topologykubernetesioregion).
	//
	// > To set the required label for a DVP node, follow the [NodeGroup documentation](https://deckhouse.io/documentation/v1/modules/040-node-manager/cr.html#nodegroup-v1-spec-nodetemplate-labels).
	// +deckhouse:ru:description:value="Название региона."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="Чтобы использовать эту настройку, на узлах DVP должен быть установлен лейбл `topology.kubernetes.io/region`."
	// +deckhouse:ru:description:value="[Подробнее о топологических лейблах](https://kubernetes.io/docs/reference/labels-annotations-taints/#topologykubernetesioregion)"
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="> Чтобы установить требуемый лейбл для узла DVP, следуйте [документации по NodeGroup](https://deckhouse.io/documentation/v1/modules/040-node-manager/cr.html#nodegroup-v1-spec-nodetemplate-labels)."
	// +optional
	Region string `json:"region,omitempty"`
	// A set of zones in which nodes can be created.
	//
	// To use this setting, the `topology.kubernetes.io/zone` label must be set on DVP nodes.
	// [Read more about topological labels.](https://kubernetes.io/docs/reference/labels-annotations-taints/#topologykubernetesioregion)
	//
	// > To set the required label for a DVP node, follow the [NodeGroup documentation](https://deckhouse.io/documentation/v1/modules/040-node-manager/cr.html#nodegroup-v1-spec-nodetemplate-labels).
	// +deckhouse:ru:description:value="Набор зон, в которых могут быть созданы узлы."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="Чтобы использовать эту настройку, на узлах DVP должна быть установлен лейбл `topology.kubernetes.io/zone`."
	// +deckhouse:ru:description:value="[Подробнее о топологических лейблах.](https://kubernetes.io/docs/reference/labels-annotations-taints/#topologykubernetesioregion)"
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="> Чтобы установить требуемый лейбл для узла DVP, обратитесь к [документации по NodeGroup](https://deckhouse.io/documentation/v1/modules/040-node-manager/cr.html#nodegroup-v1-spec-nodetemplate-labels)."
	// +kubebuilder:validation:UniqueItems=true
	// +kubebuilder:validation:items:Type=string
	// +optional
	Zones []string `json:"zones,omitempty"`
	// Static IP addresses to be assigned to the network interfaces of the virtual machines. The number of addresses must match the number of replicas being created — each IP address will be assigned to a specific virtual machine replica.
	// For example, if 3 replicas are specified and the IP addresses provided are: ip1, ip2, and ip3, then ip1 will be assigned to the first replica, ip2 to the second, and ip3 to the third.
	// > These addresses must belong to the address range specified in the virtualization module configuration in the `virtualMachineCIDRs` parameter.
	// +deckhouse:ru:description:value="Статические IP-адреса, назначаемые сетевым интерфейсам виртуальных машин. Количество адресов должно совпадать с количеством создаваемых реплик — каждый IP-адрес будет назначен отдельной реплике."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="> Эти адреса должны принадлежать диапазону адресов, заданному в конфигурации модуля виртуализации в параметре `virtualMachineCIDRs`."
	// +deckhouse:validation:AdditionalProperties:items:Pattern=`^([0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3})|(Auto)$`
	// +optional
	IPAddresses map[string][]string `json:"ipAddresses,omitempty"`
}

type CCMParameters struct{}
