---
title: "Удаление"
permalink: ru/uninstalling/
lang: ru
---


## Удаление статического кластера

Для удаления кластера, установленного вручную (например, bare metal), нужно выполнить несколько шагов:

1. [Удалите](../modules/node-manager/faq.html#как-зачистить-узел-для-последующего-ввода-в-кластер) из кластера все дополнительные узлы.

2. Узнайте канал обновления, заданный в кластере. Для этого выполните команду:

   ```shell
   kubectl get mc deckhouse  -o jsonpath='{.spec.settings.releaseChannel}'
   ```

3. Запустите инсталлятор Deckhouse:

   ```shell
   docker run --pull=always -it [<MOUNT_OPTIONS>] \
     registry.deckhouse.ru/deckhouse/<DECKHOUSE_REVISION>/install:<RELEASE_CHANNEL> bash
   ```

   где:
   - `<MOUNT_OPTIONS>` — параметры монтирования файлов в контейнер инсталлятора, таких как SSH-ключи доступа;
   - `<DECKHOUSE_REVISION>` — [редакция](../revision-comparison.html) Deckhouse — `cse`;
   - `<RELEASE_CHANNEL>` — [канал обновлений](../modules/deckhouse/configuration.html#parameters-releasechannel) Deckhouse в kebab-case. Должен совпадать с установленным в `config.yml`:
     - `alpha` — для канала обновлений *Alpha*;
     - `beta` — для канала обновлений *Beta*;
     - `early-access` — для канала обновлений *Early Access*;
     - `stable` — для канала обновлений *Stable*;
     - `rock-solid` — для канала обновлений *Rock Solid*.

   Пример запуска контейнера инсталлятора Deckhouse CE:

   ```shell
   docker run -it --pull=always \
     -v "$PWD/dhctl-tmp:/tmp/dhctl" \
     -v "$HOME/.ssh/:/tmp/.ssh/" registry.deckhouse.ru/deckhouse/ce/install:stable bash
   ```

4. Выполните команду удаления кластера:

   ```shell
   dhctl destroy --ssh-user=<USER> \
     --ssh-agent-private-keys=/tmp/.ssh/id_rsa \
     --yes-i-am-sane-and-i-understand-what-i-am-doing \
     --ssh-host=<MASTER_IP>
   ```

   где:
   - `<USER>` — пользователь удалённой машины, из-под которого производилась установка;
   - `<MASTER_IP>` — IP-адрес master-узла кластера.

Инсталлятор подключится к master-узлу и удалит на нём все компоненты Deckhouse и кластера Kubernetes.
