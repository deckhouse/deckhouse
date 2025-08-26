---
title: Обновление Deckhouse в закрытом окружении
permalink: ru/guides/airgapped-update.html
description: Инструкция по обновлению Deckhouse Kubernetes Platform в закрытом окружении.
lang: ru
layout: sidebar-guides
---

{% alert level="warning" %}
В инструкции рассматривается преимущественно EE-редакция.
{% endalert %}

## Механика обновление самой платформы

У Deckhouse обновления основаны на так называемых ["релизных каналах"](../../deckhouse-release-channels.html)

Текущий канал обновлений вашей установки можно увидеть в ModuleConfig `deckhouse`:

```bash
d8 k get mc deckhouse -o jsonpath='{.spec.settings.releaseChannel}'
```

Технически обновление Deckhouse выглядит следующим образом:

В registry лежит image всегда с одинаковым именем release-channel и тегом по имени канала, который указывает на image уже конкретной версии deckhouse (при выпуске новой версии платформы данный image заменяется на новый):

На примере EE редакции и релизного канала alpha выполним просмотр содержимого образа:

```bash
~$ crane export registry.deckhouse.ru/deckhouse/ee/release-channel:alpha | tar -xOf - version.json | jq -r '.version'
v1.71.5
```

Либо чуть подробнее:

Создадим временную папку и распакуем в эту папку содержимое образа:

```bash
mkdir /tmp/release-channel-image
crane export registry.deckhouse.ru/deckhouse/ce/release-channel:alpha | tar xf - -C /tmp/release-channel-image
```

Основной файл `/tmp/release-channel-image/version.json` содержит в себе данные о канареечном развёртывании релиза, `requirements` и `disruptions` релиза и собственно саму версию релиза в поле version.

И при изменении версии DKP применяет данный релиз (создаётся deckhouserelease и начинается процесс обновления).

Следует также отметить, что при разрыве минорной версией между версией в кластере и версией в образе release-channel deckhouse автоматически попробует восстановить промежуточные deckhouserelease для выполнения последовательного обновления.

Следует обратить внимание, что Deckhouse нельзя обновлять не последовательно, перепрыгивая через минорные релизы (это не относится к патч релизам), по следующим причинам: в минорных релизах зачастую присутствуют миграции, которые должны применяться последовательно, эти миграции время от времени удаляются, что в лучшем случае при перепрыгивании приведёт к оставленному "мусору", в худшем - некорректной работе кластера из-за невыполненных миграций.

## Механика обновления модулей платформы

Модули платформы имеют схожую механику обновления, но их релизный цикл отвязан от релизов платформы.

В кластере есть ресурс ModuleSource который листится deckhouse и на основе этого ресурса дискаверится список модулей:

```bash
crane ls registry.deckhouse.io/deckhouse/ce/modules
```

На примере модуля console:

В registry лежит image всегда с именем `release` и тегом по имени релизного канала, который указывает на image уже конкретной версии модуля console (при выпуске новой версии модуля данный image заменяется на новый):

```bash
~$ crane export registry.deckhouse.ru/deckhouse/ce/modules/console/release:alpha | grep --text '"version"'
  "version": "v1.37.3"
```

Либо чуть подробнее:

Создадим временную папку и распакуем в эту папку содержимое образа:

```bash
mkdir /tmp/release-module-console
crane export registry.deckhouse.ru/deckhouse/ce/modules/console/release:alpha | tar xf - -C /tmp/release-module-console
```

Основной файл `/tmp/release-module-console/version.json` содержит в себе собственно саму версию релиза модуля в поле version.

И при изменении версии DKP применяет данный релиз (создаётся modulerelease и начинается процесс обновления).

По аналогии cледует обратить внимание, что модули нельзя обновлять не последовательно, перепрыгивая через минорные релизы (это не относится к патч релизам), по следующим причинам: в минорных релизах зачастую присутствуют миграции, которые должны применяться последовательно, эти миграции время от времени удаляются, что в лучшем случае при перепрыгивании приведёт к оставленному "мусору", в худшем - некорректной работе кластера из-за невыполненных миграций.

Также Deckhouse сообщит при отсутствии необходимых минорных версий ошибкой вида minor version is greater than deployed $version by one

Следует также отметить, что при разрыве минорной версией между версией в кластере и версией в образе release deckhouse автоматически попробует восстановить промежуточные modulerelease для выполнения последовательного обновления.

## Механика обновления баз данных сканера уязвимостей

