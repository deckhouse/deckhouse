---
title: Подготовка к production
permalink: ru/guides/production.html
lang: ru
---

Ниже приведены рекомендации, которые могут быть не важны в тестовом кластере или кластере разработки, но могут иметь важное значение в production-кластере.

## Канал и режим обновлений

{% alert class="guides__alert" level="info" %}
Используйте канал обновлений `Early Access` или `Stable`. Установите [окно автоматических обновлений](/documentation/v1/modules/002-deckhouse/usage.html#конфигурация-окон-обновлений) или [ручной режим](/documentation/v1/modules/002-deckhouse/usage.html#ручное-подтверждение-обновлений).
{% endalert %}

Выберите [канал обновлений]( /documentation/v1/deckhouse-release-channels.html) и [режим обновлений](/documentation/v1/modules/002-deckhouse/configuration.html#parameters-releasechannel), который соответствует вашим ожиданиям. Чем стабильнее канал обновлений, тем позже вы получаете новый функционал.

По возможности используйте разные каналы обновлений для кластеров. Для кластера разработки используйте менее стабильный канал обновлений, чем для кластера тестирования, или stage-кластера (предпродуктивный кластер).

Мы рекомендуем использовать канал обновлений `Early Access` или `Stable` для production-кластеров. Если у вас более одного кластера в production-окружении, то лучше использовать для них разные каналы обновлений. Например, `Early Access` для одного, а `Stable` для другого. Если кластеры все-таки на одном канале обновлений, то используйте разные окна обновлений.

{% alert class="guides__alert" level="warning" %}
Даже в очень нагруженных и критичных кластерах не стоит отключать использование канала обновлений. Лучшая стратегия — плановое обновление. Если вы используете в кластере релиз Deckhouse который не обновлялся уже более полугода, то вам сложно будет быстро получить помощь по вашей проблеме.
{% endalert %}

Управление [окнами обновлений](/documentation/v1/modules/002-deckhouse/configuration.html#parameters-update-windows) даст возможность планово обновлять релизы Deckhouse в автоматическом режиме, когда ваш кластер не обслуживает пиковую нагрузку.

## Версия Kubernetes

{% alert class="guides__alert" level="info" %}
Используйте автоматический [выбор версии Kubernetes](/documentation/v1/installing/configuration.html#clusterconfiguration-kubernetesversion), либо установите версию явно.
{% endalert %}

В большинстве случаев лучше использовать автоматический выбор версии Kubernetes. В Deckhouse такое поведение установлено по умолчанию, но его можно изменить с помощью параметра [kubernetesVersion](/documentation/v1/installing/configuration.html#clusterconfiguration-kubernetesversion). Обновление версии Kubernetes в кластере не оказывает влияния на приложения и проходит [последовательно и безопасно](/documentation/v1/modules/040-control-plane-manager/#управление-версиями).

Если указан автоматический выбор версии Kubernetes, то Deckhouse может обновить версию Kubernetes в кластере при обновлении релиза Deckhouse (при обновлении минорной версии). Если версия Kubernetes в параметре [kubernetesVersion](/documentation/v1/installing/configuration.html#clusterconfiguration-kubernetesversion) указана явно, то Deckhouse в какой-то момент может не обновиться до свежей версии, если используемая в кластере версия Kubernetes перестанет поддерживаться.

Решите, стоит ли вам использовать автоматический выбор версии или указать конкретную версию и периодически обновлять ее вручную.

Если ваше приложение использует устаревшие версии ресурсов или требует конкретной версии Kubernetes по какой-то другой причине, то проверьте что она [поддерживается](/documentation/v1/supported_versions.html), и [установите ее явно](/documentation/v1/deckhouse-faq.html#как-обновить-версию-kubernetes-в-кластере).  

## Требования к ресурсам

{% alert class="guides__alert" level="info" %}
Используйте от 4 CPU / 8GB RAM на инфраструктурные узлы. Для мастер-узлов и узлов мониторинга используйте быстрые диски .
{% endalert %}

Мы рекомендуем следующие минимальные ресурсы для инфраструктурных узлов, в зависимости от их роли в кластере:
- **Мастер-узел** — 4 CPU, 8GB RAM; быстрый диск с не менее чем 400 IOPS.  
- **Frontend-узел** — 2 CPU, 4GB RAM;
- **Узел мониторинга** (для нагруженных кластеров) — 4 CPU, 8GB RAM; быстрый диск.
- **Системный узел**:
  - 2 CPU, 4 RAM — если в кластере есть выделенные узлы мониторинга;
  - 4 CPU, 8 RAM, быстрый диск — если в кластере нет выделенных узлов мониторинга.

Примерный расчет ресурсов, необходимых для кластера:
- **Типовой кластер**: 3 мастер-узла, 2 frontend-узла, 2 системных узла. Такая конфигурация потребует **от 24 CPU и 48GB RAM**, плюс быстрые диски с 400+ IOPS для мастер-узлов.
- **Кластер с повышенной нагрузкой** (с выделенными узлами мониторинга): 3 мастер-узла, 2 frontend-узла, 2 системных узла, 2 узла мониторинга. Такая конфигурация потребует **от 28 CPU и 64GB RAM**, плюс быстрые диски с 400+ IOPS для мастер-узлов и узлов мониторинга.
- Желательно выделить отдельный [storageClass](/documentation/v1/deckhouse-configure-global.html#parameters-storageclass) на быстрых дисках для компонентов Deckhouse.
- Добавьте к этому ресурсы, необходимые для запуска полезной нагрузки.

## Особенности конфигурации

### Мастер-узлы

{% alert class="guides__alert" level="info" %}
В кластере должно быть три мастер-узла с быстрыми дисками 400+ IOPS.
{% endalert %}

Всегда используйте три мастер-узла, так как их достаточно для отказоустойчивости. Также это дает возможность безопасно обновлять control plane кластера и сами мастер-узлы. В большем количестве мастер-узлов нет необходимости, а 2 узла (как и любое четное количество) не дают кворума.

Конфигурация мастер-узлов для облачных кластеров настраивается в параметре [masterNodeGroup](/documentation/v1/modules/030-cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration-masternodegroup).

Может быть полезно:
- [Как добавить мастер-узлы в облачном кластере...](/documentation/v1/modules/040-control-plane-manager/faq.html#как-добавить-master-узлы-в-облачном-кластере-single-master-в-multi-master)
- [Как добавить статичный узел в кластер...](/documentation/v1/modules/040-node-manager/faq.html#как-добавить-статичный-узел-в-кластер)

### Frontend-узлы

{% alert class="guides__alert" level="info" %}
Выделите два или более frontend-узла.

Используйте inlet `LoadBalancer` для AWS/GCP/Azure, inlet `HostPort` с внешним балансировщиком для bare metal или vSphere/OpenStack.
{% endalert %}

Frontend-узлы — узлы балансировки входящего трафика. Такие узлы выделены для работы Ingress-контроллеров. У [NodeGroup](/documentation/v1/modules/040-node-manager/cr.html#nodegroup) frontend-узлов установлен label `node-role.deckhouse.io/frontend`. Читайте подробнее про [выделение узлов под определенный вид нагрузки...](/documentation/v1/#выделение-узлов-под-определенный-вид-нагрузки)

Используйте более одного frontend-узла. Frontend-узлы должны выдерживать трафик при отказе как минимум одного frontend-узла.

Например, если в кластере два frontend-узла, то каждый frontend-узел должен справляться со всей нагрузкой на кластер, на случай отказа второго frontend-узла. Если в кластере три frontend-узла, то каждый frontend-узел должен выдерживать как минимум увеличение нагрузки в полтора раза.

Выберите [тип inlet'а](/documentation/v1/modules/402-ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-inlet) (он определяет способ поступления трафика).  

При развертывании кластера с помощью Deckhouse в облачной инфраструктуре, где поддерживается заказ балансировщиков (например — AWS, GCP, Azure и т.п.), используйте inlet `LoadBalancer` или `LoadBalancerWithProxyProtocol`. В средах, где автоматический заказ балансировщика недоступен (кластер bare metal, в vSphere или OpenStack), используйте inlet `HostPort` или `HostPortWithProxyProtocol`. В этом случае вы можете либо добавить несколько A-записей в DNS для соответствующего домена, либо использовать внешний сервис балансировки нагрузки (например, решения от Cloudflare, Qrator, или настроить metallb).

{% alert class="guides__alert" level="warning" %}
Inlet `HostWithFailover` подходит для кластеров с одним frontend-узлом. Он позволяет сократить время недоступности Ingress-контроллера при его обновлении. Такой тип inlet подойдет, например, для важных сред разработки, но **не рекомендуется для production**.
{% endalert %}

Алгоритм выбора inlet'а:

![Алгоритм выбора inlet'а]({{ assets["guides/going_to_production/ingress-inlet-ru.svg"].digest_path }})

### Узлы мониторинга

{% alert class="guides__alert" level="info" %}
Для нагруженных кластеров выделите два узла мониторинга с быстрыми дисками.
{% endalert %}

Узлы мониторинга — узлы, выделенные для запуска Grafana, Prometheus и других компонентов мониторинга. У [NodeGroup](/documentation/v1/modules/040-node-manager/cr.html#nodegroup) узлов мониторинга установлен label `node-role.deckhouse.io/monitoring`.

В нагруженных кластерах, где генерируется много алертов и собирается много метрик, под мониторинг рекомендуется выделить отдельные узлы. Если этого не сделать, то компоненты мониторинга будут размещены на [системных узлах](#системные-узлы).

При выделении узлов мониторинга важно выделить им быстрые диски. Это можно сделать выделив `storageClass` на быстрых дисках для всех компонентов Deckhouse (глобальный параметр [storageClass](/documentation/v1/deckhouse-configure-global.html#parameters-storageclass)), или выделить отдельный `storageClass` только компонентам мониторинга (параметры [storageClass](/documentation/v1/modules/300-prometheus/configuration.html#parameters-storageclass) и [longtermStorageClass](/documentation/v1/modules/300-prometheus/configuration.html#parameters-longtermstorageclass) модуля `prometheus`).

### Системные узлы

{% alert class="guides__alert" level="info" %}
Выделите два системных узла.
{% endalert %}

Системные узлы — узлы, выделенные для запуска модулей Deckhouse. У [NodeGroup](/documentation/v1/modules/040-node-manager/cr.html#nodegroup) системных узлов установлен label `node-role.deckhouse.io/system`.

Выделите 2 системных узла. В этом случае модули Deckhousе будут запускаться на них не пересекаясь с пользовательскими приложениями кластера. Читайте подробнее про [выделение узлов под определенный вид нагрузки...](/documentation/v1/#выделение-узлов-под-определенный-вид-нагрузки).

Компонентам Deckhouse желательно выделить быстрые диски (глобальный параметр [storageClass](/documentation/v1/deckhouse-configure-global.html#parameters-storageclass)).

## Уведомление о событиях мониторинга

{% alert class="guides__alert" level="info" %}
Настройте отправку алертов через [внутренний](/documentation/v1.44/modules/300-prometheus/faq.html#как-добавить-alertmanager) Alertmanager или подключите [внешний](/documentation/v1.44/modules/300-prometheus/faq.html#как-добавить-внешний-дополнительный-alertmanager).
{% endalert %}

Мониторинг будет работать сразу после установки Deckhouse, но при работе в Production этого недостаточно. Настройте [встроенный](/documentation/v1.44/modules/300-prometheus/faq.html#как-добавить-alertmanager) в Deckhouse Alertmanager или [подключите свой](/documentation/v1.44/modules/300-prometheus/faq.html#как-добавить-внешний-дополнительный-alertmanager) Alertmanager, чтобы получать уведомления об инцидентах.

С помощью custom resource [CustomAlertmanager](/documentation/v1.44/modules/300-prometheus/cr.html#customalertmanager) вы сможете быстро настроить отправку уведомлений например на [e-mail](/documentation/v1/modules/300-prometheus/cr.html#customalertmanager-v1alpha1-spec-internal-receivers-emailconfigs), в [Slack](/documentation/v1/modules/300-prometheus/cr.html#customalertmanager-v1alpha1-spec-internal-receivers-slackconfigs), [Telegram](/documentation/v1/modules/300-prometheus/usage.html#отправка-алертов-в-telegram), через [webhook](/documentation/v1/modules/300-prometheus/cr.html#customalertmanager-v1alpha1-spec-internal-receivers-webhookconfigs), а также другими способами.

## Сбор логов

{% alert class="guides__alert" level="info" %}
[Настройте](/documentation/v1/modules/460-log-shipper/) централизованный сбор логов.
{% endalert %}

Настройте централизованную сборку журналов с системных и пользовательских приложений с помощью модуля [log-shipper](/documentation/v1/modules/460-log-shipper/).

Вам достаточно создать custom resource описывающий *что собирать* — [ClusterLoggingConfig](/documentation/v1/modules/460-log-shipper/cr.html#clusterloggingconfig) или [PodLoggingConfig](/documentation/v1/modules/460-log-shipper/cr.html#podloggingconfig); и создать custom resource определяющий место *куда отправлять* собранные логи — [ClusterLogDestination](/documentation/v1/modules/460-log-shipper/cr.html#clusterlogdestination).

Может быть полезно:
- [Пример для Grafana Loki](/documentation/v1/modules/460-log-shipper/examples.html#чтение-логов-из-всех-podов-кластера-и-направление-их-в-loki)
- [Пример для Logstash](/documentation/v1/modules/460-log-shipper/examples.html#простой-пример-logstash)
- [Пример для Splunk](/documentation/v1/modules/460-log-shipper/examples.html#пример-интеграции-со-splunk)

## Резервное копирование

{% alert class="guides__alert" level="info" %}
Настройте [резервное копирование etcd](/documentation/v1/modules/040-control-plane-manager/faq.html#как-сделать-бекап-etcd). Напишите план восстановления.
{% endalert %}

Как минимум настройте [резервное копирование данных etcd](/documentation/v1/modules/040-control-plane-manager/faq.html#как-сделать-бекап-etcd). Это будет ваш последний шанс на восстановление кластера в случае самых неожиданных событий. Храните эти резервные копии как можно *дальше* от вашего кластера.  

Если резервные копии не работают или вы не знаете как из них восстановиться — они не помогут. Лучшей практикой будет составить [план восстановления на случай аварии](https://habr.com/ru/search/?q=%5BDRP%5D&target_type=posts&order=date) (Disaster Recovery Plan), содержащий конкретные шаги и команды по развертыванию кластера из резервной копии.

Конечно, этот план должен периодически актуализироваться и проверяться учебными тревогами.

## Сообщество

{% alert class="guides__alert" level="info" %}
Следите за новостями проекта в [Telegram](https://t.me/deckhouse_ru).
{% endalert %}

Вступите в [сообщество](https://deckhouse.ru/community/about.html), чтобы быть в курсе важных изменений и новостей. Вы сможете общаться с людьми, которые делают то же самое что и вы. Это может помочь избежать типовых проблем.

Команда Deckhouse знает, каких усилий требует выход в Production с Kubernetes. Мы будем рады, если вы достигнете успеха с Deckhouse. Поделитесь вашим успехом и вдохновите кого-нибудь на переход к Kubernetes.
