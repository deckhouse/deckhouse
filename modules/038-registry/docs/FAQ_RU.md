---
title: "Модуль registry: FAQ"
description: "Часто задаваемые вопросы о модуле registry Deckhouse Kubernetes Platform: миграция на модуль, обслуживание кеша и диагностика проблем с registry."
---

## Как устроена миграция на модуль registry?

Управлять путём загрузки образов в Deckhouse Kubernetes Platform (DKP) можно двумя способами:

- **предыдущая реализация** — секция `registry` в
  [ModuleConfig `deckhouse`](/modules/deckhouse/configuration.html#parameters-registry) с
  режимами `Unmanaged`, `Direct`, `Proxy` и `Local`;
- **текущая реализация** — этот модуль, настраиваемый через ModuleConfig `registry`.

Миграция — это передача пути загрузки от предыдущей реализации модулю. Её не нужно запускать
вручную, и отдельной команды для неё нет: модуль принимает управление автоматически после
обновления кластера на релиз, в котором он появился. Единственное условие — к этому моменту
предыдущая реализация должна освободить путь загрузки. Обе реализации настраивают на каждом
узле одно и то же — из какого registry container runtime загружает образы и с какими учётными
данными, — поэтому они никогда не управляют кластером одновременно.

Что нужно сделать для передачи управления и когда — зависит от режима, в котором работает
предыдущая реализация. Режим указан в параметре `settings.registry.mode` ModuleConfig
`deckhouse`:

```bash
d8 k get mc deckhouse -o jsonpath='{.spec.settings.registry.mode}'
```

Пустой вывод означает режим `Unmanaged`: настройки registry в этом кластере никогда не задавали.
Такому кластеру — как и любому другому в `Unmanaged` — для миграции ничего делать не нужно:
передача управления произойдёт сама.

| Режим | Что сделать | Когда |
|---|---|---|
| `Unmanaged` | [Ничего](#как-мигрировать-из-режима-unmanaged) — передача произойдёт сама | — |
| `Direct` | [Настроить модуль](#как-мигрировать-из-режима-direct): `mode: Managed` с `primary.upstream` | До обновления |
| `Proxy` | [Перевести кластер в `Unmanaged`](#как-мигрировать-из-режима-proxy) | До обновления |
| `Local` | [Выполнить процедуру для изолированных кластеров](#как-мигрировать-изолированный-кластер-из-режима-local) | До обновления |

{% alert level="danger" %}
Подготовьте кластер **до** обновления: обновление на релиз с модулем заблокировано, пока кластер
работает в режиме `Proxy` или `Local`, а также в режиме `Direct` без настроенного ModuleConfig
`registry`. В новом релизе нет ни компонентов предыдущей реализации, ни кода, который переключает
её режимы, поэтому вся подготовка выполняется на текущем релизе.
{% endalert %}

Режиму `Direct` переключение не требуется. Настройки модуля намеренно принимаются на релиз
раньше: записанные до обновления, они хранятся без эффекта и срабатывают на первой итерации
согласования модуля после обновления.

Кластер записывает, на какой реализации он фактически работает
([как это проверить](#на-какой-реализации-работает-мой-кластер)). Пока передача управления
невозможна, обновление на релиз с модулем блокируется, и в сообщении об ошибке сказано, что
сделать.

## На какой реализации работает мой кластер?

Когда модуль принимает управление путём загрузки, он записывает это в секрет
`registry-v2-switch`. Проверьте, существует ли секрет:

```bash
d8 k -n d8-system get secret registry-v2-switch >/dev/null 2>&1 \
  && echo "текущая реализация" || echo "предыдущая реализация"
```

Если кластер всё ещё на предыдущей реализации, модуль сообщает причину на каждой итерации
согласования, а в кластере срабатывает алерт
[`D8RegistryMigrationPending`](#что-означают-алерты-модуля-registry). Посмотреть, чего он ждёт:

```bash
d8 k -n d8-system get secret registry-state -o jsonpath='{.data.state}' | base64 -d | head
```

## Как мигрировать из режима Unmanaged?

В режиме `Unmanaged` предыдущая реализация не управляет путём загрузки, поэтому готовить
нечего: передача управления произойдёт сама.

1. Если кластер переводится в `Unmanaged` из другого режима, дождитесь завершения перехода. В
   статусе должно быть `mode: Unmanaged` без ожидающего целевого режима:

   ```bash
   d8 k -n d8-system get secret registry-state -o jsonpath='{.data.state}' | base64 -d | head
   ```

1. Обновите кластер на релиз с модулем (или, если он уже обновлён, просто дождитесь следующей
   итерации согласования). Модуль примет управление автоматически, и поведение не изменится:
   режим модуля по умолчанию — тоже `Unmanaged`, поэтому кластер продолжит загружать образы из
   того же registry, что и раньше.

1. Чтобы модуль начал управлять путём загрузки, задайте `mode: Managed` в ModuleConfig
   `registry` и укажите registry, из которого загружать образы. Готовая конфигурация для вашего
   кластера публикуется в секрете `registry-suggested-config`; как её применить, показано в примере
   [«Включение модуля»](examples.html#включение-модуля).

## Как мигрировать из режима Direct?

В режиме `Direct` узлы загружают образы через внутрикластерный адрес, который обслуживает
прокси предыдущей реализации. Модуль обслуживает тот же адрес, поэтому миграция — это прямая
передача адреса: переход через `Unmanaged` не нужен, и компоненты не перезапускаются.

1. Настройте модуль до обновления — без этой конфигурации обновление заблокировано. Возьмите
   значения из секции `registry.direct` в
   ModuleConfig `deckhouse`: `imagesRepo` разделяется на `host` и `path`, учётные данные — тот
   же `license` (или `username`/`password`), плюс `ca`, если registry его требует:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: registry
   spec:
     enabled: true
     version: 1
     settings:
       mode: Managed
       primary:
         upstream:
           scheme: HTTPS
           host: registry.deckhouse.io
           path: /deckhouse/ee
           auth:
             license: <LICENSE_KEY>
   ```

   Предыдущий релиз сохраняет эти настройки, но не действует по ним, поэтому до обновления в
   кластере ничего не меняется.

1. Обновите кластер. Передача произойдёт на следующей итерации согласования модуля, и загрузка
   образов всё это время работает: Service и прокси предыдущей реализации продолжают
   обслуживать внутрикластерный адрес, пока агент модуля не примет его на каждом узле, и только
   после этого контроллер их удаляет.

1. Следите за ходом передачи:

   ```bash
   d8 k -n d8-system get secret registry-v2-switch >/dev/null 2>&1 && echo "управление передано"
   d8 k get registrynode -o custom-columns='NODE:.metadata.name,READY:.status.reconciled,SERVING:.status.proxyListening'
   ```

   Для ориентира — тайминги, измеренные на тестовом кластере: передача была зафиксирована
   примерно через две минуты после старта новой версии, агент появился на узлах ещё примерно
   через семь минут, а объекты предыдущей реализации были удалены через минуту после этого.
   Загрузка образов работала на каждом из этих этапов.

## Как мигрировать из режима Proxy?

Режим `Proxy` держит на каждом узле собственный прокси со своими сертификатами — состояние,
которое модуль перенять не может. Поэтому кластер сначала нужно перевести в `Unmanaged`, и
сделать это можно только до обновления.

1. В ModuleConfig `deckhouse` задайте `registry.mode: Unmanaged`, сохранив тот же адрес
   registry и учётные данные. Готовые манифесты приведены в
   [примерах переключения режимов](examples.html#примеры-для-предыдущей-реализации). Все узлы
   будут перенастроены на загрузку напрямую из внешнего registry, поэтому кеширование, которое
   давал `Proxy`, пропадёт до шага 4.

1. Дождитесь завершения перехода — `mode: Unmanaged` без ожидающего целевого режима. Кластер,
   застигнутый посреди перехода, мигрировать нельзя; в статусе видно, в какой режим он ещё
   переключается:

   ```bash
   d8 k -n d8-system get secret registry-state -o jsonpath='{.data.state}' | base64 -d | head
   ```

1. Обновите кластер. Передача произойдёт на следующей итерации согласования модуля и не изменит
   поведения: в режиме `Unmanaged` модуль тоже не управляет путём загрузки.

1. Чтобы вернуть кеширование внутри кластера, задайте `mode: Managed` с `storage.cache: true` и
   тем же внешним registry. Учтите, что кеш модуля устроен иначе, чем `Proxy`: одно хранилище с
   репликами на master-узлах вместо прокси на каждом узле. Прежде чем включать его на кластере,
   где на master-узлах мало свободного места, прочитайте,
   [как кеш наполняется и очищается](#кеш-растёт-что-его-чистит).

## Как мигрировать изолированный кластер из режима Local?

В режиме `Local` у кластера нет внешнего registry — его роль выполняет хранилище внутри самого
кластера. Поэтому стандартная процедура миграции здесь не работает: она проходит через режим
`Unmanaged`, в котором каждый узел загружает образы напрямую из внешнего registry, а такому
кластеру загружать их неоткуда.

Выход в том, чтобы дать кластеру внешний registry на время миграции. Он запускается в том же
кластере, но вне неймспейсов DKP; кластер переключается на него и обновляется, а
когда хранилище модуля наполнится образами, временный registry удаляется. Процедура проверена
от начала до конца на тестовом кластере.

### Требования к диску

Перед началом убедитесь, что на master-узлах свободно место под **четыре набора образов**: на
пике миграции одновременно существуют три копии набора — хранилище `Local`, временный registry
и наполняющееся хранилище модуля, — а четвёртый набор нужен как запас, который узел не должен
исчерпать.

Масштаб можно оценить по цифрам тестовой миграции: набор образов в 13 ГиБ (платформа с
модулями) занял 21 ГиБ во временном registry (там лежали два релиза — тот, на котором кластер
работал, и тот, на который обновлялся) и увеличил хранилище с 13,0 до 21,4 ГиБ, не считая кеша
образов самого узла и системы. На master-узле со 100 ГиБ диска пик занял 38 ГиБ, а на узле с
50 ГиБ та же миграция закончилась вытеснением подов.

Запас нужен не для перестраховки. Когда свободное место на master-узле заканчивается, kubelet
начинает вытеснять поды и удалять образы, которые считает неиспользуемыми, — а в кластере, чей
registry работает внутри него самого, удалённым может оказаться образ самого registry. Тогда
хранилище остаётся без процесса, который его обслуживает, а узел — без места, откуда этот образ
можно загрузить. В тесте эта взаимная блокировка продержалась 99 минут, и разрешить её удалось только
ручной загрузкой образов на узел. Поэтому рассчитайте место заранее и следите за диском, пока
идёт миграция.

Часть места экономится за счёт того, что образы, уже лежащие на дисках master-узлов, хранилище
модуля проверяет и принимает на месте, а не скачивает заново (шаг 6). Но превратить два
разных релиза в один набор такая проверка не может: в хранилище оказываются и релиз, на котором
работал старый кластер, и релиз, на котором работает новый, — отсюда и размер пика.

### Порядок действий

Шаги 1–4 выполняются до обновления, на релизе, в котором ещё есть предыдущая реализация.

1. Запустите временный OCI-registry в собственном неймспейсе. Реализация registry может
   быть любой, но он должен отдавать TLS с сертификатом, который кластер сможет проверить, и
   DKP не должна им управлять: что бы ни происходило с объектами модуля, временный
   registry должен продолжать работать.

   Кроме того, registry должен быть доступен **по одному и тому же адресу из двух мест**: на
   шаге 3 из него загружают образы узлы, а на шаге 6 его читает syncer модуля, работающий в
   поде. Сертификат должен покрывать выбранный адрес. Вот варианты, проверенные на одном
   кластере:

   | Адрес | С узла | Из пода |
   |---|---|---|
   | порт `hostNetwork` на IP самого узла (`<NODE_IP>:5000`) | работает | работает |
   | имя Service (`<SERVICE>.<NAMESPACE>.svc:<PORT>`) | не разрешается | работает |
   | NodePort на IP узла | работает | не работает: `operation not permitted` |

   Используйте первый вариант: запустите registry с `hostNetwork: true` на одном узле,
   обращайтесь к нему по IP этого узла и добавьте этот IP в SAN сертификата — тогда вся
   миграция пройдёт на одном адресе. Имя Service выглядит аккуратнее, но перестаёт работать на шаге 3,
   потому что container runtime узла не разрешает имена через кластерный DNS.

1. Загрузите набор образов во временный registry: сначала скачайте его командой
   `d8 mirror pull` на машине, у которой есть доступ к registry DKP, а затем отправьте во
   временный registry командой `d8 mirror push`. Именно эта копия набора учтена выше в расчёте
   места на диске.

1. Направьте предыдущую реализацию на временный registry и переведите её в `Unmanaged`: в
   ModuleConfig `deckhouse` задайте `registry.mode: Unmanaged` вместе с адресом, сертификатом
   CA и учётными данными временного registry. После этого узлы начнут загружать образы из него,
   а хранилище `Local` уйдёт с пути загрузки — но его данные останутся на месте, в каталоге
   `/opt/deckhouse/registry` на master-узлах.

1. Убедитесь, что переход завершён и образы действительно загружаются из временного registry, —
   на этом шаге держится вся остальная миграция. Проверки те же, что и для любого кластера в
   `Unmanaged`:

   ```bash
   d8 k -n d8-system get secret registry-state -o jsonpath='{.data.state}' | base64 -d | head
   ```

1. Обновите кластер на релиз с модулем. Передача управления произойдёт на следующей итерации
   согласования, и всё это время кластер продолжит загружать образы из временного registry.

1. Включите модуль: задайте `mode: Managed` с `storage.cache: true`, укажите временный registry
   в качестве `primary.upstream` — **и добавьте `storage.source` в том же изменении**. Хранилище
   модуля запустится на том же пути на хосте, который использовал `Local`, поэтому уже лежащие
   на дисках образы не будут скачиваться заново: наполнение проверит их и дозагрузит только
   недостающее.

   Откладывать `storage.source` нельзя: без него не получится убрать внешний registry на
   следующем шаге, потому что конфигурация `Managed` без `primary.upstream` принимается только
   при заданном `storage.source`, — следующий шаг будет отклонён с ошибкой
   `'storage.source' is required when 'primary.upstream' is not set`.

   В `storage.source` поле `bundleRef` — это произвольное имя набора образов, а
   `expectedDigests` — число различных дайджестов в нём. Посчитать их можно по бандлу с
   шага 2:

   ```bash
   for tar in <BUNDLE_DIR>/*.tar; do tar -xOf "$tar" --wildcards '*index.json'; done |
     jq -r '.manifests[]?.digest' | sort -u | wc -l
   ```

   Изменение этих настроек перезапускает процесс registry, поэтому ресурс RegistryStorage примерно
   на минуту перейдёт в `Failed` с ошибкой чтения собственного хранилища. Это ожидаемо —
   дождитесь возврата в `Ready`.

1. Дождитесь, пока хранилище сообщит, что держит весь набор (`phase: Ready` и
   `safeToDropUpstream: true`), и уберите `primary.upstream` из ModuleConfig `registry`. После
   этого кластер снова изолирован — теперь уже на модуле:

   ```bash
   d8 k get registrystorage registry -o jsonpath='{.status.phase} {.status.safeToDropUpstream}{"\n"}'
   ```

1. Удалите временный registry и освободите занятый им диск.

## Что означают алерты модуля registry?

Ни один из этих алертов не означает, что кластер перестал загружать образы. Большинство
сообщает о состоянии, в котором всё работает, но не так, как задано в конфигурации, — такое
состояние легко не заметить.

`D8RegistryMigrationPending`
: Кластер всё ещё работает на предыдущей реализации. Ничего не сломано — миграция не завершена.
  Дальнейшие действия описаны в разделе
  [«Как устроена миграция на модуль registry»](#как-устроена-миграция-на-модуль-registry).

`D8RegistryConfigInvalid`
: Конфигурация отклонена; кластер продолжает работать с прежней. Причина — в
  `.status.conditions` ресурса `registryconfig/registry`.

`D8RegistryNodeNotConverged`
: Агент на части узлов не применил выданную ему конфигурацию. Эти узлы продолжают загружать
  образы по старой конфигурации, и следующие изменения до них тоже не дойдут.

`D8RegistryNodeRunningFromDisk`
: Часть узлов не может подключиться к API-серверу и маршрутизирует загрузки по копии
  конфигурации на диске. Этот резервный механизм работает как задуман: загрузки на таких узлах
  проходят успешно, поэтому больше ничто о проблеме не сообщит, — а конфигурация узлов тем
  временем может сколь угодно сильно отстать от кластерной.

`D8RegistryStorageIncomplete`
: Часть реплик кеша не держит весь ожидаемый набор образов. Пока настроен внешний registry, на
  загрузки это не влияет, но его потерю кластер бы не пережил — и именно это блокирует переход
  в air-gap.

`D8RegistryAirGapTransitionHeld`
: Вы убрали внешний registry из конфигурации, но модуль продолжает им пользоваться, потому что
  кеш пока не может обслуживать кластер сам. Это безопасный исход: отключение внешнего registry
  при неполном кеше оставило бы узлы без источника образов. Алерт не разрешится сам, если кеш
  перестал наполняться.

`D8RegistryUpstreamProbeFailing`
: Изменение основного внешнего registry отклонено, и кластер продолжает пользоваться последним
  работавшим. Метка `outcome` различает три проблемы: `unreachable` — сеть или сам registry;
  `auth` — обычно истёкший лицензионный ключ; `sentinel` — registry ответил и принял учётные
  данные, но не содержит образов DKP (обычно неверный путь репозитория).

`D8RegistryUpstreamRejected`
: Ресурс RegistryUpstream не принят, поэтому загрузки для указанного в нём registry не
  перехватываются. Метка `reason` говорит, конфликтует он с основным registry или с другим
  ресурсом, претендующим на то же имя.

`D8RegistryStorageNotReclaimed`
: Ни одна реплика не выполняла сборку мусора неделю. Сборка — единственный механизм, который
  удаляет данные из хранилища, поэтому, если она остановилась, диск рано или поздно заполнится.
  Подробнее о сборке — в разделе [«Кеш растёт. Что его чистит»](#кеш-растёт-что-его-чистит).

`D8RegistryStaleCacheData`
: На узле лежат данные кеша, которые никто не использует. Как освободить место, описано в
  разделе [«Как удалить с узла оставшиеся данные кеша»](#как-удалить-с-узла-оставшиеся-данные-кеша).

## Как удалить с узла оставшиеся данные кеша?

При выключении кеша данные в `/opt/deckhouse/registry` намеренно сохраняются: если включить кеш
обратно, он наполнится из того, что уже лежит на диске, а не будет скачивать всё заново — на
медленном канале это экономит часы. Автоматическое удаление сделало бы выключение кеша
необратимым, поэтому решение модуль оставляет вам: агент измеряет оставшиеся данные и поднимает
алерт [`D8RegistryStaleCacheData`](#что-означают-алерты-модуля-registry).

Сколько места занимают данные:

```bash
d8 k get registrynodes -o custom-columns=\
NODE:.metadata.name,STALE:.status.staleStorageDataBytes
```

Если включать кеш обратно вы не собираетесь, удалите каталог на узле:

```bash
ssh <NODE> 'du -sh /opt/deckhouse/registry && sudo rm -rf /opt/deckhouse/registry'
```

## Кеш растёт. Что его чистит?

Сборка мусора, которую по расписанию выполняют сами реплики хранилища.

Это единственный механизм, который вообще что-то удаляет из хранилища. Каждый релиз DKP
добавляет новые образы, поэтому без сборки хранилище кластера, живущего годами, рано или поздно
заполнится и перестанет принимать запись, — а изолированный кластер с заполненным хранилищем
нельзя обновить.

Сборка удаляет образы релизов, которые кластер уже прошёл. Сохраняются:

- развёрнутый релиз и предыдущий — чтобы откат не скачивал заново то, к чему откатывается;
- всё, что новее развёрнутого релиза, — это обновление в процессе или, в изолированном
  кластере, релиз, загруженный намеренно;
- все теги, которые не являются версиями: имена каналов обновлений вроде `stable`, плавающие
  теги, всё загруженное вручную. Сборщик не может знать, что они означают, поэтому не трогает их.

Сборщик намеренно осторожен. В изолированном кластере удаление ещё нужного блоба невосстановимо
без повторного `d8 mirror push`, а хранение ненужного стоит только места на диске. Поэтому
проход, который не может определить, что сохранять — например, когда не найден развёрнутый
релиз, — не делает ничего вовсе.

Посмотреть состояние и расписание сборки:

```bash
d8 k get registrystorage registry -o jsonpath='{.status.replicas}' | jq \
  'map({node, collectedAt, collectionError})'
d8 k get registrystorage registry -o jsonpath='{.spec.garbageCollection}' | jq
```

### Когда сборка запускается и почему реплика переходит в режим read-only

Сборщик registry сначала вычисляет множество достижимых блобов, а затем удаляет остальные —
поэтому блоб, загруженный между этими шагами, был бы удалён. Сборку безопасно выполнять только
на хранилище, в которое никто не пишет, поэтому реплика на время сборки не принимает запись.

Все свои образы реплика при этом продолжает отдавать. Чего она не может во время сборки:

- сохранить результат промаха кеша — агент на узле обращается к внешнему registry, то есть
  загрузка становится медленнее, но не завершается ошибкой;
- принять `d8 mirror push` — он завершается с явной ошибкой, и его можно повторить.

Одновременно сборку выполняет только одна реплика; остальные работают как обычно.

По умолчанию сборка назначается на ночной час, а если у NodeGroup `master` задано окно
обслуживания — на его начало: этот час уже объявлен допустимым для перерывов. Задать своё
расписание:

```yaml
spec:
  settings:
    storage:
      garbageCollection:
        schedule: "0 2 * * Sun"
```

Нечитаемое cron-выражение отклоняется, а не угадывается: сборка в неожиданный час хуже, чем её
отсутствие.

### Как выключить

Сборка мусора выключается в настройках модуля:

```yaml
spec:
  settings:
    storage:
      garbageCollection:
        enabled: false
```

Это имеет смысл только с диском, размера которого хватит при неограниченном росте хранилища.
Обратите внимание: алерт
[`D8RegistryStorageNotReclaimed`](#что-означают-алерты-модуля-registry) всё равно сработает
через неделю после последней сборки — снаружи «выключено» и «молча перестало работать» выглядят
одинаково.

## На узле не загружается образ. Куда смотреть?

Начните с агента: он стоит на пути каждой загрузки на узле. Агент работает статическим подом,
поэтому доступен, даже когда кластер — нет.

Логи агента:

```bash
d8 k -n kube-system logs -l component=registry-agent --tail=100
```

Какую конфигурацию агент получил и согласен ли он с кластером:

```bash
d8 k get registrynode <NODE> -o jsonpath='{.status}' | jq
```

Метрики агента — его собственный взгляд на проходящие через него загрузки. Они читаются
напрямую с узла, а не через Prometheus: агент работает статическим подом именно потому, что
должен работать при недоступном API-сервере, а kube-rbac-proxy рядом с ним аутентифицировался
бы в том самом API-сервере:

```bash
ssh <NODE> 'curl -s http://127.0.0.1:4286/metrics | grep d8_registry_agent'
```

Конфигурация, выданная container runtime. Это один файл независимо от того, сколько registry
настроено:

```bash
ssh <NODE> 'cat /etc/containerd/registry.d/_default/hosts.toml'
```

Если этого файла нет, агент ещё не применил конфигурацию: на узле не загрузится ничего, а
причина — в логе агента. Если файл есть, а загрузки всё равно завершаются ошибкой, отказ находится за
агентом — метрики выше называют, какая цель отказала и почему.

## Как посмотреть состояние внутрикластерного кеша?

Состояние кеша публикуется в статусе ресурса RegistryStorage:

```bash
d8 k get registrystorage registry -o jsonpath='{.status}' | jq
```

Пояснения к полям:

- `replicas` — единственное место, где сообщается полнота кеша; каждая запись — отчёт реплики о
  самой себе. Реплика с `full: true` рядом с `error` полной не считается: `full` говорит о том,
  что она держит, а ошибка — о том, завершился ли её последний проход.
- `leader` — реплика, которая наполняется из внешнего registry и служит источником репликации
  для остальных. Выборы намеренно несимметричны: на лидерство претендует только реплика с
  полным набором, а лидер уступает, когда полный набор появляется у другой. Именно это не даёт
  изолированному кластеру застрять в ситуации, когда лидер пуст, а полный набор есть только у
  другой реплики.

## Предыдущая реализация

Всё, что ниже, относится к кластеру, который всё ещё работает на реализации, настраиваемой
через ModuleConfig `deckhouse`.

### Как мигрировать на модуль registry?

Во время миграции, для containerd v1 будет выполнен переход на новую схему конфигурации registry.
containerd v2 использует новую схему по умолчанию. Подробнее можно ознакомиться в разделе [с описанием способов конфигурации](/modules/node-manager/latest/faq.html#как-добавить-конфигурацию-для-дополнительного-registry)

#### Для containerd v2

1. Выполните переключение на использование модуля `registry`. Для этого, укажите в ModuleConfig `deckhouse` параметры режима `Unmanaged`. Если используется registry, отличный от `registry.deckhouse.ru`, ознакомьтесь с конфигурацией модуля [deckhouse](/modules/deckhouse/latest/configuration.html) для корректной настройки.

   Посмотреть текущие настройки registry можно с помощью команды:

   ```bash
   d8 k -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller global values | yq e '.modulesImages.registry' -
   ```

   Данные настройки укажите при конфигурации `Unmanaged` режима:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: deckhouse
   spec:
     version: 1
     enabled: true
     settings:
       registry:
         mode: Unmanaged
         unmanaged:
           imagesRepo: registry.deckhouse.ru/deckhouse/ee
           scheme: HTTPS
           license: <LICENSE_KEY> # Замените на ваш лицензионный ключ
   ```

1. Дождитесь завершения переключения. Пример [статуса переключения](#как-посмотреть-статус-переключения-режима-registry):

   ```console
   conditions:
   # ...
     - lastTransitionTime: "..."
       message: ""
       reason: ""
       status: "True"
       type: Ready
   hash: ..
   mode: Unmanaged
   target_mode: Unmanaged
   ```

#### Для containerd v1

{% alert level="danger" %}
- Во время переключения containerd v1 сервис будет перезапущен.
- Во время переключения containerd v1 будет переведен на новую схему конфигурации registry.
- Во время переключения, [пользовательские конфигурации registry](/modules/node-manager/latest/faq.html#как-добавить-конфигурацию-для-дополнительного-registry) для containerd v1 будут временно недоступны.
{% endalert %}

1. Убедитесь, что на узлах с containerd v1 отсутствуют [пользовательские конфигурации registry](/modules/node-manager/latest/faq.html#как-добавить-конфигурацию-для-дополнительного-registry), расположенные в директории `/etc/containerd/conf.d`.

1. Если конфигурации присутствуют, необходимо выполнить миграцию на новый формат конфигурации registry в containerd. Для этого, необходимо добавить новые конфигурации в директорию `/etc/containerd/registry.d`. Данные конфигурации вступят в силу после переключения на модуль `registry`. Для добавления конфигураций подготовьте NodeGroupConfiguration, подробнее в разделе [с описанием способов конфигурации](/modules/node-manager/latest/faq.html#как-добавить-конфигурацию-для-дополнительного-registry). Пример:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: containerd-additional-config-auth.sh
   spec:
     # Шаг может быть любой, так как не требуется перезапуск сервиса containerd.
     weight: 0
     bundles:
       - '*'
     nodeGroups:
       - "*"
     content: |
       # Copyright 2023 Flant JSC
       #
       # Licensed under the Apache License, Version 2.0 (the "License");
       # you may not use this file except in compliance with the License.
       # You may obtain a copy of the License at
       #
       #     http://www.apache.org/licenses/LICENSE-2.0
       #
       # Unless required by applicable law or agreed to in writing, software
       # distributed under the License is distributed on an "AS IS" BASIS,
       # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
       # See the License for the specific language governing permissions and
       # limitations under the License.
       
       REGISTRY_URL=private.registry.example

       mkdir -p "/etc/containerd/registry.d/${REGISTRY_URL}"
       bb-sync-file "/etc/containerd/registry.d/${REGISTRY_URL}/hosts.toml" - << EOF
       [host]
         [host."https://${REGISTRY_URL}"]
           capabilities = ["pull", "resolve"]
           [host."https://${REGISTRY_URL}".auth]
             username = "username"
             password = "password"
       EOF
   ```

1. Примените [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration). Дождитесь появления конфигурационных файлов в директории `/etc/containerd/registry.d` на всех узлах.

1. Проверьте корректность работы конфигураций. Для этого воспользуйтесь командой:

   ```bash
   # Для https:
   ctr -n k8s.io images pull --hosts-dir=/etc/containerd/registry.d/ private.registry.example/registry/path:tag

   # Для http:
   ctr -n k8s.io images pull --hosts-dir=/etc/containerd/registry.d/ --plain-http private.registry.example/registry/path:tag
   ```

1. Выполните переключение на использование модуля `registry`. Для этого, укажите в ModuleConfig `deckhouse` параметры режима `Unmanaged`. Если используется registry, отличный от `registry.deckhouse.ru`, ознакомьтесь с конфигурацией модуля [deckhouse](/modules/deckhouse/latest/configuration.html) для корректной настройки.

   Посмотреть текущие настройки registry можно с помощью команды:

   ```bash
   d8 k -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller global values | yq e '.modulesImages.registry' -
   ```

   Данные настройки укажите при конфигурации `Unmanaged` режима:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: deckhouse
   spec:
     version: 1
     enabled: true
     settings:
       registry:
         mode: Unmanaged
         unmanaged:
           imagesRepo: registry.deckhouse.ru/deckhouse/ee
           scheme: HTTPS
           license: <LICENSE_KEY> # Замените на ваш лицензионный ключ
   ```

1. После применения, дождитесь в [статусе переключения](#как-посмотреть-статус-переключения-режима-registry) сообщение:

   Пример вывода:

   ```console
   conditions:
   # ...
   - lastTransitionTime: "2025-08-13T15:22:34Z"
     message: |
       Check current nodes configuration
       2/2 node(s) Unready:
       - master-0: has custom toml merge containerd configuration
       - worker-5e389be0-578df-s5sm5: has custom toml merge containerd configuration
     reason: Processing
     status: "False"
     type: ContainerdConfigPreflightReady
   ```

   Данное сообщение означает, что на узлах имеются старые конфигурации registry, расположенные в директории `/etc/containerd/conf.d`. И в данный момент переключение на новую конфигурацию containerd заблокировано. Для того, чтобы разрешить переключение, необходимо удалить старые конфигурационные файлы.

1. Удалите старые конфигурационные файлы, чтобы разрешить переключение на модуль `registry`. Для этого создайте [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration). Пример манифеста NodeGroupConfiguration:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: containerd-additional-config-auth-delete.sh
   spec:
     # Шаг должен выполниться до '032_configure_containerd.sh'
     weight: 0
     bundles:
       - '*'
     nodeGroups:
       - "*"
     content: |
       # Copyright 2023 Flant JSC
       #
       # Licensed under the Apache License, Version 2.0 (the "License");
       # you may not use this file except in compliance with the License.
       # You may obtain a copy of the License at
       #
       #     http://www.apache.org/licenses/LICENSE-2.0
       #
       # Unless required by applicable law or agreed to in writing, software
       # distributed under the License is distributed on an "AS IS" BASIS,
       # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
       # See the License for the specific language governing permissions and
       # limitations under the License.

       file="/etc/containerd/conf.d/old-config.toml"

       [ -f "$file" ] && rm -f "$file"
   ```
  
1. После удаления старых конфигураций, убедитесь, что переключение продолжило выполняться. Пример [статуса переключения](#как-посмотреть-статус-переключения-режима-registry):

   ```console
   conditions:
   # ...
   - lastTransitionTime: "2025-08-13T16:42:09Z"
     message: ""
     reason: ""
     status: "True"
     type: ContainerdConfigPreflightReady
   ```

1. Дождитесь завершения переключения. Пример [статуса переключения](#как-посмотреть-статус-переключения-режима-registry):

   ```console
   conditions:
   # ...
     - lastTransitionTime: "..."
       message: ""
       reason: ""
       status: "True"
       type: Ready
   hash: ..
   mode: Unmanaged
   target_mode: Unmanaged
   ```

1. Удалите [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration), созданный на шаге удаления старых конфигурационных файлов:

   ```shell
   d8 k delete nodegroupconfiguration containerd-additional-config-auth-delete.sh
   ```

   Чтобы убедиться, что NodeGroupConfiguration удалён, используйте команду:

   ```shell
   d8 k get nodegroupconfiguration
   ```

   В списке не должно быть NodeGroupConfiguration, подлежащего удалению (в этом примере — `containerd-additional-config-auth-delete.sh`).

### Как мигрировать обратно с модуля registry?

{% alert level="danger" %}
- Это устаревший (deprecated) формат управления registry.
- Во время переключения containerd v1 будет перезапущен.
- Во время переключения containerd v1 будет переведен на старую схему конфигурации registry.
- Во время переключения, [пользовательские конфигурации registry](/modules/node-manager/latest/faq.html#как-добавить-конфигурацию-для-дополнительного-registry) для containerd v1 будут временно недоступны.
{% endalert %}

1. Переведите registry в режим `Unmanaged`. Если используется registry, отличный от `registry.deckhouse.ru`, ознакомьтесь с конфигурацией модуля [deckhouse](/modules/deckhouse/latest/configuration.html) для корректной настройки.

   Пример конфигурации:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: deckhouse
   spec:
     version: 1
     enabled: true
     settings:
       registry:
         mode: Unmanaged
         unmanaged:
           imagesRepo: registry.deckhouse.ru/deckhouse/ee
           scheme: HTTPS
           license: <LICENSE_KEY> # Замените на ваш лицензионный ключ
   ```

1. Проверьте статус переключения, используя [инструкцию](#как-посмотреть-статус-переключения-режима-registry). Пример вывода:

   ```console
   conditions:
   # ...
   - lastTransitionTime: "..."
     message: ""
     reason: ""
     status: "True"
     type: Ready
   hash: ..
   mode: Unmanaged
   target_mode: Unmanaged
   ```

1. Переведите registry в неконфигурируемый режим `Unmanaged`. Пример конфигурации:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: deckhouse
   spec:
     version: 1
     enabled: true
     settings:
       registry:
         mode: Unmanaged
   ```

1. Проверьте статус переключения, используя [инструкцию](#как-посмотреть-статус-переключения-режима-registry). Пример вывода:

   ```console
   conditions:
   # ...
   - lastTransitionTime: "..."
     message: ""
     reason: ""
     status: "True"
     type: Ready
   hash: ..
   mode: Unmanaged
   target_mode: Unmanaged
   ```

1. Если используется containerd v1, и в кластере применены [пользовательские конфигурации registry](/modules/node-manager/latest/faq.html#как-добавить-конфигурацию-для-дополнительного-registry), их необходимо заменить на старый формат. Для этого, подготовьте конфигурации registry старого формата. Данные конфигурации на данном этапе применять не нужно. Пример конфигурации:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: containerd-additional-config-auth.sh
   spec:
     # Для добавления файла перед шагом '032_configure_containerd.sh'
     weight: 31
     bundles:
       - '*'
     nodeGroups:
       - "*"
     content: |
       # Copyright 2023 Flant JSC
       #
       # Licensed under the Apache License, Version 2.0 (the "License");
       # you may not use this file except in compliance with the License.
       # You may obtain a copy of the License at
       #
       #     http://www.apache.org/licenses/LICENSE-2.0
       #
       # Unless required by applicable law or agreed to in writing, software
       # distributed under the License is distributed on an "AS IS" BASIS,
       # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
       # See the License for the specific language governing permissions and
       # limitations under the License.

       REGISTRY_URL=private.registry.example

       mkdir -p /etc/containerd/conf.d
       bb-sync-file /etc/containerd/conf.d/additional_registry.toml - << EOF
       [plugins]
         [plugins."io.containerd.grpc.v1.cri"]
           [plugins."io.containerd.grpc.v1.cri".registry]
             [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
               [plugins."io.containerd.grpc.v1.cri".registry.mirrors."${REGISTRY_URL}"]
                 endpoint = ["https://${REGISTRY_URL}"]
             [plugins."io.containerd.grpc.v1.cri".registry.configs]
               [plugins."io.containerd.grpc.v1.cri".registry.configs."${REGISTRY_URL}".auth]
                 username = "username"
                 password = "password"
                 # OR
                 auth = "dXNlcm5hbWU6cGFzc3dvcmQ="
       EOF
   ```

1. Удалите секрет `registry-bashible-config`. Во время удаления, containerd v1 переключится на старый формат конфигурации containerd:

   ```bash
   d8 k -n d8-system delete secret registry-bashible-config
   ```

1. После удаления дождитесь завершения переключения. Для отслеживания используйте [инструкцию](#как-посмотреть-статус-переключения-режима-registry). Пример вывода:

   ```console
   conditions:
   # ...
   - lastTransitionTime: "..."
     message: ""
     reason: ""
     status: "True"
     type: Ready
   hash: ..
   mode: Unmanaged
   target_mode: Unmanaged
   ```

1. Если используется containerd v1, примените заготовленные этапом ранее NodeGroupConfiguration с пользовательскими конфигурациями registry.

1. Отключите модуль `registry`. Пример:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: registry
   spec:
     enabled: false
     settings: {}
     version: 1
   ```

### Как посмотреть статус переключения режима registry?

Статус переключения режима registry можно получить с помощью следующей команды:

<!-- TODO(nabokihms): заменить на подкоманду d8, когда она будет реализована -->
```bash
d8 k -n d8-system -o yaml get secret registry-state | yq -C -P '.data | del .state | map_values(@base64d) | .conditions = (.conditions | from_yaml)'
```

Пример вывода:

```console
conditions:
  - lastTransitionTime: "2025-07-15T12:52:46Z"
    message: 'registry.deckhouse.ru: all 157 items are checked'
    reason: Ready
    status: "True"
    type: RegistryContainsRequiredImages
  - lastTransitionTime: "2025-07-11T11:59:03Z"
    message: ""
    reason: ""
    status: "True"
    type: ContainerdConfigPreflightReady
  - lastTransitionTime: "2025-07-15T12:47:47Z"
    message: ""
    reason: ""
    status: "True"
    type: TransitionContainerdConfigReady
  - lastTransitionTime: "2025-07-15T12:52:48Z"
    message: ""
    reason: ""
    status: "True"
    type: InClusterProxyReady
  - lastTransitionTime: "2025-07-15T12:54:53Z"
    message: ""
    reason: ""
    status: "True"
    type: DeckhouseRegistrySwitchReady
  - lastTransitionTime: "2025-07-15T12:55:48Z"
    message: ""
    reason: ""
    status: "True"
    type: FinalContainerdConfigReady
  - lastTransitionTime: "2025-07-15T12:55:48Z"
    message: ""
    reason: ""
    status: "True"
    type: Ready
mode: Direct
target_mode: Direct
```

Вывод отображает состояние процесса переключения. Каждое условие может находиться в статусе `True` или `False`, а также содержать поле `message` с пояснением.

Описание условий:

| Условие                           | Описание                                                                                                                                                                                                                     |
| --------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `ContainerdConfigPreflightReady`  | Состояние проверки конфигурации containerd. Проверяется, что на узлах отсутствуют пользовательские auth конфигурации containerd.                                                                                             |
| `TransitionContainerdConfigReady` | Состояние подготовки конфигурации containerd в новый режим. Проверяется, что конфигурация containerd успешно подготовлена и содержит одновременно конфигурации нового и старого режима.                                      |
| `FinalContainerdConfigReady`      | Состояние завершения переключения containerd в новый режим. Проверяется, что конфигурация containerd успешно применена и содержит конфигурацию нового режима.                                                                |
| `DeckhouseRegistrySwitchReady`    | Состояние переключения Deckhouse и его компонентов на использование нового registry. Значение `True` указывает, что Deckhouse успешно переключился на сконфигурированный registry и готов к работе.                          |
| `InClusterProxyReady`             | Состояние готовности In-Cluster Proxy. Проверяется, что In-Cluster Proxy успешно запущен и работает.                                                                                                                         |
| `CleanupInClusterProxy`           | Состояние очистки In-Cluster Proxy, если прокси не нужен для работы желаемого режима. Проверяется, что все ресурсы, связанные с In-Cluster Proxy, успешно удалены.                                                           |
| `NodeServicesReady`               | Состояние готовности Node Services Manager и Static-Pod registry. Проверяется, что Node Services Manager успешно запущен и работает, и что Static-Pod registry был успешно развёрнут с помощью Node Services Manager.        |
| `CleanupNodeServices`             | Состояние очистки Node Services Manager и Static-Pod registry, если компоненты не нужны для работы желаемого режима. Проверяется, что все ресурсы, связанные с Node Services Manager и Static-Pod registry, успешно удалены. |
| `RegistryContainsRequiredImages`  | Состояние проверки registry на наличие необходимых образов.                                                                                                                                                                   |
| `Ready`                           | Общее состояние готовности registry к работе в указанном режиме. Проверяется, что все предыдущие условия выполнены и модуль готов к работе.                                                                                  |
