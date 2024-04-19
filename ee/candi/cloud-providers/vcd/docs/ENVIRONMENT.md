---
title: "Cloud provider — VMware Cloud Director: Preparing environment"
description: "Configuring VMware Cloud Director for Deckhouse cloud provider operation."
---

<!-- AUTHOR! Don't forget to update getting started if necessary -->

## List of required VCD resources

* _Organization_
* _VirtualDataCenter_
* _VApp_
* _StoragePolicy_
* _SizingPolicy_
* _Network_
* _EdgeRouter_
* _Catalog_

Organization, VirtualDataCenter, StoragePolicy, SizingPolicy, EdgeRouter and Catalog must be provided by your VMware Cloud Director service provider.
Also, in the tenant, you need to grant the following rights to change VM parameters (use the [instruction](https://kb.vmware.com/s/article/92067)):

* _guestinfo.metadata_
* _guestinfo.metadata.encoding_
* _guestinfo.userdata_
* _guestinfo.userdata.encoding_
* _disk.enableUUID_
* _guestinfo.hostname_

Network (internal network) can be configured by your VMware Cloud Director service provider, or you can configure it yourself. Next, we consider setting up the internal network yourself.

### Adding a network

Go to the _Networking_ tab and click on the _NEW button_:

![Adding a network, step 1](../../images/030-cloud-provider-vcd/network-setup/Screenshot.png)

Select the Data Center:

![Adding a network, step 2](../../images/030-cloud-provider-vcd/network-setup/Screenshot2.png)

Note that _Network type_ must be _Routed_:

![Adding a network, step 3](../../images/030-cloud-provider-vcd/network-setup/Screenshot3.png)

Connect the _EdgeRouter_ to the network:

![Adding a network, step 4](../../images/030-cloud-provider-vcd/network-setup/Screenshot4.png)

Set the network name and CIDR:

![Adding a network, step 5](../../images/030-cloud-provider-vcd/network-setup/Screenshot5.png)

Do not add Static IP Pools, because DHCP will be used:

![Adding a network, step 6](../../images/030-cloud-provider-vcd/network-setup/Screenshot6.png)

Specify the DNS server addresses:

![Adding a network, step 7](../../images/030-cloud-provider-vcd/network-setup/Screenshot7.png)

### Configuring DHCP

To provision nodes dynamically, you have to enable the DHCP server for the internal network.

{% alert level="info" %}
We recommend allocating the beginning of the network address range to system consumers (control plane, frontend nodes, system nodes) and the rest to the DHCP pool. For example, for a `/24` mask network it would be enough to allocate 20 addresses to system consumers.
{% endalert %}

Click the _Networking_ tab and open the network you created:

![DHCP, step 1](../../images/030-cloud-provider-vcd/dhcp-setup/Screenshot.png)

In the window that opens, select the _IP Management_ -> _DHCP_ -> Activate tab:

![DHCP, step 2](../../images/030-cloud-provider-vcd/dhcp-setup/Screenshot2.png)

In the _General settings_ tab, set the parameters as shown in the example:

![DHCP, step 3](../../images/030-cloud-provider-vcd/dhcp-setup/Screenshot3.png)

Next, add a pool:

![DHCP, step 3](../../images/030-cloud-provider-vcd/dhcp-setup/Screenshot4.png)

Set the DNS server addresses:

![DHCP, step 3](../../images/030-cloud-provider-vcd/dhcp-setup/Screenshot5.png)

### Adding vApp

Switch to the _Data Centers_ -> _vApps_ -> _NEW_ -> _New vApp_ tab: 

![Adding a vApp, step 1](../../images/030-cloud-provider-vcd/application-setup/Screenshot.png)

Set a name and enable the vApp:

![Adding a vApp, step 2](../../images/030-cloud-provider-vcd/application-setup/Screenshot2.png)

### Добавление сети к vApp

После создания vApp, необходимо присоединить к ней созданную внутреннюю сеть.

Перейдите во вкладку _Data Centers_ -> _vApps_, откройте необходимый _vApp_:

![Добавление сети к vApp, шаг 1](../../images/030-cloud-provider-vcd/network-in-vapp-setup/Screenshot.png)

Перейдите во вкладку _Networks_ и нажмите на кнопку _NEW_:

![Добавление сети к vApp, шаг 2](../../images/030-cloud-provider-vcd/network-in-vapp-setup/Screenshot2.png)

В появившемся окне выберите тип _Direct_ и выберите сеть:

![Добавление сети к vApp, шаг 3](../../images/030-cloud-provider-vcd/network-in-vapp-setup/Screenshot3.png)

### Входящий трафик

Входящий трафик необходимо направить на edge router (порты 80, 443) при помощи правил DNAT на выделенный адрес во внутренней сети. 
Этот адрес поднимается при помощи MetalLB в L2 режиме на выделенных frontend-узлах. 

### Настройка правила DNAT на edge gateway.

Перейдите во вкладку _Networking_ -> _Edge Gateways_, откройте edge gateway:

![Настройка правил DNAT на edge gateway, шаг 1](../../images/030-cloud-provider-vcd/edge-gateway-setup/Screenshot.png)

Перейдите во вкладку _Services_ -> _NAT_:

![Настройка правил DNAT на edge gateway, шаг 2](../../images/030-cloud-provider-vcd/edge-gateway-setup/Screenshot2.png)

Добавьте следующие правила:

![Настройка правил DNAT на edge gateway, шаг 3](../../images/030-cloud-provider-vcd/edge-gateway-setup/Screenshot3.png)

Первые два правила используются для входящего трафика, а третье — для доступа по SSH к узлу с control plane (без этого правила установка будет невозможна).

### Настрока firewall

После настройки DNAT необходимо настроить firewall. Сначала необходимо настроить наборы IP-адресов (IP sets).

Перейдите во вкладку _Security_ -> _IP Sets_:

![Настройка firewall на edge gateway, шаг 1](../../images/030-cloud-provider-vcd/edge-firewall/Screenshot.png)

Создайте следующий набор IP (тут подразумевается, что адрес MetalLB будет `.10` а адрес узла с control plane — `.2`):

![Настройка firewall на edge gateway, шаг 1](../../images/030-cloud-provider-vcd/edge-firewall/Screenshot2.png)

![Настройка firewall на edge gateway, шаг 1](../../images/030-cloud-provider-vcd/edge-firewall/Screenshot3.png)

![Настройка firewall на edge gateway, шаг 1](../../images/030-cloud-provider-vcd/edge-firewall/Screenshot4.png)

Добавьте следующие правила firewall:

![Настройка firewall на edge gateway, шаг 1](../../images/030-cloud-provider-vcd/edge-firewall/Screenshot5.png)

## Шаблон виртуальной машины
{% alert level="warning" %}
Работоспособность провайдера подтверждена только для шаблонов виртуальных машин на базе Ubuntu 22.04.
{% endalert %}

В примере используется OVA-файл предоставляемый Ubuntu, с двумя исправлениями.
Исправления необходимы для корректного заказа CloudPermanent-узлов и для возможности подключать диски, созданные CSI.

### Подготовка шаблона из OVA-файла

Скачайте [OVA-файл](https://cloud-images.ubuntu.com/jammy/):

![Настройка шаблона, шаг 1](../../images/030-cloud-provider-vcd/template/Screenshot.png)

Перейдите на вкладку _Libraries_ -> _Catalogs_ -> _Каталог организации_:

![Настройка шаблона, шаг 2](../../images/030-cloud-provider-vcd/template/Screenshot2.png)

Выберите скаченный шаблон и загрузите его в каталог:

![Настройка шаблона, шаг 3](../../images/030-cloud-provider-vcd/template/Screenshot3.png)

![Настройка шаблона, шаг 4](../../images/030-cloud-provider-vcd/template/Screenshot4.png)

![Настройка шаблона, шаг 5](../../images/030-cloud-provider-vcd/template/Screenshot5.png)

Создайте виртуальную машину из шаблона:

![Настройка шаблона, шаг 6](../../images/030-cloud-provider-vcd/template/Screenshot6.png)

![Настройка шаблона, шаг 7](../../images/030-cloud-provider-vcd/template/Screenshot7.png)

{% alert level="warning" %}
Укажите пароль по умолчанию и публичный ключ. Это необходимо для того, чтобы войти в консоль виртуальной машины.
{% endalert %}

![Настройка шаблона, шаг 8](../../images/030-cloud-provider-vcd/template/Screenshot8.png)

Для того чтобы получить возможность подключения к виртуальной машине, выполните следующие шаги: 
1. Запустите виртуальную машину
2. Дождитесь получение IP-адреса
3. _Пробросьте_ порт 22 до виртуальной машины:

![Настройка шаблона, шаг 9](../../images/030-cloud-provider-vcd/template/Screenshot9.png)

Войдите на виртуальную машину по SSH и выполните следующие команды:

```shell
echo -e '\n[deployPkg]\nwait-cloudinit-timeout=1800\n' >> /etc/vmware-tools/tools.conf
passwd -d ubuntu
passwd -d root
rm /home/ubuntu/.ssh/authorized_keys
history -c
shutdown -P now
```

Выключите виртуальную машину и создайте шаблон виртуальной машины:

![Настройка шаблона, шаг 10](../../images/030-cloud-provider-vcd/template/Screenshot10.png)

![Настройка шаблона, шаг 11](../../images/030-cloud-provider-vcd/template/Screenshot11.png)

После создания шаблона виртуальной машины, обратитесь к поставщику услуг VMware Cloud Director с просьбой включить для шаблона параметр `disk.enableUUID`.

## Использование хранилища

* VCD поддерживает CSI, диски создаются как VCD Independent Disks.
* Guest property `disk.EnableUUID` должно быть разрешено для используемых шаблонов виртуальных машин.
* Deckhouse Kubernetes Platform поддерживает изменение размера дисков с версии v1.59.1.
