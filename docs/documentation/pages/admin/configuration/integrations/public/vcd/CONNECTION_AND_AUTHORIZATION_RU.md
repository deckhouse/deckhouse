---
title: Подключение и авторизация
permalink: ru/admin/integrations/public/vcd/vcd-authorization.html
lang: ru
---

## Требования

Для корректной работы Deckhouse с VMware Cloud Director необходимо наличие следующих ресурсов:

- Organization
- VirtualDataCenter  
- vApp  
- StoragePolicy  
- SizingPolicy  
- Network  
- EdgeRouter  
- Catalog

Ресурсы Organization, VirtualDataCenter, StoragePolicy, SizingPolicy, EdgeRouter и Catalog предоставляются поставщиком услуг VMware Cloud Director.

В организации VMware Cloud Director также должны быть назначены права на изменение параметров виртуальных машин:

- `guestinfo.metadata`  
- `guestinfo.metadata.encoding`  
- `guestinfo.userdata`  
- `guestinfo.userdata.encoding`  
- `disk.enableUUID`  
- `guestinfo.hostname`

Подробнее см. [официальную инструкцию VMware](https://kb.vmware.com/s/article/92067).

## Настройка внутренней сети

Вы можете настроить внутреннюю сеть самостоятельно или воспользоваться помощью провайдера.

### Добавление сети

1. Перейдите во вкладку «Networking» и нажмите кнопку «NEW».  
   ![Шаг 1](../../../../images/cloud-provider-vcd/network-setup/Screenshot.png)

1. Выберите нужный «Data Center».  
   ![Шаг 2](../../../../images/cloud-provider-vcd/network-setup/Screenshot2.png)

1. В поле «Network type» укажите `Routed`.  
   ![Шаг 3](../../../../images/cloud-provider-vcd/network-setup/Screenshot3.png)

1. Присоедините «EdgeRouter».  
   ![Шаг 4](../../../../images/cloud-provider-vcd/network-setup/Screenshot4.png)

1. Укажите имя сети и `CIDR`.  
   ![Шаг 5](../../../../images/cloud-provider-vcd/network-setup/Screenshot5.png)

1. **Не добавляйте Static IP Pools**, так как используется DHCP.  
   ![Шаг 6](../../../../images/cloud-provider-vcd/network-setup/Screenshot6.png)

1. Укажите адреса DNS-серверов.  
   ![Шаг 7](../../../../images/cloud-provider-vcd/network-setup/Screenshot7.png)

### Настройка DHCP

1. Перейдите в «Networking» и откройте созданную сеть.  
   ![DHCP, шаг 1](../../../../images/cloud-provider-vcd/dhcp-setup/Screenshot.png)

1. Перейдите в «IP Management → DHCP → Activate».  
   ![DHCP, шаг 2](../../../../images/cloud-provider-vcd/dhcp-setup/Screenshot2.png)

1. Во вкладке «General settings» настройте параметры:  
   ![DHCP, шаг 3](../../../../images/cloud-provider-vcd/dhcp-setup/Screenshot3.png)

1. Добавьте пул:  
   ![DHCP, шаг 4](../../../../images/cloud-provider-vcd/dhcp-setup/Screenshot4.png)

1. Укажите адреса DNS-серверов.  
   ![DHCP, шаг 5](../../../../images/cloud-provider-vcd/dhcp-setup/Screenshot5.png)

> Пример: для сети `/24` можно выделить ~20 адресов под системные нагрузки, остальные — в пул DHCP.

## Создание vApp

1. Перейдите: «Data Centers → vApps → NEW → New vApp».  
   ![vApp, шаг 1](../../../../images/cloud-provider-vcd/application-setup/Screenshot.png)

1. Укажите имя и включите vApp.  
   ![vApp, шаг 2](../../../../images/cloud-provider-vcd/application-setup/Screenshot2.png)

### Добавление сети в vApp

1. Перейдите: «Data Centers → vApps», выберите нужный vApp.  
   ![vApp network, шаг 1](../../../../images/cloud-provider-vcd/network-in-vapp-setup/Screenshot.png)

1. Откройте вкладку «Networks» и нажмите «NEW».  
   ![vApp network, шаг 2](../../../../images/cloud-provider-vcd/network-in-vapp-setup/Screenshot2.png)

1. В появившемся окне выберите тип `Direct` и выберите сеть.  
   ![vApp network, шаг 3](../../../../images/cloud-provider-vcd/network-in-vapp-setup/Screenshot3.png)

## Настройка входящего трафика

- Настройте DNAT на адреса из внутренней сети, которые поднимаются с помощью MetalLB.
- Пробрасываются порты 80 (HTTP), 443 (HTTPS), 22 (SSH к control plane).

### DNAT на Edge Gateway

1. Перейдите: «Networking → Edge Gateways», откройте нужный gateway.  
   ![DNAT шаг 1](../../../../images/cloud-provider-vcd/edge-gateway-setup/Screenshot.png)

1. Перейдите: «Services → NAT».  
   ![DNAT шаг 2](../../../../images/cloud-provider-vcd/edge-gateway-setup/Screenshot2.png)

1. Добавьте правила:  
   ![DNAT шаг 3](../../../../images/cloud-provider-vcd/edge-gateway-setup/Screenshot3.png)

## Настройка firewall

1. Перейдите в «Security → IP Sets».  
   ![Firewall шаг 1](../../../../images/cloud-provider-vcd/edge-firewall/Screenshot.png)

1. Создайте набор IP-адресов:  
   ![Firewall шаг 2](../../../../images/cloud-provider-vcd/edge-firewall/Screenshot2.png)
   ![Firewall шаг 3](../../../../images/cloud-provider-vcd/edge-firewall/Screenshot3.png)
   ![Firewall шаг 4](../../../../images/cloud-provider-vcd/edge-firewall/Screenshot4.png)

1. Добавьте правила:  
   ![Firewall шаг 5](../../../../images/cloud-provider-vcd/edge-firewall/Screenshot5.png)

## Подготовка шаблона виртуальной машины

> **Важно:** Поддерживается только Ubuntu 22.04

1. Скачайте [OVA-файл Ubuntu 22.04](https://cloud-images.ubuntu.com/jammy/).  
   ![Шаблон шаг 1](../../../../images/cloud-provider-vcd/template/Screenshot.png)

1. Перейдите: «Libraries → Catalogs → Каталог организации».  
   ![Шаблон шаг 2](../../../../images/cloud-provider-vcd/template/Screenshot2.png)

1. Загрузите файл в каталог.  
   ![Шаблон шаг 3](../../../../images/cloud-provider-vcd/template/Screenshot3.png)
   ![Шаблон шаг 4](../../../../images/cloud-provider-vcd/template/Screenshot4.png)
   ![Шаблон шаг 5](../../../../images/cloud-provider-vcd/template/Screenshot5.png)

1. Создайте виртуальную машину:  
   ![Шаблон шаг 6](../../../../images/cloud-provider-vcd/template/Screenshot6.png)
   ![Шаблон шаг 7](../../../../images/cloud-provider-vcd/template/Screenshot7.png)

1. Укажите SSH-ключ и пароль.  
   ![Шаблон шаг 8](../../../../images/cloud-provider-vcd/template/Screenshot8.png)

1. Запустите ВМ, дождитесь IP-адреса и пробросьте порт 22.  
   ![Шаблон шаг 9](../../../../images/cloud-provider-vcd/template/Screenshot9.png)

1. Выполните на ВМ:

   ```bash
   echo -e '\n[deployPkg]\nwait-cloudinit-timeout=1800\n' >> /etc/vmware-tools/tools.conf
   passwd -d ubuntu
   passwd -d root
   rm /home/ubuntu/.ssh/authorized_keys
   history -c
   shutdown -P now
   ```

1. Выключите машину и сохраните как шаблон:  
   ![Шаблон шаг 10](../../../../images/cloud-provider-vcd/template/Screenshot10.png)  
   ![Шаблон шаг 11](../../../../images/cloud-provider-vcd/template/Screenshot11.png)

> После создания шаблона обязательно попросите провайдера включить параметр `disk.enableUUID`.

