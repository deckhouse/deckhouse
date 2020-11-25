---
title: Подсистема CandI (Cluster and Infrastructure)
permalink: /candi/
---

Система для развертывания и управления Kubernetes кластерами.

У системы есть несколько компонентов:
* [**bashible**](./bashible) - фреймворк, который позволяет устанавливать и обновлять необходимые компоненты на узлах кластера `Kubernetes`.
* kubeadm – TODO
* cloud-providers (layouts for terraform + extra bashible) – TODO
* Модули **Deckhouse'а**:
    * [**control-plane-manager**](/modules/040-control-plane-manager/) - установка и обновление `control-plane` для master-узлов.
    * [**node-manager**](/modules/040-node-manager/) - создание и автоматическое или управляемое обновление узлов в облаке и/или на голом железе.
    * **cloud-provider-**
        * [**openstack**](/modules/030-cloud-provider-openstack/) - модуль для взаимодействия с облаками на базе `OpenStack`.
* Installer или [**candictl**](/candi/candictl.html) - система для развертывания первого узла кластера, установки `Deckhouse` и создания первичной инфраструктуры.

## Installer

### Конфигурация

Конфигурация это один файл `.yaml`, в котором содержатся несколько документов в YAML-формате, разделенных `---`.

Пример:
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterConfiguration
clusterType: Static
podSubnetCIDR: 10.111.0.0/16
podSubnetNodeCIDRPrefix: "24"
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "1.16"
clusterDomain: cluster.local
---
apiVersion: deckhouse.io/v1alpha1
kind: InitConfiguration
deckhouse:
  imagesRepo: registry.example.com/deckhouse
  registryDockerCfg: edsfkslfklsdfkl==
  releaseChannel: Alpha
  configOverrides:
    global:
      clusterName: my-cluster-name
      project: my-project
