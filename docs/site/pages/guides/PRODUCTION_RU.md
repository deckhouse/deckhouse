---
title: Подготовка к production
permalink: ru/guides/production.html
description: Рекомендации по подготовке кластера Deckhouse Kubernetes Platform для работы в продуктивной среде.
lang: ru
layout: sidebar-guides
---

Приведенные ниже рекомендации могут быть неактуальны для тестового кластера или кластера разработки, но важны для production-кластера.

## Канал и режим обновлений

{% alert %}
Используйте канал обновлений `Early Access` или `Stable`. Установите [окно автоматических обновлений](/modules/deckhouse/usage.html#конфигурация-окон-обновлений) или [ручной режим](/modules/deckhouse/usage.html#ручное-подтверждение-обновлений).
{% endalert %}

Выберите [канал обновлений](/products/kubernetes-platform/documentation/v1/reference/release-channels.html) и [режим обновлений](/modules/deckhouse/configuration.html#parameters-releasechannel), который соответствует вашим ожиданиям. Чем стабильнее канал обновлений, тем позже до него доходит новая функциональность.

По возможности используйте разные каналы обновлений для кластеров. Для кластера разработки используйте менее стабильный канал обновлений, нежели для тестового или stage-кластера (pre-production-кластер).

Мы рекомендуем использовать канал обновлений `Early Access` или `Stable` для production-кластеров. Если в production-окружении более одного кластера, предпочтительно использовать для них разные каналы обновлений. Например, `Early Access` для одного, а `Stable` — для другого. Если использовать разные каналы обновлений по каким-либо причинам невозможно, рекомендуется устанавливать разные окна обновлений.

{% alert level="warning" %}
Даже в очень нагруженных и критичных кластерах не стоит отключать использование канала обновлений. Лучшая стратегия — плановое обновление. В инсталляциях Deckhouse, которые не обновлялись полгода или более, могут присутствовать ошибки. Как правило, эти ошибки давно устранены в новых версиях. В этом случае оперативно решить возникшую проблему будет непросто.
{% endalert %}

Управление [окнами обновлений](/modules/deckhouse/configuration.html#parameters-update-windows) позволяет планово обновлять релизы Deckhouse в автоматическом режиме в периоды «затишья», когда нагрузка на кластер далека от пиковой.

## Версия Kubernetes

{% alert %}
Используйте автоматический [выбор версии Kubernetes](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-kubernetesversion) либо установите версию явно.
{% endalert %}

В большинстве случаев предпочтительно использовать автоматический выбор версии Kubernetes. В Deckhouse такое поведение установлено по умолчанию, но его можно изменить с помощью параметра [kubernetesVersion](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-kubernetesversion). Обновление версии Kubernetes в кластере не оказывает влияния на приложения и проходит [последовательно и безопасно](/modules/control-plane-manager/#управление-версиями).

Если указан автоматический выбор версии Kubernetes, Deckhouse может обновить версию Kubernetes в кластере при обновлении релиза Deckhouse (при обновлении минорной версии). Когда версия Kubernetes явно прописана в параметре [kubernetesVersion](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-kubernetesversion), очередное обновление Deckhouse может завершиться неудачей, если окажется, что используемая в кластере версия Kubernetes более не поддерживается.

Если приложение использует устаревшие версии ресурсов или требует конкретной версии Kubernetes по какой-либо другой причине, проверьте, что эта версия [поддерживается](/products/kubernetes-platform/documentation/v1/supported_versions.html), и [установите ее](/products/kubernetes-platform/documentation/v1/deckhouse-faq.html#как-обновить-версию-kubernetes-в-кластере).

## Требования к ресурсам

{% alert %}
Выделяйте от 4 CPU / 8 ГБ RAM на инфраструктурные узлы. Для мастер-узлов и узлов мониторинга используйте быстрые диски.
{% endalert %}

Рекомендуются следующие минимальные ресурсы для инфраструктурных узлов в зависимости от их роли в кластере:
- **Мастер-узел** — 4 CPU, 8 ГБ RAM, 60 ГБ дискового пространства на быстром диске (400+ IOPS);
- **Frontend-узел** — 2 CPU, 4 ГБ RAM, 50 ГБ дискового пространства;
- **Узел мониторинга** (для нагруженных кластеров) — 4 CPU, 8 ГБ RAM; 50 ГБ дискового пространства на быстром диске (400+ IOPS).
- **Системный узел**:
  - 4 CPU, 8 ГБ RAM, 50 ГБ дискового пространства — если в кластере есть выделенные узлы мониторинга;
  - 8 CPU, 16 ГБ RAM, 60 ГБ дискового пространства на быстром диске (400+ IOPS) — если в кластере нет выделенных узлов мониторинга.
- **Worker-узел** — требования аналогичны требованиям к master-узлу, но во многом зависят от характера запускаемой на узле (узлах) нагрузки.

Примерный расчет ресурсов, необходимых для кластера:
- **Типовой кластер**: 3 мастер-узла, 2 frontend-узла, 2 системных узла. Такая конфигурация потребует **от 26 CPU и 52 ГБ RAM**, плюс быстрые диски с 400+ IOPS для мастер-узлов.
- **Кластер с повышенной нагрузкой** (с выделенными узлами мониторинга): 3 мастер-узла, 2 frontend-узла, 2 системных узла, 2 узла мониторинга. Такая конфигурация потребует **от 32 CPU и 72 ГБ RAM**, плюс быстрые диски с 400+ IOPS для мастер-узлов и узлов мониторинга.
- Для компонентов Deckhouse желательно выделить отдельный [storageClass](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-storageclass) на быстрых дисках.
- Добавьте к этому worker-узлы с учетом характера полезной нагрузки.

Также ознакомьтесь [с инструкцией](./hardware-requirements.html) по аппаратным требованиям к ресурсам кластера, в которой подробно описано, как подобрать необходимые ресурсы в зависимости от предполагаемой нагрузки.

## Особенности конфигурации

### Мастер-узлы

{% alert %}
В кластере должно быть три мастер-узла с быстрыми дисками 400+ IOPS.
{% endalert %}

Всегда используйте три мастер-узла — такое количество обеспечит отказоустойчивость и позволит безопасно выполнять обновление мастер-узлов. В большем числе мастер-узлов нет необходимости, а два узла не обеспечат кворума.

Конфигурация мастер-узлов для облачных кластеров настраивается в параметре [masterNodeGroup](/modules/cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration-masternodegroup).

Может быть полезно:

- [Как добавить мастер-узлы в облачном кластере...](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/control-plane/scaling-and-changing-master-nodes.html#добавление-master-узлов-в-облачном-кластере).
- [Работа со статическими узлами...](/products/kubernetes-platform/documentation/modules/node-manager/#работа-со-статическими-узлами).

### Frontend-узлы

{% alert %}
Выделите два или более frontend-узла.

Используйте инлет `LoadBalancer` для OpenStack и облачных сервисов, где возможен автоматический заказ балансировщика (Yandex Cloud, VK Cloud, Selectel Cloud, AWS, GCP, Azure и т. п.). Используйте инлет `HostPort` с внешним балансировщиком для bare metal или vSphere.
{% endalert %}

Frontend-узлы балансируют входящий трафик. На них работают Ingress-контроллеры. У [NodeGroup](/modules/node-manager/cr.html#nodegroup) frontend-узлов установлен label `node-role.deckhouse.io/frontend`. Читайте подробнее про [выделение узлов под определенный вид нагрузки...](/products/kubernetes-platform/documentation/v1/#выделение-узлов-под-определенный-вид-нагрузки)

Используйте более одного frontend-узла. Frontend-узлы должны выдерживать трафик при отказе как минимум одного frontend-узла.

Например, если в кластере два frontend-узла, то каждый frontend-узел должен справляться со всей нагрузкой на кластер в случае, если второй выйдет из строя. Если в кластере три frontend-узла, то каждый frontend-узел должен выдерживать увеличение нагрузки как минимум в полтора раза.

Выберите [тип инлета](/modules/ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-inlet) (определяет способ поступления трафика).

При развертывании кластера с помощью Deckhouse в облачной инфраструктуре, в которой поддерживается заказ балансировщиков (например, решения на базе OpenStack, сервисы Yandex Cloud, VK Cloud, Selectel Cloud, AWS, GCP, Azure и т. п.), используйте инлет `LoadBalancer` или `LoadBalancerWithProxyProtocol`.

В средах, в которых автоматический заказ балансировщиков недоступен (в bare-metal-кластерах, vSphere, некоторых решениях на базе OpenStack), используйте инлет `HostPort` или `HostPortWithProxyProtocol`. В этом случае можно либо добавить несколько A&#8209;записей в DNS для соответствующего домена, либо использовать внешний сервис балансировки нагрузки (например, взять решения от Cloudflare, Qrator или настроить metallb).

{% alert level="warning" %}
Инлет `HostWithFailover` подходит для кластеров с одним frontend-узлом. Он позволяет сократить время недоступности Ingress-контроллера при его обновлении. Такой тип инлета подойдет, например, для важных сред разработки, но **не рекомендуется для production**.
{% endalert %}

Алгоритм выбора инлета:

![Алгоритм выбора инлета](/images/guides/going_to_production/ingress-inlet-ru.svg)

### Узлы мониторинга

{% alert %}
Для нагруженных кластеров выделите два узла мониторинга с быстрыми дисками.
{% endalert %}

Узлы мониторинга служат для запуска Grafana, Prometheus и других компонентов мониторинга. У [NodeGroup](/modules/node-manager/cr.html#nodegroup) узлов мониторинга установлен label `node-role.deckhouse.io/monitoring`.

В нагруженных кластерах со множеством алертов и большими объемами метрик под мониторинг рекомендуется выделить отдельные узлы. Если этого не сделать, компоненты мониторинга будут размещены на [системных узлах](#системные-узлы).

При выделении узлов под мониторинг важно, чтобы на них были быстрые диски. Для этого можно привязать `storageClass` на быстрых дисках ко всем компонентам Deckhouse (глобальный параметр [storageClass](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-storageclass)) или выделить отдельный `storageClass` только для компонентов мониторинга (параметры [storageClass](/modules/prometheus/configuration.html#parameters-modules-storageclass) и [longtermStorageClass](/modules/prometheus/configuration.html#parameters-longtermstorageclass) модуля `prometheus`).

Если кластер изначально создается с узлами, выделенными под определенный вид нагрузки (системные узлы, узлы под мониторинг и т. п.), то для модулей использующих тома постоянного хранилища (например, для модуля `prometheus`), рекомендуется явно указывать соответствующий nodeSelector в конфигурации модуля. Например, для модуля `prometheus` это параметр [nodeSelector](/modules/prometheus/configuration.html#parameters-nodeselector).

### Системные узлы

{% alert %}
Выделите два системных узла.
{% endalert %}

Системные узлы предназначены для запуска модулей Deckhouse. У [NodeGroup](/modules/node-manager/cr.html#nodegroup) системных узлов установлен label `node-role.deckhouse.io/system`.

Выделите два системных узла. В этом случае модули Deckhouse будут работать на них, не пересекаясь с пользовательскими приложениями кластера. Читайте подробнее про [выделение узлов под определенный вид нагрузки...](/products/kubernetes-platform/documentation/v1/#выделение-узлов-под-определенный-вид-нагрузки).

Компонентам Deckhouse желательно выделить быстрые диски (глобальный параметр [storageClass](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-storageclass)).

## Уведомление о событиях мониторинга

{% alert %}
Настройте отправку алертов через [внутренний](/modules/prometheus/faq.html#как-добавить-alertmanager) Alertmanager или подключите [внешний](/modules/prometheus/faq.html#как-добавить-внешний-дополнительный-alertmanager).
{% endalert %}

Мониторинг будет работать сразу после установки Deckhouse, однако для production этого недостаточно. Чтобы получать уведомления об инцидентах, настройте [встроенный](/modules/prometheus/faq.html#как-добавить-alertmanager) в Deckhouse Alertmanager или [подключите свой](/modules/prometheus/faq.html#как-добавить-внешний-дополнительный-alertmanager) Alertmanager.

С помощью custom resource [CustomAlertmanager](/modules/prometheus/cr.html#customalertmanager) можно настроить отправку уведомлений на [электронную почту](/modules/prometheus/cr.html#customalertmanager-v1alpha1-spec-internal-receivers-emailconfigs), в [Slack](/modules/prometheus/cr.html#customalertmanager-v1alpha1-spec-internal-receivers-slackconfigs), в [Telegram](/modules/prometheus/usage.html#отправка-алертов-в-telegram), через [webhook](/modules/prometheus/cr.html#customalertmanager-v1alpha1-spec-internal-receivers-webhookconfigs), а также другими способами.

Список всех доступных алертов системы мониторинга Deckhouse Kubernetes Platform приведён на [отдельной странице документации](/products/kubernetes-platform/documentation/v1/reference/alerts.html).

## Сбор логов

{% alert %}
[Настройте](/modules/log-shipper/) централизованный сбор логов.
{% endalert %}

Настройте централизованный сбор логов с системных и пользовательских приложений с помощью модуля [log-shipper](/modules/log-shipper/).

Достаточно создать custom resource с описанием того, *что нужно собирать*: [ClusterLoggingConfig](/modules/log-shipper/cr.html#clusterloggingconfig) или [PodLoggingConfig](/modules/log-shipper/cr.html#podloggingconfig); кроме того, необходимо создать custom resource с данными о том, *куда отправлять* собранные логи: [ClusterLogDestination](/modules/log-shipper/cr.html#clusterlogdestination).

Дополнительная информация:
- [Пример для Grafana Loki](/modules/log-shipper/examples.html#чтение-логов-из-всех-подов-кластера-и-направление-их-в-loki)
- [Пример для Logstash](/modules/log-shipper/examples.html#простой-пример-logstash)
- [Пример для Splunk](/modules/log-shipper/examples.html#пример-интеграции-со-splunk)

## Резервное копирование

{% alert %}
Настройте [резервное копирование etcd](/modules/control-plane-manager/faq.html#как-сделать-бэкап-etcd-вручную). Напишите план восстановления.
{% endalert %}

**Важно.** Обязательно настройте [резервное копирование данных etcd](/modules/control-plane-manager/faq.html#как-сделать-бэкап-etcd-вручную) — это ваша последняя возможность восстановить кластер в случае непредвиденных событий. Храните резервные копии как можно *дальше* от кластера.

Резервные копии не помогут, если они не работают или вы не знаете, как их использовать для восстановления. Рекомендуем составить [план восстановления на случай аварии](https://habr.com/ru/search/?q=%5BDRP%5D&target_type=posts&order=date) (Disaster Recovery Plan), содержащий конкретные шаги и команды по развертыванию кластера из резервной копии.

Этот план должен периодически актуализироваться и проверяться учебными тревогами.

## Сообщество

{% alert %}
Следите за новостями проекта в [Telegram](https://t.me/deckhouse_ru).
{% endalert %}

Вступите в [сообщество](https://deckhouse.ru/community/about.html), чтобы быть в курсе важных изменений и новостей. Вы сможете общаться с людьми, занятыми общим делом. Это позволит избежать многих типичных проблем.

Команда Deckhouse знает, каких усилий требует организация работы production-кластера в Kubernetes. Мы будем рады, если Deckhouse позволит вам реализовать задуманное. Поделитесь своим опытом и вдохновите других на переход в Kubernetes.
