---
title: Модуль cni-simple-bridge
permalink: ru/architecture/network/cni-simple-bridge.html
lang: ru
search: cni-simple-bridge, bridge, host-local
description: Архитектура модуля cni-simple-bridge в Deckhouse Platform.
---

Модуль [`cni-simple-bridge`](/modules/cni-simple-bridge/) обеспечивает базовую работу сети для подов в Deckhouse Platform и реализует [Container Network Interface (CNI)](https://github.com/containernetworking/CNI).

{% alert level="info" %}
CNI-плагин `cni-simple-bridge` использовался по умолчанию для модулей [`cloud-provider-aws`](/modules/cloud-provider-aws/), [`cloud-provider-azure`](/modules/cloud-provider-azure/), [`cloud-provider-gcp`](/modules/cloud-provider-gcp/) и [`cloud-provider-yandex`](/modules/cloud-provider-yandex/) в версиях платформы Deckhouse до 1.77.0. Начиная с версии платформы 1.77.0 для указанных модулей по умолчанию используется CNI Cilium модуля [`cni-cilium`](/modules/cni-cilium/).
{% endalert %}

Модуль не имеет настраиваемых параметров.

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

* На схеме показано, что контейнеры разных подов взаимодействуют друг с другом напрямую. Фактически они взаимодействуют через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса указано над стрелкой.
* Поды могут быть запущены в нескольких репликах, однако на схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура модуля [`cni-simple-bridge`](/modules/cni-simple-bridge/) на уровне 2 модели C4 и его взаимодействия с другими компонентами DP изображена на следующей диаграмме:

![Архитектура модуля cni-simple-bridge](../../images/architecture/network/c4-l2-cni-simple-bridge.ru.svg)

## Компоненты модуля

Модуль состоит из следующих компонентов:

1. **Simple-bridge** (DaemonSet) — компонент запускается на каждом узле кластера и подготавливает CNI-плагин к работе. Для этого компонент получает подсеть подов из поля `spec.podCIDR` ресурса Node и формирует конфигурацию для запуска исполняемого файла `bridge` компонентом containerd при каждом создании или удалении сетевого пространства имён пода. При вызове `bridge` запрашивает выделение IP-адреса у плагина IP Address Management ([IPAM](../../glossary.html#sdn-ipam)) `host-local`.

   Плагины `bridge` и `host-local` являются стандартными [Open Source-плагинами CNI](https://github.com/containernetworking/plugins), устанавливаемыми на каждый узел кластера независимо от этого модуля, в рамках установки kubelet.

   Состоит из следующих контейнеров:

   * **cni-migration-init-checker** — init-контейнер, ожидающий завершения процесса миграции с другого CNI-плагина, если такая миграция была запущена;
   * **iptables-wrapper** — init-контейнер, подготавливающий исполняемые файлы для работы с iptables;
   * **simple-bridge** — основной контейнер.

## Взаимодействия модуля

Модуль взаимодействует с компонентом **Kube-apiserver**:

- читает ресурсы Node;
- следит за кастомными ресурсами [CNIMigration](../../reference/api/cr.html#cnimigration) и [CNINodeMigration](../../reference/api/cr.html#cninodemigration).
