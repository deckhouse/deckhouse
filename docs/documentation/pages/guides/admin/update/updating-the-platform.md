---
title: Обновление
permalink: ru/guides/production.html
lang: ru
---

## Обновление кластеров Kubernetes в Deckhouse Kubernetes Platform

Чтобы обновить версию Kubernetes в кластере измените параметр [kubernetesVersion](installing/configuration.html#clusterconfiguration-kubernetesversion) в структуре [ClusterConfiguration](installing/configuration.html#clusterconfiguration), выполнив следующие шаги:

1. Выполните команду:

   ```
   kubectl -n d8-system exec -ti deploy/deckhouse -- deckhouse-controller edit cluster-configuration
   ```

1. Измените параметр `kubernetesVersion`.
1. Сохраните изменения. Узлы кластера начнут последовательно обновляться. Если указанная для обновления версия с параметром [kubernetesVersion](../../installing/configuration.html#clusterconfiguration-kubernetesversion) не соответствует текущей версии control plane в кластере, запускается изменение версий компонентов:
Обновление в разных NodeGroup выполняется параллельно. Внутри каждой NogeGroup узлы обновляются последовательно, по одному.
- При upgrade:
  - Обновление происходит последовательными этапами, по одной минорной версии, например от 1.22 к 1.23, от 1.23 к 1.24, от 1.24 к 1.25.
  - На каждом этапе сначала обновляется версия `control plane`, затем происходит обновление `kubelet` на узлах кластера.  
- При downgrade:
  - Успешный downgrade гарантируется только на одну версию вниз от максимальной минорной версии `control plane`, когда-либо использовавшейся в кластере.
  - Происходит downgrade `kubelet` на узлах кластера.
  - Происходит downgrade компонентов `control plane`.
Текущая версия Kubernetes в кластере вычисляется на основании версии `control-plane` и узлов кластера.
1. Отслеживать ход обновления можно с помощью команды `kubectl get no`. Обновление можно считать завершенным, когда в выводе команды у каждого узла кластера в колонке `VERSION` появится обновленная версия.
1. Обновите **минорную версию** компонентов control plane с помощью параметра [kubernetesVersion](../../installing/configuration.html#clusterconfiguration-kubernetesversion), в котором можно выбрать [автоматический режим обновления (значение `Automatic`)](ссылка) или указать желаемую минорную версию control plane. Версию control plane, которая используется по умолчанию (при `kubernetesVersion: Automatic`), а также список поддерживаемых версий Kubernetes можно найти в [документации](../../supported_versions.html#kubernetes).

Обновление **patch-версии** компонентов `control plane` в рамках минорной версии происходит автоматически вместе с обновлением версии Deckhouse Kubernetes Platform. Управлять обновлением patch-версий нельзя.

> Обновление `control plane` выполняется безопасно и для single-master, и для multi-master-кластеров. Во время обновления может быть кратковременная недоступность API-сервера. На работу приложений в кластере обновление не влияет и может выполняться без выделения окна для регламентных работ.

Остальные этапы в Deckhouse Kubernetes Platform выполняются автоматически. Внутри подсистемы `candi` есть два модуля, которые отвечают за управление [control-plane](ссылка) и [управление узлами](ссылка). В сontrol-plane-manager автоматически отслеживаются изменения.

**Обновление будет происходит параллельно следующим образом:**

Например, если `kubelet` на всех узлах версии 1.27 и все `control-plane` компоненты версии 1.27, обновление произойдет на следующую версию 1.28 — и так далее, пока версия не будет той, что указана в конфигурации.

Также control-plane-manager следит, чтобы обновление компонентов `control-plane` выполнялось по очереди на каждом мастер-узле: для этого реализован алгоритм запроса и выдачи разрешения на обновление через специальный [диспетчер](ссылка).

Node-manager отвечает за управление узлами, в том числе и за обновление `kubelet`:

Каждый узел в кластере принадлежит к одной из NodeGroup. Как только node-manager определяет, что версия `control-plane` на всех узлах обновилась, то он приступает к обновлению версии kubelet. Если обновление узла не приводит к простою, оно считается безопасным и выполняется в автоматическом режиме. В данном случае обновление `kubelet` больше не приводит к перезапуску контейнеров.

В рамках node-manager также реализован механизм автоматической выдачи разрешений на обновление узла, что гарантирует одновременное обновление только одного узла в рамках NodeGroup. При этом обновление узлов в NodeGroup выполняется, только когда требуемое количество узлов равно текущему количеству узлов в состоянии `Ready`, то есть нет узлов в процессе заказа. (Это касается только облачных кластеров, где есть автоматический заказ новых виртуальных машин.)

## Режимы обновлений

Режим обновления минорных версий Deckhouse (обновление релиза). Не влияет на обновление patch-версий (patch-релизов).

Существуют два режима обновления:

1. **Auto (автоматический режим)** — все обновления применяются автоматически.

Обновления минорной версии релиза Deckhouse Kubernetes Platform применяются с учетом заданных [окон обновлений](ссылк) либо, если окна обновлений не заданы, по мере появления обновлений на соответствующем канале обновлений. Автоматический режим выставляется, как  **Окна обновлений не заданы** - кластер обновится сразу после появления новой версии на соответствующем [канале обновлений](https://deckhouse.ru/documentation/deckhouse-release-channels.html) и **Заданные окна обновлений** - кластер обновится в ближайшее доступное окно после появления новой версии на канале обновлений.

2. **Manual (ручной режим)** — для обновления минорной версии релиза Deckhouse Kubernetes Platform в ручном режиме необходимо [ручное подтверждение](modules/002-deckhouse/usage.html#ручное-подтверждение-обновлений).

Чтобы подтвердить обновления в соответствующем кастомном ресурсе *DeckhouseRelease* установите поле `approved` в `true`.

**Отключение обновления**

Чтобы полностью отключить механизм обновления Deckhouse Kubernetes Platform, удалите в [конфигурации](modules/002-deckhouse/configuration.html) модуля `deckhouse` параметр [releaseChannel](modules/002-deckhouse/configuration.html#parameters-releasechannel).

В этом случае Deckhouse не проверяет обновления и даже обновление на patch-релизы не выполняется.

{% alert level="danger" %}
Крайне не рекомендуется отключать автоматическое обновление! Это заблокирует обновления на patch-релизы, которые могут содержать исправления критических уязвимостей и ошибок.
{% endalert %}

**Подтверждение потенциально опасных (disruptive) обновлений**

При необходимости возможно включить подтверждение потенциально опасных (disruptive) обновлений (которые меняют значения по умолчанию или поведение некоторых модулей). Сделать это можно следующим образом:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    releaseChannel: Stable
    update:
      disruptionApprovalMode: Manual
```

В этом режиме необходимо подтверждать каждое минорное потенциально опасное (disruptive) обновление Deckhouse (без учета patch-версий) с помощью аннотации `release.deckhouse.io/disruption-approved=true` на соответствующем ресурсе [DeckhouseRelease](cr.html#deckhouserelease).

Пример подтверждения минорного потенциально опасного обновления Deckhouse `v1.36.4`:

```shell
kubectl annotate DeckhouseRelease v1.36.4 release.deckhouse.io/disruption-approved=true
```

### Автоматический режим обновления

При указании в конфигурации модуля `deckhouse` параметра `releaseChannel` Deckhouse Kubernetes Platform будет каждую минуту проверять данные о релизе на канале обновлений.

При появлении нового релиза Deckhouse Kubernetes Platform скачает его в кластер и создаст кастомный ресурс [*DeckhouseRelease*](modules/002-deckhouse/cr.html#deckhouserelease).

После появления кастомного ресурса *DeckhouseRelease* в кластере Deckhouse Kubernetes Platform выполняет обновление на соответствующую версию согласно установленным [параметрам обновления](modules/002-deckhouse/configuration.html#parameters-update) (по умолчанию — автоматически, в любое время).

Чтобы посмотреть список и состояние всех релизов, воспользуйтесь командной:

```shell
kubectl get deckhousereleases
```

{% alert %}
Patch-релизы, например, обновление на версию `1.30.2` при установленной версии `1.30.1`, устанавливаются без учета режима и окон обновления, то есть при появлении на канале обновления patch-релиза он всегда будет установлен.
{% endalert %}

**Настройка автоматического режима обновления**

Если в автоматическом режиме окна обновлений не заданы, Deckhouse Kubernetes Platform обновится сразу, как только новый релиз станет доступен.

Patch-версии (например, обновления с `1.26.1` до `1.26.2`) устанавливаются без подтверждения и без учета окон обновлений.

{% alert %}
Также можно настраивать окна disruption-обновлений узлов в custom resource [NodeGroup](../040-node-manager/cr.html#nodegroup) (параметр `disruptions.automatic.windows`).
{% endalert %}

### Ручной режим обновления

При необходимости возможно включить ручное подтверждение обновлений. Сделать это можно следующим образом:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    releaseChannel: Stable
    update:
      mode: Manual
```

В этом режиме необходимо подтверждать каждое минорное обновление Deckhouse Kubernetes Platform (без учета patch-версий).

Пример подтверждения обновления на версию `v1.43.2`:

```shell
kubectl patch DeckhouseRelease v1.43.2 --type=merge -p='{"approved": true}'
```

**Срочное ручное обновление**

Обновление без окна обновлений позволяет выполнить обновление модуля вне определенного для этого времени. Это необходимо в случае срочного обновления. 

> Применение обновлений без соблюдения определенного для этого времени может вызвать проблемы стабильности системы или конфликты с работающими приложениями. Поэтому используйте только в случае действительной необходимости.

Установите в соответствующем ресурсе [DeckhouseRelease](modules/002-deckhouse/cr.html#deckhouserelease) аннотацию `release.deckhouse.io/apply-now: "true"`, как показано напримерах ниже:

Пример команды установки аннотации пропуска окон обновлений для версии `v1.56.2`:

```shell
kubectl annotate deckhousereleases v1.56.2 release.deckhouse.io/apply-now="true"
```

Пример ресурса с установленной аннотацией пропуска окон обновлений:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  annotations:
    release.deckhouse.io/apply-now: "true"
...
```

### Как узнать режим обновления кластера?

Посмотреть режим обновления кластера можно в [конфигурации](modules/002-deckhouse/configuration.html) модуля `deckhouse`. Для этого выполните следующую команду:

```shell
kubectl get mc deckhouse -oyaml
```

Пример вывода:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  creationTimestamp: "2022-12-14T11:13:03Z"
  generation: 1
  name: deckhouse
  resourceVersion: "3258626079"
  uid: c64a2532-af0d-496b-b4b7-eafb5d9a56ee
spec:
  settings:
    releaseChannel: Stable
    update:
      windows:
      - days:
        - Mon
        from: "19:00"
        to: "20:00"
  version: 1
status:
  state: Enabled
  status: ""
  type: Embedded
  version: "1"
```

------------------

## Каналы обновлений

Существует несколько каналов обновления для кластера. Каждый из них имеет свои особенности и предназначен для определенной цели. Важно помнить, что использование неподходящего канала обновлений может привести к проблемам в работе кластера и нарушению его стабильности.

К кластерам, как элементам инфраструктуры, обычно предъявляются различные требования. Например, production-кластер, в отличие от кластера разработки, более требователен к надежности: в нем нежелательно часто обновлять или изменять какие-либо компоненты без особой необходимости, при этом сами компоненты должны быть тщательно протестированы.
По возможности используйте разные каналы обновлений для кластеров. Для кластера разработки используйте менее стабильный канал обновлений, нежели для тестового или stage-кластера (pre-production-кластер).

Для production-кластеров рекомендуется использовать канал обновлений `Early Access` или `Stable`. Если в production-окружении более одного кластера, предпочтительно использовать для них разные каналы обновлений. Например, `Early Access` для одного, а `Stable` — для другого. Если использовать разные каналы обновлений по каким-либо причинам невозможно, рекомендуется устанавливать разные окна обновлений.

Deckhouse Kubernetes Platform использует пять каналов обновлений. Между ними можно переключаться с помощью модуля [deckhouse](ссылка), достаточно указать желаемый канал обновлений в конфигурации модуля из следующего списка:

1. **Rock Solid**. Наиболее стабильный канал обновлений. Подойдет для кластеров, которым необходимо обеспечить повышенный уровень стабильности. Обновления функционала до этого канала доходят не ранее чем через месяц после их появления в релизе.
2. **Stable**. Стабильный канал обновлений для кластеров, в которых закончена активная работа и преимущественно осуществляется эксплуатация. Обновления функционала до этого канала обновлений доходят не ранее чем через две недели после их появления в релизе.
3. **Early Access**. Рекомендуемый канал обновлений, если вы не уверены в выборе. Подойдет для кластеров, в которых идет активная работа (запускаются, дорабатываются новые приложения и т. п.). Обновления функционала до этого канала обновлений доходят не ранее чем через одну неделю после их появления в релизе.
4. **Beta**. Ориентирован на кластеры разработки, как и канал обновлений Alpha. Получает версии, предварительно опробованные на канале обновлений Alpha.
5. **Alpha**. Наименее стабильный канал обновлений с наиболее частым появлением новых версий. Ориентирован на кластеры разработки с небольшим количеством разработчиков.

{% alert %}
Используйте канал обновлений `Early Access` или `Stable`. Установите [окно автоматических обновлений](/documentation/v1/modules/002-deckhouse/usage.html#конфигурация-окон-обновлений) или [ручной режим](/documentation/v1/modules/002-deckhouse/usage.html#ручное-подтверждение-обновлений).
{% endalert %}

Выберите необходимый канал обновлений и [режим обновлений](/documentation/v1/modules/002-deckhouse/configuration.html#parameters-releasechannel) <!---дать ссылку на раздел этой доки), который соответствует вашим ожиданиям. Чем стабильнее канал обновлений, тем позже до него доходит новая функциональность.-->

{% alert level="warning" %}
Даже в очень нагруженных и критичных кластерах не стоит отключать использование канала обновлений. Лучшая стратегия — плановое обновление. В инсталляциях Deckhouse Kubernetes Platform, которые не обновлялись полгода или более, могут присутствовать ошибки. Как правило, эти ошибки давно устранены в новых версиях. В этом случае оперативно решить возникшую проблему будет непросто.
{% endalert %}

**Установка желаемого канала обновлений**

Чтобы перейти на другой канал обновлений автоматически, нужно в [конфигурации](modules/002-deckhouse/configuration.html) модуля `deckhouse` изменить (установить) параметр [releaseChannel](modules/002-deckhouse/configuration.html#parameters-releasechannel).

В этом случае включится механизм [автоматической стабилизации релизного канала](#как-работает-автоматическое-обновление-deckhouse).

Пример конфигурации модуля `deckhouse` с установленным каналом обновлений `Stable`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    releaseChannel: Stable
```

Информацию о том, какая версия Deckhouse находится на копределенном канале обновлений, можно получить на [сайте](https://flow.deckhouse.io).

**Cмена канала обновлений**

* При смене канала обновлений на **более стабильный**, например, с `Alpha` на `EarlyAccess`, Deckhouse скачивает данные о релизе (в примере — из канала `EarlyAccess`) и сравнивает их с данными из существующих в кластере custom resouce'ов `DeckhouseRelease`:
  * Более *поздние* релизы, которые еще не были применены, они находятся в статусе `Pending`, удаляются.
  * Если более *поздние* релизы уже применены, они находятся в статусе `Deployed`, смены релиза не происходит. В этом случае Deckhouse Kubernetes Platform останется на таком релизе до тех пор, пока на канале обновлений `EarlyAccess` не появится более поздний релиз.
* При смене канала обновлений на **менее стабильный**? например, с `EarlyAcess` на `Alpha`, происходит следующее:
  * Deckhouse Kubernetes Platform скачивает данные о релизе (в примере — из канала `Alpha`) и сравнивает их с данными из существующих в кластере custom resource'ов `DeckhouseRelease`.
  * Затем Deckhouse Kubernetes Platform выполняет обновление согласно установленным [параметрам обновления](modules/002-deckhouse/configuration.html#parameters-update).

{% offtopic title="Схема использования параметра releaseChannel при установке и в процессе работы Deckhouse" %}
![Схема использования параметра releaseChannel при установке и в процессе работы Deckhouse](images/common/deckhouse-update-process.png)
{% endofftopic %}

**Обновился ли кластер?**

* Проверьте, что [настроен](#как-установить-желаемый-канал-обновлений) необходимый канал обновлений.
* Проверьте корректность разрешения DNS-имени хранилища образов Deckhouse Kubernetes Platform.

  Получите и сравните IP-адреса хранилища образов Deckhouse Kubernetes Platform (`registry.deckhouse.ru`) на одном из узлов и в поде Deckhouse Kubernetes Platform. Они должны совпадать.

  Пример получения IP-адреса хранилища образов Deckhouse Kubernetes Platform на узле:

  ```shell
  $ getent ahosts registry.deckhouse.ru
  185.193.90.38    STREAM registry.deckhouse.ru
  185.193.90.38    DGRAM
  185.193.90.38    RAW
  ```

  Пример получения IP-адреса хранилища образов Deckhouse Kubernetes Platform в поде Deckhouse Kubernetes Platform:
  
  ```shell
  $ kubectl -n d8-system exec -ti deploy/deckhouse -c deckhouse -- getent ahosts registry.deckhouse.ru
  185.193.90.38    STREAM registry.deckhouse.ru
  185.193.90.38    DGRAM  registry.deckhouse.ru
  ```
  
  Если полученные IP-адреса не совпадают, проверьте настройки [DNS на узле](ссылка).

## Окна обновлений

Управление [окнами обновлений](/documentation/v1/modules/002-deckhouse/configuration.html#parameters-update-windows) позволяет планово обновлять релизы Deckhouse в автоматическом режиме в периоды «затишья», когда нагрузка на кластер далека от пиковой.

  В Deckhouse реализован механизм автоматического обновления. Этот механизм использует [5 каналов обновлений](../../deckhouse-release-channels.html), различающиеся стабильностью и частотой выхода версий. Ознакомьтесь подробнее с тем, [как работает механизм автоматического обновления](../../deckhouse-faq.html#как-работает-автоматическое-обновление-deckhouse) и [как установить желаемый канал обновлений](../../deckhouse-faq.html#как-установить-желаемый-канал-обновлений).
- **[Режим обновлений](configuration.html#parameters-update-mode)** и **[окна обновлений](configuration.html#parameters-update-windows)**

**Временные настройки окон**

Временные настройки позволяют определить удобное время для обновления модулей в Deckhouse Kubernetes Platform. Это позволbт обеспечить стабильность системы во время обновлений и минимизировать возможные негативные влияния на работающие приложения.
Установка обновлений в определенное время позволяет минимизировать возможные проблемы, связанные с нагрузкой на систему во время установки обновлений, а также предотвращает возможные конфликты между обновляемыми модулями и работающими приложениями.

Настроить время, когда Deckhouse будет устанавливать обновления, можно в параметре [update.windows](configuration.html#parameters-update-windows) конфигурации модуля.

Пример настройки двух ежедневных окон обновлений: с 8:00 до 10:00 и c 20:00 до 22:00 (UTC):

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    releaseChannel: EarlyAccess
    update:
      windows: 
        - from: "8:00"
          to: "10:00"
        - from: "20:00"
          to: "22:00"
```

Также можно настроить обновления в определенные дни, например по вторникам и субботам с 18:00 до 19:30 (UTC):

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    releaseChannel: Stable
    update:
      windows: 
        - from: "18:00"
          to: "19:30"
          days:
            - Tue
            - Sat
```

## Получение уведломлений

Информацию о всех версиях Deckhouse Kubernetes Platform можно найти в [списке релизов](https://github.com/deckhouse/deckhouse/releases) Deckhouse.

Сводную информацию о важных изменениях, об обновлении версий компонентов, а также о том, какие компоненты в кластере буду перезапущены в процессе обновления, можно найти в описании нулевой patch-версии релиза. Например, [v1.46.0](https://github.com/deckhouse/deckhouse/releases/tag/v1.46.0) для релиза v1.46 Deckhouse.

Подробный список изменений можно найти в Changelog, ссылка на который есть в каждом [релизе](https://github.com/deckhouse/deckhouse/releases).

### Получение Changelog

Changelog - подробный список изменений, который можно найти для каждого обновления Deckhouse в общем списке релизов. Также, если настроены автоматические оповещения, о которых говорили выше, то ссылка на Changelog передается в строке changelogLink.
Важные изменения в кластере (обновление версии компонентов и их перезапуск, устаревшие компоненты/параметры и т.п.) внедряются в минорных версиях релиза и информацию об этих изменениях можно найти в описании нулевой patch-версии релиза. Например, в v1.49.0 для релиза v1.49 - здесь сообщается, что Docker CRI больше не поддерживается и для обновления необходимо перейти на containerd. Таким образом, перед обновлением необходимо ознакомиться с Changelog и внести соответствующие изменения в кластер, если это требуется.
Для критических изменений, из-за которых обновление невозможно, настроены алерты. Например:
* `D8NodeHasDeprecatedOSVersion` - на нодах установлена устаревшая ОС;
* `HelmReleasesHasResourcesWithDeprecatedVersions` - в helm-релизах используются устаревшие ресурсы;
* `KubernetesVersionEndOfLife` - текущая версия Kubernetes больше не поддерживается.

### Оповещение об обновлении Deckhouse Kubernetes Platform

В режиме обновлений `Auto` можно [настроить](configuration.html#parameters-update-notification) вызов webhook'а для получения оповещения о предстоящем обновлении минорной версии Deckhouse.

Пример настройки оповещения:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    update:
      releaseChannel: Stable
      mode: Auto
      notification:
        webhook: https://release-webhook.mydomain.com
```

После появления новой минорной версии Deckhouse на используемом канале обновлений, но до момента применения ее в кластере на адрес webhook'а будет выполнен [POST-запрос](configuration.html#parameters-update-notification-webhook).

Чтобы всегда иметь достаточно времени для реакции на оповещение об обновлении Deckhouse, достаточно настроить параметр [minimalNotificationTime](configuration.html#parameters-update-notification-minimalnotificationtime). В этом случае обновление случится по прошествии указанного времени с учетом окон обновлений.

Пример:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    update:
      releaseChannel: Stable
      mode: Auto
      notification:
        webhook: https://release-webhook.mydomain.com
        minimalNotificationTime: 8h
```

{% alert %}
Если не указать адрес в параметре [update.notification.webhook](configuration.html#parameters-update-notification-webhook), но указать время в параметре [update.notification.minimalNotificationTime](configuration.html#parameters-update-notification-minimalnotificationtime), применение новой версии все равно будет отложено как минимум на указанное в параметре `minimalNotificationTime` время. В этом случае оповещением о появлении новой версии можно считать появление в кластере ресурса [DeckhouseRelease](cr.html#deckhouserelease), имя которого соответствует новой версии.
{% endalert %}

### Уведомление о процедуре обновления в кластере

Получать заранее информацию об обновлении минорных версий Deckhouse на канале обновлений можно следующими способами:
- Настроить ручной [режим обновлений](modules/002-deckhouse/configuration.html#parameters-update-mode). В этом случае при появлении новой версии на канале обновлений загорится алерт `DeckhouseReleaseIsWaitingManualApproval` и в кластере появится новый custom resource [DeckhouseRelease](modules/002-deckhouse/cr.html#deckhouserelease).
- Настроить автоматический [режим обновлений](modules/002-deckhouse/configuration.html#parameters-update-mode) и указать минимальное время в параметре [minimalNotificationTime](modules/002-deckhouse/configuration.html#parameters-update-notification-minimalnotificationtime), на которое будет отложено обновление. В этом случае при появлении новой версии на канале обновлений в кластере появится новый custom resource [DeckhouseRelease](modules/002-deckhouse/cr.html#deckhouserelease). А если указать URL в параметре [update.notification.webhook](modules/002-deckhouse/configuration.html#parameters-update-notification-webhook), дополнительно произойдет вызов webhook'а.

Во время обновления:
- горит алерт `DeckhouseUpdating`;
- под `deckhouse` не в статусе `Ready`. Если под долго не переходит в статус `Ready`, это может говорить о наличии проблем в работе Deckhouse. Необходима диагностика.

### Уведомление об успешном обновлении в кластере

Если алерт `DeckhouseUpdating` погас, значит, обновление завершено.

Вы также можете проверить состояние [релизов](modules/002-deckhouse/cr.html#deckhouserelease) Deckhouse.

Пример:

```console
$ kubectl get deckhouserelease
NAME       PHASE        TRANSITIONTIME   MESSAGE
v1.46.8    Superseded   13d              
v1.46.9    Superseded   11d              
v1.47.0    Superseded   4h12m            
v1.47.1    Deployed     4h12m            
```

Статус `Deployed` у соответствующей версии говорит о том, что переключение на соответствующую версию было выполнено (но это не значит, что оно закончилось успешно).

Проверьте состояние пода Deckhouse Kubernetes Platform:

```shell
$ kubectl -n d8-system get pods -l app=deckhouse
NAME                   READY  STATUS   RESTARTS  AGE
deckhouse-7844b47bcd-qtbx9  1/1   Running  0       1d
```

* Если статус пода `Running` и в колонке READY указано `1/1` — обновление закончилось успешно.
* Если статус пода `Running` и в колонке READY указано `0/1` — обновление еще не закончилось. Если это продолжается более 20–30 минут, это может говорить о наличии проблем в работе Deckhouse. Необходима диагностика.
* Если статус пода не `Running`, это может говорить о наличии проблем в работе Deckhouse. Необходима диагностика.

{% alert level="info" %}
Возможные варианты действий, если что-то пошло не так:
- Проверьте логи, используя следующую команду:

  ```shell
  kubectl -n d8-system logs -f -l app=deckhouse | jq -Rr 'fromjson? | .msg'
  ```

- Соберите [отладочную информацию](modules/002-deckhouse/faq.html#как-собрать-информацию-для-отладки) и свяжитесь с технической поддержкой.
- Попросите помощи у [сообщества](https://deckhouse.ru/community/about.html).
{% endalert %}

## Обновление в закрытом контуре

Deckhouse Kubernetes Platform использует актуальные версии компонентов для обеспечения стабильности и безопасности системы. Обновления могут включать исправления уязвимостей, улучшение производительности и добавление новых функций.
Закрытый контур может требовать использования специфических версий компонентов или патчей, которые не доступны в стандартных репозиториях. В этом случае, можно настроить Deckhouse на работу со сторонним реестром, который содержит необходимые образы. Кроме того, обновления могут быть необходимы для обеспечения совместимости с другими компонентами в системе или для поддержки новых функций, так как Deckhouse Kubernetes Platform отвечает за то, чтобы кластер одинаково работал на любой поддерживаемой инфраструктуре из следующих:

* в облаках (смотри информацию по соответствующему cloud провайдеру - добавить ссылку);
* на виртуальных машинах или железе (включая on-premises);
* в гибридной инфраструктуре.

Образы всех компонентов Deckhouse Kubernetes Platform, включая control plane, хранятся в высокодоступном и геораспределенном container registry.

### Предварительная настройка

1. В текущем каталоге создайте новый каталог `d8-modules`.

2. Выполните аутентификацию в репозитории вендора, используя в качестве имени пользователя `license-token`, а в качестве пароля ваш лицензионный ключ:

```bash
docker login registry.deckhouse.ru
```

3. Запустите установочный контейнер командой:

```bash
docker run -ti --pull=always -v $(pwd)/d8-modules:/tmp/d8-modules registry.deckhouse.ru/deckhouse/ee/install:stable bash
```

4. Скопируйте утилиту `dhctl` из контейнера в каталог `d8-modules`:

```bash
cp /usr/bin/dhctl /tmp/d8-modules/dhctl
```

5. Завершите работу контейнера.

### Выгрузка образов модулей DKP из репозитория вендора

1. Создайте зашифрованную base64 строку для доступа клиента Docker в репозиторий вендора. Сделать это можно, например, командой ниже, заменив `YOUR_USERNAME` на `license-token`, а `YOUR_PASSWORD` — на ваш лицензионный ключ:

```bash
base64 -w0 <<EOF
  {
    "auths": {
      "registry.deckhouse.ru": {
        "auth": "$(echo -n 'YOUR_USERNAME:YOUR_PASSWORD' | base64 -w0)"
      }
    }
  }
EOF
```

2. Создайте в текущем каталоге файл `ModuleSource`, например, `ms.yml` следующего содержания:

`ms.yml`

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  name: deckhouse
spec:
  registry:
 # Укажите строку, полученную в п.1 вместо CHANGE
    dockerCfg: CHANGE
    repo: registry.deckhouse.ru/deckhouse/ee/modules
    scheme: HTTPS
  # Выберите подходящий канал обновлений: Alpha, Beta, EarlyAccess, Stable, RockSolid
  releaseChannel: "Stable"
```

3. Запустите загрузку модулей DKP из репозитория вендора в локальный каталог рабочей станции:

```bash
dhctl mirror-modules --modules-dir=$(pwd)/d8-modules --module-source=$(pwd)/ms.yml
```

В результате работы утилиты в каталог `d8-modules` будут сохранены все необходимые артефакты, необходимые для переноса модулей DKP в закрытое окружение. Примерный объём данных составляет 7 Гб.

4. Выполните перенос на рабочую станцию в закрытом окружении следующих элементов:

- каталога `d8-modules`
- исполняемого файла `dhctl`

### Загрузка образов модулей DKP в закрытый репозиторий

1. Из каталога рабочей станции в закрытом окружении, содержащего утилиту dhctl и каталог с образами модулей DKP d8-modules, выполните загрузку образов в закрытый репозиторий следующей командой:

```bash
dhctl mirror-modules \
 --modules-dir=$(pwd)/d8-modules \
 --registry="registry.example.com:5000/deckhouse/ee/modules" \
 --registry-login="YOUR_USERNAME" \
 --registry-password="YOUR_PASSWORD"
```

Если ваш репозиторий не требует авторизации, флаги `--registry-login` / `--registry-password` указывать не нужно.

Важно указать верный путь в репозитории: там должна находиться поставка DKP. Таким образом, в примере выше может потребоваться поменять `/deckhouse/ee` на правильный путь размещения образов DKP.

2. Проверьте, что `ModuleSource` с названием `deckhouse` в вашем кластере указывает на верный путь до модулей (`spec.registry.repo`), а также в нем нет ошибок (`status.moduleErrors`).

```bash
kubectl get ms deckhouse -o yaml
```

Пример вывода:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  creationTimestamp: "2024-03-11T20:33:51Z"
  finalizers:
  - modules.deckhouse.io/release-exists
  generation: 1
  labels:
    heritage: deckhouse
  name: deckhouse
  resourceVersion: "20241841"
  uid: f35d10be-3ff9-4cd9-b64c-4f58abd8f595
spec:
  registry:
    ca: |
      -----BEGIN CERTIFICATE-----
      ...
      -----END CERTIFICATE-----
    dockerCfg: ...
    repo: registry.example.com:5000/deckhouse/ee/modules
    scheme: HTTPS
  releaseChannel: ""
status:
  message: ""
  moduleErrors: []
  modules:
  - name: deckhouse-admin
    policy: deckhouse
  - name: deckhouse-commander
    policy: deckhouse
  - name: deckhouse-commander-agent
    policy: deckhouse
  - name: operator-ceph
    policy: deckhouse
  - name: operator-postgres
    policy: deckhouse
  - name: sds-drbd
    policy: deckhouse
  - name: sds-node-configurator
    policy: deckhouse
  - name: secrets-store-integration
    policy: deckhouse
  - name: stronghold
    policy: deckhouse
  - name: virtualization
    policy: deckhouse
  modulesCount: 10
  syncTime: "2024-03-28T14:25:35Z"
```

Обратите внимание, что пустое значение для `spec.releaseChannel` говорит о том, что каналы обновлений для модулей будут совпадать с каналом обновлений для DKP.

3. Проверьте доступность новых выпусков для модулей, выполнив команду:

```bash
kubectl get mr
```

Пример вывода:

```yaml
NAME                               PHASE        UPDATE POLICY   TRANSITIONTIME   MESSAGE
deckhouse-admin-v1.19.3            Superseded                   91s              
deckhouse-admin-v1.21.2            Deployed     deckhouse       91s              
deckhouse-commander-agent-v1.0.1   Deployed                     16d              
deckhouse-commander-v1.2.5         Deployed                     16d              
operator-ceph-v1.0.10              Deployed                     16d              
operator-postgres-v1.0.15          Deployed                     16d              
sds-drbd-v0.1.7                    Deployed                     16d              
sds-drbd-v0.1.8                    Pending      deckhouse       17m              Waiting for manual approval
sds-node-configurator-v0.1.3       Deployed                     16d              
sds-node-configurator-v0.1.7       Pending      deckhouse       17m              Waiting for manual approval
sds-replicated-volume-v0.2.6       Pending      deckhouse       17m              Waiting for manual approval
secrets-store-integration-v1.0.9   Deployed                     16d              
stronghold-v1.0.9                  Deployed                     16d              
virtualization-v0.9.10             Deployed                     16d 
```

Если модуль требует ручного подтверждения обновления, то это можно сделать командой вида:

```bash
kubectl annotate mr sds-drbd-v0.1.8 modules.deckhouse.io/approved="true"
```

### Доступ из изолированных контуров container registry с фиксированным набором IP-адресов

При установке Deckhouse можно настроить на работу со сторонним registry (например, проксирующий registry внутри закрытого контура). Для этого:

Установите следующие параметры в ресурсе `InitConfiguration`:

* `imagesRepo: <PROXY_REGISTRY>/<DECKHOUSE_REPO_PATH>/ee` — адрес образа Deckhouse EE в стороннем registry. Пример: `imagesRepo: registry.deckhouse.ru/deckhouse/ee`;
* `registryDockerCfg: <BASE64>` — права доступа к стороннему registry, зашифрованные в Base64.

Если разрешен анонимный доступ к образам Deckhouse в стороннем registry, `registryDockerCfg` должен выглядеть следующим образом:

```json
{"auths": { "<PROXY_REGISTRY>": {}}}
```

> Приведенное значение должно быть закодировано в Base64.

Если для доступа к образам Deckhouse Kubernetes Platform в стороннем registry необходима аутентификация, `registryDockerCfg` должен выглядеть следующим образом:

```json
{"auths": { "<PROXY_REGISTRY>": {"username":"<PROXY_USERNAME>","password":"<PROXY_PASSWORD>","auth":"<AUTH_BASE64>"}}}
```

где:

* `<PROXY_USERNAME>` — имя пользователя для аутентификации на `<PROXY_REGISTRY>`;
* `<PROXY_PASSWORD>` — пароль пользователя для аутентификации на `<PROXY_REGISTRY>`;
* `<PROXY_REGISTRY>` — адрес стороннего registry в виде `<HOSTNAME>[:PORT]`;
* `<AUTH_BASE64>` — строка вида `<PROXY_USERNAME>:<PROXY_PASSWORD>`, закодированная в Base64.

> Итоговое значение для `registryDockerCfg` должно быть также закодировано в Base64.

Для настройки нестандартных конфигураций сторонних registry в ресурсе `InitConfiguration` предусмотрены еще два параметра:

* `registryCA` — корневой сертификат, которым можно проверить сертификат registry (если registry использует самоподписанные сертификаты);
* `registryScheme` — протокол доступа к registry (`HTTP` или `HTTPS`). По умолчанию — `HTTPS`.

<div markdown="0" style="height: 0;" id="особенности-настройки-сторонних-registry"></div>

  ```
