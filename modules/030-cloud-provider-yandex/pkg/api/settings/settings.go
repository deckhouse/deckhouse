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

// Package settings contains the ModuleConfig root type for the cloud-provider-yandex module.
package settings

// Describes the configuration of the cloud-provider-yandex module.
//
// Run the following command to change the configuration in a running cluster:
//
// ```shell
// d8 k edit moduleconfig cloud-provider-yandex
// ```
//
// +deckhouse:ru:description:value="Описывает конфигурацию модуля cloud-provider-yandex."
// +deckhouse:ru:description:value=
// +deckhouse:ru:description:value="Выполните следующую команду, чтобы изменить конфигурацию в работающем кластере:"
// +deckhouse:ru:description:value=
// +deckhouse:ru:description:value="```shell"
// +deckhouse:ru:description:value="d8 k edit moduleconfig cloud-provider-yandex"
// +deckhouse:ru:description:value="```"
// +deckhouse:XDocSearch=ModuleConfig
// +deckhouse:XConfigVersion=2
// +deckhouse:DisableAdditionalProperties=true
type ModuleConfigSettings struct {
	Provider Provider `json:"provider"`
	Nodes    Nodes    `json:"nodes"`
	// +optional
	Storage Storage `json:"storage,omitempty"`
	// +optional
	CCM CCM `json:"ccm"`
}

// +deckhouse:DisableAdditionalProperties=true
type Provider struct {
	Parameters ProviderParameters `json:"parameters"`
}

// +deckhouse:DisableAdditionalProperties=true
type Nodes struct {
	// +kubebuilder:default=false
	// +optional
	Disabled   bool            `json:"disabled,omitempty"`
	Parameters NodesParameters `json:"parameters"`
}

// +deckhouse:DisableAdditionalProperties=true
type Storage struct {
	// +kubebuilder:default=false
	// +optional
	Disabled   bool              `json:"disabled,omitempty"`
	Parameters StorageParameters `json:"parameters"`
}

// +deckhouse:DisableAdditionalProperties=true
type CCM struct {
	// +kubebuilder:default=false
	// +optional
	Disabled   bool          `json:"disabled,omitempty"`
	Parameters CCMParameters `json:"parameters"`
}

// Contains settings to connect to the Yandex Cloud API.
// +deckhouse:ru:description:value="Содержит настройки для подключения к API Yandex Cloud."
// +deckhouse:DisableAdditionalProperties=true
type ProviderParameters struct {
	// The cloud ID.
	// +deckhouse:ru:description:value="Идентификатор облака."
	CloudID string `json:"cloudID"`
	// ID of the directory.
	// +deckhouse:ru:description:value="Идентификатор директории."
	FolderID string `json:"folderID"`
}

