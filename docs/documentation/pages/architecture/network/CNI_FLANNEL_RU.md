---
title: Модуль cni-flannel
permalink: ru/architecture/network/cni-flannel.html
lang: ru
search: cni-flannel, flannel, vxlan, host-gw
description: Архитектура модуля cni-flannel в Deckhouse Platform.
---

Модуль [`cni-flannel`](/modules/cni-flannel/) обеспечивает работу сети между узлами в кластере Deckhouse Platform. Модуль основан на проекте [Flannel](https://github.com/flannel-io/flannel) и реализует [Container Network Interface (CNI)](https://github.com/containernetworking/CNI).

Подробнее с описанием настройки и примерами использования модуля можно ознакомиться [в разделе документации модуля](/modules/cni-flannel/).

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

* На схеме показано, что контейнеры разных подов взаимодействуют друг с другом напрямую. Фактически они взаимодействуют через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса указано над стрелкой.
* Поды могут быть запущены в нескольких репликах, однако на схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура модуля [`cni-flannel`](/modules/cni-flannel/) на уровне 2 модели C4 и его взаимодействия с другими компонентами платформы Deckhouse изображена на следующей диаграмме:

![Архитектура модуля cni-flannel](../../images/architecture/network/c4-l2-cni-flannel.ru.svg)

## Компоненты модуля

Модуль состоит из следующих компонентов:

1. **Flannel** (DaemonSet) — компонент запускается на каждом узле кластера и реализует L3-оверлей-сеть Flannel между узлами.

   В зависимости от параметра [`podNetworkMode`](/modules/cni-flannel/configuration.html#parameters-podnetworkmode) используется соответствующий бэкенд host-gw или VXLAN.

   Компонент берёт подсеть подов из поля `spec.podCIDR` ресурса Node и записывает полученные параметры подсети (сеть, подсеть, MTU) в файл `/run/flannel/subnet.env` на узле. Этот файл используется CNI-плагином Flannel при каждом создании или удалении пода.

   Состоит из следующих контейнеров:

   * **cni-migration-init-checker** — init-контейнер, ожидающий завершения процесса миграции с другого CNI-плагина, если такая миграция была запущена;
   * **iptables-wrapper-init** — init-контейнер, выполняющий подготовку необходимых для работы с iptables исполняемых файлов;
   * **handle-vxlan-offload** — опциональный init-контейнер, исправляющий настройки UDP-сегментации на сетевом интерфейсе узла. Создаётся [контроллером Deckhouse](../../glossary.html#deckhouse-controller), если параметр [`podNetworkMode`](/modules/cni-flannel/configuration.html#parameters-podnetworkmode) принимает значение `VXLAN`;
   * **kube-flannel** — основной контейнер.

1. **Flannel (CNI-плагин)** — исполняемый файл, запускаемый компонентом containerd в соответствии со [спецификацией CNI](https://www.cni.dev/docs/spec/#cni-operations), например ADD при создании сетевого пространства имён пода и DEL при его удалении.

   При запуске компонент читает параметры подсети из файла `/run/flannel/subnet.env` и делегирует создание сетевого интерфейса пода CNI-плагину `bridge`, который, в свою очередь, запрашивает выделение IP-адреса у плагина IP Address Management ([IPAM](../../glossary.html#sdn-ipam)) `host-local`. Оба плагина, `bridge` и `host-local`, являются стандартными [Open Source-плагинами CNI](https://github.com/containernetworking/plugins), устанавливаемыми на каждый узел кластера независимо от этого модуля, в рамках установки kubelet.

## Взаимодействия модуля

Модуль взаимодействует со следующими компонентами:

1. **Kube-apiserver**:

   - следит за кастомными ресурсами [CNIMigration](../../reference/api/cr.html#cnimigration) и [CNINodeMigration](../../reference/api/cr.html#cninodemigration), а также ресурсами Pod и Node;
   - обновляет статус ресурса Node.

1. **Bridge (CNI-плагин)** — создаёт сетевой интерфейс пода.

С модулем взаимодействуют следующие внешние компоненты:

1. **Containerd** — запускает исполняемый файл flannel с определёнными командами в соответствии со спецификацией CNI.
