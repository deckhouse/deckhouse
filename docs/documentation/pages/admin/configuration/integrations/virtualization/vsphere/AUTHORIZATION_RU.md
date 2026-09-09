---
title: Подключение и авторизация в VMware vSphere
permalink: ru/admin/integrations/virtualization/vsphere/authorization.html
lang: ru
---

## Требования

Для корректной работы Deckhouse Kubernetes Platform с VMware vSphere необходимы:

- Доступ к vCenter;
- Пользователь с необходимым набором привилегий;
- Созданные теги и категории тегов в vSphere;
- Сети с DHCP и доступом в Интернет;
- Доступные shared Datastore на всех используемых ESXi;
- Версия vSphere — `7.x` или `8.x` с поддержкой механизма [`Online volume expansion`](https://github.com/kubernetes-sigs/vsphere-csi-driver/blob/v2.3.0/docs/book/features/volume_expansion.md#vsphere-csi-driver---volume-expansion);
- vCenter, доступный изнутри кластера с master-узлов;
- Созданный Datacenter, в котором должны быть настроены следующие объекты:
  1. VirtualMachine template:
     - Образ виртуальной машины должен использовать `Virtual machines with hardware version 15 or later` — это необходимо для работы online resize.
     - В образе должны быть установлены пакеты `open-vm-tools`, `cloud-init` и [`cloud-init-vmware-guestinfo`](https://github.com/vmware-archive/cloud-init-vmware-guestinfo#installation) — при использовании версии `cloud-init` ниже `21.3`.
  1. Network:
     - Сеть должна быть доступна на всех ESXi, на которых планируется создание виртуальных машин.
  1. Datastore (один или несколько):
     - Datastore должен быть подключен ко всем ESXi, на которых планируется создание виртуальных машин.
     - На Datastore должен быть назначен тег из категории, указанной в параметре [`zoneTagCategory`](/modules/cloud-provider-vsphere/configuration.html#parameters-zonetagcategory) (по умолчанию — `k8s-zone`).  Этот тег определяет зону.
     - Все Cluster в пределах одной зоны должны иметь доступ ко всем Datastore с той же зоной.
  1. Cluster:
     - В Cluster должны быть добавлены все используемые ESXi.
     - На Cluster должен быть назначен тег из категории, указанной в параметре [zoneTagCategory](/modules/cloud-provider-vsphere/configuration.html#parameters-zonetagcategory) (по умолчанию — `k8s-zone`). Этот тег определяет зону.
  1. Folder для создаваемых виртуальных машин:
     - Создавать директорию заранее не нужно, установщик создаёт её по пути из параметра [`vmFolderPath`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-vmfolderpath).
     - Если директория уже существует, задайте [`vmFolderExists: true`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-vmfolderexists), иначе установка завершится ошибкой `The name '<FOLDER>' already exists`.
     - В одной директории нельзя разместить более одного кластера.
  1. Role:
     - Роль должна содержать необходимый [набор привилегий](layout.html#список-необходимых-привилегий).
  1. User:
     - Пользователю должна быть назначена роль, указанная в предыдущем пункте.
- На созданный Datacenter должен быть назначен тег из категории, указанной в параметре [`regionTagCategory`](/modules/cloud-provider-vsphere/configuration.html#parameters-regiontagcategory) (по умолчанию — `k8s-region`). Этот тег определяет регион.

### Требования к образу виртуальной машины

Для создания шаблона виртуальной машины (`Template`) рекомендуется использовать готовый cloud-образ/OVA-файл, предоставляемый вендором ОС:

- [**Ubuntu**](https://cloud-images.ubuntu.com/)
- [**Debian**](https://cloud.debian.org/images/cloud/)
- [**CentOS**](https://cloud.centos.org/)
- [**Rocky Linux**](https://rockylinux.org/alternative-images/) (секция *Generic Cloud / OpenStack*)

{% alert %}
Если вы планируете использовать дистрибутив отечественной ОС, обратитесь к вендору ОС для получения образа/OVA-файла.
{% endalert %}

{% alert level="warning" %}
Провайдер поддерживает работу только с одним диском в шаблоне виртуальной машины. Убедитесь, что шаблон содержит только один диск.
{% endalert %}

### Подготовка образа виртуальной машины

DKP использует `cloud-init` для настройки виртуальной машины после запуска.

{% alert level="warning" %}
Отключите VMware Guest OS Customization (а также любые механизмы vApp/OS customization, если они применимы в вашей схеме) для шаблона и виртуальных машин кластера в vSphere. DKP выполняет первичную настройку узлов через `cloud-init` (datasource VMware GuestInfo). Включенная customization может конфликтовать с `cloud-init` и приводить к некорректной инициализации узла.
{% endalert %}

Чтобы подготовить `cloud-init` и образ ВМ, выполните следующие действия:

1. Установите необходимые пакеты:

   Если используется версия `cloud-init` ниже 21.3 (требуется поддержка VMware GuestInfo):

   ```shell
   sudo apt-get update
   sudo apt-get install -y open-vm-tools cloud-init cloud-init-vmware-guestinfo
   ```

   Если используется версия `cloud-init` 21.3 и выше:

   ```shell
   sudo apt-get update
   sudo apt-get install -y open-vm-tools cloud-init
   ```

1. Проверьте, что в файле `/etc/cloud/cloud.cfg` установлен параметр `disable_vmware_customization: false`.

1. Убедитесь, что в файле `/etc/cloud/cloud.cfg` указан параметр `default_user`. Он необходим для добавления SSH-ключа при запуске ВМ.

1. Добавьте datasource VMware GuestInfo — создайте файл `/etc/cloud/cloud.cfg.d/99-DataSourceVMwareGuestInfo.cfg`:

   ```yaml
   datasource:
     VMware:
       vmware_cust_file_max_wait: 10
   ```

1. Перед созданием шаблона ВМ сбросьте идентификаторы и состояние `cloud-init`, используя следующие команды:

   ```shell
   truncate -s 0 /etc/machine-id &&
   rm /var/lib/dbus/machine-id &&
   ln -s /etc/machine-id /var/lib/dbus/machine-id
   ```

1. Очистите логи событий `cloud-init`:

   ```shell
   cloud-init clean --logs --seed
   ```

{% alert level="warning" %}

После запуска виртуальной машины в ней должны быть запущены следующие службы, связанные с пакетами, установленными при подготовке `cloud-init`:

- `cloud-config.service`,
- `cloud-final.service`,
- `cloud-init.service`.

Чтобы убедиться в том, что службы включены, используйте команду:

```shell
systemctl is-enabled cloud-config.service cloud-init.service cloud-final.service
```

Пример ответа для включенных служб:

```console
enabled
enabled
enabled
```

{% endalert %}

{% alert %}
DKP создаёт диски виртуальных машин с типом `eagerZeroedThick`, но тип дисков созданных ВМ будет изменён без уведомления, согласно настроенным в vSphere `VM Storage Policy`.
Подробнее можно прочитать в [документации](https://techdocs.broadcom.com/us/en/vmware-cis/vsphere/vsphere/8-0/vsphere-single-host-management-vmware-host-client-8-0/virtual-machine-management-with-the-vsphere-host-client-vSphereSingleHostManagementVMwareHostClient/configuring-virtual-machines-in-the-vsphere-host-client-vSphereSingleHostManagementVMwareHostClient/virtual-disk-configuration-vSphereSingleHostManagementVMwareHostClient/about-virtual-disk-provisioning-policies-vSphereSingleHostManagementVMwareHostClient.html).
{% endalert %}

{% alert %}
DKP использует интерфейс `ens192`, как интерфейс по умолчанию для виртуальных машин в vSphere. Поэтому, при использовании статических IP-адресов в [`mainNetwork`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-mainnetwork), вы должны в образе ОС создать интерфейс с именем `ens192`, как интерфейс по умолчанию.
{% endalert %}

### Подготовка шаблона в vSphere Client

После подготовки образа выполните в vSphere Client следующие действия:

1. Импортируйте образ операционной системы. Выберите Datacenter или Cluster, откройте «ACTIONS» → «Deploy OVF Template...» и укажите OVA-файл вендора.

   ![Подготовка шаблона, шаг 1](../../../../images/cloud-provider-vsphere/vm-template-setup/deploy-ovf-template.png)

1. Проверьте параметры полученной виртуальной машины. Строка «Compatibility» в панели «VM Hardware» на вкладке «Summary» содержит версию оборудования. Там же перечислены подключённые устройства. Требуются версия 15 или выше и один диск.

   ![Подготовка шаблона, шаг 2](../../../../images/cloud-provider-vsphere/vm-template-setup/vm-hardware-compatibility.png)

1. Если версия оборудования ниже 15, повысьте её. Выключите виртуальную машину и откройте «ACTIONS» → «Compatibility» → «Upgrade VM Compatibility...». Для включённой машины этот пункт недоступен. Запланировать повышение версии на следующее выключение можно пунктом «Schedule VM Compatibility Upgrade...».

   ![Подготовка шаблона, шаг 3](../../../../images/cloud-provider-vsphere/vm-template-setup/upgrade-vm-compatibility.png)

1. Преобразуйте виртуальную машину в шаблон. Откройте «ACTIONS» → «Template» → «Convert to Template». В параметре `template` можно указать как шаблон, так и выключенную виртуальную машину.

   ![Подготовка шаблона, шаг 4](../../../../images/cloud-provider-vsphere/vm-template-setup/convert-to-template.png)

## Подключение к vCenter

Источник настроек подключения зависит от того, где размещён control plane кластера. Если control plane работает в облаке, адрес vCenter и учётные данные задаются в секции [`provider`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-provider) ресурса VsphereClusterConfiguration. Если control plane работает на виртуальных машинах или bare metal, те же данные задаются параметрами модуля [`host`](/modules/cloud-provider-vsphere/configuration.html#parameters-host), [`username`](/modules/cloud-provider-vsphere/configuration.html#parameters-username) и [`password`](/modules/cloud-provider-vsphere/configuration.html#parameters-password).

Платформа подключается к vCenter по TLS и проверяет сертификат. Как передать цепочку сертификатов центра сертификации, описано в разделе [«Проверка TLS-сертификата vCenter»](#проверка-tls-сертификата-vcenter).

## Проверка TLS-сертификата vCenter

DKP подключается к vCenter по TLS и проверяет его сертификат. Если сертификат vCenter выпущен собственным или корпоративным центром сертификации, передайте цепочку сертификатов этого центра в параметре [`caBundle`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-provider-cabundle). Проверка сертификата при этом остаётся включённой.

Цепочку укажите в формате PEM. Настройка одна и та же, но путь к ней зависит от того, где описано подключение к vCenter:

- при установке кластера подключение описано в секции [`provider`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-provider) ресурса [VsphereClusterConfiguration](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration) рядом с параметром [`provider.server`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-provider-server), поэтому цепочка задаётся в параметре [`provider.caBundle`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-provider-cabundle);
- в уже работающем кластере подключение описано на верхнем уровне настроек модуля [`cloud-provider-vsphere`](/modules/cloud-provider-vsphere/) рядом с параметром [`host`](/modules/cloud-provider-vsphere/configuration.html#parameters-host), поэтому цепочка задаётся в параметре [`caBundle`](/modules/cloud-provider-vsphere/configuration.html#parameters-cabundle).

Пример для устанавливаемого кластера:

```yaml
apiVersion: deckhouse.io/v1
kind: VsphereClusterConfiguration
layout: Standard
provider:
  server: '<SERVER>'
  username: '<USERNAME>'
  password: '<PASSWORD>'
  caBundle: |
    -----BEGIN CERTIFICATE-----
    <CA_CERTIFICATE_CHAIN_IN_PEM_FORMAT>
    -----END CERTIFICATE-----
```

Пример для работающего кластера:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-vsphere
spec:
  version: 2
  enabled: true
  settings:
    host: "<VCENTER_FQDN>"
    username: "<USERNAME@DOMAIN.LOCAL>"
    password: "<PASSWORD>"
    caBundle: |
      -----BEGIN CERTIFICATE-----
      <CA_CERTIFICATE_CHAIN_IN_PEM_FORMAT>
      -----END CERTIFICATE-----
```

Параметр [`insecure: true`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-provider-insecure) полностью отключает проверку сертификата vCenter. В настройках работающего кластера тот же параметр называется [`insecure`](/modules/cloud-provider-vsphere/configuration.html#parameters-insecure). Задайте либо `caBundle`, либо `insecure: true`. Если заданы оба параметра, DKP отклонит конфигурацию.

Для подключения к NSX-T цепочка сертификатов задаётся отдельным параметром [`nsxt.caBundle`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-nsxt-cabundle). В настройках работающего кластера это параметры [`nsxt.caBundle`](/modules/cloud-provider-vsphere/configuration.html#parameters-nsxt-cabundle) и [`nsxt.insecureFlag`](/modules/cloud-provider-vsphere/configuration.html#parameters-nsxt-insecureflag). Задайте либо `nsxt.caBundle`, либо [`nsxt.insecureFlag: true`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-nsxt-insecureflag), иначе DKP отклонит конфигурацию.

{% alert level="warning" %}
Модуль [`csi-vsphere`](/modules/csi-vsphere/) не поддерживает параметр `caBundle` и подключается к vCenter только с проверкой сертификата по системным центрам сертификации либо с параметром [`insecure`](/modules/csi-vsphere/configuration.html#parameters-insecure).
{% endalert %}

## Конфигурация vSphere

### Настройка через vSphere Client

#### Создание тегов и категорий тегов с использованием vSphere Client

В VMware vSphere нет понятий «регион» и «зона». «Регионом» в vSphere является Datacenter, а «зоной» — Cluster. Для создания этой связи используются теги.

1. Откройте vSphere Client и перейдите в «Menu» → «Tags & Custom Attributes» → «Tags».

   ![Создание тегов и категорий тегов, шаг 1](../../../../images/cloud-provider-vsphere/tags-categories-setup/menu-tags-and-custom-attributes.png)

1. Откройте вкладку «Categories» и нажмите «NEW». Создайте категорию для регионов (например `k8s-region`): установите значение «One tag» для параметра «Tags Per Object» и задайте связываемые типы, включая Datacenter.

   ![Создание тегов и категорий тегов, шаг 2](../../../../images/cloud-provider-vsphere/tags-categories-setup/create-category-region.png)

1. Создайте вторую категорию для зон (например, `k8s-zone`) с типами объектов Host, Cluster и Datastore.

   ![Создание тегов и категорий тегов, шаг 3](../../../../images/cloud-provider-vsphere/tags-categories-setup/create-category-zone.png)

1. Перейдите на вкладку «Tags» и создайте минимум по одному тегу в категории региона и в категории зон (например, `test-region`, `test-zone-1`).

   ![Создание тегов и категорий тегов, шаг 4](../../../../images/cloud-provider-vsphere/tags-categories-setup/tags-list.png)

1. Во вкладке «Inventory» выберите целевой Datacenter, перейдите в панель «Summary», откройте «Actions» → «Tags & Custom Attributes» → «Assign Tag» и назначьте тег региона.
   Повторите для каждого Cluster, на котором будут узлы, назначая соответствующие теги зон.

   ![Создание тегов и категорий тегов, шаг 5.1](../../../../images/cloud-provider-vsphere/tags-categories-setup/datacenter-actions-assign-tag.png)
   ![Создание тегов и категорий тегов, шаг 5.2](../../../../images/cloud-provider-vsphere/tags-categories-setup/assign-tag-to-datacenter.png)

#### Настройка Datastore с использованием vSphere Client

{% alert level="warning" %}
Для динамического заказа PersistentVolume необходимо, чтобы Datastore был доступен на **каждом** хосте ESXi в зоне (shared datastore).
{% endalert %}

Во вкладке «Inventory» выберите Datastore, перейдите в панель «Summary», затем откройте меню «Actions» → «Tags & Custom Attributes» → «Assign Tag». Назначьте Datastore тот же тег региона, что и у соответствующего Datacenter, а также тот же тег зоны, что и у соответствующего Cluster.

![Создание тегов и категорий тегов, шаг 6](../../../../images/cloud-provider-vsphere/tags-categories-setup/assign-tags-to-datastore.png)

Чтобы убедиться, что Datastore подключён ко всем хостам ESXi зоны, откройте «Menu» → «Inventory» → «Storage», выберите Datastore и перейдите на вкладку «Hosts». В списке приводятся хосты, которым Datastore доступен.

![Проверка доступности Datastore](../../../../images/cloud-provider-vsphere/datastore-setup/datastore-hosts.png)

#### Создание директории для виртуальных машин с использованием vSphere Client

Установщик создаёт директорию для виртуальных машин по пути из параметра [`vmFolderPath`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-vmfolderpath). Если директория уже существует, задайте [`vmFolderExists: true`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-vmfolderexists).

Чтобы создать директорию заранее, откройте «Menu» → «Inventory» → «Hosts and Clusters», выберите в списке объект Datacenter, затем откройте «ACTIONS» → «New Folder» → «New VM and Template Folder...» и задайте имя.

![Создание директории для виртуальных машин](../../../../images/cloud-provider-vsphere/vm-folder-setup/new-vm-and-template-folder.png)

#### Создание пула ресурсов с использованием vSphere Client

В каждой зоне установщик создаёт пул ресурсов с именем из параметра [`cloud.prefix`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-cloud-prefix) ресурса ClusterConfiguration. Если задан параметр [`baseResourcePool`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-baseresourcepool), пул создаётся внутри указанного в нём пула. Родительский пул должен существовать, установщик его не создаёт. То же относится к пулу из параметра [`resourcePool`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-nodegroups-instanceclass-resourcepool) в описании группы узлов.

Чтобы создать пул, откройте «Menu» → «Inventory» → «Hosts and Clusters», выберите в списке объект Cluster, затем откройте «ACTIONS» → «New Resource Pool...» и задайте имя.

![Создание пула ресурсов](../../../../images/cloud-provider-vsphere/resource-pool-setup/new-resource-pool.png)

Со значением [`useNestedResourcePool: false`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-usenestedresourcepool) установщик не создаёт вложенный пул. Виртуальные машины размещаются в пуле из параметра `resourcePool`. Если этот параметр не задан, машины размещаются в корневом пуле Cluster.

#### Создание и назначение роли с использованием vSphere Client

1. Откройте «Menu» → «Administration». Откроется отдельная страница управления vCenter, на которой в левой панели в разделе «Access Control» находится пункт «Roles».

   ![Создание и назначение роли, шаг 1](../../../../images/cloud-provider-vsphere/role-setup/access-control-roles.png)

1. Нажмите «NEW», введите имя роли (например, `deckhouse`) и добавьте привилегии из [списка](layout.html#список-необходимых-привилегий).

   ![Создание и назначение роли, шаг 2](../../../../images/cloud-provider-vsphere/role-setup/new-role-privileges.png)

1. Назначьте роль для учётной записи Deckhouse, во вкладке «Menu» → «Administration» → «Access Control» → «Global Permissions» нажмите «ADD» и выберите пользователя и роль `deckhouse`.

   ![Создание и назначение роли, шаг 3](../../../../images/cloud-provider-vsphere/role-setup/add-global-permission.png)

### Настройка через govc

#### Установка govc

Для дальнейшей настройки vSphere потребуется CLI-утилита [govc](https://github.com/vmware/govmomi/tree/master/govc#installation).

После установки задайте переменные окружения для работы с vCenter.

{% alert level="warning" %}
Обязательно указывайте имя пользователя вместе с доменом, например: `username@vsphere.local`.
{% endalert %}

```shell
export GOVC_URL=example.com
export GOVC_USERNAME=<USERNAME>@vsphere.local
export GOVC_PASSWORD=<PASSWORD>
export GOVC_INSECURE=1
```

#### Создание тегов и категорий тегов с использованием govc

В VMware vSphere нет понятий «регион» и «зона». «Регионом» в vSphere является Datacenter, а «зоной» — Cluster. Для создания этой связи используются теги.

Создайте категории тегов с помощью команд:

```shell
govc tags.category.create -d "Kubernetes Region" k8s-region
govc tags.category.create -d "Kubernetes Zone" k8s-zone
```

Создайте теги в каждой категории. Если вы планируете использовать несколько «зон» (Cluster), создайте тег для каждой из них:

```shell
govc tags.create -d "Kubernetes Region" -c k8s-region test-region
govc tags.create -d "Kubernetes Zone Test 1" -c k8s-zone test-zone-1
govc tags.create -d "Kubernetes Zone Test 2" -c k8s-zone test-zone-2
```

Назначьте тег «региона» на Datacenter:

```shell
govc tags.attach -c k8s-region test-region /<DATACENTER_NAME>
```

Назначьте теги «зон» на объекты Cluster:

```shell
govc tags.attach -c k8s-zone test-zone-1 /<DATACENTER_NAME>/host/<CLUSTER_NAME_1>
govc tags.attach -c k8s-zone test-zone-2 /<DATACENTER_NAME>/host/<CLUSTER_NAME_2>
```

#### Настройка Datastore с использованием govc

{% alert level="warning" %}
Для динамического заказа PersistentVolume необходимо, чтобы Datastore был доступен на **каждом** хосте ESXi (shared datastore).
{% endalert %}

Для автоматического создания StorageClass в кластере Kubernetes назначьте созданные ранее теги «региона» и «зоны» на объекты Datastore:

```shell
govc tags.attach -c k8s-region test-region /<DATACENTER_NAME>/datastore/<DATASTORE_NAME_1>
govc tags.attach -c k8s-zone test-zone-1 /<DATACENTER_NAME>/datastore/<DATASTORE_NAME_1>

govc tags.attach -c k8s-region test-region /<DATACENTER_NAME>/datastore/<DATASTORE_NAME_2>
govc tags.attach -c k8s-zone test-zone-2 /<DATACENTER_NAME>/datastore/<DATASTORE_NAME_2>
```

#### Создание и назначение роли с использованием govc

{% alert %}
Ввиду разнообразия подключаемых к vSphere SSO-провайдеров шаги по созданию пользователя в данной статье не рассматриваются.

Роль, которую предлагается создать далее, включает в себя привилегии из раздела [«Список необходимых привилегий»](layout.html#список-необходимых-привилегий). При необходимости более гранулярных прав обратитесь в техподдержку Deckhouse.
{% endalert %}

Создайте роль с необходимыми привилегиями:

```shell
govc role.create deckhouse \
   Cns.Searchable \
   Datastore.AllocateSpace Datastore.Browse Datastore.FileManagement \
   Folder.Create Folder.Delete Folder.Move Folder.Rename \
   Global.GlobalTag Global.SystemTag \
   InventoryService.Tagging.AttachTag InventoryService.Tagging.CreateCategory \
   InventoryService.Tagging.CreateTag InventoryService.Tagging.DeleteCategory \
   InventoryService.Tagging.DeleteTag InventoryService.Tagging.EditCategory \
   InventoryService.Tagging.EditTag InventoryService.Tagging.ModifyUsedByForCategory \
   InventoryService.Tagging.ModifyUsedByForTag InventoryService.Tagging.ObjectAttachable \
   Network.Assign \
   Resource.AssignVMToPool Resource.CreatePool Resource.DeletePool Resource.EditPool Resource.RenamePool \
   StorageProfile.View \
   System.Anonymous System.Read System.View \
   VApp.ApplicationConfig VApp.AssignResourcePool VApp.AssignVM VApp.Create VApp.Delete \
   VApp.ExtractOvfEnvironment VApp.Import VApp.InstanceConfig VApp.PowerOff VApp.PowerOn VApp.ResourceConfig \
   VirtualMachine.Config.AddExistingDisk VirtualMachine.Config.AddNewDisk VirtualMachine.Config.AddRemoveDevice \
   VirtualMachine.Config.AdvancedConfig VirtualMachine.Config.Annotation VirtualMachine.Config.CPUCount \
   VirtualMachine.Config.ChangeTracking VirtualMachine.Config.DiskExtend VirtualMachine.Config.DiskLease \
   VirtualMachine.Config.EditDevice VirtualMachine.Config.ManagedBy VirtualMachine.Config.Memory \
   VirtualMachine.Config.QueryUnownedFiles VirtualMachine.Config.RawDevice VirtualMachine.Config.ReloadFromPath \
   VirtualMachine.Config.RemoveDisk VirtualMachine.Config.Rename VirtualMachine.Config.ResetGuestInfo \
   VirtualMachine.Config.Resource VirtualMachine.Config.Settings VirtualMachine.Config.SwapPlacement \
   VirtualMachine.Config.UpgradeVirtualHardware \
   VirtualMachine.GuestOperations.Query \
   VirtualMachine.Interact.AnswerQuestion VirtualMachine.Interact.DeviceConnection \
   VirtualMachine.Interact.GuestControl VirtualMachine.Interact.PowerOff VirtualMachine.Interact.PowerOn \
   VirtualMachine.Interact.Reset VirtualMachine.Interact.SetCDMedia VirtualMachine.Interact.ToolsInstall \
   VirtualMachine.Inventory.Create VirtualMachine.Inventory.CreateFromExisting VirtualMachine.Inventory.Delete \
   VirtualMachine.Inventory.Move \
   VirtualMachine.Provisioning.Clone VirtualMachine.Provisioning.Customize VirtualMachine.Provisioning.DeployTemplate \
   VirtualMachine.Provisioning.GetVmFiles VirtualMachine.Provisioning.PutVmFiles VirtualMachine.Provisioning.ReadCustSpecs \
   VirtualMachine.State.CreateSnapshot VirtualMachine.State.RemoveSnapshot VirtualMachine.State.RenameSnapshot
```

Назначьте пользователю роль на объекте vCenter.

{% alert level="warning" %}
Обязательно указывайте имя пользователя вместе с доменом, например: `username@vsphere.local`.
{% endalert %}

```shell
govc permissions.set -principal <USERNAME>@vsphere.local -role deckhouse /
```

{% alert level="info" %}
Описание привилегий vSphere приведено в [документации VMware](https://techdocs.broadcom.com/us/en/vmware-cis/vsphere/vsphere/8-0/vsphere-security/defined-privileges.html).
{% endalert %}

### Область назначения роли

Роль назначается на корневом объекте vCenter, а не на директории с виртуальными машинами кластера. Компоненты платформы обращаются к объектам за пределами этой директории:

- CSI-драйвер определяет топологию томов по хостам ESXi, связанным с Datastore, поэтому обращается к объектам Cluster и Host;
- компонент обнаружения ресурсов ищет диски CNS во всём vCenter, и для этого ему нужна привилегия `Cns.Searchable` из группы [«Хранилище»](layout.html#хранилище);
- установщик создаёт пул ресурсов в объекте Cluster и директорию в Datacenter, для чего нужны привилегии из группы [«Размещение виртуальных машин»](layout.html#размещение-виртуальных-машин).

Если ограничить роль директорией виртуальных машин, эти операции завершатся ошибкой.

### Проверка привилегий

CSI-драйвер проверяет привилегии учётной записи на каждом Datastore и исключает те, на которых привилегий недостаточно. Такой Datastore не попадает в список доступных, и заказ PersistentVolume через соответствующий StorageClass не проходит.

Если для размеченного тегами Datastore не создаётся рабочий StorageClass, проверьте привилегии учётной записи на этом объекте:

```shell
govc permissions.ls /<DATACENTER_NAME>/datastore/<DATASTORE_NAME>
```

Привилегии учётной записи на объекте можно посмотреть и в vSphere Client. Выберите объект, откройте вкладку «Permissions» и найдите учётную запись в списке. Колонка «Defined In» показывает объект, на котором задано разрешение. Значение «Global Permission» означает, что разрешение назначено глобально.

![Просмотр разрешений на объекте](../../../../images/cloud-provider-vsphere/permissions-check/datastore-permissions.png)
