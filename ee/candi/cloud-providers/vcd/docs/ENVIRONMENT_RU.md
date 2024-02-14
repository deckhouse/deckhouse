---
title: "Cloud provider — Vcloud Director: подготовка окружения."
description: "Подготовка окружения Vcloud Director для работы Deckhouse cloud provider."
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

[](../../images/030-cloud-provider-vcd/network-setup/Screenshot.png)
[](../../images/030-cloud-provider-vcd/network-setup/Screenshot2.png)
[](../../images/030-cloud-provider-vcd/network-setup/Screenshot3.png)
[](../../images/030-cloud-provider-vcd/network-setup/Screenshot4.png)
[](../../images/030-cloud-provider-vcd/network-setup/Screenshot5.png)
[](../../images/030-cloud-provider-vcd/network-setup/Screenshot6.png)

### Добавление vApp

[](../../images/030-cloud-provider-vcd/application-setup/Screenshot.png)
[](../../images/030-cloud-provider-vcd/application-setup/Screenshot2.png)

### Добавление сети к vApp

[](../../images/030-cloud-provider-vcd/network-in-vapp-setup/Screenshot.png)
[](../../images/030-cloud-provider-vcd/network-in-vapp-setup/Screenshot2.png)

### Настройка правил DNAT на EDGE gateway

[](../../images/030-cloud-provider-vcd/edge-gateway-setup/Screenshot.png)
[](../../images/030-cloud-provider-vcd/edge-gateway-setup/Screenshot2.png)

## Каталог

* Вы можете загрузить облачные образы дистрибутивов (например, для [Ubuntu](https://cloud-images.ubuntu.com/)) в Каталог и использовать их в дальнейшем при создании машин.
* Облачный образ должен поддерживать cloud-init.

### Входящий трафик

* Вы должны направить входящий трафик на EDGE router (порты 80, 443) при помощи правил DNAT на выделенный адрес во внутренней сети.
* Этот адрес поднимается при помощи MetalLB в L2 режиме на выделенных frontend-узлах.

### Использование хранилища

* VCD поддерживает CSI, диски создаются как VCD Independent Disks.
* Известное ограничение - CSI диски не поддерживают ресайз.