Базы уязвимостей обновляются раз в какое-то время и сам trivy периодически их скачивает из registry по определённому таймауту (4-6 часов?).

Образы баз уязвимостей на примере [EE-редакции](https://deckhouse.ru/products/kubernetes-platform/documentation/v1/modules/operator-trivy/) платформы находятся по пути:

```bash
registry.deckhouse.ru/deckhouse/ee/security/trivy-db:2
registry.deckhouse.ru/deckhouse/ee/security/trivy-java-db:1
registry.deckhouse.ru/deckhouse/ee/security/trivy-checks:0
```

## Обновление платформы, "не внутренних" модулей и Баз данных уязвимостей

Исходя из выше описанного проведите обновление платформы по следующей инструкции:

1. Получите версию DKP в Вашем кластере с помощью конструкции:

```bash
d8 k -n d8-system get deployment deckhouse -o json | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image'
```

1. Скачайте бандл только образов версий платформы (так как модули имеют свой релизный цикл их необходимо скачивать отдельно), начиная с версии установленной в кластере с помощью указания флага `--since-version` конструкцией вида:

```bash
d8 mirror pull --source='registry.deckhouse.ru/deckhouse/ee' --license='YOUR_LICENSE_TOKEN' --no-modules --no-security-db --since-version 1.68.13 $(pwd)/d8-bundle-platform
```

Будут скачаны последние патч релизы всех минорных версий до актуальных версий, находящихся на релизных каналах (с версиями можно ознакомиться по ссылке [https://releases.deckhouse.ru/](https://releases.deckhouse.ru/))

1. Получите список "не внутренних" установленных модулей в кластере и скачайте обновления для этих модулей:

Получим список установленных модулей и их версий

```bash
d8 k get mr | grep Deployed
```

Пример:

```bash
~$ d8 k get mr | grep Deployed
commander-agent-v1.2.4           Deployed                     12d
console-v1.35.1             Deployed                     12d
```

И составьте команду скачивания модулей начиная с версий, которые установлены в кластере, вида:

```bash
d8 mirror pull --source='registry.deckhouse.ru/deckhouse/ee' --license='<license-key>' --no-platform --no-security-db --include-module "commander-agent@1.2.4" --include-module "console@1.35.1" $(pwd)/d8-bundle-modules
```

Будут скачаны последние патч релизы всех минорных (и мажорных?) версий до актуальных версий, находящихся на релизных каналах (с версиями можно ознакомиться по ссылке [https://releases.deckhouse.ru/ee](https://releases.deckhouse.ru/ee) (или другой используемой у Вас редакции платформы))

1. Скачайте обновлённые базы для сканера trivy:

```bash
d8 mirror pull --source='registry.deckhouse.ru/deckhouse/ee' --license='Srme5sSxQ27bLe5b5RnrHbemAKqJqSLc' --no-platform --no-modules $(pwd)/d8-bundle-security-db
```

1. Создайте папку для итогового бандла и загрузите полученный bundle в ваш registry командой вида:

```bash
mkdir $(pwd)/d8-bundle
mv $(pwd)/d8-bundle-platform/* $(pwd)/d8-bundle
mv $(pwd)/d8-bundle-modules/* $(pwd)/d8-bundle
mv $(pwd)/d8-bundle-security-db/* $(pwd)/d8-bundle
d8 mirror push $(pwd)/d8-bundle private-registry.company.name:5050/dkp/ee --registry-login LOGIN --registry-password PASSWORD --tls-skip-verify
```

Или загрузите отдельно:

```bash
d8 mirror push $(pwd)/d8-bundle-platform private-registry.company.name:5050/dkp/ee --registry-login LOGIN --registry-password PASSWORD --tls-skip-verify
d8 mirror push $(pwd)/d8-bundle-modules private-registry.company.name:5050/dkp/ee --registry-login LOGIN --registry-password PASSWORD --tls-skip-verify
d8 mirror push $(pwd)/d8-bundle-security-db private-registry.company.name:5050/dkp/ee --registry-login LOGIN --registry-password PASSWORD --tls-skip-verify
```

1. Проверьте состояние обновления в кластере с помощью команд

```bash
d8 k get deckhousereleases.deckhouse.io
d8 k get modulereleases.deckhouse.io
d8 system queue list
```

При необходимости (если автоматически не создались промежуточные deckhousereleases - примените файл DeckhouseRelease manifests)

1. Вы восхитительны.
