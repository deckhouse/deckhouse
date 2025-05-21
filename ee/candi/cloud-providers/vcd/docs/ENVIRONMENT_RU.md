---
title: "Cloud provider — VMware Cloud Director: подготовка окружения."
description: "Подготовка окружения VMware Cloud Director для работы Deckhouse cloud provider."
---

<!-- AUTHOR! Don't forget to update getting started if necessary -->

## Список необходимых ресурсов VCD

* _Organization_
* _VirtualDataCenter_
* _vApp_
* _StoragePolicy_
* _SizingPolicy_
* _Network_
* _EdgeRouter_
* _Catalog_

Organization, VirtualDataCenter, StoragePolicy, SizingPolicy, EdgeRouter и Catalog должны быть предоставлены вашим поставщиком услуг VMware Cloud Director.

Network (внутренняя сеть) может быть настроена вашим поставщиком услуг VMware Cloud Director, либо вы можете настроить ее самостоятельно. Далее рассматривается настройка внутренней сети самостоятельно.

### Права пользователя

Пользователь под которым будет осуществляться доступ к API VMware Cloud Director должен иметь права:

* Роль _Organization Administrator_ с дополнительным правилом `Preserve All ExtraConfig Elements During OVF Import and Export`.
* Правило `Preserve All ExtraConfig Elements During OVF Import and Export` должно быть продублировано в используемом _Right Bundle_ пользователя.

### Добавление сети

Перейдите во вкладку _Networking_ и нажмите на кнопку _NEW_:

![Добавление сети, шаг 1](../../images/cloud-provider-vcd/network-setup/Screenshot.png)

Выберите необходимый Data Center:

![Добавление сети, шаг 2](../../images/cloud-provider-vcd/network-setup/Screenshot2.png)

_Network type_ должен быть _Routed_:

![Добавление сети, шаг 3](../../images/cloud-provider-vcd/network-setup/Screenshot3.png)

Присоедините _EdgeRouter_ к сети:

![Добавление сети, шаг 4](../../images/cloud-provider-vcd/network-setup/Screenshot4.png)

Установите имя сети и CIDR:

![Добавление сети, шаг 5](../../images/cloud-provider-vcd/network-setup/Screenshot5.png)

Не добавляйте Static IP Pools, т.к. будет использоваться DHCP:

![Добавление сети, шаг 6](../../images/cloud-provider-vcd/network-setup/Screenshot6.png)

Укажите адреса DNS-серверов:

![Добавление сети, шаг 7](../../images/cloud-provider-vcd/network-setup/Screenshot7.png)

### Настройка DHCP

Для динамического заказа узлов необходимо включить DHCP-сервер для внутренней сети.

{% alert level="info" %}
Рекомендуем выделить начало диапазона адресов сети на системные нагрузки (control plane, frontend-узлы, системные узлы), а остальное выделить на DHCP-пул. Например, для сети по маске `/24` будет достаточно выделения 20 адресов под системные нагрузки.
{% endalert %}

Перейдите во вкладку _Networking_ и откройте созданную сеть:

![DHCP, шаг 1](../../images/cloud-provider-vcd/dhcp-setup/Screenshot.png)

В открытом окне откройте вкладку _IP Management_ -> _DHCP_ -> Activate:

![DHCP, шаг 2](../../images/cloud-provider-vcd/dhcp-setup/Screenshot2.png)

Во вкладке _General settings_ настройте параметры аналогично примеру:

![DHCP, шаг 3](../../images/cloud-provider-vcd/dhcp-setup/Screenshot3.png)

Добавьте пул:

![DHCP, шаг 3](../../images/cloud-provider-vcd/dhcp-setup/Screenshot4.png)

Установите адреса DNS-серверов:

![DHCP, шаг 3](../../images/cloud-provider-vcd/dhcp-setup/Screenshot5.png)

### Добавление vApp

Перейдите во вкладку _Data Centers_ -> _vApps_ -> _NEW_ -> _New vApp_:

![Добавление vApp, шаг 1](../../images/cloud-provider-vcd/application-setup/Screenshot.png)

Установите имя и включите vApp:

![Добавление vApp, шаг 2](../../images/cloud-provider-vcd/application-setup/Screenshot2.png)

### Добавление сети к vApp

