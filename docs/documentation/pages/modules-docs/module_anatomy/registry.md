---
title: "Container registry"
permalink: en/modules-docs/module-anatomy/registry/
---

После сборки модуль сохраняется в registry контейнера.<!-- уточнить репозиторий: Для распространения и обновления модулей Deckhouse Kubernetes Platform используется только этот репозиторий.--> Ниже рассмотрено, как выглядит модуль в registry контейнера и из чего он состоит.

> В примерах используется утилита [crane](https://github.com/google/go-containerregistry/tree/main/cmd/crane#crane). Установите ее по [инструкции](https://github.com/google/go-containerregistry/tree/main/cmd/crane#installation). Для MacOS воспользуйтесь brew.

## Состав модуля

Модуль состоит из трех частей:
- **Образы контейнеров приложений** запускаются в кластере Deckhouse и указываются в шаблонах. Образы описаны в папке [images](module_folder.md#images). Образы содержат content-based теги. Подробнее о том, как ставить теги можно почитать в документации [werf](https://werf.io/documentation/v1.2/usage/build/process.html#tagging-images).
- **Образ модуля** загружается в registry аналогично контейнеру. В качестве тегов образов используется `semver`.
- **Релиз** описывает релиз в `release.yaml`, который загружается в registry. Релизы создаются каждый раз при выходе новой версии и используются в Deckhouse Kubernetes Platform для обновления модуля в кластере. У релизов выставляется два типа тегов: `semver`, как у образа модуля, и тег, соответствующий каналу обновлений, например, `alpha`, `beta`.

## Источник модулей (Module Source)

Модули загружаются в источник модулей: вложенная абстракция, с которой потом работает Deckhouse Kubernetes Platform.

Пример того, как выглядит источник модулей внутри registry, представлен ниже.

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

Источник модулей имеет вложенную структуру репозиториев, и registry контейнера должен поддерживать эту функцию. Примеры подобных registry: [Docker Registry v2](https://github.com/distribution/distribution), [Harbor](https://goharbor.io/).

Для доставки модулей в закрытые (air-gapped) окружения есть специальные скрипты в репозитории [tools](https://fox.flant.com/deckhouse/modules/tools).

### Список полезных команд для работы с источником модулей

* Вывод списка модулей в источнике модулей:

  ```shell
  crane ls registry.example.io/modules-source
  ```

  Пример ответа:

  ```none
  module-1
  module-2
  ```

  Готово, в `module source` присутствует два модуля.

* Вывод списка образов модуля `module-1`:

  ```shell
  crane ls registry.example.io/modules-source/module-1
  ```

  Пример ответа:

  ```none
  v1.23.1
  d4bf3e71015d1e757a8481536eeabda98f51f1891d68b539cc50753a-1589714365467
  e6073b8f03231e122fa3b7d3294ff69a5060c332c4395e7d0b3231e3-1589714362300
  v1.23.2
  ```

  Готово, в модуле `module-1` присутствуют два образа модуля и два образа контейнеров приложений.

* Вывод файлов в образе модуля `v1.23.1`:

  ```shell
  $ crane export registry.example.io/modules-source/module-1:v1.23.1 - \
    | tar -tf -
  ```

  Ответ будет достаточно большим.

* Вывод образов контейнеров приложений, которые используются для модуля версии `v1.23.1`:

  ```shell
  $ crane export registry.example.io/modules-source/module-1:v1.23.1 - \
    | tar -Oxf - images_digests.json
  ```

  Пример ответа:

  ```json
  {
    "backend": "sha256:fcb04a7fed2c2f8def941e34c0094f4f6973ea6012ccfe2deadb9a1032c1e4fb",
    "frontend": "sha256:f31f4b7da5faa5e320d3aad809563c6f5fcaa97b571fffa5c9cab103327cc0e8"
  }
  ```

* Просмотр списка релизов:

  ```sh
  crane ls registry.example.io/modules-source/module-1/release
  ```

  Пример ответа:

  ```none
  v1.23.1
  v1.23.2
  alpha
  beta
  ```

  Готово, в этом registry было два релиза и используются два канала обновлений: `alpha` и `beta`.

* Вывод версии на канале обновлений `alpha`:

  ```shell
  $ crane export registry.example.io/modules-source/module-1/release:alpha - \
    | tar -Oxf - version.json
  ```

  Пример ответа:

  ```json
  {"version":"v1.23.2"}
  `
