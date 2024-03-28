---
title: "Cloud provider — VMware Cloud Director: подготовка окружения."
description: "Подготовка окружения VMware Cloud Director для работы Deckhouse cloud provider."
---

<!-- AUTHOR! Don't forget to update getting started if necessary -->

## Список необходимых ресурсов VCD

* **Organization**
* **VirtualDataCenter**
* **StoragePolicy**
* **SizingPolicy**
* **Network**
* **EdgeRouter**
* **Catalog**

### Добавление сети

Create internal network and connect it to Edge Gateway.

![Добавление сети, шаг 1](../../images/030-cloud-provider-vcd/network-setup/Screenshot.png)
![Добавление сети, шаг 2](../../images/030-cloud-provider-vcd/network-setup/Screenshot2.png)
![Добавление сети, шаг 3](../../images/030-cloud-provider-vcd/network-setup/Screenshot3.png)
![Добавление сети, шаг 4](../../images/030-cloud-provider-vcd/network-setup/Screenshot4.png)
![Добавление сети, шаг 5](../../images/030-cloud-provider-vcd/network-setup/Screenshot5.png)
![Добавление сети, шаг 6](../../images/030-cloud-provider-vcd/network-setup/Screenshot6.png)

### Добавление vApp

![Добавление vApp, шаг 1](../../images/030-cloud-provider-vcd/application-setup/Screenshot.png)
![Добавление vApp, шаг 2](../../images/030-cloud-provider-vcd/application-setup/Screenshot2.png)

### Добавление сети к vApp

![Добавление сети к vApp, шаг 1](../../images/030-cloud-provider-vcd/network-in-vapp-setup/Screenshot.png)
![Добавление сети к vApp, шаг 2](../../images/030-cloud-provider-vcd/network-in-vapp-setup/Screenshot2.png)

### Настройка правил DNAT на EDGE gateway

![Настройка правил DNAT на EDGE gateway, шаг 1](../../images/030-cloud-provider-vcd/edge-gateway-setup/Screenshot.png)
![Настройка правил DNAT на EDGE gateway, шаг 2](../../images/030-cloud-provider-vcd/edge-gateway-setup/Screenshot2.png)

## Каталог

* Вы можете загрузить облачные образы дистрибутивов (например, для [Ubuntu](https://cloud-images.ubuntu.com/)) в Каталог и использовать их в дальнейшем при создании машин.
* Облачный образ должен поддерживать cloud-init.

### Входящий трафик

* Вы должны направить входящий трафик на EDGE router (порты 80, 443) при помощи правил DNAT на выделенный адрес во внутренней сети.
* Этот адрес поднимается при помощи MetalLB в L2 режиме на выделенных frontend-узлах.

### Использование хранилища

* VCD поддерживает CSI, диски создаются как VCD Independent Disks.
* Guest property `disk.EnableUUID` должно быть разрешено для используемых темплейтов машин.
* Известное ограничение - CSI диски не поддерживают ресайз.
