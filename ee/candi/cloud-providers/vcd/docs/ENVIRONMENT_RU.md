---
title: "Cloud provider — VMware Cloud Director: подготовка окружения."
description: "Подготовка окружения VMware Cloud Director для работы Deckhouse cloud provider."
---

<!-- AUTHOR! Don't forget to update getting started if necessary -->

## Список необходимых ресурсов VCD

* _Organization_
* _VirtualDataCenter_
* _VApp_
* _StoragePolicy_
* _SizingPolicy_
* _Network_
* _EdgeRouter_
* _Catalog_

Organization, VirtualDataCenter, StoragePolicy, SizingPolicy, EdgeRouter и Catalog должны быть предоставлены провайдером vCloud Director.
Также в тенанте по инструкции https://kb.vmware.com/s/article/92067 нужно выдать следующие права на изменение параметров ВМ:

* _guestinfo.metadata_
* _guestinfo.metadata.encoding_
* _guestinfo.userdata_
* _guestinfo.userdata.encoding_
* _disk.enableUUID_
* _guestinfo.hostname_

Network (внутренная сеть) так же может подготавливать провайдер vCloud Director. Рассмотрим как можно настроить внутренную сеть самостоятельно:

### Добавление сети
Переходим во вкладку _Networking_ и кликаем по кнопке _NEW_:

![Добавление сети, шаг 1](../../images/030-cloud-provider-vcd/network-setup/Screenshot.png)

Выбираем необходимый Data Center:

![Добавление сети, шаг 2](../../images/030-cloud-provider-vcd/network-setup/Screenshot2.png)

_Network type_ должен быть _Routed_:

![Добавление сети, шаг 3](../../images/030-cloud-provider-vcd/network-setup/Screenshot3.png)

Присодиняем _EdgeRouter_ к сети:

![Добавление сети, шаг 4](../../images/030-cloud-provider-vcd/network-setup/Screenshot4.png)

Устанавливаем имя сети и CIDR:

![Добавление сети, шаг 5](../../images/030-cloud-provider-vcd/network-setup/Screenshot5.png)

Static IP Pools не добавляем, мы будем использовать DHCP:

![Добавление сети, шаг 6](../../images/030-cloud-provider-vcd/network-setup/Screenshot6.png)

Устанавливаем адреса DNS-серверов:

![Добавление сети, шаг 7](../../images/030-cloud-provider-vcd/network-setup/Screenshot7.png)

### Настройка DHCP

Для динамического заказа нод необходимо включить DHCP для внутренней сети.
Для /24 сети рекомендуется  выделить первые 20 адресов на системные нагрузки (control-plane, frontend, system),
остальные адреса выделить на DHCP-пул.

Переходим во вкладку _Networking_ и кликаем созданной сети:

![DHCP, шаг 1](../../images/030-cloud-provider-vcd/dhcp-setup/Screenshot.png)

В открытом окне кликаем по вкладке _IP Management_ -> _DHCP_ -> Activate:

![DHCP, шаг 2](../../images/030-cloud-provider-vcd/dhcp-setup/Screenshot2.png)

Во вкладке _General settings_ настраиваем так же как и на изображении:

![DHCP, шаг 3](../../images/030-cloud-provider-vcd/dhcp-setup/Screenshot3.png)

Добавляем пул:

![DHCP, шаг 3](../../images/030-cloud-provider-vcd/dhcp-setup/Screenshot4.png)

Устанавливаем адреса DNS-серверов:

![DHCP, шаг 3](../../images/030-cloud-provider-vcd/dhcp-setup/Screenshot5.png)

### Добавление vApp

Далее необходимо создать vApp.

Переходим во вкладку _Data Centers_ -> _vApps_ -> _NEW_ -> _New vApp_: 

![Добавление vApp, шаг 1](../../images/030-cloud-provider-vcd/application-setup/Screenshot.png)

Устанавливаем имя и включаем vApp:

![Добавление vApp, шаг 2](../../images/030-cloud-provider-vcd/application-setup/Screenshot2.png)

### Добавление сети к vApp

Так же необходимо присоединить внутренную сеть к vApp.
Переходим во вкладку _Data Centers_ -> _vApps_, кликаем на необходимый _vApp_:

![Добавление сети к vApp, шаг 1](../../images/030-cloud-provider-vcd/network-in-vapp-setup/Screenshot.png)

Переходим во вкладку _Networks_ и кликаем по кнопке _NEW_:

![Добавление сети к vApp, шаг 2](../../images/030-cloud-provider-vcd/network-in-vapp-setup/Screenshot2.png)

В появившемся окне выбираем тип _Direct_ и выбираем сеть:

![Добавление сети к vApp, шаг 3](../../images/030-cloud-provider-vcd/network-in-vapp-setup/Screenshot3.png)

### Входящий трафик
Вы должны направить входящий трафик на EDGE router (порты 80, 443) при помощи правил DNAT на выделенный адрес во внутренней сети. 
Этот адрес поднимается при помощи MetalLB в L2 режиме на выделенных frontend-узлах. 

