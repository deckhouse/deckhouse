---
title: Обновление Deckhouse в закрытом окружении
permalink: ru/guides/airgapped-update.html
description: Инструкция по обновлению Deckhouse Kubernetes Platform в закрытом окружении.
lang: ru
layout: sidebar-guides
---

{% alert level="warning" %}
В руководстве рассматривается EE-редакция DKP, но механизмы аналогичны для других редакций.
{% endalert %}

{% alert level="info" %}
Руководство тестировалось на версии d8 v0.17.1.
{% endalert %}

{% alert level="info" %}
В руководстве используется сторонняя утилита [crane](https://github.com/google/go-containerregistry?tab=readme-ov-file#crane) для анализа Container Registry.
{% endalert %}

## Механика обновления платформы с помощью релизных каналов

У Deckhouse обновления основаны на так называемых ["релизных каналах"](https://deckhouse.ru/products/kubernetes-platform/documentation/v1/deckhouse-release-channels.html)

Текущий канал обновлений вашей установки можно увидеть в ModuleConfig `deckhouse`:

```bash
d8 k get mc deckhouse -o jsonpath='{.spec.settings.releaseChannel}'
```

Технически обновление Deckhouse выглядит следующим образом:

В registry лежит image всегда с одинаковым именем `release-channel` и тегом по имени канала, который указывает на image уже конкретной версии deckhouse (при выпуске новой версии платформы данный image заменяется на новый):

Рассмотрим образ:

```bash
crane export registry.deckhouse.ru/deckhouse/ee/release-channel:alpha | tar -tf -
```

Пример:

```bash
~$ crane export registry.deckhouse.ru/deckhouse/ee/release-channel:alpha | tar -tf -
changelog.yaml
version.json
.werf
.werf/stapel
.werf/tmp
.werf/tmp/ssh-auth-sock
```

Основные файлы в образе `changelog.yaml`, который содержит собственно описание изменений и `version.json`. Просмотрим содержимое `version.json`:

Пример `crane export registry.deckhouse.ru/deckhouse/ee/release-channel:alpha | tar -xOf - version.json | jq`:

```json
~$ crane export registry.deckhouse.ru/deckhouse/ee/release-channel:alpha | tar -xOf - version.json | jq
{
  "canary": {
    "alpha": {
      "enabled": true,
      "waves": 2,
      "interval": "5m"
    },
    "beta": {
      "enabled": false,
      "waves": 1,
      "interval": "1m"
    },
    "early-access": {
      "enabled": true,
      "waves": 6,
      "interval": "30m"
    },
    "stable": {
      "enabled": true,
      "waves": 6,
      "interval": "30m"
    },
    "rock-solid": {
      "enabled": false,
      "waves": 5,
      "interval": "5m"
    }
  },
  "requirements": {
    "k8s": "1.29",
    "disabledModules": "delivery,l2-load-balancer,ceph-csi",
    "migratedModules": "",
    "autoK8sVersion": "1.31",
    "ingressNginx": "1.9",
    "nodesMinimalOSVersionUbuntu": "18.04",
    "nodesMinimalOSVersionDebian": "10",
    "istioMinimalVersion": "1.19",
    "metallbHasStandardConfiguration": "true",
    "unmetCloudConditions": "true",
    "nodesMinimalLinuxKernelVersion": "5.8.0"
  },
  "disruptions": {
    "1.36": [
      "ingressNginx"
    ]
  },
  "version": "v1.71.5"
}
```

Основной файл `version.json` содержит в себе данные о канареечном развёртывании релиза `canary`, требования `requirements` и нарушения `disruptions` ([устаревшее поле](https://deckhouse.ru/products/kubernetes-platform/documentation/v1/cr.html#deckhouserelease-v1alpha1-spec-disruptions)) релиза и собственно саму версию релиза в поле `version`.

И при изменении версии DKP применяет данный релиз (создаётся `deckhouserelease` и начинается процесс обновления).

Следует также отметить, что при разрыве минорной версией между версией в кластере и версией в образе release-channel deckhouse автоматически попробует восстановить промежуточные `deckhouserelease` для выполнения последовательного обновления.

Следует обратить внимание, что Deckhouse нельзя обновлять не последовательно, перепрыгивая через минорные релизы (это не относится к патч релизам), по следующим причинам: в минорных релизах зачастую присутствуют миграции, которые должны применяться последовательно, эти миграции время от времени удаляются, что в лучшем случае при перепрыгивании приведёт к оставленному "мусору", в худшем - некорректной работе кластера из-за невыполненных миграций.

## Механика обновления модулей платформы

Модули платформы имеют схожую механику обновления, но их релизный цикл отвязан от релизов платформы и он полностью самостоятелен.

В кластере есть ресурс ModuleSource который листится deckhouse и на основе этого ресурса дискаверится список модулей:

```bash
d8 k get ms deckhouse -o jsonpath='{.spec.registry.repo}'
```

Просмотрим этот registry:

```bash
crane ls registry.deckhouse.io/deckhouse/ee/modules
```

На примере модуля `console`:

В registry лежит image всегда с одинаковым именем `release` и тегом по имени канала, который указывает на image уже конкретной версии модуля console (при выпуске новой версии модуля данный image заменяется на новый):

Рассмотрим образ:

```bash
crane export registry.deckhouse.ru/deckhouse/ce/modules/console/release:alpha | tar -tf -
```

Пример:

```bash
~$ crane export registry.deckhouse.ru/deckhouse/ce/modules/console/release:alpha | tar -tf -
changelog.yaml
version.json
```

Образ модуля, аналогично образу самой платформы DKP, содержит файлы `changelog.yaml`, который содержит собственно описание изменений и `version.json`. Просмотрим содержимое `version.json`:

Пример `crane export registry.deckhouse.ru/deckhouse/ce/modules/console/release:alpha | tar -xOf - version.json | jq`:

```bash
~$ crane export registry.deckhouse.ru/deckhouse/ce/modules/console/release:alpha | tar -xOf - version.json | jq
{
  "version": "v1.39.4"
}
```

Основной файл `version.json` содержит в себе собственно саму версию релиза модуля в поле `version`.

И при изменении версии Deckhouse применяет данный релиз (создаётся `modulerelease` и начинается процесс обновления).

По аналогии c механизмами обновления самой платформы следует обратить внимание, что модули нельзя обновлять не последовательно, перепрыгивая через минорные релизы (это не относится к патч релизам), по следующим причинам: в минорных релизах зачастую присутствуют миграции, которые должны применяться последовательно, эти миграции время от времени удаляются, что в лучшем случае при перепрыгивании приведёт к оставленному "мусору", в худшем - некорректной работе модуля из-за невыполненных миграций.

Также Deckhouse сообщит при отсутствии необходимых минорных версий ошибкой вида `minor version is greater than deployed $version by one`

Следует также отметить, что при разрыве минорной версией между версией в кластере и версией в образе release deckhouse автоматически попробует восстановить промежуточные `modulerelease` для выполнения последовательного обновления.

## Механика обновления баз данных сканера уязвимостей

{% alert level="warning" %}
Доступно в редакциях: EE, CSE Lite (1.67), CSE Pro (1.67)
{% endalert %}

Базы уязвимостей обновляются раз в 6 часов и сам trivy в кластере их скачивает из registry каждые 6 часов.

Образы баз уязвимостей на примере [EE-редакции](https://deckhouse.ru/products/kubernetes-platform/documentation/v1/modules/operator-trivy/) платформы имеют постоянные имена и теги и находятся по пути:

```bash
registry.deckhouse.ru/deckhouse/ee/security/trivy-db:2
registry.deckhouse.ru/deckhouse/ee/security/trivy-java-db:1
registry.deckhouse.ru/deckhouse/ee/security/trivy-checks:0
```

Для настройки периодического обновления образов баз данных уязвимостей используйте конструкцию вида:

```bash
d8 mirror pull --source='registry.deckhouse.ru/deckhouse/ee' --license='YOUR_LICENSE_TOKEN' --no-platform --no-modules $(pwd)/d8-bundle-security-db && d8 mirror push $(pwd)/d8-bundle-security-db YOUR_PRIVATE_REGISTRY_HOSTNAME:5050/dkp/ee --registry-login='YOUR_REGISTRY_LOGIN' --registry-password='YOUR_REGISTRY_PASSWORD' --tls-skip-verify
```

## Пример сценария обновления платформы, используемых модулей и баз данных уязвимостей до актуальных версий

Если просто запустить конструкцию `d8 mirror pull --source='registry.deckhouse.ru/deckhouse/ee' --license='YOUR_LICENSE_TOKEN' $(pwd)/d8-bundle` для скачивания всех образов, находящихся на релизных каналах и всех модулей (которых уже более 30 для EE-редакции), то Вы получите довольно объёмный `d8-bundle` (на пример на дату 2025-09-17 объём такой директории `d8-bundle` составил более 50G).

Исходя из выше описанного проведите обновление платформы по следующей инструкции:

1. Получите версию DKP в Вашем кластере с помощью конструкции:

    ```bash
    d8 k -n d8-system get deployment deckhouse -o json | jq -r '.metadata.annotations | {"core.deckhouse.io/edition","core.deckhouse.io/version"}'
    ```

    Пример:

    ```bash
    ~ $ d8 k -n d8-system get deployment deckhouse -o json | jq -r '.metadata.annotations | {"core.deckhouse.io/edition","core.deckhouse.io/version"}'
    {
      "core.deckhouse.io/edition": "EE",
      "core.deckhouse.io/version": "v1.68.13"
    }
    ```

1. Получите список установленных модулей в кластере и скачайте обновления для этих модулей:

    Получим список установленных модулей и их версий

    ```bash
    d8 k get mr | grep Deployed
    ```

    Пример:

    ```bash
    ~$ d8 k get mr | grep Deployed
    commander-agent-v1.2.4             Deployed                     13d
    console-v1.35.1                    Deployed                     7d4h
    ```

    И переведите данный вывод в опции для d8 mirror pull вида: `--include-module='commander-agent@v1.2.4' --include-module='console@v1.35.1'`

    Или используйте однострочник вида:

    ```bash
    d8 k get mr -o json | jq -r '.items[] | select(.status.phase == "Deployed") | "--include-module='\''\(.spec.moduleName)@\(.spec.version)'\''"' | paste -sd " " -
    ```

1. Составьте финальную команду для скачивания образов

    Используя полученные данные конструкция будет иметь вид:

    ```bash
    d8 mirror pull --source='registry.deckhouse.ru/deckhouse/ee' --license='YOUR_LICENSE_TOKEN' --since-version='v1.68.13' --include-module='commander-agent@1.2.4' --include-module='console@1.35.1' $(pwd)/d8-bundle
    ```

    {% alert level="info" %}
    Если вы настроили периодическое скачивание и закачивание в Ваш registry баз данных уязвимостей, то Вы можете добавить флаг --no-security-db для исключения их из процесса перекачивания образов.
    {% endalert %}

    Будут скачаны последние патч релизы всех минорных версий платформы и указанных модулей начиная с последних патч версий указанных минорных версий релиза до актуальных версий, находящихся на релизных каналах (с версиями можно ознакомиться по ссылке [https://releases.deckhouse.ru/ee](https://releases.deckhouse.ru/ee))

1. Загрузите полученные артефакты в Ваш registry командой вида:

    ```bash
    d8 mirror push $(pwd)/d8-bundle YOUR_PRIVATE_REGISTRY_HOSTNAME:5050/dkp/ee --registry-login='YOUR_REGISTRY_LOGIN' --registry-password='YOUR_REGISTRY_PASSWORD' --tls-skip-verify
    ```

1. Проверьте состояние обновления в кластере с помощью команд

    ```bash
    d8 k get deckhousereleases.deckhouse.io
    d8 k get modulereleases.deckhouse.io
    d8 system queue list
    ```

## Возможные проблемы

### Release is suspended

При попытке скачать образы платформы получаем ошибку вида:

```bash
~$ d8 mirror pull d8-bundle/ --license='YOUR_LICENSE_KEY'
Sep  9 00:10:57.145 INFO  ╔ Pull Deckhouse Kubernetes Platform
Sep  9 00:11:01.532 ERROR Pull Deckhouse Kubernetes Platform failed error="Find tags to mirror: Find versions to mirror: get stable release version from registry: Cannot mirror Deckhouse: source registry contains suspended release channel \"stable\", try again later"
Error: pull failed, see the log for details
```

Пример такого флага в образе при остановке релиза на Stable канале:

```bash
~$ crane export registry.deckhouse.ru/deckhouse/ee/release-channel:stable | tar -xOf - version.json | jq '[.version,.suspend]'
[
  "v1.71.3",
  true
]
```

Если релиз не остановлен, то флаг вовсе отсутствует:

```bash
~$ crane export registry.deckhouse.ru/deckhouse/ee/release-channel:rock-solid | tar -xOf - version.json | jq '[.version,.suspend]'
[
  "v1.70.17",
  null
]
```

Это значит, что на одном из релизных каналов выкат релиза остановлен. То есть в образ релизного канала поступает версия на которую обновляться. Если случилась ситуация, что релиз остановлен - образ релизного канала патчится и в него добавляется флажок `suspend`.

Тем не менее Вы можете выкачать версию платформы в таком случае с указанием флага `--deckhouse-tag` для `d8 mirror pull`, пример:

```bash
~$ d8 mirror pull d8-bundle/ --license='YOUR_LICENSE_KEY' --deckhouse-tag='v1.71.3'
Sep 16 12:56:25.074 INFO  ╔ Pull Deckhouse Kubernetes Platform
Sep 16 12:56:25.713 INFO  ║ Skipped releases lookup as tag "v1.71.3" is specifically requested with --deckhouse-tag
Sep 16 12:56:25.714 INFO  ║ Creating OCI Image Layouts
Sep 16 12:56:25.720 INFO  ║ Resolving tags
Sep 16 12:56:26.715 INFO  ║╔ Pull release channels and installers
Sep 16 12:56:26.716 INFO  ║║ Beginning to pull Deckhouse release channels information
Sep 16 12:56:26.717 INFO  ║║ [1 / 1] Pulling registry.deckhouse.ru/deckhouse/ee/release-channel:v1.71.3
Sep 16 12:56:27.087 INFO  ║║ Deckhouse release channels are pulled!
Sep 16 12:56:27.087 INFO  ║║ Beginning to pull installers
Sep 16 12:56:27.087 INFO  ║║ [1 / 1] Pulling registry.deckhouse.ru/deckhouse/ee/install:v1.71.3
...
```

## Особенности при работе с сертифицированной редакцией платформы

Обновление по механизму "релизных каналов" на текущий момент для CSE редакции не реализовано

Адрес registry: `registry-cse.deckhouse.ru/deckhouse/cse`

Процесс обновления описан на странице [Обновления DKP Certified Security Edition](https://deckhouse.ru/products/kubernetes-platform/certified-security-edition/updates/)
