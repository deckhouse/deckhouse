---
title: Разметка и объём дисков
permalink: ru/guides/fs-requirements.html
description: Руководство по выбору объёма дисков и разметке файловой системы перед установкой Deckhouse Platform
lang: ru
---

В [разделе «Требования к ресурсам»](../requirements/#требования-к-ресурсам) для различных типов узлов Deckhouse Platform указаны объёмы дисков, которые необходимо выделить для успешной установки и работы Deckhouse Platform. Важно также корректно сконфигурировать файловую систему — иначе пространство может неожиданно закончиться даже при верно рассчитанном объёме на этапе установки.

{% alert level="info" %}
Причина сбоев может крыться в поведении [установщика](../glossary.html#installer) дистрибутива Linux. Например, Astra Linux при установке может выделить 15 ГБ под корень файловой системы (`/`), 15 ГБ под домашний каталог пользователя (`/home`), а остальное оставить неразмеченным, несмотря на подключённый диск общим объёмом в 60 ГБ, рекомендуемых в руководстве. При такой конфигурации установка Deckhouse Platform завершается ошибкой отсутствия требуемого дискового пространства.
{% endalert %}

Чтобы избежать проблем в будущем, перед установкой убедитесь, что разделы файловой системы диска, выделенного для машины, соответствуют требованиям Deckhouse Platform по объёму.

## Где и что хранит Deckhouse Platform

Deckhouse Platform размещает данные разных типов в определённых каталогах файловой системы. Ниже — краткий разбор ключевых каталогов:

* `/etc/kubernetes/`, `/etc/containerd` и т. д. — каталоги с конфигурацией компонентов Kubernetes;
* `/var/lib/containerd` — слои образов компонентов Deckhouse Platform и прочих контейнеров на узле. Чем больше компонентов и контейнеров, тем больше свободного места требуется в каталоге.
* `/var/lib/kubelet` — в этом каталоге хранится два типа информации:
  * данные о запущенных в кластере подах;
  * данные `ephemeral-storage` — например, если на [master-узле](../glossary.html#master-node) запрашивается 7 ГБ под `ephemeral-storage`, и в этом каталоге места будет недостаточно, поды не будут запланированы на этот узел.
* `/var/lib/etcd` — база данных etcd, в которой хранится необходимая для работы кластера Kubernetes информация;
* `/var/lib/deckhouse/downloaded/` — хранилище конфигураций релизов модулей Deckhouse Platform ([ModuleRelease](../reference/api/cr.html#modulerelease));
* `/var/lib/deckhouse/stronghold/` — хранилище данных [Stronghold](/products/stronghold/) (если включён соответствующий модуль);
* `/var/log/pods/` — хранилище логов подов;
* `/opt/deckhouse/` — служебные компоненты Deckhouse Platform, такие как kubelet, containerd, статические утилиты (например, `lsblk`) и т. д.;
* `/opt/deckhouse/registry/` (`/var/lib/deckhouse/registry/`, если у NodeGroup master-узлов `systemType: DeckhouseEngine`) — хранилище модуля [`registry`](/modules/registry/), есть только на master-узлах. В нём находится внутрикластерный кеш ([`storage.cache`](/modules/registry/configuration.html#parameters-storage-cache)) или, в реализации v1 (Deckhouse Platform до версии 1.77 включительно), данные режимов `Proxy` и `Local`. При выключении кеша данные намеренно сохраняются (подробнее — в разделе [об удалении оставшихся данных кеша](/modules/registry/faq.html#как-удалить-с-узла-оставшиеся-данные-кеша)). Сколько места нужно хранилищу, описано [ниже](#хранилище-модуля-registry-на-master-узлах);
* `/opt/local-path-provisioner/` — каталог для хранения данных при использовании [локального хранилища Local Path Provisioner](../admin/configuration/storage/sds/local-path-provisioner.html). Каталог задаётся в обязательном параметре `spec.path` LocalPathProvisioner, [примеры конфигурации](../admin/configuration/storage/sds/local-path-provisioner.html#примеры-ресурсов-localpathprovisioner) используют `/opt/local-path-provisioner`.

## Рекомендации по объёму дискового пространства

Ниже приведены ориентиры по объёму диска, который занимают разные компоненты кластера.

{% alert level="info" %}
Суммарные значения из таблиц могут превышать минимально рекомендуемый объём диска для узла, указанный в «Быстром старте» или в разделе [«Требования к ресурсам»](../requirements/#требования-к-ресурсам). Это связано с тем, что в таблицах приведены максимальные требования по компонентам, а в минимальных требованиях указан усреднённый объём для типового узла.
{% endalert %}

### Master-узлы

В таблице представлены рекомендуемые объёмы пространства для каталогов, используемых Deckhouse Platform на master-узлах кластера.

<table>
  <thead>
    <tr>
      <th>Каталог</th>
      <th>Объём диска, ГБ</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td><code>/mnt/vector-data</code></td>
      <td>1</td>
    </tr>
    <tr>
      <td><code>/opt</code>
      <p style="font-size: 0.9em; color: #666;">Без учёта хранилища модуля <code>registry</code>: внутрикластерного кеша или режимов <code>Proxy</code> и <code>Local</code> реализации v1 (<a href="#хранилище-модуля-registry-на-master-узлах">подробнее...</a>).</p>
      </td>
      <td>1</td>
    </tr>
    <tr>
      <td><code>/tmp</code></td>
      <td>1</td>
    </tr>
    <tr>
      <td><code>/var/lib</code> <button type="button" onclick="toggleDetails('varlib-details')" style="background: none; border: none; cursor: pointer; font-size: 0.9em; color: #666;">[{{ site.data.i18n.common["show_details"][page.lang] }}]</button></td>
      <td>75</td>
    </tr>
    <tbody id="varlib-details" style="display: none;">
      <tr>
        <td style="padding-left: 3em;"><code>/var/lib/containerd</code></td>
        <td>30</td>
      </tr>
      <tr>
        <td style="padding-left: 3em;"><code>/var/lib/deckhouse</code></td>
        <td>5</td>
      </tr>
      <tr>
        <td style="padding-left: 3em;"><code>/var/lib/etcd</code></td>
        <td>10</td>
      </tr>
      <tr>
        <td style="padding-left: 3em;"><code>/var/lib/kubelet</code></td>
        <td>25</td>
      </tr>
      <tr>
        <td style="padding-left: 3em;"><code>/var/lib/upmeter</code></td>
        <td>5</td>
      </tr>
    </tbody>
    <tr>
      <td><code>/var/log/kube-audit</code></td>
      <td>2</td>
    </tr>
    <tr>
      <td><code>/var/log/pods</code></td>
      <td>18 (<a href="#хранилище-логов-подов">подробнее...</a>)</td>
    </tr>
  </tbody>
</table>

### Worker-узлы

В таблице представлены рекомендуемые объёмы пространства для каталогов, используемых Deckhouse Platform на [worker-узлах](../glossary.html#worker-node) кластера.

<table>
  <thead>
    <tr>
      <th>Каталог</th>
      <th>Объём диска, ГБ</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td><code>/mnt/vector-data</code></td>
      <td>1</td>
    </tr>
    <tr>
      <td><code>/opt</code></td>
      <td>1</td>
    </tr>
    <tr>
      <td><code>/opt/local-path-provisioner</code>
      <p style="font-size: 0.9em; color: #666;">Объём зависит от настроек хранилища, заданных пользователем. Рекомендуется вынести на отдельный раздел.</p>
      </td>
      <td>100</td>
    </tr>
    <tr>
      <td><code>/tmp</code></td>
      <td>1</td>
    </tr>
    <tr>
      <td><code>/var/lib</code> <button type="button" onclick="toggleDetails('varlib-worker-details')" style="background: none; border: none; cursor: pointer; font-size: 0.9em; color: #666;">[{{ site.data.i18n.common["show_details"][page.lang] }}]</button></td>
      <td>55</td>
    </tr>
    <tbody id="varlib-worker-details" style="display: none;">
      <tr>
        <td style="padding-left: 3em;"><code>/var/lib/bashible</code></td>
        <td>1</td>
      </tr>
      <tr>
        <td style="padding-left: 3em;"><code>/var/lib/containerd</code></td>
        <td>30</td>
      </tr>
      <tr>
        <td style="padding-left: 3em;"><code>/var/lib/kubelet</code></td>
        <td>24</td>
      </tr>
    </tbody>
    <tr>
      <td><code>/var/log/pods</code></td>
      <td>5 (<a href="#хранилище-логов-подов">подробнее...</a>)</td>
    </tr>
  </tbody>
</table>

### Системные узлы

[Системные узлы](../glossary.html#system-node) (system-узел) — это узлы, на которых запускаются компоненты Deckhouse Platform. При добавлении таких узлов в кластер учитывайте, что на них размещается нагрузка мониторинга, включая:

- [Prometheus](/modules/prometheus/);
- [loki](/modules/loki/);
- [upmeter](/modules/upmeter/) и другие компоненты Deckhouse Platform.

Если данные мониторинга хранятся локально на узлах, для каждого системного узла рекомендуется дополнительно выделить ≥ 100 ГБ свободного дискового пространства.

{% alert level="info" %}
Если в кластере не используются выделенные системные узлы, указанная выше нагрузка будет распределена по другим узлам. Учтите рекомендуемые объёмы дискового хранилища при выборе их конфигурации.
{% endalert %}

### Хранилище логов подов

Логи подов хранятся в каталоге `/var/log/pods/`. Объём, занимаемый логами, зависит от количества контейнеров и настроек Deckhouse Platform. В среднем, на [master-узле](../glossary.html#master-node) при использовании [набора модулей Default](../admin/configuration/#наборы-модулей) работает около 90 контейнеров, По умолчанию для каждого контейнера хранится до 4 файлов логов размером до 50 МБ каждый, то есть до 200 МБ на контейнер. Соответственно, в каталоге `/var/log/pods/` должно быть доступно как минимум `90 * 200 МБ = 18 ГБ` места.

Параметры хранения логов можно переопределить в параметрах `spec.kubelet.containerLogMaxSize` и `spec.kubelet.containerLogMaxFiles` [NodeGroup](/modules/node-manager/cr.html#nodegroup) (подробнее — [в разделе о настройке узлов](../admin/configuration/platform-scaling/node/node-customization.html)). Значения по умолчанию:

```yaml
spec:
  kubelet:
    containerLogMaxSize: 50Mi
    containerLogMaxFiles: 4
```

### Хранилище баз уязвимостей Trivy

Deckhouse Platform имеет встроенную [систему сканирования образов на уязвимости](../admin/configuration/security/scanning.html) на базе [Trivy](https://github.com/aquasecurity/trivy), которая сканирует все контейнерные образы, используемые в подах кластера. Для сканирования используются как публичные [базы уязвимостей](../glossary.html#vulnerability-database), так и обогащённые данные из Astra Linux, ALT Linux и РЕД ОС. Суммарный объём занимаемого базами дискового пространства составляет 5 ГБ, поэтому его необходимо учитывать при выборе конфигурации разделов диска.

Базы данных хранятся на системных узлах кластера, а в случае, если такие узлы в кластере отсутствуют, базы будут расположены на worker-узле.

### Хранилище модуля registry на master-узлах

Таблица для master-узлов выше не учитывает хранилище внутрикластерного кеша модуля [`registry`](/modules/registry/). Если кеш включён ([`storage.cache`](/modules/registry/configuration.html#parameters-storage-cache)), на каждом master-узле есть своя копия хранилища в `/opt/deckhouse/registry` (`/var/lib/deckhouse/registry`, если у NodeGroup master-узлов `systemType: DeckhouseEngine`), и на файловой системе, где находится этот каталог, нужно дополнительное место:

- под объём хранилища, заданный в [`storage.size`](/modules/registry/configuration.html#parameters-storage-size): закладывайте три релиза набора образов, который использует кластер (развёрнутый, предыдущий и следующий), вместе с его модулями и каталогом модулей;
- под резерв, который хранилище всегда оставляет на этой файловой системе свободным для узла. Из-за резерва файловая система должна быть заметно больше `storage.size`, в том числе когда под хранилище выделен отдельный диск.

В каталог хранилища рекомендуется смонтировать отдельный диск: если хранилище делит диск с etcd, etcd замедляется, пока кеш заполняется. Без отдельного диска место нужно выделить на файловой системе, где находится каталог хранилища (если у NodeGroup master-узлов `systemType: DeckhouseEngine` — на той, где находится `/var/lib/deckhouse`). Если `/opt` — отдельный раздел размером по таблице выше (1 ГБ), хранилище в него не поместится.

Режимы `Proxy` и `Local` реализации v1 тоже хранят данные в `/opt/deckhouse/registry`.

Подробнее о расчёте объёма — в разделе [«Требования к использованию модуля»](/modules/registry/#требования-к-использованию-модуля) документации модуля `registry`.

## Если настроены лимиты по ресурсам

Если для объектов кластера заданы лимиты на диск (квоты, ограничения на объём), требуемое свободное место всё равно должно быть физически доступно на узле. При его отсутствии произойдёт вытеснение нагрузки (eviction) с соответствующих узлов.

## Локальное хранилище на основе LVM

В кластере Deckhouse Platform можно настроить [локальное хранилище на узлах](../admin/configuration/storage/sds/lvm-local.html) с использованием LVM.

Требования и порядок размещения:

- На узле должны быть доступны свободные блочные устройства (разделы диска).
- Эти устройства будут задействованы модулем [sds-local-volume](/modules/sds-local-volume/) для создания StorageClass.
- Объём свободного пространства на блочном устройстве должен соответствовать объёму, который планируется предоставлять через создаваемый StorageClass.

{% include table-toggle-details.js %}
