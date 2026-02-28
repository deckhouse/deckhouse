---
title: Подключение и авторизация в VMware Cloud Director
permalink: ru/admin/integrations/virtualization/vcd/connection-and-authorization.html
lang: ru
---

## Подготовка ресурсов

{% alert level="warning" %}
Провайдер поддерживает работу только с одним диском в шаблоне виртуальной машины. Убедитесь, что шаблон содержит только один диск.
{% endalert %}

Для управления ресурсами в VCD с помощью Deckhouse Kubernetes Platform необходимо настроить в системе следующие ресурсы:

* Organization
* VirtualDataCenter
* vApp (при схеме размещения `Standard`)
* StoragePolicy
* SizingPolicy
* Network (при схеме размещения `Standard`)
* EdgeRouter
* Catalog

Ресурсы Organization, VirtualDataCenter, StoragePolicy, SizingPolicy, EdgeRouter и Catalog должны быть предоставлены вашим поставщиком услуг VMware Cloud Director.

Network (внутренняя сеть) может быть настроена вашим поставщиком услуг VMware Cloud Director, либо вы можете настроить ее самостоятельно. При выборе схемы размещения `StandardWithNetwork` сеть создается автоматически. Далее описан способ самостоятельной настройки внутренней сети.

### Права пользователя

Пользователь, под которым будет осуществляться доступ к API VMware Cloud Director, должен иметь следующие права:

* Роль `Organization Administrator` с дополнительным правилом `Preserve All ExtraConfig Elements During OVF Import and Export`;
* Правило `Preserve All ExtraConfig Elements During OVF Import and Export` должно быть продублировано в используемом `Right Bundle` пользователя.

### Добавление сети

{% alert level="info" %}
Инструкция актуальна только для схемы размещения `Standard`.
{% endalert %}

1. Перейдите на вкладку «Networking» и нажмите «NEW»:

   ![Добавление сети, шаг 1](../../../../images/cloud-provider-vcd/network-setup/Screenshot.png)

1. Выберите необходимый Data Center:

   ![Добавление сети, шаг 2](../../../../images/cloud-provider-vcd/network-setup/Screenshot2.png)

1. На этапе «Network type» выберите «Routed»:

   ![Добавление сети, шаг 3](../../../../images/cloud-provider-vcd/network-setup/Screenshot3.png)

1. Присоедините `EdgeRouter` к сети:

   ![Добавление сети, шаг 4](../../../../images/cloud-provider-vcd/network-setup/Screenshot4.png)

1. Укажите имя сети и CIDR:

   ![Добавление сети, шаг 5](../../../../images/cloud-provider-vcd/network-setup/Screenshot5.png)

1. Не добавляйте «Static IP Pools», поскольку будет использоваться DHCP:

   ![Добавление сети, шаг 6](../../../../images/cloud-provider-vcd/network-setup/Screenshot6.png)

1. Укажите адреса DNS-серверов:

   ![Добавление сети, шаг 7](../../../../images/cloud-provider-vcd/network-setup/Screenshot7.png)

### Настройка DHCP

{% alert level="info" %}
Инструкция актуальна только для схемы размещения `Standard`.
{% endalert %}

Для динамического заказа узлов включите DHCP-сервер для внутренней сети.

{% alert level="info" %}
Рекомендуем выделить начало диапазона адресов сети на системные нагрузки (control plane, frontend-узлы, системные узлы), а остальное выделить на DHCP-пул. Например, для сети по маске `/24` будет достаточно выделения 20 адресов под системные нагрузки.
{% endalert %}

1. Перейдите на вкладку «Networking» и откройте созданную сеть:

   ![DHCP, шаг 1](../../../../images/cloud-provider-vcd/dhcp-setup/Screenshot.png)

1. В открывшемся окне выберите пункт «IP Management» → «DHCP» → «Activate»:

   ![DHCP, шаг 2](../../../../images/cloud-provider-vcd/dhcp-setup/Screenshot2.png)

1. На вкладке «General settings» настройте параметры аналогично примеру:

   ![DHCP, шаг 3](../../../../images/cloud-provider-vcd/dhcp-setup/Screenshot3.png)

1. Добавьте пул:

   ![DHCP, шаг 3](../../../../images/cloud-provider-vcd/dhcp-setup/Screenshot4.png)

1. Укажите адреса DNS-серверов:

   ![DHCP, шаг 3](../../../../images/cloud-provider-vcd/dhcp-setup/Screenshot5.png)

