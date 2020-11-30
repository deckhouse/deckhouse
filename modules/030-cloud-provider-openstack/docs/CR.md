---
title: "Сloud provider — OpenStack: Custom Resources"
---

## OpenStackInstanceClass

Ресурс описывает параметры группы OpenStack servers, которые будет использовать `machine-controller-manager` из модуля [node-manager](/modules/040-node-manager/). На этот ресурс ссылается ресурс `CloudInstanceClass` из вышеупомянутого модуля.

Все опции идут в `.spec`.

* `flavorName` — тип заказываемых server'ов
    * Получить список всех доступных flavor'ов можно с помощью команды: `openstack flavor list`
    * Обязательный параметр.
    * Формат — строкa.
* `imageName` — имя образа.
    * **Внимание!** Сейчас поддерживается и тестируется только `Ubuntu 18.04` и `Ubuntu 20.04`.
    * Получить список всех доступных образов можно с помощью команды: `openstack image list`
    * Опциональный параметр.
    * Формат — строкa.
    * По умолчанию будет установлено значение либо из OpenStackCloudDiscoveryData, либо из настроек `instances.imageName`.
* `rootDiskSize` — если параметр не указан, то для инстанса используется локальный диск с размером указанным в flavor.
  Если параметр присутствует, то OpenStack server будет создан на Cinder volume с указанным размером и стандартным для кластера типом.
    * Опциональный параметр.
    * Формат — integer. В гигабайтах.
    > Если в *cloud provider* существует несколько типов дисков, то для выбора конкретного типа диска виртуальной машины у используемого образа можно установить тип диска по умолчанию, для этого необходимо в метаданных образа указать имя определённого типа диска
    > Для этого также может понадобиться создать свой собственный image в OpenStack, как это сделать описано в разделе ["Загрузка image в OpenStack"](faq.html#как-загрузить-image-в-openstack)
      > ```bash
        openstack volume type list
        openstack image set ubuntu-18-04-cloud-amd64 --property cinder_img_volume_type=VOLUME_NAME
        ```

* `mainNetwork` — путь до network, которая будет подключена к виртуальной машине, как основная сеть (шлюз по умолчанию).
    * Опциональный параметр.
    * Формат — строкa.
    * По умолчанию будет установлено значение из OpenStackCloudDiscoveryData.
* `additionalNetworks` - список сетей, которые будут подключены к инстансу.
    * Опциональный параметр.
    * Формат — массив строк.
    * Пример:

      ```yaml
      - enp6t4snovl2ko4p15em
      - enp34dkcinm1nr5999lu
      ```
    * По умолчанию будет установлено значение из OpenStackCloudDiscoveryData.
* `additionalSecurityGroups` — Список `securityGroups`, которые необходимо прикрепить к instances `OpenStackInstanceClass` в дополнение к указанным в конфигурации cloud провайдера. Используется для задания firewall правил по отношению к заказываемым instances.
    > SecurityGroups могут не поддерживаться облачным провайдером
    * Опциональный параметр.
    * Формат — массив строк.
    * Пример:

      ```yaml
      - sec_group_1
      - sec_group_2
      ```
* `additionalTags` — Словарь тегов, которые необходимо прикрепить к instances `OpenStackInstanceClass` в дополнение к указанным в конфигурации cloud провайдера.
    * Опциональный параметр.
    * Формат — ключ-значение.
    * Пример:
      ```yaml
      project: cms-production
      severity: critical
      ```
