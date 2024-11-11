---
title: "Установка платформы"
permalink: ru/virtualization-platform/documentation/admin/install/steps/install.html
lang: ru
---

TODO: вставить инфу про инсталлятор и конифиг файлы

> При установке платформы отличной от [редакции](../../editions.html) Community Edition из официального container registry `registry.deckhouse.io` необходимо предварительно авторизоваться с помощью лицензионного ключа:
>
> ```shell
> docker login -u license-token registry.deckhouse.io
> ```

Запуск контейнера инсталлятора из публичного container registry Deckhouse в общем случае выглядит так:

```shell
docker run --pull=always -it [<MOUNT_OPTIONS>] registry.deckhouse.io/deckhouse/<DECKHOUSE_REVISION>/install:<RELEASE_CHANNEL> bash
```

где:
- `<DECKHOUSE_REVISION>` — [редакция](../revision-comparison.html) Deckhouse (например `ee` — для Enterprise Edition, `ce` — для Community Edition и т. д.)
- `<MOUNT_OPTIONS>` — параметры монтирования файлов в контейнер инсталлятора, таких как:
  - SSH-ключи доступа;
  - файл конфигурации;
  - файл ресурсов и т. д.
- `<RELEASE_CHANNEL>` — [канал обновлений](../modules/002-deckhouse/configuration.html#parameters-releasechannel) Deckhouse в kebab-case. Должен совпадать с установленным в `config.yml`:
  - `alpha` — для канала обновлений *Alpha*;
  - `beta` — для канала обновлений *Beta*;
  - `early-access` — для канала обновлений *Early Access*;
  - `stable` — для канала обновлений *Stable*;
  - `rock-solid` — для канала обновлений *Rock Solid*.

Пример запуска контейнера инсталлятора Deckhouse CE:

```shell
docker run -it --pull=always \
  -v "$PWD/config.yaml:/config.yaml" \
  -v "$PWD/resources.yml:/resources.yml" \
  -v "$PWD/dhctl-tmp:/tmp/dhctl" \
  -v "$HOME/.ssh/:/tmp/.ssh/" registry.deckhouse.io/deckhouse/ce/install:stable bash
```

Установка Deckhouse запускается в контейнере инсталлятора с помощью команды `dhctl`:
- Для запуска установки Deckhouse с развертыванием кластера (это все случаи, кроме установки в существующий кластер) используйте команду `dhctl bootstrap`.
- Для запуска установки Deckhouse в существующем кластере используйте команду `dhctl bootstrap-phase install-deckhouse`.

> Для получения справки по параметрам выполните `dhctl bootstrap -h`.

Пример запуска установки Deckhouse с развертыванием кластера в облаке:

```shell
dhctl bootstrap \
  --ssh-user=<SSH_USER> --ssh-agent-private-keys=/tmp/.ssh/id_rsa \
  --config=/config.yml --config=/resources.yml
```

где:
- `/config.yml` — файл конфигурации установки;
- `/resources.yml` — файл манифестов ресурсов;
- `<SSH_USER>` — пользователь на сервере для подключения по SSH;
- `--ssh-agent-private-keys` — файл приватного SSH-ключа для подключения по SSH.
