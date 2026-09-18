/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

// Package v2 contains the ModuleConfig settings root type for the cloud-provider-zvirt module.
package v2

import (
	"reflect"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

var (
	_ cpapi.ModuleSettingsObject = (*ModuleConfigSettings)(nil)
)

// Describes the configuration of a cloud cluster in zVirt.
//
// Used by the cloud provider if a cluster's control plane is hosted in the cloud.
//
// Run the following command to change the configuration in a running cluster:
//
// ```shell
// d8 k edit moduleconfig cloud-provider-zvirt
// ```
//
// > After updating the node parameters, you need to run the `dhctl converge` command for the changes to take effect.
// +deckhouse:ru:description:value="Описывает конфигурацию облачного кластера в Zvirt."
// +deckhouse:ru:description:value=
// +deckhouse:ru:description:value="Используется облачным провайдером, если управляющий слой (control plane) кластера размещен в облаке."
// +deckhouse:ru:description:value=
// +deckhouse:ru:description:value="Выполните следующую команду, чтобы изменить конфигурацию в работающем кластере:"
// +deckhouse:ru:description:value=
// +deckhouse:ru:description:value="```shell"
// +deckhouse:ru:description:value="d8 k edit moduleconfig cloud-provider-zvirt"
// +deckhouse:ru:description:value="```"
// +deckhouse:ru:description:value=
// +deckhouse:ru:description:value="> После изменения параметров узлов необходимо выполнить команду `dhctl converge`, чтобы изменения вступили в силу."
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

// Parameters for connecting to the Zvirt.
// +deckhouse:ru:description:value="Параметры для подключения к Zvirt."
// +deckhouse:DisableAdditionalProperties=true
type Provider struct {
	Parameters ProviderParameters `json:"parameters"`
}

// Nodes subsystem settings.
//
// Controls node management in the cluster.
// +deckhouse:ru:description:value="Настройки подсистемы управления узлами."
// +deckhouse:ru:description:value=
// +deckhouse:ru:description:value="Управляет узлами в кластере."
// +deckhouse:DisableAdditionalProperties=true
type Nodes struct {
	// Disables the node management subsystem.
	// +deckhouse:ru:description:value="Отключает подсистему управления узлами."
	// +kubebuilder:default=false
	// +optional
	Disabled   bool            `json:"disabled,omitempty"`
	Parameters NodesParameters `json:"parameters"`
}

// Storage subsystem settings.
//
// Controls disk provisioning in the cluster.
// +deckhouse:ru:description:value="Настройки подсистемы хранения данных."
// +deckhouse:ru:description:value=
// +deckhouse:ru:description:value="Управляет возможностью заказа дисков в кластере."
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
// CCM integrates the cluster with the cloud provider — for example, it manages load balancers.
// You can enable or disable CCM independently of other subsystems. For instance, leave CCM enabled
// if you only need load balancer management in the cluster.
// +deckhouse:ru:description:value="Настройки подсистемы Cloud Controller Manager (CCM)."
// +deckhouse:ru:description:value=
// +deckhouse:ru:description:value="CCM обеспечивает интеграцию кластера с облачным провайдером — например, управляет балансировщиками нагрузки."
// +deckhouse:ru:description:value="CCM можно включать и отключать независимо от других подсистем. Например, оставьте CCM включённым, если в кластере нужно управлять только балансировщиками нагрузки."
// +deckhouse:DisableAdditionalProperties=true
type CCM struct {
	// Disables the Cloud Controller Manager.
	//
	// Set to `true` if CCM is not required. Leave enabled (`false`) when you need cloud load balancer management.
	// +deckhouse:ru:description:value="Отключает Cloud Controller Manager."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="Установите в `true`, если CCM не требуется. Оставьте включённым (`false`), если нужно управление облачными балансировщиками нагрузки."
	// +kubebuilder:default=false
	// +optional
	Disabled bool `json:"disabled,omitempty"`
}

// Contains settings to connect to the Zvirt API.
//
// The login ID and the user's password are stored separately, in a Secret of the
// `cloud-provider.deckhouse.io/credentials` type.
// +deckhouse:ru:description:value="Содержит настройки для подключения к API Zvirt."
// +deckhouse:ru:description:value=
// +deckhouse:ru:description:value="Логин и пароль пользователя хранятся отдельно, в секрете типа `cloud-provider.deckhouse.io/credentials`."
// +deckhouse:DisableAdditionalProperties=true
type ProviderParameters struct {
	// The URL to the Zvirt API endpoint.
	// +deckhouse:ru:description:value="Хост или IP-адрес zvirt сервера."
	Server string `json:"server"`
	// Cluster ID with shared storage domains and CPUs of the same type to create virtual machines.
	// +deckhouse:ru:description:value="ID кластера с общими доменами хранения и ЦП одного типа для создания виртуальных машин."
	// +kubebuilder:validation:Pattern=`^[\da-fA-F]{8}\-[\da-fA-F]{4}\-[\da-fA-F]{4}\-[\da-fA-F]{4}\-[\da-fA-F]{12}$`
	// +deckhouse:XDocExamples:value="49bb4594-0cd4-4eb7-8288-8594eafd5a86"
	ClusterID string `json:"clusterID"`
	// CA certificate in base64.
	// +deckhouse:ru:description:value="CA сертификат закодированный base64."
	// +optional
	CABundle string `json:"caBundle,omitempty"`
	// Set to `true` if Zvirt has a self-signed certificate.
	// +deckhouse:ru:description:value="Установите значение `true`, если Zvirt имеет самоподписанный сертификат."
	// +kubebuilder:default=false
	// +deckhouse:XDocDefault:value=false
	// +optional
	Insecure bool `json:"insecure,omitempty"`
}

// Parameters of the nodes subsystem.
// +deckhouse:ru:description:value="Параметры подсистемы управления узлами."
// +deckhouse:DisableAdditionalProperties=true
type NodesParameters struct {
	// A public key for accessing nodes.
	// +deckhouse:ru:description:value="Публичный ключ для доступа на узлы."
	// +deckhouse:XRules=sshPublicKey
	SSHPublicKey string `json:"sshPublicKey"`
	// The way resources are located in the cloud.
	//
	// Read [more](https://deckhouse.io/modules/cloud-provider-zvirt/layouts.html) about possible provider layouts.
	// +deckhouse:ru:description:value="Название схемы размещения."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="[Подробнее](https://deckhouse.ru/modules/cloud-provider-zvirt/layouts.html) о возможных схемах размещения провайдера."
	// +kubebuilder:validation:Enum=Standard
	Layout string `json:"layout"`
	// Static network configuration of the cluster nodes, keyed by CloudPermanent NodeGroup name.
	//
	// Addresses are assigned by node index: the first address of `networkInterfaceAddresses` goes
	// to the node with index 0, the second to the node with index 1, and so on. The list must
	// therefore hold at least as many addresses as the NodeGroup has replicas.
	//
	// Nodes of a NodeGroup that is not listed here are configured over DHCP.
	// +deckhouse:ru:description:value="Статическая настройка сети узлов кластера. Ключ — имя группы узлов (NodeGroup) с типом CloudPermanent."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="Адреса назначаются по порядковому номеру узла: первый адрес `networkInterfaceAddresses` получит узел с индексом 0, второй — узел с индексом 1 и так далее. Поэтому в списке должно быть не меньше адресов, чем реплик в группе узлов."
	// +deckhouse:ru:description:value=
	// +deckhouse:ru:description:value="Узлы группы, не указанной в этом параметре, настраиваются по DHCP."
	// +optional
	CustomNetworkConfigs map[string]CustomNetworkConfig `json:"customNetworkConfigs,omitempty"`
}

// CustomNetworkConfig describes a static network configuration of the node group interfaces.
// +deckhouse:ru:description:value="Описывает статическую настройку сетевых интерфейсов группы узлов."
// +deckhouse:DisableAdditionalProperties=true
type CustomNetworkConfig struct {
	// Name of the network interface to apply the static configuration to.
	// +deckhouse:ru:description:value="Имя сетевого интерфейса, для которого будет применена статическая настройка."
	// +deckhouse:XDocExamples:value="enp1s0"
	NetworkInterfaceName string `json:"networkInterfaceName"`

	// List of IP addresses to assign to the interface (one IP address per node).
	// +deckhouse:ru:description:value="Список IP-адресов, которые нужно назначить интерфейсам (по адресу для каждого узла)."
	// +deckhouse:XDocExamples:value={"192.168.1.10","192.168.1.11"}
	// +kubebuilder:validation:MinItems=1
	NetworkInterfaceAddresses []string `json:"networkInterfaceAddresses"`

	// Subnet mask for the interface.
	// +deckhouse:ru:description:value="Маска подсети для интерфейса."
	// +deckhouse:XDocExamples:value="255.255.255.0"
	NetworkInterfaceNetmask string `json:"networkInterfaceNetmask"`

	// Default network gateway.
	// +deckhouse:ru:description:value="Шлюз по умолчанию."
	// +deckhouse:XDocExamples:value="192.168.1.1"
	NetworkInterfaceGateway string `json:"networkInterfaceGateway"`

	// List of DNS servers.
	// +deckhouse:ru:description:value="Список DNS-серверов."
	// +deckhouse:XDocExamples:value={"8.8.8.8","8.8.4.4"}
	// +optional
	DNSServers []string `json:"dnsServers,omitempty"`
}

// Parameters of the storage subsystem.
// +deckhouse:ru:description:value="Параметры подсистемы хранения данных."
// +deckhouse:DisableAdditionalProperties=true
type StorageParameters struct {
	// A list of StorageClass names (or regex expressions for names) to exclude from creation in the cluster.
	// +deckhouse:ru:description:value="Список имён StorageClass (или регулярных выражений для имён), которые не нужно создавать в кластере."
	// +optional
	ExcludedStorageClasses []string `json:"excludedStorageClasses,omitempty"`
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