```

Для валидации и проставления значений по умолчанию используются спецификации OpenAPI.

| Kind                           | Description        |  OpenAPI path       | 
| ------------------------------ | ------------------ | ------------------ |
| ClusterConfiguration           | Основная часть конфигурации кластера Kubernetes | [candi/openapi/cluster_configuration.yaml](https://github.com/deckhouse/deckhouse/blob/master/candi/openapi/cluster_configuration.yaml) |
| InitConfiguration              | Часть конфигурации кластера, которая нужна только при создании | [candi/openapi/init_configuration.yaml](https://github.com/deckhouse/deckhouse/blob/master/candi/openapi/init_configuration.yaml)|
| StaticClusterConfiguration     | Конфигурация статического кластера Kubernetes | [candi/openapi/static_cluster_configuration.yaml](https://github.com/deckhouse/deckhouse/blob/master/candi/openapi/static_cluster_configuration.yaml)|
| OpenStackClusterConfiguration  | Конфигурации кластера Kubernetes в OpenStack | [candi/cloud-providers/openstack/openapi/openapi/cluster_configuration.yaml](https://github.com/deckhouse/deckhouse/blob/master/candi/cloud-providers/openstack/openapi/cluster_configuration.yaml) |
| AWSClusterConfiguration   | Конфигурации кластера Kubernetes в AWS | [candi/cloud-providers/aws/openapi/openapi/cluster_configuration.yaml](https://github.com/deckhouse/deckhouse/blob/master/candi/cloud-providers/aws/openapi/cluster_configuration.yaml) |
| GCPClusterConfiguration   | Конфигурации кластера Kubernetes в GCP | [candi/cloud-providers/gcp/openapi/openapi/cluster_configuration.yaml](https://github.com/deckhouse/deckhouse/blob/master/candi/cloud-providers/gcp/openapi/cluster_configuration.yaml) |
| VsphereClusterConfiguration    | Конфигурации кластера Kubernetes в VSphere | [candi/cloud-providers/vsphere/openapi/openapi/cluster_configuration.yaml](https://github.com/deckhouse/deckhouse/blob/master/candi/cloud-providers/vsphere/openapi/cluster_configuration.yaml) |
| YandexClusterConfiguration     | Конфигурации кластера Kubernetes в Yandex | [candi/cloud-providers/yandex/openapi/openapi/cluster_configuration.yaml](https://github.com/deckhouse/deckhouse/blob/master/candi/cloud-providers/yandex/openapi/cluster_configuration.yaml) |
| BashibleTemplateData           | Данные для компиляции Bashible Bundle (используется только для candictl render bashible-bunble) | [candi/bashible/openapi.yaml](https://github.com/deckhouse/deckhouse/blob/master/candi/bashible/openapi.yaml) |
| KubeadmConfigTemplateData      | Данные для компиляции Kubeadm config (используется только для candictl render kubeadm-config) | [candi/control-plane-kubeadm/openapi.yaml](https://github.com/deckhouse/deckhouse/blob/master/candi/control-plane-kubeadm/openapi.yaml)|

### Bootstrap
Процесс развертывания кластера при помощи `candictl` делится на несколько этапов:

#### Terraform
Существуют три варианта запуска:
* `base-infrastructure` - создает в облаке компоненты инфраструктуры: сети, роутеры, ssh-ключи, политики безопасности и так далее.
    * Через механизм [ouput](https://www.terraform.io/docs/configuration/outputs.html) на данном этапе в installer передаются данные:
        * `cloud_discovery_data` - информация, необходима для корректной работы cloud-provider'а в дальнейшем, будет сохранена в secret `d8-provider-cluster-configuration` в namespace `kube-system`.

* `master-node` - создает master-узлы для кластера.
    * Через механизм [ouput](https://www.terraform.io/docs/configuration/outputs.html) на данном этапе в installer передаются данные:
        * `master_ip_address_for_ssh` - адрес из "внешней" сети, по нему мы будем производить подключение к первому узлу.
        * `node_internal_ip_address` - адрес из "внутренней" сети, будет использован для настройки control-plane компонентов.
        * `kubernetes_data_device_path` - имя девайса, предназначенного для хранения данных Kubernetes.

* `static-node` - создает статический узел для кластера.

> State terraform'а будет сохранен в secret в namespace'е d8-system после каждой фазы

**Внимание!!** для bare metal кластеров terraform не выполняется, вместо этого обязательным становится параметр командной строки `--ssh-host`, чтобы candictl знал, куда ему нужно подключиться.

#### Подготовительный этап
Во время подготовительного этапа происходит:
* **Подключение к созданному (или указанному) узлу по SSH**: Если к указанному узлу подключится не получится, то процесс установки прервётся с ошибкой.
* **Обнаружение bashible bundle**: на узле выполняется скрипт `/candi/bashible/detect_bundle.sh`. Результат выполнения - имя bundle, отправленное в stdout.
* **Подготовка и запуск скриптов bootstrap.sh и bootstrap-network.sh**: скрипты необходимы для установки зависимости и первичной настройки сети для правильной работы Kubernetes

**Внимание!!** Первое подключение по ssh происходит только для проверки соединения. Далее скрипты загружаются на сервер по протоколу scp и запускаются через ssh на удаленном сервере.

#### Bashible Bundle
Bundle представляет собой tar-архив со всеми необходимыми файлами с такой же структурой папок, которая должна быть на удаленном сервере. 

В bundle входят:
1. Подготовленные step'ы из всех директорий (подробнее можно узнать о расположении степов из [описания bashible](/candi/bashible/)).
2. Подготовленный файл конфигурации для kubeadm (подробнее можно узнать о конфигурации из [описания control-plane-kubeadm](/candi//control-plane-kubeadm/)).
3. Объединенный в один файл bashbooster.

Далее архив загружается по scp на сервер и распаковывается, после чего выполняется `/var/lib/bashible/bashible.sh --local`.

#### Установка Deckhouse
Для доступа к API только что созданного кластера Kubernetes candictl делает две вещи:
* Запускает на сервере Kubernetes команду `kubectl proxy --port=0` для поднятия прокси на свободном порту.
* Открывает ssh-туннель со свободного локального порта на порта прокси на удаленном сервере.

После получения доступа к API `candictl` создает (или обновляет):
* Cluster Role `cluster-administrator`
* Service Account для `deckhouse`
* Cluster Role Binding роли `cluster-administrator` для sa `deckhouse`
* Secret для доступа к docker registry для deckhouse `deckhouse-registry`
* ConfigMap для `deckhouse`
* Deployment для `deckhouse`
* Secret'ы с данными создания кластера (если такие данные есть):
    * `d8-cluster-configuration`
    * `d8-provider-cluster-configuration`
* Secret'ы, содержащие состояние terraform
    * `d8-cluster-terraform-state`
    * `d8-node-terraform-state-.*`

 После установки `candictl` ожидает, когда pod `deckhouse` станет `Ready`. Readiness-проба устроена так, что контейнер переходит в состояние Ready только после того, как в очереди `deckhouse` не останется ни одного задания, связанного с установкой или обновлением модуля.
 
 Состояние `Ready` - сигнал для `candictl`, что можно создать в кластере объект `NodeGroup` для master-узлов.
 
#### Создание дополнительных master-узлов и статических узлов
При создании дополнительных узлов candictl взаимодействует с API Kubernetes. 
* Создает необходимые NodeGroup объекты
* Дожидается появления Secret'ов, содержащих cloud-config для создания узлов в этой группе
* Запускает соответствующий terraform (master-node  или static-node)
* При успешном выполнении сохраняет state в кластер Kubernetes

> Deckhouse-candi ожидает перехода узлов в каждой NodeGroup в состояние Ready, иначе процесс их создания будет завершен с ошибкой

#### Создание дополнительных ресурсов
При указании пути до файла с манифестами (флаг `--resources`), candictl отсортирует их по `apiGroup/kind`, дождется регистрации этих типов в API Kubernetes и создаст их.
> Процесс описан подробнее в документации к candictl.
