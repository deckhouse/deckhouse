---
title: Обновление Deckhouse Kubernetes Platform в закрытом окружении
permalink: ru/guides/airgapped-update.html
description: Инструкция по обновлению Deckhouse Kubernetes Platform в закрытом окружении.
lang: ru
layout: sidebar-guides
---

{% alert level="warning" %}
В руководстве рассматривается EE-редакция DKP, но механизмы аналогичны [для других редакций](../documentation/v1/revision-comparison.html).
{% endalert %}

{% alert level="info" %}
Руководство тестировалось на версии [d8 v0.17.1](../documentation/v1/deckhouse-cli/).

В руководстве используется сторонняя утилита [crane](https://github.com/google/go-containerregistry?tab=readme-ov-file#crane) для анализа container registry. Перед началом работ установите её в соответствии [с официальной инструкцией](https://github.com/google/go-containerregistry/blob/main/cmd/crane/README.md#installation).
{% endalert %}

## Механика обновления платформы с помощью релизных каналов

Обновления Deckhouse Kubernetes Platform основаны [на каналах обновлений](https://deckhouse.ru/products/kubernetes-platform/documentation/v1/deckhouse-release-channels.html). Канал обновлений, заданный для установленной у вас копии DKP, можно посмотреть [в ModuleConfig `deckhouse`](../documentation/v1/modules/deckhouse/configuration.html), выполнив команду:

```bash
d8 k get mc deckhouse -o jsonpath='{.spec.settings.releaseChannel}'
```

{% offtopic title="Пример вывода команды..." %}
```console
$ d8 k get mc deckhouse -o jsonpath='{.spec.settings.releaseChannel}'
Stable
```
{% endofftopic %}

Технически обновление DKP выглядит следующим образом: в registry лежит образ с неизменным именем `release-channel` и тегом по названию канала обновлений, который указывает на образ уже конкретной версии DKP (при выпуске новой версии этот образ заменяется на новый).

Рассмотрим образ:

```bash
$ crane export registry.deckhouse.ru/deckhouse/ee/release-channel:alpha | tar -tf -
changelog.yaml
version.json
.werf
.werf/stapel
.werf/tmp
.werf/tmp/ssh-auth-sock
```

В образе лежат два основных файла:

* `changelog.yaml` – содержит описание изменений;
* `version.json` – содержит данные о канареечном развёртывании релиза (`canary`), требования (`requirements`) и нарушения (`disruptions`) ([устаревшее поле](https://deckhouse.ru/products/kubernetes-platform/documentation/v1/cr.html#deckhouserelease-v1alpha1-spec-disruptions)) релиза, а также саму версию релиза в поле `version`.

  ```bash
  $ crane export registry.deckhouse.ru/deckhouse/ee/release-channel:alpha | tar -xOf - version.json | jq
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

При изменении значения в поле версии DKP применяет новый релиз (создаётся `deckhouserelease` и начинается процесс обновления).

При разрыве минорных версий между версией в кластере и версией в образе `release-channel` DKP автоматически попробует восстановить промежуточные `deckhouserelease` для выполнения последовательного обновления.

{% alert level="warning" %}
Обратите внимание, что DKP нельзя обновлять непоследовательно, перепрыгивая через минорные релизы (это не относится к патч-релизам), по следующим причинам: в минорных релизах зачастую присутствуют миграции, которые должны применяться последовательно, эти миграции время от времени удаляются, что в лучшем случае при перепрыгивании приведёт к оставленному «мусору», в худшем - некорректной работе кластера из-за невыполненных миграций.
{% endalert %}

## Механика обновления модулей платформы

Модули платформы имеют схожую механику обновления, но их релизный цикл отвязан от релизов платформы и полностью самостоятелен.

В кластере есть ресурсы [ModuleSource](../documentation/v1/cr.html#modulesource), которые наблюдаются DKP, и на основе которых обнаруживается список доступных модулей. Посмотрим, из какого репозитория они будут устанавливаться:

```bash
$ d8 k get ms deckhouse -o jsonpath='{.spec.registry.repo}'
registry.deckhouse.ru/deckhouse/ee/modules
```

Заглянем в полученный registry:

```bash
crane ls registry.deckhouse.ru/deckhouse/ee/modules
```

{% offtopic title="Пример вывода команды..." %}
```console
$ crane ls registry.deckhouse.ru/deckhouse/ee/modules
commander-agent
console
csi-ceph
csi-nfs
observability
operator-postgres
pod-reloader
prompp
sds-local-volume
sds-node-configurator
sds-replicated-volume
secrets-store-integration
snapshot-controller
stronghold
virtualization
```
{% endofftopic %}

Теперь взглянем на модуль `console`. В registry лежит образ с неизменным именем `release` и тегом по имени канала, указывающим на образ уже конкретной версии модуля `console` (при выпуске новой версии модуля этот образ заменяется на новый). Посмотрим внутрь этого образа:

```bash
$ crane export registry.deckhouse.ru/deckhouse/ee/modules/console/release:alpha | tar -tf -
changelog.yaml
version.json
```

Образ модуля, аналогично образу самой платформы DKP, содержит файлы `changelog.yaml` и `version.json`. Посмотрим содержимое последнего:

```bash
$ crane export registry.deckhouse.ru/deckhouse/ee/modules/console/release:alpha | tar -xOf - version.json | jq
{
  "version": "v1.39.4"
}
```

В поле `version` содержится версия модуля. При её изменении DKP применяет новый релиз (создаётся `modulerelease` и начинается процесс обновления).

{% alert level="warning" %}
Обратите внимание, что модули нельзя обновлять непоследовательно, перепрыгивая через минорные релизы (это не относится к патч-релизам), по следующим причинам: в минорных релизах зачастую присутствуют миграции, которые должны применяться последовательно, эти миграции время от времени удаляются, что в лучшем случае при перепрыгивании приведёт к оставленному «мусору», а в худшем - некорректной работе модуля из-за невыполненных миграций.
{% endalert %}

При отсутствии необходимых минорных версий DKP выведет ошибку вида `minor version is greater than deployed $version by one`.

При разрыве минорных версий между версией в кластере и версией в образе `release` DKP автоматически попробует восстановить промежуточные `modulerelease` для выполнения последовательного обновления.

## Механика обновления баз данных сканера уязвимостей

{% alert level="warning" %}
Доступно в редакциях: EE, CSE Lite (1.67), CSE Pro (1.67)
{% endalert %}

Базы уязвимостей обновляются раз в 6 часов – trivy в кластере самостоятельно скачивает их из registry один раз за этот промежуток.

Образы баз уязвимостей на примере [EE-редакции](../documentation/v1/modules/operator-trivy/) платформы имеют постоянные имена и теги и находятся по путям:

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

Если запустить конструкцию `d8 mirror pull --source='registry.deckhouse.ru/deckhouse/ee' --license='YOUR_LICENSE_TOKEN' $(pwd)/d8-bundle` для скачивания всех образов, находящихся на релизных каналах, и всех модулей (которых уже более 30 для EE-редакции), то в результате получится объёмный `d8-bundle` (например, на момент написания этого гайда объём такой директории `d8-bundle` составил более 50 гигабайт).

Чтобы этого не происходило, выкачивать следует только соответствующие вашей версии образы по следующей инструкции:

1. Получите версию DKP в Вашем кластере с помощью конструкции:

   ```bash
   d8 k -n d8-system get deployment deckhouse -o json | jq -r '.metadata.annotations | {"core.deckhouse.io/edition","core.deckhouse.io/version"}'
   ```
   Пример вывода команды:

   ```bash
   $ d8 k -n d8-system get deployment deckhouse -o json | jq -r '.metadata.annotations | {"core.deckhouse.io/edition","core.deckhouse.io/version"}'
   {
     "core.deckhouse.io/edition": "EE",
     "core.deckhouse.io/version": "v1.68.13"
   }
   ```

1. Получите список установленных модулей в кластере:

   ```bash
   d8 k get mr | grep Deployed
   ```

   Пример вывода команды:

   ```bash
   ~$ d8 k get mr | grep Deployed
   commander-agent-v1.2.4             Deployed                     13d
   console-v1.35.1                    Deployed                     7d4h
   ```

   Добавите полученный список к команду `d8 mirror pull` в виде ключей: `--include-module='commander-agent@v1.2.4' --include-module='console@v1.35.1'`.

   Или используйте однострочник вида:

   ```bash
   d8 k get mr -o json | jq -r '.items[] | select(.status.phase == "Deployed") | "--include-module='\''\(.spec.moduleName)@\(.spec.version)'\''"' | paste -sd " " -
   ```

1. Составьте финальную команду для скачивания образов, используя полученные ранее параметры:

   ```bash
   d8 mirror pull --source='registry.deckhouse.ru/deckhouse/ee' --license='YOUR_LICENSE_TOKEN' --since-version='v1.68.13' --include-module='commander-agent@1.2.4' --include-module='console@1.35.1' $(pwd)/d8-bundle
   ```

   > Если вы настроили периодическое скачивание и закачивание в ваш registry баз данных уязвимостей, то можно добавить флаг `--no-security-db` для исключения их из процесса перекачивания образов.

   В результате выполнения команды будут скачаны последние патч-релизы всех минорных версий платформы и указанных модулей, начиная с последних патч-версий минорных версий релиза до актуальных версий, находящихся на релизных каналах (с версиями можно ознакомиться [по ссылке](https://releases.deckhouse.ru/ee)).

1. Загрузите полученные артефакты в ваш registry следующей командой:

   ```bash
   d8 mirror push $(pwd)/d8-bundle YOUR_PRIVATE_REGISTRY_HOSTNAME:5050/dkp/ee --registry-login='YOUR_REGISTRY_LOGIN' --registry-password='YOUR_REGISTRY_PASSWORD' --tls-skip-verify
   ```

1. Проверьте состояние обновления в кластере с помощью команд:

   ```bash
   d8 k get deckhousereleases.deckhouse.io
   d8 k get modulereleases.deckhouse.io
   d8 system queue list
   ```

## Возможные проблемы

### Release is suspended

При попытке скачать образы платформы возможно возникновение следующей ошибки:

```bash
~$ d8 mirror pull d8-bundle/ --license='YOUR_LICENSE_KEY'
Sep  9 00:10:57.145 INFO  ╔ Pull Deckhouse Kubernetes Platform
Sep  9 00:11:01.532 ERROR Pull Deckhouse Kubernetes Platform failed error="Find tags to mirror: Find versions to mirror: get stable release version from registry: Cannot mirror Deckhouse: source registry contains suspended release channel \"stable\", try again later"
Error: pull failed, see the log for details
```

Это значит, что на одном из каналов обновлений выкат релиза остановлен. Такая ситуация возникает, если в образ канала обновлений поступает версия, на которую нужно обновляться, но случилась ситуация, что дальнейший выкат релиза на канал остановлен — образ канала обновлений патчится, и в него добавляется флаг `suspend`.

Тем не менее выкачать версию платформы в таком случае все равно возможно с указанием флага `--deckhouse-tag` для `d8 mirror pull`. Например:

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

Обновление по механизму «каналов обновлений» на текущий момент для CSE редакции не реализовано.

Адрес registry: `registry-cse.deckhouse.ru/deckhouse/cse`.

Процесс обновления описан на странице [Обновления DKP Certified Security Edition](https://deckhouse.ru/products/kubernetes-platform/certified-security-edition/updates/).
