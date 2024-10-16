# Регистрация cloud-provider

Для добавления нового Deckhouse cloud-provider необходимо определиться с его названием и именем модуля, которое он будет использовать.

Предположим, наш провайдер будет называться `Sample` и имя модуля будет `cloud-provider-sample`.

Название необходимо зарегистрировать в `candi/openapi/cluster-configuration.yaml`:

```yaml
provider:
  type: string
  description: |
    Cloud provider.
  enum:
  - "OpenStack"
  - "AWS"
  - "GCP"
  - "Yandex"
  - "vSphere"
  - "Azure"
  - "VCD"
  - "Zvirt"
  - "Sample" #<<---
```

Провайдеры, которые не планируется включать в редакцию Community Edition так же необходимо добавить в список в `deckhouse/ee/candi/cloud-provider/.build.yaml`.
Название провайдера должно быть в той же форме, в которой оно будет описано в имени модуля, но без префикса `cloud-provider-`:

```yaml
openstack
vsphere
vcd
zvirt
sample #<<---
```

## Определение InstanceClass

Для каждого провайдера должен быть определен ресурс типа InstanceClass.

Имя такого ресурса формируется из имени провайдера, которое было зарегистрировано в `cloud-configuration.yaml` и суффикса `InstanceClass`.

InstanceClass регистрируется в `deckhouse/candi/openapi/node-group.yaml`:

```yaml
classReference:
  description: |
    The reference to the `InstanceClass` object. It is unique for each `cloud-provider-*` module.
  type: object
  properties:
  kind:
    description: |
      The object type (e.g., `OpenStackInstanceClass`). The object type is specified in the documentation of the corresponding `cloud-provider-` module.
    type: string
    enum:
    - OpenStackInstanceClass
    - GCPInstanceClass
    - VsphereInstanceClass
    - AWSInstanceClass
    - YandexInstanceClass
    - AzureInstanceClass
    - VCDInstanceClass
    - ZvirtInstanceClass
    - SampleInstanceClass #<<---
```

>Обратите внимание, это нужно сделать в нескольких местах данного файла

## Провайдер Terraform

Для первоначальной установки инфраструктуры используется Terraform 0.14.8.
Необходимо добавить информацию об используемом модулем провайдере Terraform в `deckhouse/candi/terraform_versions.yaml`:

```yaml
sample:
  namespace: terraform-provider-sample
  type: sample
  version: 0.0.1
  artifact: terraform-provider-sample
  artifactBinary: terraform-provider-sample
  destinationBinary: terraform-provider-sample
```

Кроме того, необходимо добавить информацию о том, в каких редакциях будет использоваться этот Terraform provider в файл `deckhouse/editions.yaml`:

```yaml
editions:
- name: EE
  modulesDir: "ee/modules"
  terraformProviders:
    - openstack
    - vsphere
    - vcd
    - sample #<<---
```

## Openapi

Для нового провайдера также необходимо определить ресурсы:

* <cloud-provider-name>CloudDiscoveryData
* <cloud-provider-name>ClusterConfiguration
* <cloud-provider-name>InstanceClass для провайдера.

### <cloud-provider-name>CloudDiscoveryData

Данный ресурс описывает набор данных (справочников) возвращаемых провайдером.

### <cloud-provider-name>ClusterConfiguration

Данный ресурс описывает провайдер-зависимые параметры для развертывания кластера

### <cloud-provider-name>InstanceClass

Данный ресурс описывает структуру данных, необходимую для создания виртуальной машины