После создания vApp, необходимо присоединить к ней созданную внутреннюю сеть.

Перейдите во вкладку _Data Centers_ -> _vApps_, откройте необходимый _vApp_:

![Добавление сети к vApp, шаг 1](../../images/cloud-provider-vcd/network-in-vapp-setup/Screenshot.png)

Перейдите во вкладку _Networks_ и нажмите на кнопку _NEW_:

![Добавление сети к vApp, шаг 2](../../images/cloud-provider-vcd/network-in-vapp-setup/Screenshot2.png)

В появившемся окне выберите тип _Direct_ и выберите сеть:

![Добавление сети к vApp, шаг 3](../../images/cloud-provider-vcd/network-in-vapp-setup/Screenshot3.png)

### Входящий трафик

Входящий трафик необходимо направить на edge router (порты 80, 443) при помощи правил DNAT на выделенный адрес во внутренней сети.
Этот адрес поднимается при помощи MetalLB в L2 режиме на выделенных frontend-узлах.

### Настройка правил DNAT/SNAT на edge gateway

Перейдите во вкладку _Networking_ -> _Edge Gateways_, откройте edge gateway:

![Настройка правил DNAT на edge gateway, шаг 1](../../images/cloud-provider-vcd/edge-gateway-setup/Screenshot.png)

Перейдите во вкладку _Services_ -> _NAT_:

![Настройка правил DNAT на edge gateway, шаг 2](../../images/cloud-provider-vcd/edge-gateway-setup/Screenshot2.png)

Добавьте следующие правила:

![Настройка правил DNAT на edge gateway, шаг 3](../../images/cloud-provider-vcd/edge-gateway-setup/Screenshot3.png)

Первые два правила используются для входящего трафика, а третье — для доступа по SSH к узлу с control plane (без этого правила установка будет невозможна).

Чтобы настроить возможность виртуальным машинам выходить в интернет необходимо настроить правила SNAT, для этого добавьте следующие правила:

![Настройка правил SNAT на edge gateway, шаг 1](../../images/cloud-provider-vcd/edge-gateway-setup/Screenshot4.png)

Данное правило позволит виртуальным машинам из подсети `192.168.199.0/24` выходить в интернет.

### Настройка firewall

После настройки DNAT необходимо настроить firewall. Сначала необходимо настроить наборы IP-адресов (IP sets).

Перейдите во вкладку _Security_ -> _IP Sets_:

![Настройка firewall на edge gateway, шаг 1](../../images/cloud-provider-vcd/edge-firewall/Screenshot.png)

Создайте следующий набор IP (тут подразумевается, что адрес MetalLB будет `.10` а адрес узла с control plane — `.2`):

![Настройка firewall на edge gateway, шаг 1](../../images/cloud-provider-vcd/edge-firewall/Screenshot2.png)

![Настройка firewall на edge gateway, шаг 1](../../images/cloud-provider-vcd/edge-firewall/Screenshot3.png)

![Настройка firewall на edge gateway, шаг 1](../../images/cloud-provider-vcd/edge-firewall/Screenshot4.png)

Добавьте следующие правила firewall:

![Настройка firewall на edge gateway, шаг 1](../../images/cloud-provider-vcd/edge-firewall/Screenshot5.png)

## Шаблон виртуальной машины

{% alert level="warning" %}
Работоспособность провайдера подтверждена только для шаблонов виртуальных машин на базе Ubuntu 22.04.
{% endalert %}

{% include notice_envinronment.liquid %}

В примере используется OVA-файл, предоставляемый Ubuntu, с двумя исправлениями.
Исправления необходимы для корректного заказа CloudPermanent-узлов и для возможности подключать диски, созданные CSI.

### Подготовка шаблона из OVA-файла

