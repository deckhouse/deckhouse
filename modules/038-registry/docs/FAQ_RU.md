---
title: "Модуль registry: FAQ"
description: "Часто задаваемые вопросы о модуле registry Deckhouse Kubernets Platform, включая процедуры миграции, конфигурацию containerd и устранение проблем с registry."
---

## На какой реализации работает мой кластер?

Модуль записывает то, на чём кластер работает фактически, а не то, что запрошено в конфигурации:

```bash
d8 k -n d8-system get secret registry-v2-switch >/dev/null 2>&1 \
  && echo "текущая реализация" || echo "предыдущая реализация"
```

Если предыдущая, модуль сообщает причину на каждой итерации согласования и поднимает
[`D8RegistryMigrationPending`](#что-означают-алерты-модуля-registry):

```bash
d8 k -n d8-system get secret registry-state -o jsonpath='{.data.state}' | base64 -d | head
```

## Как завершить миграцию?

Реализацию выбирать не нужно — передача автоматическая, и она ждёт одного: чтобы предыдущая
реализация отпустила путь загрузки образов. Обе настраивают на каждом узле одно и то же — какой
registry спрашивает container runtime и с какими учётными данными, — поэтому работа обеих не
объединила бы эти ответы, а столкнула бы их.

1. Переведите настройки registry в ModuleConfig `deckhouse` в `Unmanaged`. Если кластер сейчас в
   `Direct`, `Proxy` или `Local`, воспользуйтесь
   [примерами переключения режимов](examples.html#примеры-для-предыдущей-реализации).

1. Дождитесь завершения перехода — `mode: Unmanaged` без ожидающего целевого режима:

   ```bash
   d8 k -n d8-system get secret registry-state -o jsonpath='{.data.state}' | base64 -d | head
   ```

1. Больше ничего не требуется. На следующей итерации согласования модуль примет управление, и
   кластер всё это время продолжает загружать образы из того же registry: при `mode: Unmanaged` —
   значении по умолчанию текущей реализации — он тоже не управляет ничем, поэтому передача не
   меняет поведения.

1. Чтобы модуль начал управлять путём загрузки, задайте `mode: Managed` в ModuleConfig `registry`
   вместе с указанием registry. Готовая конфигурация для вашего кластера публикуется в секрете
   `registry-suggested-config` — см. [включение модуля](examples.html#включение-модуля).

## Что означают алерты модуля registry?

Ни один из них не означает, что кластер перестал загружать образы. Большинство означает, что он
загружает их способом, который работает, но не тем, о котором просили, — то есть состояние,
которое иначе осталось бы незамеченным.

`D8RegistryMigrationPending`
: Кластер всё ещё на предыдущей реализации. Ничего не деградировало, миграция не завершена. См.
  [выше](#как-завершить-миграцию).

`D8RegistryConfigInvalid`
: Конфигурация отвергнута, кластер сохраняет то, что было. Что именно не так — в
  `registryconfig/registry` в `.status.conditions`.

`D8RegistryNodeNotConverged`
: Агент на части узлов не применил выданную ему конфигурацию. Эти узлы продолжают загружать
  образы так, как были настроены раньше, — значит, изменение до них не дошло, и следующее тоже не
  дойдёт.

`D8RegistryNodeRunningFromDisk`
: Часть узлов не может дозвониться до API-сервера и маршрутизирует по копии на диске. Это копия,
  работающая как задумано, и именно поэтому об этом стоит сказать: такие узлы загружают образы
  нормально, поэтому со всех остальных сторон это выглядит как успех, а их конфигурация может
  отставать от кластерной сколь угодно далеко.

`D8RegistryStorageIncomplete`
: Часть реплик кэша не держит весь ожидаемый набор образов. При настроенном upstream'е это ничего
  не стоит при загрузке, но кластер не пережил бы его потерю — и именно это задерживает переход в
  air-gap.

`D8RegistryAirGapTransitionHeld`
: Вы убрали upstream, а модуль всё ещё им пользуется, потому что кэш пока не может стоять сам.
  Это безопасный исход и единственный переход здесь, который иначе мог бы оставить все узлы без
  источника образов. Сам он не разрешится, если кэш перестал наполняться.

`D8RegistryUpstreamProbeFailing`
: Изменение основного upstream'а отвергнуто, и кластер продолжает пользоваться последним
  работавшим. Метка `outcome` различает три разные проблемы: `unreachable` — сеть или registry,
  `auth` — обычно истёкший лицензионный ключ, `sentinel` — registry ответил и принял учётные
  данные, но не содержит образов Deckhouse, обычно из-за неверного пути репозитория.

`D8RegistryUpstreamRejected`
: `RegistryUpstream` не принят, поэтому загрузки для указанного в нём registry нигде не
  перехватываются. Метка `reason` говорит, конфликтует он с основным registry или с другим
  ресурсом, претендующим на то же имя.

`D8RegistryStorageNotReclaimed`
: Ни одна реплика не освобождала диск неделю. Сборка — единственное, что вообще что-то удаляет
  из хранилища, поэтому кластер, где она остановилась, идёт к полному диску. См.
  [ниже](#кэш-растёт-что-его-чистит).

`D8RegistryStaleCacheData`
: На узле лежат данные кэша, которые никто не использует. См.
  [ниже](#на-узле-остались-данные-кэша-которые-никто-не-использует).

## На узле остались данные кэша, которые никто не использует

При выключении кэша блобы в `/opt/deckhouse/registry` намеренно остаются: если кэш включить
обратно, он дольётся из того, что уже есть, а не начнёт с нуля, и на медленном канале это разница
в часы. Автоматическое удаление сделало бы решение необратимым в единственном направлении, где
это больно.

Чего модуль не делает — не оставляет их молча: про занятое ими место больше не сказал бы никто,
потому что хранилище, которое их записало, уже удалено. Поэтому агент их измеряет и сообщает:

```bash
d8 k get registrynodes -o custom-columns=\
NODE:.metadata.name,STALE:.status.staleStorageDataBytes
```

Чтобы освободить место, удалите каталог на узле:

```bash
ssh <node> 'du -sh /opt/deckhouse/registry && sudo rm -rf /opt/deckhouse/registry'
```

## Кэш растёт. Что его чистит?

Сборка мусора по расписанию, которую выполняют сами реплики.

Она нужна потому, что больше ничто и никогда ничего не удаляет. Каждый релиз добавляет срез
репозитория, поэтому кластер, живущий годами, заполняет хранилище и перестаёт иметь возможность
в него писать — а в изолированном кластере это означает, что его нельзя обновить.

Удаляются срезы релизов, которые кластер прошёл. Остаются:

- развёрнутый релиз и предыдущий, чтобы откат не скачивал заново то, к чему откатывается;
- всё новее развёрнутого — это обновление в процессе, а в air-gap ещё и релиз, который кто-то
  залил намеренно;
- любой тег, который вообще не является версией: имена каналов вроде `stable`, плавающие теги,
  всё залитое руками. Это не «это мусор», а «неизвестно, что это», и это разные вещи.

Эта асимметрия выдержана везде. Удалить блоб, который ещё нужен, в air-gap невосстановимо без
повторного `d8 mirror push`; сохранить ненужный — стоит места на диске. Поэтому проход, который
не может установить, что сохранять — например, если не развёрнут ни один релиз, — не делает
ничего вовсе, а не делает что может.

Посмотреть состояние:

```bash
d8 k get registrystorage registry -o jsonpath='{.status.replicas}' | jq \
  'map({node, collectedAt, collectionError})'
d8 k get registrystorage registry -o jsonpath='{.spec.garbageCollection}' | jq
```

### Когда это происходит и почему реплика уходит в read-only

Сборка освобождает блобы обходом хранилища, а собственный коллектор registry сначала вычисляет
множество достижимых блобов, а потом удаляет остальные — то есть блоб, загруженный между этими
шагами, будет удалён. Единственный безопасный способ это запустить — против хранилища, в которое
никто не пишет, поэтому реплика на время отказывается принимать запись.

При этом она продолжает отдавать все свои образы. Чего она не может — сохранить результат
промаха кэша (агент на узле уходит на upstream, то есть загрузка становится медленнее, а не
неудачной) и принять `d8 mirror push` (он падает видимо, и его можно повторить). Собирает
одновременно только одна реплика, остальные всё это время работают как обычно.

Поэтому расписание по умолчанию — ночной час, а если у NodeGroup `master` задано окно
обслуживания, то начало этого окна: час, про который уже сказано, что нарушение в нём допустимо.
Чтобы задать своё:

```yaml
spec:
  settings:
    storage:
      garbageCollection:
        schedule: "0 2 * * Sun"
```

Нечитаемое выражение отвергается, а не угадывается: собрать в другой час, чем вы написали, хуже,
чем не собрать вовсе.

### Как выключить

```yaml
spec:
  settings:
    storage:
      garbageCollection:
        enabled: false
```

Это имеет смысл только с диском, достаточно большим, чтобы неограниченный рост хранилища никогда
не имел значения. [`D8RegistryStorageNotReclaimed`](#что-означают-алерты-модуля-registry) всё
равно сработает через неделю после последней сборки, потому что «выключено» и «молча
перестало» снаружи выглядят одинаково.

## На узле не загружается образ. Куда смотреть?

Агент стоит на пути каждой загрузки на узле, поэтому начинать нужно с него. Он работает
статическим подом, поэтому присутствует даже когда кластер — нет:

```bash
d8 k -n kube-system logs -l component=registry-agent --tail=100
```

Что агент считает нужным делать и согласен ли он с кластером:

```bash
d8 k get registrynode <node> -o jsonpath='{.status}' | jq
```

Его собственный взгляд на проходящие через него загрузки, читается с узла. Prometheus его не
скрейпит, и это намеренно: агент работает статическим подом потому, что должен работать при
недоступном API-сервере, а kube-rbac-proxy рядом с ним аутентифицировался бы в том самом
API-сервере.

```bash
ssh <node> 'curl -s http://127.0.0.1:4286/metrics | grep d8_registry_agent'
```

Что сказано container runtime — это один файл, и он не зависит от того, сколько registry
настроено:

```bash
ssh <node> 'cat /etc/containerd/registry.d/_default/hosts.toml'
```

Если этого файла нет, агент ещё не применил конфигурацию: на узле не загрузится ничего, а причина
— в его логе. Если файл есть, а загрузки всё равно не идут, отказ находится за агентом — метрики
выше называют, какая цель отказала и почему.

## Как посмотреть состояние внутрикластерного кэша?

```bash
d8 k get registrystorage registry -o jsonpath='{.status}' | jq
```

`replicas` — единственное место, где сообщается полнота, и каждая запись это отчёт реплики о
себе. Реплика с `full: true` рядом с `error` полной не является: `full` говорит, что она держит,
а ошибка — закончился ли её последний проход.

`Leader` — реплика, которая наливается из upstream'а и служит источником репликации для
остальных. Это не обычные выборы: за lease борется только реплика, держащая весь набор, а
держащая его уступает, когда набор появляется у другой. Именно это условие не даёт изолированному
кластеру застрять с пустым лидером и полным follower'ом.

## Предыдущая реализация

Всё, что ниже, относится к кластеру, который всё ещё работает на реализации, настраиваемой
через ModuleConfig `deckhouse`.

### Как мигрировать на модуль registry?

Во время миграции, для containerd v1 будет выполнен переход на новую схему конфигурации registry.
containerd v2 использует новую схему по умолчанию. Подробнее можно ознакомиться в разделе [с описанием способов конфигурации](/modules/node-manager/latest/faq.html#как-добавить-конфигурацию-для-дополнительного-registry)

#### Для containerd v2

1. Выполните переключение на использование модуля `registry`. Для этого, укажите в `moduleConfig` `deckhouse` параметры `Unmanaged` режима. Если используется registry, отличный от `registry.deckhouse.ru`, ознакомьтесь с конфигурацией модуля [deckhouse](/modules/deckhouse/latest/configuration.html) для корректной настройки.

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

1. Дождитесь завершения переключения. Пример [статуса переключения](./faq.html#как-посмотреть-статус-переключения-режима-registry):

   ```yaml
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

1. Если конфигурации присутствуют, необходимо выполнить миграцию на новый формат конфигурации registry в containerd. Для этого, необходимо добавить новые конфигурации в директорию `/etc/containerd/registry.d`. Данные конфигурации вступят в силу после переключения на модуль `registry`. Для добавления конфигураций подготовьте `NodeGroupConfiguration`, подробнее в разделе [с описанием способов конфигурации](/modules/node-manager/latest/faq.html#как-добавить-конфигурацию-для-дополнительного-registry). Пример:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: containerd-additional-config-auth.sh
   spec:
     # Шаг может быть любой, т.к. не требуется перезапуск сервиса containerd
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

1. Выполните переключение на использование модуля `registry`. Для этого, укажите в `moduleConfig` `deckhouse` параметры `Unmanaged` режима. Если используется registry, отличный от `registry.deckhouse.ru`, ознакомьтесь с конфигурацией модуля [deckhouse](/modules/deckhouse/latest/configuration.html) для корректной настройки.

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

1. После применения, дождитесь в [статусе переключения](faq.html#как-посмотреть-статус-переключения-режима-registry) сообщение:

   Пример вывода:

   ```yaml
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
  
1. После удаления старых конфигураций, убедитесь, что переключение продолжило выполняться. Пример [статуса переключения](faq.html#как-посмотреть-статус-переключения-режима-registry):

   ```yaml
   conditions:
   # ...
   - lastTransitionTime: "2025-08-13T16:42:09Z"
     message: ""
     reason: ""
     status: "True"
     type: ContainerdConfigPreflightReady
   ```

1. Дождитесь завершения переключения. Пример [статуса переключения](faq.html#как-посмотреть-статус-переключения-режима-registry):

   ```yaml
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

1. Удалите  [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration), созданный на шаге удаления старых конфигурационных файлов:

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

1. Проверьте статус переключения, используя [инструкцию](./faq.html#как-посмотреть-статус-переключения-режима-registry). Пример вывода:

   ```yaml
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

1. Проверьте статус переключения, используя [инструкцию](./faq.html#как-посмотреть-статус-переключения-режима-registry). Пример вывода:

   ```yaml
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

1. После удаления дождитесь завершения переключения. Для отслеживания используйте [инструкцию](faq.html#как-посмотреть-статус-переключения-режима-registry). Пример вывода:

   ```yaml
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

1. Если используется containerd v1, примените заготовленные этапом ранее `NodeGroupConfiguration` с пользовательскими конфигурациями registry.

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

```yaml
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
