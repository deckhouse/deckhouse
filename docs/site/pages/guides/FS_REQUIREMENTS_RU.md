---
title: Разметка и объем дисков
permalink: ru/guides/fs-requirements.html
description: Руководство по выбору объема дисков и разметке файловой системы перед установкой Deckhouse Kubernetes Platform
lang: ru
layout: sidebar-guides
---

В [гайде по выбору минимально требуемого дискового пространства](hardware-requirements.html#выбор-ресурсов-для-узлов) для различных типов узлов Deckhouse Kubernetes Platform (DKP) указаны объемы дисков, которые необходимо выделить для успешной установки и работы DKP. Важно также корректно сконфигурировать файловую систему — иначе пространство может неожиданно закончиться даже при верно рассчитанном объёме на этапе установки.

{% alert level="info" %}
Причина сбоев может крыться в поведении установщика дистрибутива Linux. Например, Astra Linux при установке может выделить 15 ГБ под корень файловой системы (`/`), 15 ГБ под домашний каталог пользователя (`/home`), а остальное оставить неразмеченным, несмотря на подключённый диск общим объемом в 60 ГБ, рекомендуемых в гайде. При такой конфигурации установка DKP завершается ошибкой отсутствия требуемого дискового пространства.
{% endalert %}

Чтобы избежать проблем в будущем, перед установкой лучше убедиться, что разделы файловой системы диска, выделенного для машины, соответствуют требованиям DKP по объёму.

## Где и что хранит DKP

DKP размещает данные разных типов в определённых каталогах файловой системы. Ниже — краткий разбор ключевых каталогов:

* `/etc/kubernetes/`, `/etc/containerd` и т.д. — каталоги с конфигурацией компонентов Kubernetes;
* `/var/lib/containerd` — слои образов компонентов DKP и прочих контейнеров на узле. Чем больше компонентов и контейнеров, тем больше свободного места требуется в каталоге.
* `/var/lib/kubelet` — в этом каталоге хранится два типа информации:
  * данные о запущенных в кластере подах;
  * данные `ephemeral-storage` — например, если на master-узле запрашивается 7 ГБ под `ephemeral-storage`, и в этом каталоге места будет недостаточно, поды не будут запланированы на этот узел.
* `/var/lib/etcd` — база данных etcd, в которой хранится необходимая для работы кластера Kubernetes информация;
* `/var/lib/deckhouse/downloaded/` — хранилище конфигураций релизов модулей Deckhouse DKP ([ModuleRelease](../documentation/v1/reference/api/cr.html#modulerelease));
* `/var/lib/deckhouse/stronghold/` — хранилище данных [Stronghold](../../stronghold/) (если включён соответствующий модуль);
* `/var/log/pods/` — хранилище логов подов;
* `/opt/deckhouse/` — служебные компоненты DKP, такие как kubelet, containerd, статические утилиты (например, `lsblk`) и т.д.;
* `/opt/local-path-provisioner/` — каталог для хранения данных при использовании [локального хранилища Local Path Provisioner](../documentation/v1/admin/configuration/storage/sds/local-path-provisioner.html) (может быть переопределён [в конфигурации](../documentation/v1/admin/configuration/storage/sds/local-path-provisioner.html#примеры-ресурсов-localpathprovisioner)).

## Рекомендации по объёму дискового пространства

Ниже приведены ориентиры по объёму диска, который занимают разные компоненты кластера.

{% alert level="info" %}
Суммарные значения из таблиц могут превышать минимально рекомендуемый объём диска для узла, указанный в «Быстром старте» или [«Требованиях к кластеру на bare metal»](./hardware-requirements.html). Это связано с тем, что в таблицах приведены максимальные требования по компонентам, а в минимальных требованиях указан усреднённый объём для типового узла.
{% endalert %}

### Master-узлы

В таблице представлены рекомендуемые объёмы пространства для каталогов, используемых DKP на master-узлах кластера.

<table>
  <thead>
    <tr>
      <th>Каталог</th>
      <th>Объем диска, ГБ</th>
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
      <td>5 (<a href="#хранилище-логов-подов">подробнее...</a>)</td>
    </tr>
  </tbody>
</table>

### Worker-узлы

В таблице представлены рекомендуемые объёмы пространства для каталогов, используемых DKP на worker-узлах кластера.

<table>
  <thead>
    <tr>
      <th>Каталог</th>
      <th>Объем диска, ГБ</th>
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
      <p style="font-size: 0.9em; color: #666;">Объем зависит от настроек хранилища, заданных пользователем. Рекомендуется вынести на отдельный раздел.</p>
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

Системные узлы (system-узел) — это узлы, на которых запускаются компоненты DKP. При добавлении таких узлов в кластер учитывайте, что на них размещается нагрузка мониторинга, включая:
- [Prometheus](../../../modules/prometheus/);
- [loki](../../../modules/loki/);
- [upmeter](../../../modules/upmeter/) и другие компоненты DKP.

Если данные мониторинга хранятся локально на узлах, для каждого системного узла рекомендуется дополнительно выделить ≥ 100 ГБ свободного дискового пространства.

{% alert level="info" %}
Если в кластере не используются выделенные системные узлы, указанная выше нагрузка будет распределена по другим узлам. Учтите рекомендуемые объёмы дискового хранилища при выборе их конфигурации.
{% endalert %}

### Хранилище логов подов

Логи подов хранятся в каталоге `/var/log/pods/`. Объём, занимаемый логами, зависит от количества контейнеров и настроек DKP. В среднем, на master-узле при использовании [набора модулей Default](../documentation/v1/admin/configuration/#наборы-модулей) работает около 90 контейнеров, на логи каждого из которых по умолчанию выделяется около 50 Мб места. Соответственно, в каталоге `/var/log/pods/` должно быть доступно как минимум `90 * 50 Мб = 4,5 ГБ` места.

Параметры хранения логов также могут быть переопределены в параметре `containerLogMaxSize` [группы узлов](../documentation/v1/admin/configuration/platform-scaling/node/node-customization.html):

```yaml
containerLogMaxSize: 50Mi
containerLogMaxFiles: 4
```

### Хранилище баз уязвимостей Trivy

DKP имеет встроенную [систему сканирования образов на уязвимости](../documentation/v1/admin/configuration/security/scanning.html) на базе [Trivy](https://github.com/aquasecurity/trivy), которая сканирует все контейнерные образы, используемые в подах кластера. Для сканирования используются как публичные базы уязвимостей, так и обогащённые данные из Astra Linux, ALT Linux и РЕД ОС. Суммарный объем занимаемого базами дискового пространства составляет 5 ГБ, поэтому его необходимо учитывать при выборе конфигурации разделов диска.

Базы данных хранятся на системных узлах кластера, а в случае, если такие узлы в кластере отсутствуют, базы будут расположены на worker-узле.

## Если настроены лимиты по ресурсам

Если для объектов кластера заданы лимиты на диск (квоты, ограничения на объём), требуемое свободное место всё равно должно быть физически доступно на узле. При его отсутствии произойдёт вытеснение нагрузки (eviction) с соответствующих узлов.

## Локальное хранилище на основе LVM

В кластере DKP можно настроить [локальное хранилище на узлах](../documentation/v1/admin/configuration/storage/sds/lvm-local.html) с использованием LVM.

Требования и порядок размещения:

- На узле должны быть доступны свободные блочные устройства (разделы диска).
- Эти устройства будут задействованы модулем [sds-local-volume](../../../modules/sds-local-volume/) для создания StorageClass.
- Объём свободного пространства на блочном устройстве должен соответствовать объёму, который планируется предоставлять через создаваемый StorageClass.

{% include table-toggle-details.js %}
