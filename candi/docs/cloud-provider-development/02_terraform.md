# Terraform, Cloud Layouts и все что с ними связано

## Структура папок

По пути `<редакция deckhouse>/candi/cloud-providers` находятся ресурсы, используемые для первоначальной настройки инфраструктуры средствами Terraform.

Для каждого cloud-provider предусмотрена своя директория, имя которой совпадает с именем модуля без префикса `cloud-provider-`.
В случае с провайдером Proxmox путь будет выглядеть как `deckhouse/ee/candi/cloud-providers/proxmox`.

Структура директории для всех провайдеров приблизительно одинакова и выглядет так:

```text
layouts/ <<-- Возможные схемы расположения ресурсов в облаке, как минимум всегда присутсвует схема standard
  standard/
    base-infrastructure/ <<--- Здесь находится описание конфигурации облачной инфраструктуры, такой как подсети, маршрутизаторы, балансировщики и т.д.
    master-node/ <<-- Описание конфигурации master-узлов кластера.
    static-node/ <<-- Описание конфигурации cloud-permanent-узлов кластера.
    variables.tf <<-- Переменные, общие для всей схемы.
openapi/ <<--- Здесь находится описание ресурсов, которыми оперирует Deckhouse во время своей работы.
  cloud_discovery_data.yaml <<--- Схема структуры информации о базовой инфраструктуре облака, вычисляемой при бутстрапе (зоны доступности, адрес балансировщика и т.п.)
  cluster_configuration.yaml <<--- Схема структуры кластера и провайдеро-зависимой части кластера.
  doc-ru-cluster_configuration.yaml <<--- Русскоязычное описание схемы структуры кластера и провайдеро-зависимой части кластера.
  doc-ru-instance_class.yaml <<--- Русскоязычное описание схемы ресурса InstanceClass для провайдера (в нашем случае ProxmoxInstanceClass)
  instance_class.yaml <<--- Схема ресурса InstanceClass для провайдера (в нашем случае ProxmoxInstanceClass)
terraform-modules/ <<--- Здесь описываются ресурсы для работы с динамическими ресурсами, размещаемыми в облачной инфраструктуре
  master-node/ <<--- Master-узлы. Базовое описание, используется для формирования макетов.
    main.tf
    outputs.tf
    providers.tf
    variables.tf
    versions.tf
  static-node/ <<--- Узлы Cloud-permanent. Базовое описание, используется для формирования макетов.
    main.tf
    providers.tf
    variables.tf
    versions.tf
  providers.tf <<--- Описание конфигурации провайдера
  variables.tf
  versions.tf <<--- Описание версии провайдера
```

Чтобы не дублировать код, нужно импользовать относительные simlinks. Например:

```text
/deckhouse/ee/layouts/standart/base-infrastructure/providers.tf -> ../../../terraform-modules/providers.tf
/deckhouse/ee/layouts/standart/base-infrastructure/vercions.tf -> ../../../terraform-modules/versions.tf
/deckhouse/ee/layouts/standart/master-node -> ../../terraform-modules/master-node
/deckhouse/ee/layouts/standart/static-node -> ../../terraform-modules/static-node
```

## Возвращаемые значения из Terraform

### Base infrastructure

Необходимо вернуть данные для CloudDiscoveryData. Если провайдер не поддерживает ```zones``` необходимо вернуть список с зоной ```default```.
Пример:

```text
output "cloud_discovery_data" {
  value = {
    "apiVersion"       = "deckhouse.io/v1"
    "kind"             = "ZvirtCloudProviderDiscoveryData"
    "storageDomains"   = []
    "zones"            = ["default"]
  }
}
```

### Master node

Необходимо вернуть ```master_ip_address_for_ssh``` ip адрес master-node ```node_internal_ip_address``` внутренний ip адрес master-node и ```kubernetes_data_device_path``` путь к устройству диска для etcd
Пример:

```text
output "master_ip_address_for_ssh" {
  value = local.master_vm_ip
}

output "node_internal_ip_address" {
  value = local.master_vm_ip
}

output "kubernetes_data_device_path" {
  value = "/dev/sdb"
}
```
