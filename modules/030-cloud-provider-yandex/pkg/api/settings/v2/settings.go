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
package v2

import (
	"reflect"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

var (
	_ cpapi.ModuleSettingsObject = (*ModuleConfigSettings)(nil)
)

// +deckhouse:XDocSearch=ModuleConfig
// +deckhouse:XConfigVersion=2
// +deckhouse:DisableAdditionalProperties=true
type ModuleConfigSettings struct {
	Provider Provider `json:"provider"`
	Nodes    Nodes    `json:"nodes"`
	// +optional
	Storage Storage `json:"storage,omitempty"`
	// +optional
	CCM CCM `json:"ccm,omitempty"`
}

// Yandex Cloud connection settings.
//
// The provider reads the service account key from the `d8-credentials` Secret.
// +deckhouse:ru:description:value="Настройки подключения к Yandex Cloud."
// +deckhouse:ru:description:value=
// +deckhouse:ru:description:value="Ключ сервисного аккаунта провайдер читает из секрета `d8-credentials`."
// +deckhouse:DisableAdditionalProperties=true
type Provider struct {
	Parameters ProviderParameters `json:"parameters"`
}

// Nodes subsystem settings.
//
// The subsystem describes the infrastructure the cluster nodes are created in, including the layout, the networks, the zones, and the SSH key.
// +deckhouse:ru:description:value="Настройки подсистемы узлов."
// +deckhouse:ru:description:value=
// +deckhouse:ru:description:value="Подсистема описывает инфраструктуру, в которой создаются узлы кластера, включая схему размещения, сети, зоны и публичный SSH-ключ."
// +deckhouse:DisableAdditionalProperties=true
type Nodes struct {
	// Disables the nodes subsystem.
	//
	// When set to `true`, the cluster creates no nodes in Yandex Cloud, and the cloud controller manager does not start either.
	// +deckhouse:ru:description:value="Отключает подсистему узлов."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="При значении `true` кластер не создаёт узлы в Yandex Cloud, а cloud controller manager не запускается."
	// +kubebuilder:default=false
	// +optional
	Disabled   bool            `json:"disabled,omitempty"`
	Parameters NodesParameters `json:"parameters"`
}

// Storage subsystem settings.
//
// The subsystem provides the CSI driver and the StorageClasses for Yandex Cloud disk types.
// +deckhouse:ru:description:value="Настройки подсистемы хранения данных."
// +deckhouse:ru:description:value=
// +deckhouse:ru:description:value="Подсистема отвечает за CSI-драйвер и StorageClass для типов дисков Yandex Cloud."
// +deckhouse:DisableAdditionalProperties=true
type Storage struct {
	// Disables the storage subsystem.
	//
	// When set to `true`, disk provisioning in the cluster is unavailable.
	// +deckhouse:ru:description:value="Отключает подсистему хранения данных."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="При значении `true` заказ дисков в кластере недоступен."
	// +kubebuilder:default=false
	// +optional
	Disabled   bool              `json:"disabled,omitempty"`
	Parameters StorageParameters `json:"parameters"`
}

// Cloud Controller Manager (CCM) subsystem settings.
//
// CCM creates Network Load Balancers and target groups for Service objects of the `LoadBalancer` type, maintains routes to pod subnets in the route table, and keeps node metadata up to date.
// +deckhouse:ru:description:value="Настройки подсистемы Cloud Controller Manager (CCM)."
// +deckhouse:ru:description:value=
// +deckhouse:ru:description:value="CCM создаёт Network Load Balancer и целевые группы для Service с типом `LoadBalancer`, поддерживает маршруты до подсетей подов в таблице маршрутизации и обновляет метаданные узлов."
// +deckhouse:DisableAdditionalProperties=true
type CCM struct {
	// Disables the Cloud Controller Manager.
	//
	// When set to `true`, the cluster does not manage load balancers and routes in Yandex Cloud.
	// +deckhouse:ru:description:value="Отключает Cloud Controller Manager."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="При значении `true` кластер не управляет балансировщиками нагрузки и маршрутами в Yandex Cloud."
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

// Parameters of the nodes subsystem.
// +deckhouse:ru:description:value="Параметры подсистемы узлов."
// +deckhouse:DisableAdditionalProperties=true
type NodesParameters struct {
	// A public key for accessing nodes.
	// +deckhouse:ru:description:value="Публичный ключ для доступа на узлы."
	// +deckhouse:XRules=sshPublicKey
	SSHPublicKey string `json:"sshPublicKey"`
	// The way resources are located in the cloud.
	//
	// The available layouts are described in ["Layouts"](layouts.html).
	// +deckhouse:ru:description:value="Название схемы размещения."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="Доступные схемы размещения описаны в разделе [«Схемы размещения»](layouts.html)."
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
	// **Warning.** With the [`cni-simple-bridge`](/modules/cni-simple-bridge/) module, DP creates a route table that has to be associated with the listed subnets manually. A subnet accepts only one route table, so two clusters using `cni-simple-bridge` cannot share subnets. Starting with DP 1.76, new clusters in Yandex Cloud use the [`cni-cilium`](/modules/cni-cilium/) module in [VXLAN](/modules/cni-cilium/configuration.html#parameters-tunnelmode) mode, where pod traffic routing does not depend on Yandex Cloud route tables.
	// +deckhouse:ru:description:value="Одна или несколько ранее существовавших подсетей, сопоставленных с соответствующей зоной."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="**Внимание.** С модулем [`cni-simple-bridge`](/modules/cni-simple-bridge/) DP создаёт таблицу маршрутизации, которую нужно вручную привязать к перечисленным подсетям. К подсети привязывается только одна таблица маршрутизации, поэтому два кластера с `cni-simple-bridge` не могут использовать одни и те же подсети. Начиная с DP 1.76 новые кластеры в Yandex Cloud используют модуль [`cni-cilium`](/modules/cni-cilium/) в режиме [VXLAN](/modules/cni-cilium/configuration.html#parameters-tunnelmode), в котором маршрутизация трафика подов не зависит от таблиц маршрутизации Yandex Cloud."
	// +optional
	ExistingZoneToSubnetIDMap map[string]string `json:"existingZoneToSubnetIDMap,omitempty"`
	// The ID of the existing route table associated with the subnets.
	//
	// Normally the value is produced when DP creates the infrastructure. Set the parameter explicitly
	// in a cluster where DP does not create the infrastructure, for example in a cluster with static
	// master nodes that adds ephemeral nodes in Yandex Cloud.
	// +deckhouse:ru:description:value="ID существующей таблицы маршрутизации, привязанной к подсетям."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="Обычно значение формируется, когда DP создаёт инфраструктуру. Задайте параметр явно в кластере, где DP инфраструктуру не создаёт, например в кластере со статичными master-узлами, в который добавляются эфемерные узлы в Yandex Cloud."
	// +optional
	ExistingRouteTableID string `json:"existingRouteTableID,omitempty"`
	// A list of DHCP parameters to use for all subnets.
	//
	// Setting the parameter can disrupt name resolution on the nodes. The symptoms and the ways to address them are described in [DHCPOptions-related problems and ways to address them](faq.html#dhcpoptions-related-problems-and-ways-to-address-them).
	// +deckhouse:ru:description:value="Список DHCP-опций, которые будут установлены на все подсети."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="Параметр может нарушить разрешение имён на узлах. Признаки и способы решения описаны в разделе [«Проблемы DHCPOptions и пути их решения»](faq.html#проблемы-dhcpoptions-и-пути-их-решения)."
	// +optional
	DHCPOptions DHCPOptions `json:"dhcpOptions,omitempty"`
	// External IP addresses per node group name.
	//
	// The key is the node group name (e.g., `system`, `master`, `worker`), and the value is the list of external IP addresses for nodes in that group, listed in the order of the zones where nodes will be created.
	//
	// The following values can be specified in the list:
	// - IP address from an additional external network for the corresponding zone, listed in the [`externalSubnetIDs`](#parameters-nodes-parameters-externalsubnetids) parameter;
	// - [reserved public IP address](faq.html#reserving-a-public-ip-address), if the [`externalSubnetIDs`](#parameters-nodes-parameters-externalsubnetids) parameter is not set;
	// - `Auto`, to order a public IP address in the corresponding zone.
	//
	// +deckhouse:ru:description:value="Внешние IP-адреса для каждой группы узлов."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="Ключ — имя группы узлов (например, `system`, `master`, `worker`), значение — список внешних IP-адресов для узлов этой группы, перечисленных в порядке зон, в которых будут создаваться узлы."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="В списке можно указывать следующие значения:"
	// +deckhouse:ru:description:value="- IP-адрес из дополнительной внешней сети для соответствующей зоны, перечисленной в параметре [`externalSubnetIDs`](#parameters-nodes-parameters-externalsubnetids);"
	// +deckhouse:ru:description:value="- [зарезервированный публичный IP-адрес](faq.html#резервирование-публичного-ip-адреса), если параметр [`externalSubnetIDs`](#parameters-nodes-parameters-externalsubnetids) не задан;"
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
	// Limits the zones the cluster creates nodes in. When the parameter is not set, every zone covered by the cluster subnets is used.
	// +deckhouse:ru:description:value="Ограничивает набор зон, в которых создаются узлы кластера. Если параметр не задан, используются все зоны, которые покрывают подсети кластера."
	// +kubebuilder:validation:UniqueItems=true
	// +kubebuilder:validation:items:Enum=ru-central1-a;ru-central1-b;ru-central1-d;ru-central1-e
	// +optional
	Zones []string `json:"zones,omitempty"`
}

// DHCPOptions defines DHCP parameters to use for all subnets.
// +deckhouse:DisableAdditionalProperties=true
type DHCPOptions struct {
	// Domain appended to an incomplete host name when the node resolves it.
	// +deckhouse:ru:description:value="Домен, который узел добавляет к неполному имени хоста при разрешении имён."
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
	// CIDR of an automatically created subnet for the internal interface. Takes precedence over the [`internalSubnetID`](#parameters-nodes-parameters-withnatinstance-internalsubnetid) parameter.
	// +deckhouse:ru:description:value="CIDR автоматически создаваемой подсети для внутреннего интерфейса. Имеет приоритет над параметром [`internalSubnetID`](#parameters-nodes-parameters-withnatinstance-internalsubnetid)."
	// +optional
	InternalSubnetCIDR string `json:"internalSubnetCIDR,omitempty"`
	// A [reserved external IP address](faq.html#reserving-a-public-ip-address). When the [`externalSubnetID`](#parameters-nodes-parameters-withnatinstance-externalsubnetid) parameter is set, specify an address from that subnet.
	// +deckhouse:ru:description:value="Внешний [зарезервированный IP-адрес](faq.html#резервирование-публичного-ip-адреса). Если задан параметр [`externalSubnetID`](#parameters-nodes-parameters-withnatinstance-externalsubnetid), укажите адрес из этой подсети."
	// +kubebuilder:validation:Pattern=`^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$`
	// +optional
	NATInstanceExternalAddress string `json:"natInstanceExternalAddress,omitempty"`
	// Internal IP address of the NAT instance. To let Yandex Cloud assign the address, leave this parameter unset.
	// +deckhouse:ru:description:value="Внутренний IP-адрес NAT-инстанса. Чтобы адрес назначил Yandex Cloud, не задавайте этот параметр."
	// +kubebuilder:validation:Pattern=`^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$`
	// +optional
	NATInstanceInternalAddress string `json:"natInstanceInternalAddress,omitempty"`
	// Computing resources that are allocated to the NAT instance. If not specified, the default values will be used.
	//
	// **Warning.** When these parameters change, the [`terraform-manager`](/modules/terraform-manager/) module restarts the NAT instance automatically if the [`autoConvergerEnabled`](/modules/terraform-manager/configuration.html#parameters-autoconvergerenabled) parameter is set to `true`. The restart interrupts cluster network traffic for a short time.
	// +deckhouse:ru:description:value="Вычислительные ресурсы, выделяемые для NAT-инстанса. Если параметр не указан, используются значения по умолчанию."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="**Внимание.** При изменении этих параметров модуль [`terraform-manager`](/modules/terraform-manager/) перезапускает NAT-инстанс автоматически, если параметр [`autoConvergerEnabled`](/modules/terraform-manager/configuration.html#parameters-autoconvergerenabled) имеет значение `true`. Перезапуск на короткое время прерывает сетевой трафик в кластере."
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

// Parameters of the storage subsystem.
// +deckhouse:ru:description:value="Параметры подсистемы хранения данных."
// +deckhouse:DisableAdditionalProperties=true
type StorageParameters struct {
	// Defines additional StorageClasses or overrides the parameters of the ones created by default.
	//
	// A StorageClass created by default is overridden when its name matches the `name` field exactly.
	// +deckhouse:ru:description:value="Определяет дополнительные StorageClass или переопределяет параметры создаваемых по умолчанию."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="Создаваемый по умолчанию StorageClass переопределяется, если его имя точно совпадает со значением поля `name`."
	// +optional
	ProvisionedStorageClasses []ProvisionedStorageClass `json:"provisionedStorageClasses,omitempty"`
	// List of StorageClasses to exclude from use in the cluster.
	//
	// Each item is a regular expression matched against the whole StorageClass name. For example, `network-ssd` excludes only the `network-ssd` StorageClass, and `network-ssd.*` excludes every StorageClass whose name starts with `network-ssd`.
	//
	// The list is applied after [`provisionedStorageClasses`](#parameters-storage-parameters-provisionedstorageclasses), so it also filters out the StorageClass defined there.
	// +deckhouse:ru:description:value="Список StorageClass, исключаемых из использования в кластере."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="Каждый элемент задаёт регулярное выражение, которое сопоставляется с именем StorageClass целиком. Например, `network-ssd` исключает только StorageClass `network-ssd`, а `network-ssd.*` исключает все StorageClass, имя которых начинается с `network-ssd`."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="Список применяется после параметра [`provisionedStorageClasses`](#parameters-storage-parameters-provisionedstorageclasses), поэтому исключает и заданные в нём StorageClass."
	// +optional
	ExcludedStorageClasses []string `json:"excludedStorageClasses,omitempty"`
}

// +deckhouse:DisableAdditionalProperties=true
type ProvisionedStorageClass struct {
	// The name of the StorageClass to create.
	// +deckhouse:ru:description:value="Имя создаваемого StorageClass."
	Name string `json:"name" yaml:"name"`
	// The disk type.
	// +deckhouse:ru:description:value="Тип диска."
	// +kubebuilder:validation:Enum=network-hdd;network-ssd;network-ssd-nonreplicated;network-ssd-io-m3
	Type string `json:"type" yaml:"type"`
	// The block size of disks created by this StorageClass.
	//
	// If the parameter is omitted, the default Yandex Cloud block size is used — `4Ki`.
	//
	// The block size determines the maximum size of a network disk: for a `4Ki` block it is `8Ti`, and it doubles with each next increase of the block size — up to `256Ti` for a `128Ki` block.
	//
	// After a disk is created, its block size cannot be changed. Changing the `blockSize` parameter recreates the StorageClass, but does not change the block size of previously provisioned volumes.
	//
	// For details, see the [official Yandex Cloud documentation](https://yandex.cloud/en/docs/compute/operations/disk-create/empty-disk-blocksize).
	// +deckhouse:ru:description:value="Размер блока дисков, создаваемых этим StorageClass."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="Если параметр не указан, используется размер блока по умолчанию для Yandex Cloud — `4Ki`."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="Размер блока определяет максимальный размер сетевого диска: для блока `4Ki` он составляет `8Ti`, а при каждом следующем увеличении размера блока удваивается — вплоть до `256Ti` для блока `128Ki`."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="После создания диска изменить размер его блока нельзя. Изменение параметра `blockSize` приводит к пересозданию StorageClass, но не изменяет размер блока у ранее созданных томов."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="Подробнее в [официальной документации Yandex Cloud](https://cloud.yandex.ru/docs/compute/operations/disk-create/empty-disk-blocksize)."
	// +kubebuilder:validation:Enum="4Ki";"8Ki";"16Ki";"32Ki";"64Ki";"128Ki"
	// +optional
	BlockSize string `json:"blockSize,omitempty" yaml:"blockSize,omitempty"`
}

// Parameters of the Cloud Controller Manager subsystem.
// +deckhouse:ru:description:value="Параметры подсистемы Cloud Controller Manager."
// +deckhouse:DisableAdditionalProperties=true
type CCMParameters struct {
	// Additional external network IDs to be recognized as external networks by the CCM.
	// +deckhouse:ru:description:value="Дополнительные ID внешних сетей, которые будут распознаваться CCM как внешние сети."
	// +optional
	AdditionalExternalNetworkIDs []string `json:"additionalExternalNetworkIDs,omitempty"`
	// Additional internal network IDs to be recognized as internal networks by the CCM.
	// +deckhouse:ru:description:value="Дополнительные ID внутренних сетей, которые будут распознаваться CCM как внутренние сети."
	// +optional
	AdditionalInternalNetworkIDs []string `json:"additionalInternalNetworkIDs,omitempty"`
}

// HasProviderSection reports whether the provider settings section is set.
func (s *ModuleConfigSettings) HasProviderSection() bool {
	return s != nil && !reflect.DeepEqual(s.Provider, Provider{})
}

// HasNodesSection reports whether the nodes settings section is set.
func (s *ModuleConfigSettings) HasNodesSection() bool {
	return s != nil && !reflect.DeepEqual(s.Nodes, Nodes{})
}

// HasStorageSection reports whether the storage settings section is set.
func (s *ModuleConfigSettings) HasStorageSection() bool {
	return s != nil && !reflect.DeepEqual(s.Storage, Storage{})
}

// HasCCMSection reports whether the ccm settings section is set.
func (s *ModuleConfigSettings) HasCCMSection() bool {
	return s != nil && !reflect.DeepEqual(s.CCM, CCM{})
}