### Добавление vApp

{% alert level="info" %}
Инструкция актуальна только для схемы размещения `Standard`.
{% endalert %}

1. Перейдите на вкладку «Data Centers» → «vApps» → «NEW» → «New vApp»:

   ![Добавление vApp, шаг 1](../../../../images/cloud-provider-vcd/application-setup/Screenshot.png)

1. Укажите имя и включите vApp:

   ![Добавление vApp, шаг 2](../../../../images/cloud-provider-vcd/application-setup/Screenshot2.png)

### Добавление сети к vApp

{% alert level="info" %}
Инструкция актуальна только для схемы размещения `Standard`.
{% endalert %}

После создания vApp присоедините к ней созданную внутреннюю сеть.

1. Перейдите на вкладку «Data Centers» → «vApps» и откройте необходимый vApp:

   ![Добавление сети к vApp, шаг 1](../../../../images/cloud-provider-vcd/network-in-vapp-setup/Screenshot.png)

1. Перейдите на вкладку «Networks» и нажмите «NEW»:

   ![Добавление сети к vApp, шаг 2](../../../../images/cloud-provider-vcd/network-in-vapp-setup/Screenshot2.png)

1. В появившемся окне выберите тип «Direct» и выберите сеть:

   ![Добавление сети к vApp, шаг 3](../../../../images/cloud-provider-vcd/network-in-vapp-setup/Screenshot3.png)

### Входящий трафик

Входящий трафик необходимо направить на edge router (порты `80`, `443`) при помощи правил DNAT на выделенный адрес во внутренней сети.
Этот адрес поднимается при помощи MetalLB в L2-режиме на выделенных frontend-узлах.

### Настройка правил DNAT/SNAT на edge gateway

1. Перейдите на вкладку «Networking» → «Edge Gateways», откройте edge gateway:

   ![Настройка правил DNAT на edge gateway, шаг 1](../../../../images/cloud-provider-vcd/edge-gateway-setup/Screenshot.png)

1. Перейдите на вкладку «Services» → «NAT»:

   ![Настройка правил DNAT на edge gateway, шаг 2](../../../../images/cloud-provider-vcd/edge-gateway-setup/Screenshot2.png)

1. Добавьте следующие правила:

   ![Настройка правил DNAT на edge gateway, шаг 3](../../../../images/cloud-provider-vcd/edge-gateway-setup/Screenshot3.png)

   Первые два правила используются для входящего трафика, а третье — для доступа по SSH к узлу с control plane (без этого правила установка будет невозможна).

1. Чтобы виртуальные машины могли выходить в интернет, настройте правила SNAT, следуя примеру:

   ![Настройка правил SNAT на edge gateway, шаг 1](../../../../images/cloud-provider-vcd/edge-gateway-setup/Screenshot4.png)

   Данное правило позволит виртуальным машинам из подсети `192.168.199.0/24` выходить в интернет.

### Выпуск сертификатов Let's Encrypt

{% alert level="info" %}
Инструкция актуальна только для схемы размещения `Standard`.
{% endalert %}

При использовании [`cert-manager`](/modules/cert-manager/) и ACME-проверки типа `HTTP-01` может потребоваться настройка `Hairpin NAT` (NAT loopback) на `Edge Gateway`.

Это необходимо в сценарии, когда доменное имя Ingress резолвится во внешний IP-адрес `Edge Gateway`, а запрос к этому адресу выполняется **изнутри** кластера (из сети узлов). В таком случае трафик должен быть корректно возвращён обратно во внутреннюю сеть на адрес Ingress (например, адрес [`MetalLB`](/modules/metallb/)).

Это важно для выпуска сертификатов Let's Encrypt, так как [`cert-manager`](/modules/cert-manager/) выполняет предварительную проверку доступности challenge URL до обращения к ACME-провайдеру. Если URL недоступен изнутри кластера, выпуск сертификата не начнётся, даже если снаружи адрес открывается корректно.

Пример (сеть кластера `192.168.199.0/24`):

- внутренняя сеть узлов: `192.168.199.0/24`;
- внешний IP-адрес `Edge Gateway`: `194.117.83.19`;
- внутренний адрес Ingress (например, `MetalLB`): `192.168.199.251`.

В этом случае необходимо настроить `Hairpin NAT` для трафика из сети `192.168.199.0/24`, направленного на внешний IP `194.117.83.19`, с трансляцией на внутренний адрес Ingress `192.168.199.251`.

