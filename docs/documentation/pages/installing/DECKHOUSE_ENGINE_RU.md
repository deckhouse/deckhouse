---
title: Установка кластера на bare metal с Deckhouse Engine
permalink: ru/installing/deckhouse-engine.html
description: Установка кластера Deckhouse Platform с тремя master-узлами на Deckhouse Engine и добавление worker-узла на bare metal.
lang: ru
---

На этой странице представлена информация по установке Deckhouse Platform на машины с использованием [Deckhouse Engine](../reference/deckhouse-engine.html). В качестве примера будут использоваться три master-узла и один worker-узел.

## Предварительные требования

Подготовьте четыре машины архитектуры x86-64 с загрузкой через Unified Extensible Firmware Interface (UEFI). Для каждой машины нужен пустой системный диск от 20 ГиБ. В этом примере у каждого master-узла также есть отдельный пустой диск на 10 ГиБ для etcd. При установке выбранные диски форматируются.

{% alert level="info" %}
Для правильного подбора количества машин и их характеристик в зависимости от планируемой нагрузки ознакомьтесь [с аппаратными требованиями к кластеру](../guides/hardware-requirements.html).
{% endalert %}

На хосте должны быть доступны TCP-порт `50000` всех трёх машин под master-узлы и TCP-порт `50001` первого master-узла. Подробнее о портах — [в требованиях к сетевому взаимодействию](../reference/network_interaction.html).

На машине, с которой будет производиться установка, должны быть установлены Docker и `jq`.

## Подготовка машин

Запустите установщик Deckhouse Platform выбранной редакции. Подробнее о редакциях — [в таблице сравнения редакций](../reference/revision-comparison.html).

1. Получите дайджест образа с установочным ISO из установочного образа той редакции и того канала обновлений, которые будете устанавливать:

   ```shell
   docker run --rm --pull=always 'registry.deckhouse.ru/deckhouse/<EDITION>/install:<CHANNEL>' \
     cat /deckhouse/candi/images_digests.json | jq -r .nodeManager.engineMetalIso
   ```

   Не забудьте заменить в команде `<EDITION>` и `<CHANNEL>`.

1. Извлеките ISO-файл из образа:

   ```shell
   docker create --name engine-iso 'registry.deckhouse.ru/deckhouse/<EDITION>@<ISO_DIGEST>' true
   docker cp engine-iso:/disk/metal.iso .
   docker rm engine-iso
   ```

1. Загрузите каждую машину с полученного `metal.iso` (USB-носитель или виртуальный CD-ROM). Установщик получит адрес по DHCP и будет ждать конфигурацию. Если DHCP в сети нет, задайте адрес вручную.

1. Убедитесь, что машина ждёт конфигурацию:

   ```shell
   curl -s 'http://<NODE_ADDRESS>:50000/whoami'
   ```

   Ответ `installer` означает, что машина готова. Ответ `agent` означает, что на машине уже установлен узел, и конфигурацию она не примет.

1. Посмотрите диски машины, чтобы указать системный диск явно:

   ```shell
   curl -s 'http://<NODE_ADDRESS>:50000/inventory'
   ```

   Для выбора диска используйте атрибуты `wwid`, `serial` или `busPath`. В отличие от имени `/dev/sdX`, эти атрибуты не меняются между загрузками.

## Установка кластера

