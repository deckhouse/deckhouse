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
     - Параметр опционален.
     - По умолчанию используется корневой каталог виртуальных машин.
  1. Role:
     - Роль должна содержать необходимый [набор привилегий](#список-необходимых-привилегий).
  1. User:
     - Пользователю должна быть назначена роль, указанная в предыдущем пункте.
- На созданный Datacenter должен быть назначен тег из категории, указанной в параметре [`regionTagCategory`](/modules/cloud-provider-vsphere/configuration.html#parameters-regiontagcategory) (по умолчанию — `k8s-region`). Этот тег определяет регион.

### Требования к образу виртуальной машины

Для создания шаблона виртуальной машины (Template) рекомендуется использовать готовый cloud-образ/OVA-файл, предоставляемый вендором ОС:

* [**Ubuntu**](https://cloud-images.ubuntu.com/)
* [**Debian**](https://cloud.debian.org/images/cloud/)
* [**CentOS**](https://cloud.centos.org/)
* [**Rocky Linux**](https://rockylinux.org/alternative-images/) (секция *Generic Cloud / OpenStack*)

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

## Конфигурация vSphere

Подготовку инфраструктуры vSphere для DKP можно выполнить через веб-интерфейс **vSphere Client** или CLI-утилиту **govc**. Ниже описана настройка через vSphere Client; альтернативный способ — в разделе [«Настройка через govc»](#настройка-через-govc).

{% alert level="info" %}
Для выполнения описанных действий требуются права администратора vCenter (роль **Administrator**). Подробнее о модели авторизации в vSphere — в [официальной документации VMware](https://techdocs.broadcom.com/us/en/vmware-cis/vsphere/vsphere/8-0/vsphere-security/vsphere-permissions-and-user-management-tasks/understanding-authorization-in-vsphere.html).
{% endalert %}

### Настройка через vSphere Client

#### Модель регионов и зон

В VMware vSphere нет встроенных понятий «регион» и «зона» availability zone, как в публичных облаках. DKP сопоставляет их с объектами инвентаря vSphere через [теги](https://techdocs.broadcom.com/us/en/vmware-cis/vsphere/vsphere/8-0/vcenter-and-host-management/vsphere-tags-and-attributes-host-management/vsphere-tags-host-management.html):

| Понятие DKP | Объект vSphere | Параметр конфигурации | Категория тега (по умолчанию) |
|-------------|----------------|----------------------|-------------------------------|
| Регион | Datacenter | [`region`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-region) | `k8s-region` ([`regionTagCategory`](/modules/cloud-provider-vsphere/configuration.html#parameters-regiontagcategory)) |
| Зона | Cluster (Compute Cluster) | элемент списка [`zones`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-zones) | `k8s-zone` ([`zoneTagCategory`](/modules/cloud-provider-vsphere/configuration.html#parameters-zonetagcategory)) |

Имена тегов, которые вы создаёте в vSphere (например, `moscow` и `zone-a`), должны совпадать со значениями `region` и `zones` в конфигурации кластера DKP.

#### Подготовка инвентаря vSphere

Перед созданием тегов и ролей убедитесь, что в целевом Datacenter подготовлены объекты инвентаря:

1. **Datacenter** — контейнер для всех ресурсов кластера DKP.
1. **Cluster** — один или несколько Compute Cluster с подключёнными хостами ESXi. Каждый Cluster, на котором будут размещаться узлы, станет отдельной «зоной».
1. **Сети (Networks)** — distributed port group или standard switch port group, доступные на всех ESXi в целевых Cluster. Сеть должна обеспечивать DHCP и доступ в интернет для узлов кластера.
1. **Datastore** — shared datastore, подключённый ко **всем** ESXi-хостам в зоне. Необходим как для root-дисков ВМ, так и для динамического заказа PVC (подробнее — в разделе [«Хранилище»](storage.html)).
1. **Папка для виртуальных машин** — каталог в дереве **Hosts and Clusters** → **VMs and Templates**, в котором DKP будет создавать ВМ кластера (параметр [`vmFolderPath`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-vmfolderpath)).
1. **Шаблон виртуальной машины (Template)** — подготовленный образ ОС с `cloud-init` (см. [«Требования к образу виртуальной машины»](#требования-к-образу-виртуальной-машины)).

{% alert %}
В одну папку (`vmFolderPath`) можно разместить только один кластер DKP. Для каждого нового кластера создайте отдельную папку.
{% endalert %}

##### Создание папки для виртуальных машин кластера

1. В vSphere Client перейдите в **Menu** → **Hosts and Clusters**.
1. Выберите Datacenter → правый клик на папке **VMs and Templates** (или на существующей папке внутри неё).
1. Выберите **New Folder** → **New Virtual Machine and Template Folder**.
1. Укажите имя папки (например, `k8s-prod`). Этот путь будет использоваться в параметре `vmFolderPath` (например, `k8s-prod` или `parent/k8s-prod`).

##### Проверка доступности Datastore

1. Перейдите в **Menu** → **Storage**.
1. Выберите целевой Datastore.
1. На вкладке **Hosts** убедитесь, что Datastore подключён ко всем ESXi-хостам Cluster, в котором планируется размещение узлов.
1. На вкладке **Summary** проверьте наличие свободного места.

Если Datastore не отображается на одном из хостов зоны, его нельзя использовать для динамического заказа PVC в этой зоне.

#### Создание категорий тегов {#создание-тегов-и-категорий-тегов-с-использованием-vsphere-client}

[Категория тегов](https://techdocs.broadcom.com/us/en/vmware-cis/vsphere/vsphere/8-0/vcenter-and-host-management/vsphere-tags-and-attributes-host-management/vsphere-tags-host-management.html) определяет, к каким типам объектов можно применять теги и сколько тегов из категории допустимо на одном объекте.

1. Откройте vSphere Client и перейдите в **Menu** → **Tags & Custom Attributes** → **Tags**.

   ![Создание тегов и категорий тегов, шаг 1](/modules/cloud-provider-vsphere/images/tags-categories-setup/Screenshot-1.png)

1. На вкладке **Categories** нажмите **NEW**.

   ![Создание тегов и категорий тегов, шаг 2](/modules/cloud-provider-vsphere/images/tags-categories-setup/Screenshot-2.png)

1. Создайте категорию для регионов. Заполните поля:

   | Поле | Значение для категории региона |
   |------|-------------------------------|
   | **Category Name** | `k8s-region` (или имя из параметра `regionTagCategory`) |
   | **Description** | Kubernetes Region |
   | **Tags Per Object** | **One Tag** — на объект можно назначить только один тег из категории |
   | **Associable Object Types** | **Datacenter** |

   Нажмите **Create**.

1. Снова нажмите **NEW** и создайте категорию для зон:

   | Поле | Значение для категории зоны |
   |------|----------------------------|
   | **Category Name** | `k8s-zone` (или имя из параметра `zoneTagCategory`) |
   | **Description** | Kubernetes Zone |
   | **Tags Per Object** | **One Tag** |
   | **Associable Object Types** | **Cluster Compute Resource**, **Host**, **Datastore** |

   ![Создание тегов и категорий тегов, шаг 3](/modules/cloud-provider-vsphere/images/tags-categories-setup/Screenshot-3.png)

   Нажмите **Create**.

{% alert level="info" %}
Согласно [документации vSphere](https://techdocs.broadcom.com/us/en/vmware-cis/vsphere/vsphere/8-0/vcenter-and-host-management/vsphere-tags-and-attributes-host-management/vsphere-tags-host-management.html), после создания категории с параметром **One Tag** изменить его на **Many Tags** можно, но обратное изменение невозможно. Если изначально выбран **All Objects**, ограничить типы объектов позже нельзя — задайте нужные типы сразу при создании.
{% endalert %}

#### Создание тегов

1. На вкладке **Tags** (в том же разделе **Tags & Custom Attributes**) нажмите **NEW**.
1. Создайте тег региона:
   - **Name** — имя региона, которое будет указано в параметре `region` конфигурации DKP (например, `moscow`);
   - **Description** — произвольное описание;
   - **Category** — `k8s-region`.
1. Создайте теги зон — по одному на каждый Cluster, где будут размещаться узлы (например, `zone-a`, `zone-b`). Категория — `k8s-zone`.

   ![Создание тегов и категорий тегов, шаг 4](/modules/cloud-provider-vsphere/images/tags-categories-setup/Screenshot-4.png)

Имена тегов должны совпадать со значениями `region` и `zones` в [`VsphereClusterConfiguration`](/modules/cloud-provider-vsphere/cluster_configuration.html).

#### Назначение тегов на объекты инвентаря

[Назначение тега](https://techdocs.broadcom.com/us/en/vmware-cis/vsphere/vsphere/8-0/assign-or-remove-a-tag.html) выполняется через контекстное меню объекта в дереве инвентаря:

1. В **Menu** → **Hosts and Clusters** выберите объект.
1. Нажмите **Actions** → **Tags and Custom Attributes** → **Assign Tag**.
1. Выберите тег из списка и нажмите **Assign**.

Назначьте теги на следующие объекты:

| Объект | Тег региона | Тег зоны |
|--------|-------------|----------|
| Datacenter | Да | — |
| Cluster (зона) | — | Да |
| Datastore (в зоне) | Да | Да (тот же, что у Cluster) |

![Создание тегов и категорий тегов, шаг 5.1](/modules/cloud-provider-vsphere/images/tags-categories-setup/Screenshot-5-1.png)
![Создание тегов и категорий тегов, шаг 5.2](/modules/cloud-provider-vsphere/images/tags-categories-setup/Screenshot-5-2.png)

Пример для двух зон:

- Datacenter `DC1` → тег `moscow` (категория `k8s-region`);
- Cluster `Cluster-A` → тег `zone-a` (категория `k8s-zone`);
- Cluster `Cluster-B` → тег `zone-b` (категория `k8s-zone`);
- Datastore `lun-01` (доступен в `Cluster-A`) → теги `moscow` + `zone-a`;
- Datastore `lun-02` (доступен в `Cluster-B`) → теги `moscow` + `zone-b`.

{% alert level="warning" %}
Все Cluster в пределах одной зоны должны иметь доступ ко всем Datastore с тегом этой зоны. Datastore без тега зоны не будет обнаружен компонентом `cloud-data-discoverer` и не появится как StorageClass в кластере.
{% endalert %}

#### Настройка Datastore {#настройка-datastore-с-использованием-vsphere-client}

{% alert level="warning" %}
Для динамического заказа PersistentVolume Datastore должен быть доступен на **каждом** ESXi-хосте в зоне (shared datastore).
{% endalert %}

1. Перейдите в **Menu** → **Storage**.
1. Выберите Datastore.
1. Убедитесь на вкладке **Hosts**, что Datastore подключён ко всем хостам целевого Cluster.
1. Назначьте теги региона и зоны: **Actions** → **Tags and Custom Attributes** → **Assign Tag**.

   ![Создание тегов и категорий тегов, шаг 6](/modules/cloud-provider-vsphere/images/tags-categories-setup/Screenshot-6.png)

Подробнее о хранилище в кластере — в разделе [«Хранилище и балансировка нагрузки»](storage.html).

#### Создание пользовательской роли {#создание-и-назначение-роли-с-использованием-vsphere-client}

DKP требует [набор привилегий](#список-необходимых-привилегий), которых нет в стандартных ролях vSphere. Создайте пользовательскую роль по [инструкции VMware](https://techdocs.broadcom.com/us/en/vmware-cis/vsphere/vsphere/8-0/vsphere-security/vsphere-permissions-and-user-management-tasks/using-roles-to-assign-privileges.html):

1. Перейдите в **Menu** → **Administration** → **Access Control** → **Roles**.

   ![Создание и назначение роли, шаг 1](/modules/cloud-provider-vsphere/images/role-setup/Screenshot-1.png)

1. Нажмите **NEW**.
1. Введите имя роли (например, `deckhouse`).
1. В списке категорий привилегий отметьте необходимые привилегии из [таблицы](#список-необходимых-привилегий). Для удобства используйте фильтры **Show selected** / **Show all** в диалоге создания роли.

   Основные категории привилегий для DKP:

   | Категория в UI | Что включить |
   |----------------|--------------|
   | **Cns** | Searchable |
   | **Datastore** | Allocate space, Browse datastore, Low level file operations |
   | **Folder** | Create folder, Delete folder, Move folder, Rename folder |
   | **Global** | Global tag, System tag |
   | **vSphere Tagging** | Все перечисленные в таблице привилегии |
   | **Network** | Assign network |
   | **Resource** | Все операции с resource pool |
   | **VM Storage Policies** | View VM storage policies |
   | **vApp** | Все перечисленные в таблице привилегии |
   | **Virtual Machine** | Change Configuration, Edit Inventory, Guest Operations, Interaction, Provisioning, Snapshot Management — все перечисленные в таблице |

   ![Создание и назначение роли, шаг 2](/modules/cloud-provider-vsphere/images/role-setup/Screenshot-2.png)

1. Нажмите **Create**.

{% alert level="info" %}
Альтернативный способ — клонировать существующую роль (**Clone**), затем отредактировать (**Edit**) и добавить недостающие привилегии. Просмотреть привилегии любой роли можно на вкладке **Privileges** в разделе **Roles**.
{% endalert %}

#### Назначение прав учётной записи DKP

Права назначаются парой «пользователь/группа + роль» на объект инвентаря ([модель авторизации vSphere](https://techdocs.broadcom.com/us/en/vmware-cis/vsphere/vsphere/8-0/vsphere-security/vsphere-permissions-and-user-management-tasks/understanding-authorization-in-vsphere.html)). Для DKP рекомендуется начать с **Global Permissions** — это самый простой вариант для первичной настройки.

##### Global Permissions (рекомендуется для начальной настройки)

1. Перейдите в **Menu** → **Administration** → **Access Control** → **Global Permissions**.
1. Нажмите **ADD**.
1. В диалоге **Add Permission**:
   - **Domain** — выберите домен SSO (например, `vsphere.local`);
   - **User/Group** — найдите и выберите учётную запись DKP (например, `deckhouse@vsphere.local`);
   - **Role** — выберите созданную роль `deckhouse`;
   - **Propagate to children** — оставьте включённым, чтобы права распространялись на все объекты инвентаря.
1. Нажмите **OK**.

   ![Создание и назначение роли, шаг 3](/modules/cloud-provider-vsphere/images/role-setup/Screenshot-3.png)

{% alert level="warning" %}
Указывайте имя пользователя вместе с доменом SSO, например `deckhouse@vsphere.local`. Формат зависит от настроенного identity source (Active Directory, LDAP, `vsphere.local`).
{% endalert %}

##### Права на уровне объектов (альтернатива)

Вместо Global Permissions можно назначить роль `deckhouse` на конкретные объекты инвентаря с включённым **Propagate to children**. Это снижает уровень привилегий учётной записи — подробнее в разделе [«Гранулярная модель прав»](#гранулярная-модель-прав).

Для назначения прав на объект:

1. В дереве инвентаря выберите объект (Datacenter, Cluster, папку ВМ, Datastore).
1. Перейдите на вкладку **Permissions**.
1. Нажмите **Add** → выберите пользователя, роль и при необходимости включите **Propagate to children**.

{% alert %}
Шаги по созданию пользователя в SSO-провайдере vSphere (Active Directory, LDAP, `vsphere.local`) зависят от вашей инфраструктуры и в данной статье не рассматриваются. Пользователь должен быть создан в identity source, подключённом к vCenter Single Sign-On, до назначения прав.
{% endalert %}

#### Проверка конфигурации

После завершения настройки убедитесь, что:

- [ ] Datacenter имеет тег региона;
- [ ] каждый целевой Cluster имеет тег зоны;
- [ ] shared Datastore имеют теги региона и зоны;
- [ ] папка для ВМ кластера создана;
- [ ] шаблон ВМ подготовлен и содержит один диск;
- [ ] учётная записи DKP назначена роль с [необходимыми привилегиями](#список-необходимых-привилегий);
- [ ] vCenter доступен из сети, где будут размещаться master-узлы кластера.

### Настройка через govc

#### Установка govc

Для дальнейшей настройки vSphere потребуется CLI-утилита [govc](https://github.com/vmware/govmomi/tree/master/govc#installation).

После установки задайте переменные окружения для работы с vCenter.

{% alert level="warning" %}
Обязательно указывайте имя пользователя вместе с доменом, например: `username@vsphere.local`.
{% endalert %}

```shell
export GOVC_URL=example.com
export GOVC_USERNAME=<username>@vsphere.local
export GOVC_PASSWORD=<password>
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
govc tags.attach -c k8s-region test-region /<DatacenterName>
```

Назначьте теги «зон» на объекты Cluster:

```shell
govc tags.attach -c k8s-zone test-zone-1 /<DatacenterName>/host/<ClusterName1>
govc tags.attach -c k8s-zone test-zone-2 /<DatacenterName>/host/<ClusterName2>
```

#### Настройка Datastore с использованием govc

{% alert level="warning" %}
Для динамического заказа PersistentVolume необходимо, чтобы Datastore был доступен на **каждом** хосте ESXi (shared datastore).
{% endalert %}

Для автоматического создания StorageClass в кластере Kubernetes назначьте созданные ранее теги «региона» и «зоны» на объекты Datastore:

```shell
govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName1>
govc tags.attach -c k8s-zone test-zone-1 /<DatacenterName>/datastore/<DatastoreName1>

govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName2>
govc tags.attach -c k8s-zone test-zone-2 /<DatacenterName>/datastore/<DatastoreName2>
```

#### Создание и назначение роли с использованием govc

{% alert %}
Ввиду разнообразия подключаемых к vSphere SSO-провайдеров шаги по созданию пользователя в данной статье не рассматриваются.

Роль, которую предлагается создать далее, включает в себя привилегии из раздела [«Список необходимых привилегий»](#список-необходимых-привилегий). При необходимости более гранулярных прав см. раздел [«Гранулярная модель прав»](#гранулярная-модель-прав).
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
govc permissions.set -principal <username>@vsphere.local -role deckhouse /
```

{% alert level="info" %}
Для более детальной настройки прав обратитесь к [официальной документации](https://pkg.go.dev/github.com/vmware/govmomi).
{% endalert %}

## Список необходимых привилегий

Детальный список привилегий, необходимых для работы Deckhouse Kubernetes Platform в vSphere:

<table>
  <thead>
    <tr>
      <th>Категория привилегий в UI</th>
      <th>Список привилегий в UI</th>
      <th>Список привилегий в API</th>
      <th>Назначение привилегий в Deckhouse</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td>—</td>
      <td>— (назначаются по умолчанию при создании роли)</td>
      <td>
        <code>System.Anonymous</code><br/>
        <code>System.Read</code><br/>
        <code>System.View</code>
      </td>
      <td>Базовый доступ к объектам vSphere Inventory, необходимый для работы всех компонентов интеграции Deckhouse с vSphere.</td>
    </tr>
    <tr>
      <td>Cns</td>
      <td>Searchable</td>
      <td><code>Cns.Searchable</code></td>
      <td>Поиск и сопоставление объектов Container Native Storage при работе CSI-драйвера с томами Kubernetes.</td>
    </tr>
    <tr>
      <td>Datastore</td>
      <td>
        Allocate space,<br/>
        Browse datastore,<br/>
        Low level file operations
      </td>
      <td>
        <code>Datastore.AllocateSpace</code><br/>
        <code>Datastore.Browse</code><br/>
        <code>Datastore.FileManagement</code>
      </td>
      <td>Выделение дисков при создании виртуальных машин и заказе <code>PersistentVolumes</code> в кластере.</td>
    </tr>
    <tr>
      <td>Folder</td>
      <td>
        Create folder,<br/>
        Delete folder,<br/>
        Move folder,<br/>
        Rename folder
      </td>
      <td>
        <code>Folder.Create</code><br/>
        <code>Folder.Delete</code><br/>
        <code>Folder.Move</code><br/>
        <code>Folder.Rename</code>
      </td>
      <td>Группировка кластера Deckhouse Kubernetes Platform в одном <code>Folder</code> в vSphere Inventory.</td>
    </tr>
    <tr>
      <td>Global</td>
      <td>
        Global tag,<br/>
        System tag
      </td>
      <td>
        <code>Global.GlobalTag</code><br/>
        <code>Global.SystemTag</code>
      </td>
      <td>Доступ к глобальным и системным тегам, используемым Deckhouse Kubernetes Platform при работе с объектами vSphere.</td>
    </tr>
    <tr>
      <td>vSphere Tagging</td>
      <td>
        Assign or Unassign vSphere Tag,<br/>
        Assign or Unassign vSphere Tag on Object,<br/>
        Create vSphere Tag,<br/>
        Create vSphere Tag Category,<br/>
        Delete vSphere Tag,<br/>
        Delete vSphere Tag Category,<br/>
        Edit vSphere Tag,<br/>
        Edit vSphere Tag Category,<br/>
        Modify UsedBy Field for Category,<br/>
        Modify UsedBy Field for Tag
      </td>
      <td>
        <code>InventoryService.Tagging.AttachTag</code><br/>
        <code>InventoryService.Tagging.ObjectAttachable</code><br/>
        <code>InventoryService.Tagging.CreateTag</code><br/>
        <code>InventoryService.Tagging.CreateCategory</code><br/>
        <code>InventoryService.Tagging.DeleteTag</code><br/>
        <code>InventoryService.Tagging.DeleteCategory</code><br/>
        <code>InventoryService.Tagging.EditTag</code><br/>
        <code>InventoryService.Tagging.EditCategory</code><br/>
        <code>InventoryService.Tagging.ModifyUsedByForCategory</code><br/>
        <code>InventoryService.Tagging.ModifyUsedByForTag</code>
      </td>
      <td>Deckhouse Kubernetes Platform использует теги для определения доступных ему объектов <code>Datacenter</code>, <code>Cluster</code> и <code>Datastore</code>, а также для определения виртуальных машин, находящихся под его управлением.</td>
    </tr>
    <tr>
      <td>Network</td>
      <td>Assign network</td>
      <td><code>Network.Assign</code></td>
      <td>Подключение сетей и port group к виртуальным машинам кластера Deckhouse Kubernetes Platform.</td>
    </tr>
    <tr>
      <td>Resource</td>
      <td>
        Assign virtual machine to resource pool,<br/>
        Create resource pool,<br/>
        Modify resource pool,<br/>
        Remove resource pool,<br/>
        Rename resource pool
      </td>
      <td>
        <code>Resource.AssignVMToPool</code><br/>
        <code>Resource.CreatePool</code><br/>
        <code>Resource.DeletePool</code><br/>
        <code>Resource.EditPool</code><br/>
        <code>Resource.RenamePool</code>
      </td>
      <td>Размещение виртуальных машин кластера Deckhouse Kubernetes Platform в целевом пуле ресурсов и управление этим пулом.</td>
    </tr>
    <tr>
      <td>VM Storage Policies (<em>Profile-driven Storage Privileges</em> в vSphere 7)</td>
      <td>View VM storage policies (<em>Profile-driven storage view</em> в vSphere 7)</td>
      <td><code>StorageProfile.View</code></td>
      <td>Просмотр политик хранения, используемых при создании виртуальных машин и динамическом заказе томов в кластере.</td>
    </tr>
    <tr>
      <td>vApp</td>
      <td>
        Add virtual machine,<br/>
        Assign resource pool,<br/>
        Create,<br/>
        Delete,<br/>
        Import,<br/>
        Power Off,<br/>
        Power On,<br/>
        View OVF Environment,<br/>
        vApp application configuration,<br/>
        vApp instance configuration,<br/>
        vApp resource configuration
      </td>
      <td>
        <code>VApp.ApplicationConfig</code><br/>
        <code>VApp.AssignResourcePool</code><br/>
        <code>VApp.AssignVM</code><br/>
        <code>VApp.Create</code><br/>
        <code>VApp.Delete</code><br/>
        <code>VApp.ExtractOvfEnvironment</code><br/>
        <code>VApp.Import</code><br/>
        <code>VApp.InstanceConfig</code><br/>
        <code>VApp.PowerOff</code><br/>
        <code>VApp.PowerOn</code><br/>
        <code>VApp.ResourceConfig</code>
      </td>
      <td>Управление операциями, связанными с развертыванием и конфигурацией vApp и OVF-шаблонов, используемых при создании виртуальных машин.</td>
    </tr>
    <tr>
      <td>Virtual Machine > Change Configuration</td>
      <td>
        Add existing disk,<br/>
        Add new disk,<br/>
        Add or remove device,<br/>
        Advanced configuration,<br/>
        Set annotation,<br/>
        Change CPU count,<br/>
        Toggle disk change tracking,<br/>
        Extend virtual disk,<br/>
        Acquire disk lease,<br/>
        Modify device settings,<br/>
        Configure managedBy,<br/>
        Change Memory,<br/>
        Query unowned files,<br/>
        Configure Raw device,<br/>
        Reload from path,<br/>
        Remove disk,<br/>
        Rename,<br/>
        Reset guest information,<br/>
        Change resource,<br/>
        Change Settings,<br/>
        Change Swapfile placement,<br/>
        Upgrade virtual machine compatibility
      </td>
      <td>
        <code>VirtualMachine.Config.AddExistingDisk</code><br/>
        <code>VirtualMachine.Config.AddNewDisk</code><br/>
        <code>VirtualMachine.Config.AddRemoveDevice</code><br/>
        <code>VirtualMachine.Config.AdvancedConfig</code><br/>
        <code>VirtualMachine.Config.Annotation</code><br/>
        <code>VirtualMachine.Config.CPUCount</code><br/>
        <code>VirtualMachine.Config.ChangeTracking</code><br/>
        <code>VirtualMachine.Config.DiskExtend</code><br/>
        <code>VirtualMachine.Config.DiskLease</code><br/>
        <code>VirtualMachine.Config.EditDevice</code><br/>
        <code>VirtualMachine.Config.ManagedBy</code><br/>
        <code>VirtualMachine.Config.Memory</code><br/>
        <code>VirtualMachine.Config.QueryUnownedFiles</code><br/>
        <code>VirtualMachine.Config.RawDevice</code><br/>
        <code>VirtualMachine.Config.ReloadFromPath</code><br/>
        <code>VirtualMachine.Config.RemoveDisk</code><br/>
        <code>VirtualMachine.Config.Rename</code><br/>
        <code>VirtualMachine.Config.ResetGuestInfo</code><br/>
        <code>VirtualMachine.Config.Resource</code><br/>
        <code>VirtualMachine.Config.Settings</code><br/>
        <code>VirtualMachine.Config.SwapPlacement</code><br/>
        <code>VirtualMachine.Config.UpgradeVirtualHardware</code>
      </td>
      <td>Управление жизненным циклом виртуальных машин кластера Deckhouse Kubernetes Platform.</td>
    </tr>
    <tr>
      <td>Virtual Machine > Edit Inventory</td>
      <td>
        Create new,<br/>
        Create from existing,<br/>
        Remove,<br/>
        Move
      </td>
      <td>
        <code>VirtualMachine.Inventory.Create</code><br/>
        <code>VirtualMachine.Inventory.CreateFromExisting</code><br/>
        <code>VirtualMachine.Inventory.Delete</code><br/>
        <code>VirtualMachine.Inventory.Move</code>
      </td>
      <td>Создание, удаление и перемещение виртуальных машин кластера Deckhouse Kubernetes Platform в инвентаре vSphere.</td>
    </tr>
    <tr>
      <td>Virtual Machine > Guest Operations</td>
      <td>Guest Operation Queries</td>
      <td><code>VirtualMachine.GuestOperations.Query</code></td>
      <td>Получение информации из гостевой операционной системы виртуальных машин.</td>
    </tr>
    <tr>
      <td>Virtual Machine > Interaction</td>
      <td>
        Answer question,<br/>
        Device connection,<br/>
        Guest operating system management by VIX API,<br/>
        Power Off,<br/>
        Power On,<br/>
        Reset,<br/>
        Configure CD media,<br/>
        Install VMware Tools
      </td>
      <td>
        <code>VirtualMachine.Interact.AnswerQuestion</code><br/>
        <code>VirtualMachine.Interact.DeviceConnection</code><br/>
        <code>VirtualMachine.Interact.GuestControl</code><br/>
        <code>VirtualMachine.Interact.PowerOff</code><br/>
        <code>VirtualMachine.Interact.PowerOn</code><br/>
        <code>VirtualMachine.Interact.Reset</code><br/>
        <code>VirtualMachine.Interact.SetCDMedia</code><br/>
        <code>VirtualMachine.Interact.ToolsInstall</code>
      </td>
      <td>Управление состоянием виртуальных машин, подключением устройств и взаимодействием с гостевой операционной системой.</td>
    </tr>
    <tr>
      <td>Virtual Machine > Provisioning</td>
      <td>
        Clone virtual machine,<br/>
        Customize guest,<br/>
        Deploy template,<br/>
        Allow virtual machine download,<br/>
        Allow virtual machine files upload,<br/>
        Read customization specifications
      </td>
      <td>
        <code>VirtualMachine.Provisioning.Clone</code><br/>
        <code>VirtualMachine.Provisioning.Customize</code><br/>
        <code>VirtualMachine.Provisioning.DeployTemplate</code><br/>
        <code>VirtualMachine.Provisioning.GetVmFiles</code><br/>
        <code>VirtualMachine.Provisioning.PutVmFiles</code><br/>
        <code>VirtualMachine.Provisioning.ReadCustSpecs</code>
      </td>
      <td>Клонирование шаблонов виртуальных машин, их настройка и развертывание при создании узлов кластера Deckhouse Kubernetes Platform.</td>
    </tr>
    <tr>
      <td>Virtual Machine > Snapshot Management</td>
      <td>
        Create snapshot,<br/>
        Remove Snapshot,<br/>
        Rename Snapshot
      </td>
      <td>
        <code>VirtualMachine.State.CreateSnapshot</code><br/>
        <code>VirtualMachine.State.RemoveSnapshot</code><br/>
        <code>VirtualMachine.State.RenameSnapshot</code>
      </td>
      <td>Управление снимками виртуальных машин и томов в сценариях, где эта функциональность используется компонентами платформы.</td>
    </tr>
  </tbody>
</table>

## Гранулярная модель прав

Вместо назначения роли `deckhouse` через **Global Permissions** на весь vCenter можно ограничить область действия прав, назначив роли на конкретные объекты инвентаря. Это снижает уровень привилегий учётной записи DKP.

Рекомендуемая схема назначения:

| Объект vSphere | Уровень доступа | Наследование | Назначение |
|----------------|-----------------|--------------|------------|
| vCenter (корень) | Read-only | Нет | Обзор инвентаря, работа с тегами |
| Datacenter | Read-only | Нет | Доступ к объектам датацентра |
| Cluster (зона) | Полный доступ (роль `deckhouse`) | Да | Создание resource pool, размещение ВМ |
| Папка ВМ ([`vmFolderPath`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-vmfolderpath)) | Полный доступ (роль `deckhouse`) | Да | Создание, удаление и управление ВМ кластера |
| Папка Datastore (тома CNS) | Полный доступ (роль `deckhouse`) | Да | Заказ PersistentVolume через CSI |
| Distributed Switch | Read-only | Нет | Просмотр сетевой инфраструктуры |
| Distributed Port Group | Полный доступ | Да | Подключение сетей к ВМ |

{% alert level="warning" %}
Гранулярная модель прав требует тщательной настройки и тестирования. При возникновении ошибок доступа в логах компонентов `cloud-controller-manager`, `cloud-data-discoverer` или CSI-драйвера проверьте, что учётная запись имеет необходимые привилегии на всех затронутых объектах.
{% endalert %}

{% alert %}
При использовании гранулярной модели роль с полным набором привилегий из [таблицы выше](#список-необходимых-привилегий) назначается только на объекты, где DKP выполняет операции записи (Cluster, папка ВМ, папка Datastore, port group). На остальных объектах достаточно роли Read-only с привилегиями для работы с тегами (`InventoryService.Tagging.*`, `StorageProfile.View`, `System.*`).
{% endalert %}