> Для `Edge Gateway` на базе `NSX-V` настройка `Hairpin NAT` может быть обязательной.
>
> Для `NSX-T` loopback-сценарий часто поддерживается по умолчанию, но фактическое поведение зависит от конфигурации провайдера VMware Cloud Director.

### Настройка firewall

{% alert level="info" %}
Инструкция актуальна только для схемы размещения `Standard`.
{% endalert %}

После настройки DNAT настройте firewall. Сначала необходимо настроить наборы IP-адресов (IP sets).

1. Перейдите на вкладку «Security» → «IP Sets»:

   ![Настройка firewall на edge gateway, шаг 1](../../../../images/cloud-provider-vcd/edge-firewall/Screenshot.png)

1. Создайте следующий набор IP (подразумевается, что адрес MetalLB будет `.10` а адрес узла с control plane — `.2`):

   ![Настройка firewall на edge gateway, шаг 2-1](../../../../images/cloud-provider-vcd/edge-firewall/Screenshot2.png)

   ![Настройка firewall на edge gateway, шаг 2-2](../../../../images/cloud-provider-vcd/edge-firewall/Screenshot3.png)

   ![Настройка firewall на edge gateway, шаг 2-3](../../../../images/cloud-provider-vcd/edge-firewall/Screenshot4.png)

1. Добавьте следующие правила firewall:

   ![Настройка firewall на edge gateway, шаг 3](../../../../images/cloud-provider-vcd/edge-firewall/Screenshot5.png)

## Шаблон виртуальной машины

{% alert level="warning" %}
Работоспособность провайдера подтверждена только для шаблонов виртуальных машин на базе Ubuntu 22.04.
{% endalert %}

{% alert level="warning" %}
Отключите vApp/Guest OS Customization (Guest Customization, vApp Customization и аналогичные механизмы) для шаблона и виртуальных машин кластера в VMware Cloud Director. DKP выполняет первичную настройку узлов через `cloud-init` (datasource OVF/VMware GuestInfo). Включенная customization может конфликтовать с `cloud-init` и приводить к некорректной инициализации узла.
{% endalert %}

{% include notice_envinronment.liquid %}

В примере используется OVA-файл, предоставляемый Ubuntu, с двумя исправлениями.
Исправления необходимы для корректного заказа CloudPermanent-узлов и для возможности подключать диски, созданные CSI.

### Подготовка шаблона из OVA-файла

