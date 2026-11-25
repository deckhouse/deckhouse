---
title: Настройка интеграции с Basis Dynamix
permalink: ru/admin/integrations/private/dynamix/configuration.html
lang: ru
---

## Обязательные параметры

- `layout` — [схема размещения](../../../../glossary.html#layout);
- `sshPublicKey` — публичный ключ для доступа к узлам;
- `location` — имя локации Basis Dynamix, в которой создаётся группа ресурсов кластера;
- `account` — имя аккаунта в облаке;
- `storagePolicy` — имя политики хранения (storage policy), которая по умолчанию используется для дисков виртуальных машин;
- `provider.controllerUrl`, `oAuth2Url`, `appId`, `appSecret` — параметры доступа к API;
- `imageName` — название образа ОС;
- `externalNetwork` — имя внешней сети;
- `nodeNetworkCIDR` — адресное пространство внутренней сети (только для схемы `StandardWithInternalNetwork`).

{% alert level="warning" %}
Смена `storagePolicy` пересоздаёт узлы с типом `CloudEphemeral`. Какие узлы пересоздаются, описано [в разделе «Смена политики хранения»](load-balancing-and-storage.html#смена-политики-хранения).
{% endalert %}

Чтобы изменения параметров вступили в силу, выполните [`dhctl converge`](../../../configuration/#изменение-конфигурации-кластера).

## DNS-серверы

Если внешняя сеть не передаёт узлам адреса DNS-серверов, задайте их в параметре [`nameservers`](/modules/cloud-provider-dynamix/cluster_configuration.html#dynamixclusterconfiguration-nameservers).