// +deckhouse:DisableAdditionalProperties=true
type NodesParameters struct {
	// A public key for accessing nodes.
	// +deckhouse:ru:description:value="Публичный ключ для доступа на узлы."
	// +deckhouse:XRules=sshPublicKey
	SSHPublicKey string `json:"sshPublicKey"`
	// The way resources are located in the cloud.
	//
	// [Read more](https://deckhouse.io/modules/cloud-provider-yandex/layouts.html) about possible provider layouts.
	// +deckhouse:ru:description:value="Название схемы размещения."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="[Подробнее](https://deckhouse.ru/modules/cloud-provider-yandex/layouts.html) о возможных схемах размещения провайдера."
	// +kubebuilder:validation:Enum=Standard;WithoutNAT;WithNATInstance
	Layout string `json:"layout"`
	// This subnet will be split into **three** equal parts.
	//
	// They will serve as a basis for subnets in three Yandex Cloud zones.
	// +deckhouse:ru:description:value="Данная подсеть будет разделена на **четыре** равные части."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="Они будут использоваться в качестве основы для подсетей в четырёх зонах Yandex Cloud."
	NodeNetworkCIDR string `json:"nodeNetworkCIDR"`
	// Settings for the [`WithNATInstance`](https://deckhouse.io/modules/cloud-provider-yandex/layouts.html#withnatinstance) layout.
	// +deckhouse:ru:description:value="Настройки для схемы размещения [`WithNATInstance`](https://deckhouse.ru/modules/cloud-provider-yandex/layouts.html#withnatinstance)."
	// +optional
	WithNATInstance NATInstanceParameters `json:"withNATInstance,omitempty"`
	// The ID of the existing VPC Network.
	// +deckhouse:ru:description:value="ID существующей VPC Network."
	// +optional
	ExistingNetworkID string `json:"existingNetworkID,omitempty"`
	// One or more pre-existing subnets mapped to respective zone.
	//
	// **Warning.** When using `cni-simple-bridge`, DKP creates a route table that must be manually associated with the specified subnets. Only one route table can be associated with a subnet, so multiple clusters using `cni-simple-bridge` cannot be deployed in the same subnets. Starting with DKP 1.76, new clusters in Yandex Cloud use `cni-cilium` in `VXLAN` mode by default. In this mode, pod traffic routing does not depend on Yandex Cloud route tables.
	// +deckhouse:ru:description:value="Одна или несколько ранее существовавших подсетей, сопоставленных с соответствующей зоной."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="**Внимание.** При использовании `cni-simple-bridge` DKP создаст таблицу маршрутизации, которую необходимо вручную привязать к указанным подсетям. К одной подсети можно привязать только одну таблицу маршрутизации, поэтому невозможно развернуть несколько кластеров с `cni-simple-bridge` в одних и тех же подсетях. Начиная с DKP 1.76 для новых кластеров в Yandex Cloud по умолчанию используется `cni-cilium` в режиме `VXLAN`. В этом режиме маршрутизация трафика подов не зависит от таблиц маршрутизации Yandex Cloud."
	// +optional
	ExistingZoneToSubnetIDMap map[string]string `json:"existingZoneToSubnetIDMap,omitempty"`
	// A list of DHCP parameters to use for all subnets.
	//
	// Note that setting dhcpOptions may lead to [problems](https://deckhouse.io/modules/cloud-provider-yandex/faq.html#dhcpoptions-related-problems-and-ways-to-address-them).
	// +deckhouse:ru:description:value="Список DHCP-опций, которые будут установлены на все подсети."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="[Возможные проблемы](https://deckhouse.ru/modules/cloud-provider-yandex/faq.html#проблемы-dhcpoptions-и-пути-их-решения) при использовании."
	// +optional
	DHCPOptions DHCPOptions `json:"dhcpOptions,omitempty"`
	// External IP addresses per node group name.
	//
	// The key is the node group name (e.g., `system`, `master`, `worker`), and the value is the list of external IP addresses for nodes in that group, listed in the order of the zones where nodes will be created.
	//
	// The following values can be specified in the list:
	// - IP address from an additional external network for the corresponding zone (parameter `externalSubnetIDs`);
	// - [reserved public IP address](faq.html#how-to-reserve-a-public-ip-address), if the list of additional external networks is not defined (parameter `externalSubnetIDs`);
	// - `Auto`, to order a public IP address in the corresponding zone.
	//
	// +deckhouse:ru:description:value="Внешние IP-адреса для каждой группы узлов."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="Ключ — имя группы узлов (например, `system`, `master`, `worker`), значение — список внешних IP-адресов для узлов этой группы, перечисленных в порядке зон, в которых будут создаваться узлы."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="В списке можно указывать следующие значения:"
	// +deckhouse:ru:description:value="- IP-адрес из дополнительной внешней сети для соответствующей зоны (параметр `externalSubnetIDs`);"
	// +deckhouse:ru:description:value="- [зарезервированный публичный IP-адрес](faq.html#как-зарезервировать-публичный-ip-адрес), если список дополнительных внешних сетей не определён (параметр `externalSubnetIDs`);"
	// +deckhouse:ru:description:value="- `Auto` — для заказа публичного IP-адреса в соответствующей зоне."
	// +deckhouse:validation:AdditionalProperties:items:Pattern=`^([0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3})|(Auto)$`
	// +optional
	ExternalIPAddresses map[string][]string `json:"externalIPAddresses,omitempty"`
	// IDs of additional external networks per node group name.
	//
	// The key is the node group name (e.g., `system`, `master`, `worker`), and the value is the list of external subnet IDs for nodes in that group, listed in the order of the zones where nodes will be created.
	// +deckhouse:ru:description:value="ID дополнительных внешних сетей для каждой группы узлов."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="Ключ — имя группы узлов (например, `system`, `master`, `worker`), значение — список ID внешних подсетей для узлов этой группы, перечисленных в порядке зон, в которых будут создаваться узлы."
	// +optional
	ExternalSubnetIDs map[string][]string `json:"externalSubnetIDs,omitempty"`
	// Labels to attach to resources created in the Yandex Cloud.
	//
	// Note that you have to re-create all the machines to add new labels if labels were modified in the running cluster.
	// +deckhouse:ru:description:value="Лейблы, проставляемые на ресурсы, создаваемые в Yandex Cloud."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="Если поменять лейблы в рабочем кластере, после применения изменений необходимо пересоздать все машины."
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
	// The globally restricted set of zones that this cloud provider works with.
	// +deckhouse:ru:description:value="Глобальное ограничение набора зон, с которыми работает данный cloud-провайдер."
	// +kubebuilder:validation:UniqueItems=true
	// +kubebuilder:validation:items:Enum=ru-central1-a;ru-central1-b;ru-central1-d;ru-central1-e
	// +optional
	Zones []string `json:"zones,omitempty"`
}