Укажите `DeckhouseEngine` в параметре [`spec.systemType`](/modules/node-manager/cr.html#nodegroup-v1-spec-systemtype) в NodeGroup `master` и `worker`. Установщик использует параметры группы `master` для настройки всех трёх master-узлов и создаст группу `worker` для последующего добавления узлов.

Сохраните конфигурацию установки в файл `config.yml`. Укажите в нём диапазоны сетей и шаблон домена для вашего кластера.

{% offtopic title="Конфигурация установки" %}

```yaml

apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "Automatic"
clusterDomain: "cluster.local"
---
apiVersion: deckhouse.io/v1
kind: StaticClusterConfiguration
internalNetworkCIDRs:
  - 192.168.1.0/24
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  enabled: true
  settings:
    bundle: Default
    releaseChannel: Stable
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 1
  settings:
    modules:
      publicDomainTemplate: "%s.example.com"
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cni-cilium
spec:
  version: 1
  enabled: true
  settings:
    tunnelMode: VXLAN
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  nodeType: Static
  systemType: DeckhouseEngine
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
  systemType: DeckhouseEngine
---
# Конфигурация первого master-узла. Остальные master-узлы описываются так же.
apiVersion: internal.deckhouse.io/v1alpha1
kind: NodeConfig
metadata:
  name: master-0
spec:
  storage:
    diskSelector:
      serial: S3Z8NB0K700001
    mounts:
      # Пустой диск под etcd: узел отформатирует его и подключит в /var/lib/etcd.
      - name: kubernetes-data
        partitionSelector:
          blank: true
          size: ">=10Gi"
```

{% endofftopic %}

Обратите внимание, что если `diskSelector` не указан, системный диск выбирается по размеру (`size: ">=20Gi"`).

1. Создайте директорию для журналов установки и kubeconfig и запустите контейнер установщика:

   ```shell
   docker run -it --pull=always \
   -v "$PWD/config.yml:/config.yml" \
   -v "$PWD/dhctl-tmp:/tmp/dhctl" \
   'registry.deckhouse.ru/deckhouse/<EDITION>/install:<CHANNEL>' bash
   ```

1. В контейнере запустите `dhctl bootstrap`. Подставьте адреса трёх master-машин вместо адресов из примера. Установка начинается с машины в первом параметре `--master-host`. В примере это `master-0`.

   ```shell
   dhctl bootstrap \
     --config=/config.yml \
     --master-host master-0=192.168.1.10 \
     --master-host master-1=192.168.1.11 \
     --master-host master-2=192.168.1.12
   ```

   Указывать параметры подключения по SSH не нужно — установщик подключится к Deckhouse Engine по порту `50000` и выполнит все работы.

После успешной установки с master-узла будет загружен конфигурационный файл kubeconfig, путь к которому отобразится в логе установки. Его можно использовать для подключения к API Kubernetes.

{% alert level="warning" %}
Установка из бандла (режим `Local` модуля [registry](/modules/registry/)) для master-узлов на Deckhouse Engine не поддерживается. Для закрытого окружения используйте модуль `registry` в режиме `Managed` с хранилищем образов внутри периметра, заполненным через `d8 mirror push`.
{% endalert %}

## Добавление worker-узла

Используйте kubeconfig, полученный при установке, для подключения к кластеру.

В NodeGroup `worker` укажите [`nodeType: Static`](/modules/node-manager/cr.html#nodegroup-v1-spec-nodetype) и [`systemType: DeckhouseEngine`](/modules/node-manager/cr.html#nodegroup-v1-spec-systemtype).

1. Загрузите машину с `metal.iso` и убедитесь, что она ждёт конфигурацию (GET-запрос `/whoami` отвечает `installer`).

1. Добавьте машину в группу `worker`:

   ```shell
   d8 node add 192.168.1.20 --group worker
   ```

   Команда прочитает шаблон NodeConfig группы и диски машины, определит системный диск, способ настройки сети и имя узла, передаст машине конфигурацию и дождётся, пока узел зарегистрируется в кластере.

   Для автоматизации все ответы можно передать флагами:

   ```shell
   d8 node add 192.168.1.20 --group worker \
     --name worker-0 --disk-selector serial=S3Z8NB0K700002 --network dhcp --yes
   ```

   Флаг `--dry-run` покажет результат без отправки, скрыв секреты.

1. Проверьте, что узел появился в кластере и его NodeConfig применён:

   ```shell
   d8 k get nodes
   d8 k get nodeconfigs.internal.deckhouse.io
   ```

Для команды `d8 node add` требуются право на чтение `nodeconfigtemplates` и сетевой доступ к порту `50000` машины. Право на чтение входит в capability `d8:system-capability:node-manager:edit`, которую включает [роль подсистемы `d8:subsystem:cluster:manager`](../admin/configuration/access/authorization/rbac-experimental.html#системные-и-подсистемные-роли). Порт установщика не проверяет, кто передаёт конфигурацию, поэтому вводите машины в кластер из доверенной сети.
