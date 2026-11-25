---
title: Хранилище и балансировка нагрузки в VMware Cloud Director
permalink: ru/admin/integrations/virtualization/vcd/load-balancing-and-storage.html
lang: ru
---

## Использование хранилища

* VCD поддерживает CSI. Диски создаются как VCD Independent Disks.
* Guest property `disk.EnableUUID` должно быть разрешено для используемых шаблонов виртуальных машин.
* Deckhouse Platform поддерживает изменение размера дисков с версии v1.59.1.

Имя StorageClass'а, который будет использоваться в кластере по умолчанию, можно настроить (параметр [default](/modules/cloud-provider-vcd/configuration.html#parameters-storageclass-default)) и отфильтровать ненужные StorageClass'ы (параметр [exclude](/modules/cloud-provider-vcd/configuration.html#parameters-storageclass-exclude)).

## Использование балансировщика нагрузки

* Компоненты Deckhouse Platform поддерживают ресурсов Service типа LoadBalancer при установке в VMware Cloud Director (VCD).
* В качестве балансировщика используется VMware NSX Advanced Load Balancer (ALB или Avi Networks).
* Поддержка доступна **только** при использовании платформы виртуализации сети `NSX-T`.
* Балансировка должна быть включёна на Edge Gateway вашим провайдером VCD. Проверить, включёна ли балансировка, можно в разделе Edge Gateway → Load Balancer → General Settings — параметр `State` должен быть в статусе `Active`.
* Если балансировщик был активирован после успешного создания кластера Deckhouse Platform, компоненты автоматически подхватят изменения в течение часа (дополнительных действий не требуется).
* Для каждого открытого порта создаётся связка Pool + Virtual Service.
* При наличии межсетевого экрана необходимо создать разрешающее правило для внешнего IP-адреса балансировщика и соответствующих портов.
