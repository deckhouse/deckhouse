---
title: Подготовка DVP к production
permalink: ru/virtualization-platform/guides/production.html
description: Рекомендации по подготовке Deckhouse Virtualization Platform для работы в продуктивной среде.
documentation_state: develop
lang: ru
---

Приведенные ниже рекомендации могут быть неактуальны для тестового кластера или кластера разработки, но важны для production-кластера.

## Канал и режим обновлений

{% alert %}
Используйте канал обновлений `Early Access` или `Stable`. Установите [окно автоматических обновлений](/products/kubernetes-platform/documentation/v1/modules/deckhouse/usage.html#конфигурация-окон-обновлений) или [ручной режим](/products/kubernetes-platform/documentation/v1/modules/deckhouse/usage.html#ручное-подтверждение-обновлений).
{% endalert %}

Выберите [канал обновлений](../documentation/about/release-channels.html) и [режим обновлений](../documentation/admin/update/update.html##каналы-обновлений), который соответствует вашим ожиданиям. Чем стабильнее канал обновлений, тем позже до него доходит новая функциональность.

По возможности используйте разные каналы обновлений для кластеров. Для кластера разработки используйте менее стабильный канал обновлений, нежели для тестового или stage-кластера (pre-production-кластер).

Мы рекомендуем использовать канал обновлений `Early Access` или `Stable` для production-кластеров. Если в production-окружении больше одного кластера, предпочтительно использовать для них разные каналы обновлений. Например, `Early Access` для одного, а `Stable` — для другого. Если использовать разные каналы обновлений по каким-либо причинам невозможно, рекомендуется устанавливать разные окна обновлений.

{% alert level="warning" %}
Даже в очень нагруженных и критичных кластерах не стоит отключать использование канала обновлений. Лучшая стратегия — плановое обновление. В инсталляциях платформы, которые не обновлялись полгода или более, могут присутствовать ошибки. Как правило, эти ошибки давно устранены в новых версиях. В этом случае оперативно решить возникшую проблему будет непросто.
{% endalert %}

Управление [окнами обновлений](/products/kubernetes-platform/documentation/v1/modules/deckhouse/configuration.html#parameters-update-windows) позволяет планово обновлять релизы платформы в автоматическом режиме в периоды «затишья», когда нагрузка на кластер далека от пиковой.

## Версия Kubernetes

{% alert %}
Используйте автоматический [выбор версии Kubernetes](/products/kubernetes-platform/documentation/v1/installing/configuration.html#clusterconfiguration-kubernetesversion) либо установите версию явно.
{% endalert %}

В большинстве случаев предпочтительно использовать автоматический выбор версии Kubernetes. В платформе такое поведение установлено по умолчанию, но его можно изменить с помощью параметра [kubernetesVersion](/products/kubernetes-platform/documentation/v1/installing/configuration.html#clusterconfiguration-kubernetesversion). Обновление версии Kubernetes в кластере не оказывает влияния на приложения и проходит [последовательно и безопасно](/products/kubernetes-platform/documentation/v1/modules/control-plane-manager/#управление-версиями).

Если указан автоматический выбор версии Kubernetes, платформа может обновить версию Kubernetes в кластере при обновлении релиза платформы (при обновлении минорной версии). Когда версия Kubernetes явно прописана в параметре [kubernetesVersion](/products/kubernetes-platform/documentation/v1/installing/configuration.html#clusterconfiguration-kubernetesversion), очередное обновление платформы может завершиться неудачей, если окажется, что используемая в кластере версия Kubernetes более не поддерживается.

Если приложение использует устаревшие версии ресурсов или требует конкретной версии Kubernetes по какой-либо другой причине, проверьте, что эта версия [поддерживается](/products/kubernetes-platform/documentation/v1/supported_versions.html), и [установите ее явно](/products/kubernetes-platform/documentation/v1/deckhouse-faq.html#как-обновить-версию-kubernetes-в-кластере).

## Требования к ресурсам

{% alert %}
Выделяйте от 4 CPU / 8 ГБ RAM на инфраструктурные узлы. Для мастер-узлов и узлов мониторинга используйте быстрые диски.
Учтите, что при использовании программно определяемых хранилищ, на узлах потребуются дополнительные диски для хранения данных.
{% endalert %}

Рекомендуются следующие минимальные ресурсы для инфраструктурных узлов в зависимости от их роли в кластере:

- **Мастер-узел** — 4 CPU, 8 ГБ RAM, 60 ГБ дискового пространства на быстром диске (400+ IOPS);
- **Frontend-узел** — 2 CPU, 4 ГБ RAM, 50 ГБ дискового пространства;
- **Узел мониторинга** (для нагруженных кластеров) — 4 CPU, 8 ГБ RAM; 50 ГБ дискового пространства на быстром диске (400+ IOPS).
- **Системный узел**:
  - 2 CPU, 4 ГБ RAM, 50 ГБ дискового пространства — если в кластере есть выделенные узлы мониторинга;
  - 4 CPU, 8 ГБ RAM, 60 ГБ дискового пространства на быстром диске (400+ IOPS) — если в кластере нет выделенных узлов мониторинга.
- **Worker-узел** — требования аналогичны требованиям к master-узлу, но во многом зависят от характера запускаемой на узле (узлах) нагрузки.

Примерный расчет ресурсов, необходимых для кластера:

- **Типовой кластер**: 3 мастер-узла, 2 frontend-узла, 2 системных узла. Такая конфигурация потребует **от 24 CPU и 48 ГБ RAM**, плюс быстрые диски с 400+ IOPS для мастер-узлов.
- **Кластер с повышенной нагрузкой** (с выделенными узлами мониторинга): 3 мастер-узла, 2 frontend-узла, 2 системных узла, 2 узла мониторинга. Такая конфигурация потребует **от 28 CPU и 64 ГБ RAM**, плюс быстрые диски с 400+ IOPS для мастер-узлов и узлов мониторинга.
- Для компонентов платформы желательно выделить отдельный [storageClass](/products/kubernetes-platform/documentation/v1/deckhouse-configure-global.html#parameters-storageclass) на быстрых дисках.
- Добавьте к этому worker-узлы с учетом характера полезной нагрузки.

## Особенности конфигурации

### Мастер-узлы

{% alert %}
В кластере должно быть три мастер-узла с быстрыми дисками 400+ IOPS.
{% endalert %}

Всегда используйте три мастер-узла — такое количество обеспечит отказоустойчивость и позволит безопасно выполнять обновление мастер-узлов. В большем числе мастер-узлов нет необходимости, а два узла не обеспечат кворума.

Может быть полезно:

- [Работа со статическими узлами...](/products/kubernetes-platform/documentation/latest/modules/node-manager/#работа-со-статическими-узлами)

### Frontend-узлы

{% alert %}
Выделите два или более frontend-узла.

Используйте инлет `HostPort` с внешним балансировщиком.
{% endalert %}

Frontend-узлы балансируют входящий трафик. На них работают Ingress-контроллеры. У NodeGroup frontend-узлов установлен label `node-role.deckhouse.io/frontend`. Читайте подробнее про [выделение узлов под определенный вид нагрузки...](/products/kubernetes-platform/documentation/v1/#выделение-узлов-под-определенный-вид-нагрузки)

Используйте более одного frontend-узла. Frontend-узлы должны выдерживать трафик при отказе как минимум одного frontend-узла.

Например, если в кластере два frontend-узла, то каждый frontend-узел должен справляться со всей нагрузкой на кластер в случае, если второй выйдет из строя. Если в кластере три frontend-узла, то каждый frontend-узел должен выдерживать увеличение нагрузки как минимум в полтора раза.

Платформа поддерживает три способа поступления трафика из внешнего мира:

- `HostPort` — устанавливается Ingress-контроллер, который доступен на портах узлов через `hostPort`;
- `HostPortWithProxyProtocol` — устанавливается Ingress-контроллер, который доступен на портах узлов через `hostPort` и использует proxy-protocol для получения настоящего IP-адреса клиента;
- `HostWithFailover` — устанавливаются два Ingress-контроллера (основной и резервный).

{% alert level="warning" %}
Инлет `HostWithFailover` подходит для кластеров с одним frontend-узлом. Он позволяет сократить время недоступности Ingress-контроллера при его обновлении. Такой тип инлета подойдет, например, для важных сред разработки, но **не рекомендуется для production**.
{% endalert %}

Подробнее про настройку сети: [управление сетью](/products/virtualization-platform/documentation/admin/platform-management/network/vm-network.html)

### Узлы мониторинга

{% alert %}
Для нагруженных кластеров выделите два узла мониторинга с быстрыми дисками.
{% endalert %}

Узлы мониторинга служат для запуска Grafana, Prometheus и других компонентов мониторинга. У [NodeGroup](/products/kubernetes-platform/documentation/v1/modules/node-manager/cr.html#nodegroup) узлов мониторинга установлен label `node-role.deckhouse.io/monitoring`.

В нагруженных кластерах со множеством алертов и большими объемами метрик под мониторинг рекомендуется выделить отдельные узлы. Если этого не сделать, компоненты мониторинга будут размещены на [системных узлах](#системные-узлы).

При выделении узлов под мониторинг важно, чтобы на них были быстрые диски. Для этого можно привязать `storageClass` на быстрых дисках ко всем компонентам платформы (глобальный параметр [storageClass](/products/kubernetes-platform/documentation/v1/deckhouse-configure-global.html#parameters-storageclass)) или выделить отдельный `storageClass` только для компонентов мониторинга (параметры [storageClass](/products/kubernetes-platform/documentation/v1/modules/prometheus/configuration.html#parameters-storageclass) и [longtermStorageClass](/products/kubernetes-platform/documentation/v1/modules/prometheus/configuration.html#parameters-longtermstorageclass) модуля `prometheus`).

Если кластер изначально создается с узлами, выделенными под определенный вид нагрузки (системные узлы, узлы под мониторинг и т. п.), то для модулей использующих тома постоянного хранилища (например, для модуля `prometheus`), рекомендуется явно указывать соответствующий nodeSelector в конфигурации модуля. Например, для модуля `prometheus` это параметр [nodeSelector](/products/kubernetes-platform/documentation/v1/modules/prometheus/configuration.html#parameters-nodeselector).

### Системные узлы

{% alert %}
Выделите два системных узла.
{% endalert %}

Системные узлы предназначены для запуска модулей платформы. У [NodeGroup](/products/kubernetes-platform/documentation/v1/modules/node-manager/cr.html#nodegroup) системных узлов установлен label `node-role.deckhouse.io/system`.

Выделите два системных узла. В этом случае модули платформы будут работать на них, не пересекаясь с пользовательскими приложениями кластера. Читайте подробнее про [выделение узлов под определенный вид нагрузки...](/products/kubernetes-platform/documentation/v1/#выделение-узлов-под-определенный-вид-нагрузки).

Компонентам платформы желательно выделить быстрые диски (глобальный параметр [storageClass](/products/kubernetes-platform/documentation/v1/deckhouse-configure-global.html#parameters-storageclass)).

## Уведомление о событиях мониторинга

{% alert %}
Настройте отправку алертов через [внутренний](/products/kubernetes-platform/documentation/v1/modules/prometheus/faq.html#как-добавить-alertmanager) Alertmanager или подключите [внешний](/products/kubernetes-platform/documentation/v1/modules/prometheus/faq.html#как-добавить-внешний-дополнительный-alertmanager).
{% endalert %}

Мониторинг будет работать сразу после установки платформы, однако для production этого недостаточно. Чтобы получать уведомления об инцидентах, настройте [встроенный](/products/kubernetes-platform/documentation/v1/modules/prometheus/faq.html#как-добавить-alertmanager) в платформе Alertmanager или [подключите свой](/products/kubernetes-platform/documentation/v1/modules/prometheus/faq.html#как-добавить-внешний-дополнительный-alertmanager) Alertmanager.

С помощью custom resource [CustomAlertmanager](/products/kubernetes-platform/documentation/v1/modules/prometheus/cr.html#customalertmanager) можно настроить отправку уведомлений на [электронную почту](/products/kubernetes-platform/documentation/v1/modules/prometheus/cr.html#customalertmanager-v1alpha1-spec-internal-receivers-emailconfigs), в [Slack](/products/kubernetes-platform/documentation/v1/modules/prometheus/cr.html#customalertmanager-v1alpha1-spec-internal-receivers-slackconfigs), в [Telegram](/products/kubernetes-platform/documentation/v1/modules/prometheus/usage.html#отправка-алертов-в-telegram), через [webhook](/products/kubernetes-platform/documentation/v1/modules/prometheus/cr.html#customalertmanager-v1alpha1-spec-internal-receivers-webhookconfigs), а также другими способами.

<!-- ## Сбор логов

{% alert %}
[Настройте](/products/kubernetes-platform/documentation/v1/modules/log-shipper/) централизованный сбор логов.
{% endalert %}

Настройте централизованный сбор логов с системных и пользовательских приложений с помощью модуля [log-shipper](/products/kubernetes-platform/documentation/v1/modules/log-shipper/).

Достаточно создать custom resource с описанием того, *что нужно собирать*: [ClusterLoggingConfig](/products/kubernetes-platform/documentation/v1/modules/log-shipper/cr.html#clusterloggingconfig) или [PodLoggingConfig](/products/kubernetes-platform/documentation/v1/modules/log-shipper/cr.html#podloggingconfig); кроме того, необходимо создать custom resource с данными о том, *куда отправлять* собранные логи: [ClusterLogDestination](/products/kubernetes-platform/documentation/v1/modules/log-shipper/cr.html#clusterlogdestination).

Дополнительная информация:
- [Пример для Grafana Loki](/products/kubernetes-platform/documentation/v1/modules/log-shipper/examples.html#чтение-логов-из-всех-подов-кластера-и-направление-их-в-loki)
- [Пример для Logstash](/products/kubernetes-platform/documentation/v1/modules/log-shipper/examples.html#простой-пример-logstash)
- [Пример для Splunk](/products/kubernetes-platform/documentation/v1/modules/log-shipper/examples.html#пример-интеграции-со-splunk)
-->

## Резервное копирование

{% alert %}
Настройте [резервное копирование etcd](/products/virtualization-platform/documentation/admin/platform-management/control-plane-settings/etcd.html#резервное-копирование-etcd). Напишите план восстановления.
{% endalert %}

Обязательно настройте [резервное копирование данных etcd](/products/virtualization-platform/documentation/admin/platform-management/control-plane-settings/etcd.html#резервное-копирование-etcd). Это будет ваш последний шанс на восстановление кластера в случае самых неожиданных событий. Храните резервные копии как можно *дальше* от кластера.

Резервные копии не помогут, если они не работают или вы не знаете, как их использовать для восстановления. Рекомендуем составить [план восстановления на случай аварии](https://habr.com/ru/search/?q=%5BDRP%5D&target_type=posts&order=date) (Disaster Recovery Plan), содержащий конкретные шаги и команды по развертыванию кластера из резервной копии.

Этот план должен периодически актуализироваться и проверяться учебными тревогами.

## Сообщество

{% alert %}
Следите за новостями проекта в [Telegram](https://t.me/deckhouse_ru).
{% endalert %}

Вступите в [сообщество](/community/about.html), чтобы быть в курсе важных изменений и новостей. Вы сможете общаться с людьми, занятыми общим делом. Это позволит избежать многих типичных проблем.

Команда платформы знает, каких усилий требует организация работы production-кластера в Kubernetes. Мы будем рады, если платформа позволит вам реализовать задуманное. Поделитесь своим опытом и вдохновите других на переход в Kubernetes.