Скачайте [OVA-файл](https://cloud-images.ubuntu.com/jammy/):

![Настройка шаблона, шаг 1](../../images/cloud-provider-vcd/template/Screenshot.png)

Перейдите на вкладку _Libraries_ -> _Catalogs_ -> _Каталог организации_:

![Настройка шаблона, шаг 2](../../images/cloud-provider-vcd/template/Screenshot2.png)

Выберите скачанный шаблон и загрузите его в каталог:

![Настройка шаблона, шаг 3](../../images/cloud-provider-vcd/template/Screenshot3.png)

![Настройка шаблона, шаг 4](../../images/cloud-provider-vcd/template/Screenshot4.png)

![Настройка шаблона, шаг 5](../../images/cloud-provider-vcd/template/Screenshot5.png)

Создайте виртуальную машину из шаблона:

![Настройка шаблона, шаг 6](../../images/cloud-provider-vcd/template/Screenshot6.png)

![Настройка шаблона, шаг 7](../../images/cloud-provider-vcd/template/Screenshot7.png)

{% alert level="warning" %}
Укажите пароль по умолчанию и публичный ключ. Это необходимо для того, чтобы войти в консоль виртуальной машины.
{% endalert %}

![Настройка шаблона, шаг 8](../../images/cloud-provider-vcd/template/Screenshot8.png)

Для того чтобы получить возможность подключения к виртуальной машине, выполните следующие шаги:
1. Запустите виртуальную машину
2. Дождитесь получение IP-адреса
3. _Пробросьте_ порт 22 до виртуальной машины:

![Настройка шаблона, шаг 9](../../images/cloud-provider-vcd/template/Screenshot9.png)

Войдите на виртуальную машину по SSH и выполните следующие команды:

```shell
rm /etc/netplan/99-netcfg-vmware.yaml
echo -e '\n[deployPkg]\nwait-cloudinit-timeout=1800\n' >> /etc/vmware-tools/tools.conf
echo 'disable_vmware_customization: true' > /etc/cloud/cloud.cfg.d/91_vmware_cust.cfg
dpkg-reconfigure cloud-init
```

В появившемся диалоговом окне оставьте галочку только у `OVF: Reads data from OVF transports`, остальные пункты необходимо отключить:

![Настройка шаблона, OVF](../../images/cloud-provider-vcd/template/OVF.png)

Выполните оставшиеся команды:

```shell
truncate -s 0 /etc/machine-id
rm /var/lib/dbus/machine-id
ln -s /etc/machine-id /var/lib/dbus/machine-id
cloud-init clean --logs --seed
passwd -d ubuntu
passwd -d root
rm /home/ubuntu/.ssh/authorized_keys
history -c

shutdown -P now
```

### Настройка шаблона в VCD

Выключите виртуальную машину и удалите все заполненные поля _Guest Properties_:

![Настройка шаблона, Guest Properties 1](../../images/cloud-provider-vcd/template/GuestProperties1.png)

![Настройка шаблона, Guest Properties 5](../../images/cloud-provider-vcd/template/GuestProperties5.png)

Cоздайте шаблон виртуальной машины:

![Настройка шаблона, шаг 10](../../images/cloud-provider-vcd/template/Screenshot10.png)

![Настройка шаблона, шаг 11](../../images/cloud-provider-vcd/template/Screenshot11.png)

В созданном шаблоне перейдите во вкладку _Metadata_ и добавьте 6 полей:

* _guestinfo.metadata_
* _guestinfo.metadata.encoding_
* _guestinfo.userdata_
* _guestinfo.userdata.encoding_
* _disk.enableUUID_
* _guestinfo.hostname_

![Настройка шаблона, Guest Properties 2](../../images/cloud-provider-vcd/template/GuestProperties2.png)

![Настройка шаблона, Guest Properties 3](../../images/cloud-provider-vcd/template/GuestProperties3.png)

В панели управления vCenter для шаблона включите параметр `disk.EnableUUID`:

![Настройка шаблона, vCenter 1](../../images/cloud-provider-vcd/template/vCenter1.png)

![Настройка шаблона, vCenter 2](../../images/cloud-provider-vcd/template/vCenter2.png)

![Настройка шаблона, vCenter 3](../../images/cloud-provider-vcd/template/vCenter3.png)

![Настройка шаблона, vCenter 4](../../images/cloud-provider-vcd/template/vCenter4.png)

![Настройка шаблона, vCenter 5](../../images/cloud-provider-vcd/template/vCenter5.png)

## Использование хранилища

* VCD поддерживает CSI, диски создаются как VCD Independent Disks.
* Guest property `disk.EnableUUID` должно быть разрешено для используемых шаблонов виртуальных машин.
* Deckhouse Kubernetes Platform поддерживает изменение размера дисков с версии v1.59.1.
