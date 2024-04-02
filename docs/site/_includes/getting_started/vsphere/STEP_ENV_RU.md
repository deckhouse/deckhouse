{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

## Список необходимых ресурсов vSphere

* **User** с необходимым [набором прав](#создание-и-назначение-роли).
* **Network** с DHCP и доступом в интернет.
* **Datacenter** с соответствующим тегом [`k8s-region`](#создание-тегов-и-категорий-тегов).
* **Cluster** с соответствующим тегом [`k8s-zone`](#создание-тегов-и-категорий-тегов).
* **Datastore** в любом количестве, с соответствующими [тегами](#конфигурация-datastore).
* **Template** — [подготовленный](#подготовка-образа-виртуальной-машины) образ виртуальной машины.

## Конфигурация vSphere

### Установка govc

Для дальнейшей конфигурации vSphere вам понадобится vSphere CLI — [govc](https://github.com/vmware/govmomi/tree/master/govc#installation).

После установки задайте переменные окружения для работы с vCenter:

{% snippetcut %}
```shell
export GOVC_URL=example.com
export GOVC_USERNAME=<username>@vsphere.local
export GOVC_PASSWORD=<password>
export GOVC_INSECURE=1
```
{% endsnippetcut %}

### Создание тегов и категорий тегов

В VMware vSphere нет понятий «регион» и «зона». «Регионом» в vSphere является `Datacenter`, а «зоной» — `Cluster`. Для создания этой связи используются теги.

Создайте категории тегов с помощью команд:

{% snippetcut %}
```shell
govc tags.category.create -d "Kubernetes Region" k8s-region
govc tags.category.create -d "Kubernetes Zone" k8s-zone
```
{% endsnippetcut %}

Создайте теги в каждой категории. Если вы планируете использовать несколько «зон» (`Cluster`), создайте тег для каждой из них:

{% snippetcut %}
```shell
govc tags.create -d "Kubernetes Region" -c k8s-region test-region
govc tags.create -d "Kubernetes Zone Test 1" -c k8s-zone test-zone-1
govc tags.create -d "Kubernetes Zone Test 2" -c k8s-zone test-zone-2
```
{% endsnippetcut %}

Назначьте тег «региона» на `Datacenter`:

{% snippetcut %}
```shell
govc tags.attach -c k8s-region test-region /<DatacenterName>
```
{% endsnippetcut %}

Назначьте теги «зон» на объекты `Cluster`:

{% snippetcut %}
```shell
govc tags.attach -c k8s-zone test-zone-1 /<DatacenterName>/host/<ClusterName1>
govc tags.attach -c k8s-zone test-zone-2 /<DatacenterName>/host/<ClusterName2>
```
{% endsnippetcut %}

#### Конфигурация Datastore

{% alert level="warning" %}
Для динамического заказа `PersistentVolume` необходимо, чтобы `Datastore` был доступен на **каждом** хосте ESXi (shared datastore).
{% endalert %}

Для автоматического создания `StorageClass` в кластере Kubernetes назначьте созданные ранее теги «региона» и «зоны» на объекты `Datastore`:

{% snippetcut %}
```shell
govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName1>
govc tags.attach -c k8s-zone test-zone-1 /<DatacenterName>/datastore/<DatastoreName1>

govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName1>
govc tags.attach -c k8s-zone test-zone-2 /<DatacenterName>/datastore/<DatastoreName2>
```
{% endsnippetcut %}

### Создание и назначение роли

{% alert %}
Ввиду разнообразия подключаемых к vSphere SSO-провайдеров шаги по созданию пользователя в данной статье не рассматриваются.

Роль, которую предлагается создать далее, включает в себя все возможные права для всех компонентов Deckhouse.
Для получения детального списка привилегий, обратитесь [к документации](/documentation/v1/modules/030-cloud-provider-vsphere/configuration.html#список-привилегий-для-использования-модуля).
При необходимости получения более гранулярных прав обратитесь в техподдержку Deckhouse.
{% endalert %}

Создайте роль с необходимыми правами:

{% snippetcut %}
```shell
govc role.create deckhouse \
   Cns.Searchable Datastore.AllocateSpace Datastore.Browse Datastore.FileManagement \
   Global.GlobalTag Global.SystemTag Network.Assign StorageProfile.View \
   $(govc role.ls Admin | grep -F -e 'Folder.' -e 'InventoryService.' -e 'Resource.' -e 'VirtualMachine.')
```
{% endsnippetcut %}

Назначьте пользователю роль на объекте `vCenter`:

{% snippetcut %}
```shell
govc permissions.set -principal <username>@vsphere.local -role deckhouse /
```
{% endsnippetcut %}

### Подготовка образа виртуальной машины

Для создания шаблона виртуальной машины (`Template`) рекомендуется использовать готовый cloud-образ/OVA-файл, предоставляемый вендором ОС:

* [**Ubuntu**](https://cloud-images.ubuntu.com/)
* [**Debian**](https://cloud.debian.org/images/cloud/)
* [**CentOS**](https://cloud.centos.org/)
* [**Rocky Linux**](https://rockylinux.org/alternative-images/) (секция *Generic Cloud / OpenStack*)

{% alert %}
Если вы планируете использовать дистрибутив отечественной ОС, обратитесь к вендору ОС для получения образа/OVA-файла.
{% endalert %}

Если вам необходимо использовать собственный образ, обратитесь к [документации](/documentation/v1/modules/030-cloud-provider-vsphere/environment.html#требования-к-образу-виртуальной-машины).