### Настроим правила DNAT на EDGE gateway.

Переходим во вкладку _Networking_ -> _Edge Gateways_, кликаем по edge gateway:

![Настройка правил DNAT на EDGE gateway, шаг 1](../../images/030-cloud-provider-vcd/edge-gateway-setup/Screenshot.png)

Переходим во вкладку _Services_ -> _NAT_:

![Настройка правил DNAT на EDGE gateway, шаг 2](../../images/030-cloud-provider-vcd/edge-gateway-setup/Screenshot2.png)

И добавляем следующие правила:

![Настройка правил DNAT на EDGE gateway, шаг 3](../../images/030-cloud-provider-vcd/edge-gateway-setup/Screenshot2.png)

Первые два правила для входящего трафика, а третье правило для доступа по SSH к control-plane узлу (без этого правила установка будет невозможна)

## Настрока firewall

После настройки DNAT необходимо настроить firewall. Сначала необходимо настроить так наборы IP-адресов (IP sets).

Переходим во вкладку _Security_ -> _IP Sets_:

![Настройка firewall на EDGE gateway, шаг 1](../../images/030-cloud-provider-vcd/edge-firewall/Screenshot.png)

Далее создаем следующий набор IP (тут подразумевается, что адрес MetalLB будет .10 а control-plane узла .2)

![Настройка firewall на EDGE gateway, шаг 1](../../images/030-cloud-provider-vcd/edge-firewall/Screenshot2.png)

![Настройка firewall на EDGE gateway, шаг 1](../../images/030-cloud-provider-vcd/edge-firewall/Screenshot3.png)

![Настройка firewall на EDGE gateway, шаг 1](../../images/030-cloud-provider-vcd/edge-firewall/Screenshot4.png)

Далее добавляем следующие правила firewall

![Настройка firewall на EDGE gateway, шаг 1](../../images/030-cloud-provider-vcd/edge-firewall/Screenshot5.png)

## Шаблон виртуальной машины
**Внимание! В данный момент работоспособность проверена только для Ubuntu 22.04!**

Мы используем OVA-файл, предоставляемый Ubuntu, с двумя исправлениями.
Исправления необходимы для корректного заказа CloudPermanent узлов и для возможности подключать диски, созданные CSI.

### Подготовка шаблона из OVA-файла

Скачиваем [OVA-файл](https://cloud-images.ubuntu.com/jammy/):

![Настройка шаблона, шаг 1](../../images/030-cloud-provider-vcd/template/Screenshot.png)

Переходим _Libraries_ -> _Catalogs_ -> _Каталог организации_:

![Настройка шаблона, шаг 2](../../images/030-cloud-provider-vcd/template/Screenshot2.png)

Выбираем скаченный шаблон и загружаем в каталог:

![Настройка шаблона, шаг 3](../../images/030-cloud-provider-vcd/template/Screenshot3.png)

![Настройка шаблона, шаг 4](../../images/030-cloud-provider-vcd/template/Screenshot4.png)

![Настройка шаблона, шаг 5](../../images/030-cloud-provider-vcd/template/Screenshot5.png)

Создаем виртуальную машину из шаблона:

![Настройка шаблона, шаг 6](../../images/030-cloud-provider-vcd/template/Screenshot6.png)

![Настройка шаблона, шаг 7](../../images/030-cloud-provider-vcd/template/Screenshot7.png)

Важно указать пароль по умолчанию и публичный ключ, он необходим для того, чтобы войти в консоль виртуальной машины:

![Настройка шаблона, шаг 8](../../images/030-cloud-provider-vcd/template/Screenshot8.png)

Чтобы получить возможность подключения к виртуальной машины, запускаем виртуальную машину, ждем получение IP-адреса и пробрасываем порт 23 до виртуальной машины:

![Настройка шаблона, шаг 9](../../images/030-cloud-provider-vcd/template/Screenshot9.png)

Далее, входим по ssh на виртуальную машину и выполняем следующие команды:

```shell
echo -e '\n[deployPkg]\nwait-cloudinit-timeout=1800\n' >> /etc/vmware-tools/tools.conf
passwd -d ubuntu
passwd -d root
rm /home/ubuntu/.ssh/authorized_keys
history -c
shutdown -P now
```

После выключения виртуальной машины, создаем шаблон виртуальной машины:

![Настройка шаблона, шаг 10](../../images/030-cloud-provider-vcd/template/Screenshot10.png)

![Настройка шаблона, шаг 11](../../images/030-cloud-provider-vcd/template/Screenshot11.png)

После создания шаблона, обращаемся к провайдеру vCloud Director с просьбой включить для шаблона параметр `disk.enableUUID`

## Использование хранилища

* VCD поддерживает CSI, диски создаются как VCD Independent Disks.
* Guest property `disk.EnableUUID` должно быть разрешено для используемых темплейтов машин.
* DKP поддерживает ресайз дисков с версии v1.59.1
