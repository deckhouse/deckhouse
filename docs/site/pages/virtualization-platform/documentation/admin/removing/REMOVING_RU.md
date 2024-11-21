---
title: "Удаление платформы"
permalink: ru/virtualization-platform/documentation/admin/removing/removing.html
lang: ru
---

Для удаления платформы нужно выполнить несколько шагов:

1. Удалите из кластера все узлы кроме master-узлов:

    1.1. Удалите узел из кластера Kubernetes:

      ```shell
      d8 k drain <node> --ignore-daemonsets --delete-local-data
      d8 k delete node <node>
      ```

    1.2. Запустите на узле скрипт очистки:

      ```shell
      bash /var/lib/bashible/cleanup_static_node.sh --yes-i-am-sane-and-i-understand-what-i-am-doing
      ```

2. Узнайте канал обновления, заданный в кластере:

   ```shell
   d8 k get mc deckhouse  -o jsonpath='{.spec.settings.releaseChannel}'
   ```

3. Запустите инсталлятор платформы:

   ```shell
   docker run --pull=always -it [<MOUNT_OPTIONS>] \
     registry.deckhouse.ru/deckhouse/<REVISION>/install:<RELEASE_CHANNEL> bash
   ```

   где:
   - `<MOUNT_OPTIONS>` — параметры монтирования файлов в контейнер инсталлятора, таких как SSH-ключи доступа;
   - `<REVISION>` — [редакция платформы](../editions.html), например, `ee` — для Enterprise Edition, `ce` — для Community Edition и т. д;
   - `<RELEASE_CHANNEL>` — [канал обновлений](../update-channels.html) платформы в kebab-case. Должен совпадать с установленным в `config.yaml`, указанным при [установке платформы](../install/steps/install.html):
     - `alpha` — для канала обновлений *Alpha*;
     - `beta` — для канала обновлений *Beta*;
     - `early-access` — для канала обновлений *Early Access*;
     - `stable` — для канала обновлений *Stable*;
     - `rock-solid` — для канала обновлений *Rock Solid*.

   Пример запуска контейнера инсталлятора платформы в редакции CE:

   ```shell
   docker run -it --pull=always \
     -v "$HOME/.ssh/:/tmp/.ssh/" registry.deckhouse.io/deckhouse/ce/install:stable bash
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

Инсталлятор подключится к master-узлу и удалит на нём все компоненты платформы и кластера Kubernetes.
