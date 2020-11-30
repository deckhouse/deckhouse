---
title: Фреймворк - Bashible
---

Фреймворк для настройки и управления узлами в кластере Kubernetes.

###  Описание
Bashible состоит из набора выполняемых скриптов или `step`'ов. 
* Каждый step описывает какое-то действие, например установка docker или настройка kubelet.

* Step'ы выполняются в алфавитном порядке, поэтому имя каждого степа начинается с числа:
    * Пример: `051_install_kubernetes_api_proxy_configurator.sh.tpl`

* Каждый step это `.tpl` файл в формате Go-шаблонов. Это необходимо чтобы:
    * Работать с динамической конфигурацией узлов в кластере: если настройки изменились мы можем легко создать новые step'ы из шаблонов.
    * Полностью не переписывать код для этапов: установка кластера, повторный запуск в кластере, подготовка образа виртуальной машины.

* Файлы step'ов и входная точка для запуска bashible лежат в директории `/var/lib/bashible`.

* На каждом узле происходят периодические запуски bashible при помощи системных unit'ов.

* Для написания step'ов используется доработанная нами SCM (software configuration management) система, созданная при помощи чистого bash - [Bash Booster](./candi/bashible/bashbooster).

* Наличие тех или иных step'ов зависит от имени bashible bundle и cloud-provider.
   * bashible bundle - основывается на операционной системе сервера, сейчас доступны:
       * ubuntu-20.04
       * ubuntu-18.04
       * centos-7
   * cloud-provider - имя облачного провайдера, сейчас доступны:
       * openstack
       
* `runType` - тип запуска скрипта, передается при компиляции Go-шаблонов, доступны три варианты:
   * ClusterBootstrap - подготовка первого узла кластера
   * Normal - последующие запуски bashible на первом и других узлах
   * ImageBuilding - подготовка образа виртуальной машины    
       
### Расположение step'ов
* `bashible/`
    * `bashbooster/` – директория, содержащая фреймворк Bash Booster
    * `bundles/` – список директорий в этой директории это и есть список поддерживаемых bundle'ов
        * `имя_бандла/`
           * `all/` - доступен при любых runType
           * `cluster-bootstrap/` - только при runType: ClusterBootstrap
           * `node-group/` - только НЕ при runType: ImageBuilding
    * `common-steps/` - общие step'ы для всех bashible bundl'ов
        * `all/` - доступен при любых runType
        * `cluster-bootstrap/` - только при runType: ClusterBootstrap
        * `node-group/` - только НЕ при runType: ImageBuilding
    * `bashible.sh.tpl` - входная точка для запуска step'ов
    * `detect_bundle.sh` - скрипт, который определяет bashible bundle, используется при создании первого узла
* `cloud-providers/` - список cloud-provider'ов
  * `имя_cloud_provider/`
      * `bashible/`
          * `bundles/` – при "компиляции" этих степов дополнительно передается .cloudProviderClusterConfiguration.
              * `имя_бандла/`
                  * `all/` - доступен при любых runType
                  * `node-group/` - только НЕ при runType: ImageBuilding
                  * `bootstrap-networks.sh.tpl` – минимальный скрипт, задача которого сделать возможной работу bashible: обеспечить доступ к API-серверу. nodeGroup доступна, выполняется только при runType in Normal или ClusterBootstrap.
          * `common_steps/` – общие step'ы для всех bundle'ов
              * `bootstrap-networks.sh.tpl` – если этот файл есть, то в бандлах НЕ может быть такого файла.

### Как скомпилировать bashible?
Скомпилировать bundle можно воспользовавшись утилитой candictl.
```bash
candictl render bashible-bundle --config=/config.yaml
```
Пример `config.yaml`:
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: BashibleTemplateData
bundle: ubuntu-18.04
provider: OpenStack
runType: ClusterBootstrap
clusterBootstrap:
  clusterDNSAddress: 10.222.0.10
  clusterDomain: cluster.local
  nodeIP: 192.168.199.23
kubernetesVersion: "1.16"
nodeGroup:
  cloudInstances:
    classReference:
      kind: OpenStackInstanceClass
      name: master
  instanceClass:
    flavorName: m1.large
    imageName: ubuntu-18-04-cloud-amd64
    mainNetwork: shared
    rootDiskSizeInGb: 20
  maxPerZone: 3
  minPerZone: 1
  name: master
  nodeType: Cloud
  zones:
  - nova
```
