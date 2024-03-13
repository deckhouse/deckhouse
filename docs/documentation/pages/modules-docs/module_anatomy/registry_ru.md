---
title: "Container registry"
permalink: ru/modules-docs/module-anatomy/registry/
lang: ru
---

После сборки модуль загружается в container registry. Для дистрибьюции и обновления модулей Deckhouse использует только этот источник.
Как выглядит модуль в container registry и из чего состоит мы разберем в этой главе.

> В примерах мы будем использовать утилиту [crane](https://github.com/google/go-containerregistry/tree/main/cmd/crane#crane). Как установить ее указано в [документации](https://github.com/google/go-containerregistry/tree/main/cmd/crane#installation) (для MacOS это можно сделать через brew).

## Из чего состоит артефакт модуля?

Модуль состоит из трех частей:
- **Образы контейнеров приложений** -- самая понятная часть. Это то что мы будем запускать в кластере Deckhouse, то что указываем в шаблонах. Именно эти образы мы описываем в папке [images](module_folder.md#images). Образы имеют content-based теги (подробнее о стратегии тегирования можно почитать в документации [werf](https://werf.io/documentation/v1.2/usage/build/process.html#tagging-images)).
- **Образ модуля** -- папка с модулем загружается в registry как контейнер. В качестве тегов образов используется semver.
- **Релиз** -- отдельный файл с описанием релиза `release.yaml`, который тоже загружается в registry. Релизы создаются каждый раз при выходе новой версии и используются Deckhouse для обновления модуля в кластере. У релизов два тега: semver (как у образа модуля) и тег, соответствующий каналу обновлений (alpha, beta, и т.д.).

## Источник модулей (Module Source)

Модули загружаются в источник модулей: вложенная абстракция, с которой потом работает Deckhouse.

Пример того, как выглядит Module Source внутри registry.

```tree
registry.example.io

    📁 modules-source
    ├─ 📁 module-1
    │  ├─ 📦 v1.23.1
    │  ├─ 📦 d4bf3e71015d1e757a8481536eeabda98f51f1891d68b539cc50753a-1589714365467
    │  ├─ 📦 e6073b8f03231e122fa3b7d3294ff69a5060c332c4395e7d0b3231e3-1589714362300
    │  ├─ 📦 v1.23.2
    │  └─ 📁 release
    │     ├─ 📝 v1.23.1
    │     ├─ 📝 v1.23.2
    │     ├─ 📝 alpha
    │     └─ 📝 beta
    └─ 📁 module-2
       ├─ 📦 v0.30.147
       ├─ 📦 d4bf3e71015d1e757a8481536eeabda98f51f1891d68b539cc50753a-1589714365467
       ├─ 📦 e6073b8f03231e122fa3b7d3294ff69a5060c332c4395e7d0b3231e3-1589714362300
       ├─ 📦 v0.31.1
       └─ 📁 release
          ├─ 📝 v0.30.147
          ├─ 📝 v0.31.1
          ├─ 📝 alpha
          └─ 📝 beta
```

> Поскольку источник модулей имеет вложенную структуру репозиториев, container registry должен поддерживать эту функцию. Примеры таких registry: [Docker Registry v2](https://github.com/distribution/distribution), [Harbor](https://goharbor.io/).
>
> Для доставки модулей в закрытые (air-gapped) окружения есть специальные скрипты в репозитории [tools](https://fox.flant.com/deckhouse/modules/tools).  

### Список полезных команд для работы с Module Source

#### Список модулей в Module Source

```sh
crane ls registry.example.io/modules-source
```

```
module-1
module-2
```

Видим, что в module source есть два модуля.

#### Список образов модуля

```sh
crane ls registry.example.io/modules-source/module-1
```

```
v1.23.1
d4bf3e71015d1e757a8481536eeabda98f51f1891d68b539cc50753a-1589714365467
e6073b8f03231e122fa3b7d3294ff69a5060c332c4395e7d0b3231e3-1589714362300
v1.23.2
```

Видим что для `module-1` есть два образа модуля и два образа контейнеров приложений.

#### Какие файлы лежат в образе модуля `v1.23.1`

```sh
$ crane export registry.example.io/modules-source/module-1:v1.23.1 - \
  | tar -tf -
```

> Вывод будет достаточно большим

#### Какие образы контейнеров приложений используются для модуля версии `v1.23.1`

```sh
$ crane export registry.example.io/modules-source/module-1:v1.23.1 - \
  | tar -Oxf - images_digests.json
```

```json
{
  "backend": "sha256:fcb04a7fed2c2f8def941e34c0094f4f6973ea6012ccfe2deadb9a1032c1e4fb",
  "frontend": "sha256:f31f4b7da5faa5e320d3aad809563c6f5fcaa97b571fffa5c9cab103327cc0e8"
}
```

#### Посмотреть список релизов

```sh
crane ls registry.example.io/modules-source/module-1/release
```

```
v1.23.1
v1.23.2
alpha
beta
```

Видим, что в этом registry было два релиза, а еще что там используются два канала обновлений: alpha и beta.

#### Какая версия сейчас находится на канале обновлений alpha

```sh
$ crane export registry.example.io/modules-source/module-1/release:alpha - \
  | tar -Oxf - version.json
```

```json
{"version":"v1.23.2"}
```
