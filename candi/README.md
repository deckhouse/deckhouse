Candi - Cluster and Infrastructure
==================================
Система для развертывания и управления Kubernetes кластерами.

У системы есть несколько компонентов:
* [**bashible**](./bashible) - фреймворк, который позволяет устанавливать и обновлять необходимые компоненты на узлах кластера `Kubernetes`.
* kubeadm – TODO
* cloud-providers (layouts for terraform + extra bashible) – TODO
* Модули **Deckhouse'а**:
    * [**control-plane-manager**](../modules/040-control-plane-manager) - установка и обновление `control-plane` для master-узлов.
    * [**node-manager**](../modules/040-node-manager) - создание и автоматическое или управляемое обновление узлов в облаке и/или на голом железе.
    * **cloud-provider-**
        * [**openstack**](../modules/030-cloud-provider-openstack) - модуль для взаимодействия с облаками на базе `OpenStack`.
* Installer или [**deckhouse-candi**](../deckhouse-candi) - система для развертывания первого узла кластера, установки `Deckhouse` и создания первичной инфраструктуры.

## Installer

### Конфигурация

Конфигурация это один файл `.yaml`, в котором содержатся несколько документов в YAML-формате, разделенных `---`.

Пример:
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterConfiguration
clusterType: Static
...
---
apiVersion: deckhouse.io/v1alpha1
kind: InitConfiguration
...
```

Для валидации и проставления значений по умолчанию используются спецификации OpenAPI.

| Kind          | Description        | OpenAPI path       |
| ------------- | ------------------ | ------------------ |
| ClusterConfiguration  | Основная часть конфигурации кластера Kubernetes | [candi/openapi/cluster_configuration.yaml](./openapi/cluster_configuration.yaml) |
| InitConfiguration     | Часть конфигурации кластера, которая нужна только при создании | [candi/openapi/init_configuration.yaml](./openapi/init_configuration.yaml)|
| OpenStackClusterConfiguration  | Основная часть конфигурации кластера Kubernetes в OpenStack | [candi/cloud-providers/openstack/openapi/openapi/cluster_configuration.yaml](./cloud-providers/openstack/openapi/cluster_configuration.yaml) |
| OpenStackInitConfiguration     | Часть конфигурации, которая нужна только при создании кластера в OpenStack | [candi/cloud-providers/openstack/openapi/init_configuration.yaml](./cloud-providers/openstack/openapi/init_configuration.yaml)|
| BashibleTemplateData  | Данные для компиляции Bashible Bundle (используется только для deckhouse-candi render bashible-bunble) | [candi/bashible/openapi.yaml](./bashible/openapi.yaml) |
| KubeadmConfigTemplateData | Данные для компиляции Kubeadm config (используется только для deckhouse-candi render kubeadm-config) | [candi/control-plane-kubeadm/openapi.yaml](./control-plane-kubeadm/openapi.yaml)|

### Bootstrap
Процесс развертывания кластера при помощи `deckhouse-candi` делится на несколько этапов:

#### Terraform
Запуск terraform разделен на два этапа:
* `base-infrastructure` - создает в облаке основные компоненты для создания инфраструктуры: сети, роутеры, ssh-ключи, security-группы.
    * Через механизм [ouput](https://www.terraform.io/docs/configuration/outputs.html) на данном этапе в installer передаются данные:
        * `cloud_discovery_data` - информация, необходима для корректной работы cloud-provider'а в дальнейшем, будет сохранена в secret `d8-provider-cluster-configuration` в namespace `kube-system`.
        * `deckhouse_config` - часть конфигурации Deckhouse, которая в будущем будет слита и сохранена в configmap `deckhouse` в namespace `d8-system`. 
    * State terraform'а после выполнения данной фазы будет сохранен в secret `d8-cluster-teraform-state`.

* `master-node-bootstrap` - создает первый узел кластера.
    * Через механизм [ouput](https://www.terraform.io/docs/configuration/outputs.html) на данном этапе в installer передаются данные:
        * `master_instance_class` - `OpenStackInstanceClass` для создания master-узлов.
        * `master_ip` - адрес из "внешней" сети, по нему мы будем производить подключение к первому узлу.
        * `node_ip` - адрес из "внутренней" сети, будет использован для настройки control-plane компонентов.
    * State terraform'а после выполнения данной фазы сохранен не будет.

**Внимание!!** для baremetal кластеров terraform не выполняется, вместо этого обязательным становится параметр командной строки `--ssh-host`, чтобы deckhouse-candi знал, куда ему нужно подключиться.

#### Подготовительный этап
Во время подготовительного этапа происходит:
* **Подключение к созданному (или указанному) узлу по SSH**: Если к указанному узлу подключится не получится, то процесс установки прервётся с ошибкой.
* **Обнаружение bashible bundle**: на узле выполняется скрипт `/candi/bashible/detect_bundle.sh`. Результат выполнения - имя bundle, отправленное в stdout.
* **Подготовка и запуск скриптов bootstrap.sh и bootstrap-network.sh**: скрипты необходимы для установки зависимости и первичной настройки сети для правильной работы Kubernetes

**Внимание!!** Первое подключение по ssh происходит только для проверки соединения. Далее скрипты загружаются на сервер по протоколу scp и запускаются через ssh на удаленном сервере.

#### Bashible Bundle
Bundle представляет собой tar-архив со всеми необходимыми файлами с такой же структурой папок, которая должна быть на удаленном сервере. 

В bundle входят:
1. Подготовленные step'ы из всех директорий (подробнее можно узнать о расположении степов из [описания bashible](./bashible/README.md)). 
2. Подготовленный файл конфигурации для kubeadm (подробнее можно узнать о конфигурации из [описания control-plane-kubeadm](./control-plane-kubeadm/README.md)). 
3. Объединенный в один файл bashbooster.

Далее архив загружается по scp на сервер и распаковывается, после чего выполняется `/var/lib/bashible/bashible.sh --local`.

#### Установка Deckhouse
Для доступа к API свежеустановленного кластера Kubernetes deckhouse-candi делает две вещи:
* Запускает на сервере Kubernetes команду `kubectl proxy --port=0` для поднятия прокси на свободном порту.
* Открывает ssh-туннель со свободного локального порта на порта прокси на удаленном сервере.

После получения доступа к API `deckhouse-candi` создает (или обновляет):
* Cluster Role `cluster-administrator`
* Service Account для `deckhouse`
* Cluster Role Binding роли `cluster-administrator` для sa `deckhouse`
* Secret для доступа к docker `registry`
* ConfigMap для `deckhouse`
* Deployment для `deckhouse`
* Secret'ы с данными создания кластера (если такие данные есть):
    * `d8-cluster-configuration`
    * `d8-cluster-terraform-state`
    * `d8-provider-cluster-configuration`
    
 После установки `deckhouse-candi` ожидает, когда pod `deckhouse` станет `Ready`. Readiness-проба устроена так, что контейнер переходит в состояние Ready только после того, как в очереди `deckhouse` не останется ни одного задания, связанного с установкой или обновлением модуля.
 
 Состояние `Ready` - сигнал для `deckhouse-candi`, что можно создать в кластере объект `NodeGroup` для master-узлов.
 
 На этом процесс развертывания кластера заканчивается.