// DHCPOptions defines DHCP parameters to use for all subnets.
// +deckhouse:DisableAdditionalProperties=true
type DHCPOptions struct {
	// The name of the search domain.
	// +deckhouse:ru:description:value="Search-домен."
	// +optional
	DomainName string `json:"domainName,omitempty"`
	// A list of recursive DNS addresses.
	// +deckhouse:ru:description:value="Список адресов рекурсивных DNS."
	// +deckhouse:validation:items:Pattern=`^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$`
	// +optional
	DomainNameServers []string `json:"domainNameServers,omitempty"`
}

// +deckhouse:DisableAdditionalProperties=true
type NATInstanceParameters struct {
	// If specified, an additional network interface will be added to the node (the latter will use it as a default route).
	// +deckhouse:ru:description:value="Подключаемый к узлу дополнительный сетевой интерфейс, в который будет идти маршрут по умолчанию."
	// +optional
	ExternalSubnetID string `json:"externalSubnetID,omitempty"`
	// ID of a subnet for the internal interface.
	// +deckhouse:ru:description:value="ID подсети для внутреннего интерфейса."
	// +optional
	InternalSubnetID string `json:"internalSubnetID,omitempty"`
	// CIDR of an automatically created subnet for the internal interface. Overrides `internalSubnetID` parameter.
	// +deckhouse:ru:description:value="CIDR автоматически создаваемой подсети для внутреннего интерфейса. Если указан вместе с `internalSubnetID`, `internalSubnetCIDR` имеет приоритет."
	// +optional
	InternalSubnetCIDR string `json:"internalSubnetCIDR,omitempty"`
	// A [reserved external IP address](https://deckhouse.io/modules/cloud-provider-yandex/faq.html#how-to-reserve-a-public-ip-address) (or `externalSubnetID` address if specified).
	// +deckhouse:ru:description:value="Внешний [зарезервированный IP-адрес](https://deckhouse.ru/modules/cloud-provider-yandex/faq.html#как-зарезервировать-публичный-ip-адрес) или адрес из `externalSubnetID` при указании опции."
	// +kubebuilder:validation:Pattern=`^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$`
	// +optional
	NATInstanceExternalAddress string `json:"natInstanceExternalAddress,omitempty"`
	// Consider using automatically generated address instead.
	// +deckhouse:ru:description:value="Лучше не использовать эту опцию, а использовать автоматически назначаемые адреса."
	// +kubebuilder:validation:Pattern=`^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$`
	// +optional
	NATInstanceInternalAddress string `json:"natInstanceInternalAddress,omitempty"`
	// Computing resources that are allocated to the NAT instance. If not specified, the default values will be used.
	//
	// > **Warning.** If these parameters are changed, `terraform-auto-converger` will automatically restart NAT-instance if [autoConvergerEnabled](../terraform-manager/configuration.html#parameters-autoconvergerenabled) is set to `true`. This may result in a temporary interruption of network traffic in the cluster.
	// +deckhouse:ru:description:value="Вычислительные ресурсы, выделяемые для NAT-инстанса. Если параметр не указан, будут использоваться значения по умолчанию."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="> **Внимание.** При изменении этих параметров, `terraform-auto-converger` перезапустит машину NAT-инстанса автоматически, если включена настройка [autoConvergerEnabled](../terraform-manager/configuration.html#parameters-autoconvergerenabled). Это может привести к временному прерыванию трафика в кластере."
	// +optional
	NATInstanceResources NATInstanceResources `json:"natInstanceResources,omitempty"`
}

// +deckhouse:DisableAdditionalProperties=true
type NATInstanceResources struct {
	// Amount of CPU cores to provision on the NAT instance.
	// +deckhouse:ru:description:value="Количество ядер у создаваемого NAT-инстанса."
	// +kubebuilder:default=2
	// +optional
	Cores int `json:"cores,omitempty"`
	// Amount of primary memory in MB provision on the NAT instance.
	// +deckhouse:ru:description:value="Количество оперативной памяти (в мегабайтах) у создаваемого NAT-инстанса."
	// +kubebuilder:default=2048
	// +optional
	Memory int `json:"memory,omitempty"`
	// Processor platform type on the NAT instance.
	// +deckhouse:ru:description:value="Тип платформы процессора у создаваемого NAT-инстанса."
	// +kubebuilder:default=standard-v2
	// +optional
	Platform string `json:"platform,omitempty"`
}

// +deckhouse:DisableAdditionalProperties=true
type StorageParameters struct {
	// List of storage classes to exclude from use in the cluster.
	// +deckhouse:ru:description:value="Список классов хранения, исключаемых из использования в кластере."
	// +optional
	ExcludedStorageClasses []string `json:"excludedStorageClasses,omitempty"`
}

// +deckhouse:DisableAdditionalProperties=true
type CCMParameters struct {
	// Additional external network IDs to be recognized as external networks by the CCM.
	// +deckhouse:ru:description:value="Дополнительные ID внешних сетей, которые будут распознаваться CCM как внешние сети."
	// +optional
	AdditionalExternalNetworkIDs []string `json:"additionalExternalNetworkIDs,omitempty"`
}
