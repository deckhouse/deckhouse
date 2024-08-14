---
title: "Uninstallation"
permalink: en/uninstalling/
lang: en
---

## Удаление кластера, развернутого в облачном провайдере

Для удаления кластера, развернутого в облачном провайдере, запустите инсталлятор Deckhouse:

```shell
docker run --pull=always -it [<MOUNT_OPTIONS>] registry.deckhouse.io/deckhouse/<DECKHOUSE_REVISION>/install:<RELEASE_CHANNEL> bash
```

где:
- `<MOUNT_OPTIONS>` — параметры монтирования файлов в контейнер инсталлятора, таких как:
  - SSH-ключи доступа;
- `<DECKHOUSE_REVISION>` — [редакция](../revision-comparison.html) Deckhouse (например `ee` — для Enterprise Edition, `ce` — для Community Edition и т. д.)
- `<RELEASE_CHANNEL>` — [канал обновлений](../modules/002-deckhouse/configuration.html#parameters-releasechannel) Deckhouse в kebab-case. Должен совпадать с установленным в `config.yml`:
  - `alpha` — для канала обновлений *Alpha*;
  - `beta` — для канала обновлений *Beta*;
  - `early-access` — для канала обновлений *Early Access*;
  - `stable` — для канала обновлений *Stable*;
  - `rock-solid` — для канала обновлений *Rock Solid*.

Пример запуска контейнера инсталлятора Deckhouse CE:

```shell
docker run -it --pull=always \
  -v "$PWD/dhctl-tmp:/tmp/dhctl" \
  -v "$HOME/.ssh/:/tmp/.ssh/" registry.deckhouse.io/deckhouse/ce/install:stable bash
```

В запустившемся контейнере выполните команду:

```shell
dhctl destroy --ssh-user=<USER> --ssh-agent-private-keys=/tmp/.ssh/id_rsa  --yes-i-am-sane-and-i-understand-what-i-am-doing --ssh-host=<MASTER_IP>
```

где:
- `<USER>` — пользователь удаленной машины, из-под которого производилась установка;
- `<MASTER_IP>` — IP-адрес master-узла кластера.

Инсталлятор подключится к кластеру, получит его состояние и произведёт удаление всех компонентов, дисков, балансировщиков и узлов, из которых состоит кластер.

## Удаление кластера, установленного вручную

Для удаления кластера, установленного вручную (например, bare metal), нужно выполнить несколько шагов.

1. [Удалите](../modules/040-node-manager/faq.html#как-зачистить-узел-для-последующего-ввода-в-кластер) из кластера все дополнительные узлы.

1. Запустите инсталлятор Deckhouse как в разделе выше:
  ```shell
  docker run --pull=always -it [<MOUNT_OPTIONS>] registry.deckhouse.io/deckhouse/<DECKHOUSE_REVISION>/install:<RELEASE_CHANNEL> bash
  ```

1. Выполните команду удаления кластера:
  ```shell
  dhctl destroy --ssh-user=<USER> --ssh-agent-private-keys=/tmp/.ssh/id_rsa  --yes-i-am-sane-and-i-understand-what-i-am-doing --ssh-host=<MASTER_IP>
  ```
где:
- `<USER>` — пользователь удаленной машины, из-под которого производилась установка;
- `<MASTER_IP>` — IP-адрес master-узла кластера.

Инсталлятор подключится к master-узлу и удалит все компоненты Deckhouse и кластера Kubernetes.