1. Скачайте [OVA-файл](https://cloud-images.ubuntu.com/jammy/):

   ![Настройка шаблона, шаг 1](../../../../images/cloud-provider-vcd/template/Screenshot.png)

1. Перейдите на вкладку «Libraries» → «Catalogs» → «Каталог организации»:

   ![Настройка шаблона, шаг 2](../../../../images/cloud-provider-vcd/template/Screenshot2.png)

1. Выберите скачанный шаблон и загрузите его в каталог:

   ![Настройка шаблона, шаг 3](../../../../images/cloud-provider-vcd/template/Screenshot3.png)

   ![Настройка шаблона, шаг 4](../../../../images/cloud-provider-vcd/template/Screenshot4.png)

   ![Настройка шаблона, шаг 5](../../../../images/cloud-provider-vcd/template/Screenshot5.png)

1. Создайте виртуальную машину из шаблона:

   ![Настройка шаблона, шаг 6](../../../../images/cloud-provider-vcd/template/Screenshot6.png)

   ![Настройка шаблона, шаг 7](../../../../images/cloud-provider-vcd/template/Screenshot7.png)

{% alert level="warning" %}
Укажите пароль по умолчанию и публичный ключ. Они понадобятся, чтобы войти в консоль виртуальной машины.
{% endalert %}

![Настройка шаблона, шаг 8](../../../../images/cloud-provider-vcd/template/Screenshot8.png)

Для подключения к виртуальной машине выполните следующие шаги:

1. Запустите виртуальную машину.
2. Дождитесь получение IP-адреса.
3. _Пробросьте_ порт `22` до виртуальной машины:

   ![Настройка шаблона, шаг 9](../../../../images/cloud-provider-vcd/template/Screenshot9.png)

Войдите на виртуальную машину по SSH и выполните следующие команды:

```shell
rm /etc/netplan/99-netcfg-vmware.yaml
echo -e '\n[deployPkg]\nwait-cloudinit-timeout=1800\n' >> /etc/vmware-tools/tools.conf
echo 'disable_vmware_customization: true' > /etc/cloud/cloud.cfg.d/91_vmware_cust.cfg
dpkg-reconfigure cloud-init
```

В появившемся диалоговом окне оставьте галочку только у `OVF: Reads data from OVF transports`. Остальные пункты отключите:

![Настройка шаблона, OVF](../../../../images/cloud-provider-vcd/template/OVF.png)

Убедитесь, что в конфигурации `cloud-init` задан параметр `datasource_list`, с помощью следующей команды:

```shell
cat /etc/cloud/cloud.cfg.d/90_dpkg.cfg
```

Если вывод окажется пустым, выполните следующую команду:

```shell
echo "datasource_list: [ OVF, VMware, None ]" > /etc/cloud/cloud.cfg.d/90_dpkg.cfg
```

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

1. Выключите виртуальную машину и удалите все заполненные поля «Guest Properties»:

   ![Настройка шаблона, Guest Properties 1](../../../../images/cloud-provider-vcd/template/GuestProperties1.png)

   ![Настройка шаблона, Guest Properties 5](../../../../images/cloud-provider-vcd/template/GuestProperties5.png)

1. Cоздайте шаблон виртуальной машины:

   ![Настройка шаблона, шаг 10](../../../../images/cloud-provider-vcd/template/Screenshot10.png)

   ![Настройка шаблона, шаг 11](../../../../images/cloud-provider-vcd/template/Screenshot11.png)

1. В созданном шаблоне перейдите на вкладку «Metadata» и добавьте шесть полей:

   * `guestinfo.metadata`;
   * `guestinfo.metadata.encoding`;
   * `guestinfo.userdata`;
   * `guestinfo.userdata.encoding`;
   * `disk.enableUUID`;
   * `guestinfo.hostname`.

   ![Настройка шаблона, Guest Properties 2](../../../../images/cloud-provider-vcd/template/GuestProperties2.png)

   Для **каждого** поля в форме добавления/редактирования укажите:

   * «Type»: `Text` (текстовое значение);
   * «User access»: `Read/Write`;
   * «Value»: один пробел (space).

   > Интерфейс VCD может не сохранять метаданные с пустым значением. Пробел используется как техническое заполнение и не влияет на работу. Фактические значения будут подставлены автоматически при создании виртуальных машин.

   ![Настройка шаблона, Guest Properties 3](../../../../images/cloud-provider-vcd/template/GuestProperties3.png)

1. В панели управления vCenter для шаблона включите параметр `disk.EnableUUID`:

   ![Настройка шаблона, vCenter 1](../../../../images/cloud-provider-vcd/template/vCenter1.png)

   ![Настройка шаблона, vCenter 2](../../../../images/cloud-provider-vcd/template/vCenter2.png)

   ![Настройка шаблона, vCenter 3](../../../../images/cloud-provider-vcd/template/vCenter3.png)

   ![Настройка шаблона, vCenter 4](../../../../images/cloud-provider-vcd/template/vCenter4.png)

   ![Настройка шаблона, vCenter 5](../../../../images/cloud-provider-vcd/template/vCenter5.png)

## Использование хранилища

* VCD поддерживает CSI. Диски создаются как VCD Independent Disks.
* Guest property `disk.EnableUUID` должно быть разрешено для используемых шаблонов виртуальных машин.
* Deckhouse Kubernetes Platform поддерживает изменение размера дисков с версии v1.59.1.

## Использование балансировщика нагрузки

* Компоненты DKP поддерживают ресурсов Service типа LoadBalancer при установке в VMware Cloud Director (VCD).
* В качестве балансировщика используется VMware NSX Advanced Load Balancer (ALB или Avi Networks).
* Поддержка доступна **только** при использовании платформы виртуализации сети `NSX-T`.
* Балансировка должна быть включёна на Edge Gateway вашим провайдером VCD. Проверить, включёна ли балансировка, можно в разделе Edge Gateway → Load Balancer → General Settings — параметр `State` должен быть в статусе `Active`.
* Если балансировщик был активирован после успешного создания кластера DKP, компоненты автоматически подхватят изменения в течение часа (дополнительных действий не требуется).
* Для каждого открытого порта создаётся связка Pool + Virtual Service.
* При наличии межсетевого экрана необходимо создать разрешающее правило для внешнего IP-адреса балансировщика и соответствующих портов.
