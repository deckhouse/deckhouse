---
title: "Хранилище данных vSphere"
permalink: ru/admin/configuration/storage/external/vsphere.html
lang: ru
---

Модуль `csi-vsphere` предназначен для организации заказа (provisioning) дисков в статических кластерах на базе VMware vSphere, где отсутствует возможность использовать модуль [`cloud-provider-vsphere`](/modules/cloud-provider-vsphere/).

## Системные требования

- Все виртуальные машины кластера должны быть созданы с использованием инструментов vSphere.
- Имя виртуальной машины в vSphere должно точно совпадать с именем узла (hostname) в кластере Deckhouse Kubernetes Platform.
- В настройках каждой виртуальной машины необходимо включить параметр `disk.EnableUUID:TRUE`. Этот параметр обеспечивает корректную работу модуля с дисковыми ресурсами и позволяет DKP идентифицировать подключенные тома.

## Включение модуля

Для работы с хранилищами данных на базе VMware vSphere, где невозможно использовать модуль `cloud-provider-vsphere`, включите модуль `csi-vsphere`. Это приведет к тому, что на всех узлах кластера будут:

- зарегистрирован CSI-драйвер;
- запущены служебные поды компонента `csi-vsphere`.

Для влючения модуля выполните команду:

```yaml
d8 k apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: csi-vsphere
spec:
  enabled: true
  version: 1
  settings:
    # Обязательные параметры.
    host: myhost
    password: myPaSsWd
    region: myreg
    regionTagCategory: myregtagcat
    username: myuname
    vmFolderPath: dev/test
    zoneTagCategory: myzonetagcat
    zones:
      - zonea
      - zoneb
EOF
```

Дождитесь, когда модуль перейдет в состояние `Ready`. Проверить состояние можно, выполнив следующую команду:

```shell
d8 k get module csi-vsphere -w
```

В результате будет выведена информация о модуле `csi-vsphere`:

```console
NAME         WEIGHT    STATE     SOURCE     STAGE   STATUS
csi-vsphere   910      Enabled   Embedded           Ready
```

## Подготовка окружения

### Список необходимых ресурсов

* **User** с необходимым [набором прав](#создание-и-назначение-роли).
* **Network** с DHCP и доступом в интернет.
* **Datacenter** с соответствующим тегом [`k8s-region`](#создание-тегов-и-категорий-тегов).
* **Cluster** с соответствующим тегом [`k8s-zone`](#создание-тегов-и-категорий-тегов).
* **Datastore** в любом количестве с соответствующими [тегами](#конфигурация-datastore).

### Установка govc

Для дальнейшей конфигурации `csi-vsphere` вам понадобится vSphere CLI — [govc](https://github.com/vmware/govmomi/tree/master/govc#installation).

После установки задайте переменные окружения для работы с vCenter:

```shell
export GOVC_URL=example.com
export GOVC_USERNAME=<username>@vsphere.local
export GOVC_PASSWORD=<password>
export GOVC_INSECURE=1
```

### Создание тегов и категорий тегов

В `csi-vsphere` нет понятий «регион» и «зона». «Регионом» в `csi-vsphere` является `Datacenter`, а «зоной» — `Cluster`. Для создания этой связи используются теги.

Создайте категории тегов с помощью команд:

```shell
govc tags.category.create -d "Kubernetes Region" k8s-region
govc tags.category.create -d "Kubernetes Zone" k8s-zone
```

Создайте теги в каждой категории. Если вы планируете использовать несколько «зон» (`Cluster`), создайте тег для каждой из них:

```shell
govc tags.create -d "Kubernetes Region" -c k8s-region test-region
govc tags.create -d "Kubernetes Zone Test 1" -c k8s-zone test-zone-1
govc tags.create -d "Kubernetes Zone Test 2" -c k8s-zone test-zone-2
```

Назначьте тег «региона» на `Datacenter`:

```shell
govc tags.attach -c k8s-region test-region /<DatacenterName>
```

Назначьте теги «зон» на объекты `Cluster`:

```shell
govc tags.attach -c k8s-zone test-zone-1 /<DatacenterName>/host/<ClusterName1>
govc tags.attach -c k8s-zone test-zone-2 /<DatacenterName>/host/<ClusterName2>
```

### Конфигурация Datastore

{% alert level="warning" %}
Для динамического заказа `PersistentVolume` необходимо, чтобы `Datastore` был доступен на **каждом** хосте ESXi (shared datastore).
{% endalert %}

Для автоматического создания StorageClass в кластере назначьте созданные ранее теги «региона» и «зоны» на объекты `Datastore`:

```shell
govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName1>
govc tags.attach -c k8s-zone test-zone-1 /<DatacenterName>/datastore/<DatastoreName1>

govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName2>
govc tags.attach -c k8s-zone test-zone-2 /<DatacenterName>/datastore/<DatastoreName2>
```

### Создание и назначение роли

{% alert %}
Ввиду разнообразия подключаемых к `csi-vsphere` SSO-провайдеров шаги по созданию пользователя в данной статье не рассматриваются.

Роль, которую предлагается создать далее, включает в себя все возможные права для всех компонентов DKP.
Для получения детального списка привилегий, обратитесь [к документации](/modules/cloud-provider-vsphere/configuration.html#список-необходимых-привилегий).
{% endalert %}

Создайте роль с необходимыми правами:

```shell
govc role.create deckhouse \
   Cns.Searchable Datastore.AllocateSpace Datastore.Browse Datastore.FileManagement \
   Global.GlobalTag Global.SystemTag Network.Assign StorageProfile.View \
   $(govc role.ls Admin | grep -F -e 'Folder.' -e 'InventoryService.' -e 'Resource.' -e 'VirtualMachine.')
```

Назначьте пользователю роль на объекте `vCenter`:

```shell
govc permissions.set -principal <username>@vsphere.local -role deckhouse /
```

## Создание StorageClass

Модуль автоматически создает StorageClass для каждого Datastore и DatastoreCluster из зон.

Также он позволяет настроить имя StorageClass’а, который будет использоваться в кластере по умолчанию (параметр [default](../../../reference/api/global.html#parameters-defaultclusterstorageclass)) и отфильтровать ненужные StorageClass’ы (параметр [exclude](/modules/cloud-provider-vsphere/configuration.html#parameters-storageclass-exclude)).
