---
title: "Установка базовой платформы"
permalink: ru/virtualization-platform/documentation/admin/install/steps/base-cluster.html
lang: ru
---

## Подготовка конфигурации

Для установки платформы нужно подготовить YAML-файл конфигурации установки.

### Файл конфигурации установки

YAML-файл конфигурации установки содержит параметры нескольких ресурсов (манифесты):

- [ClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration) — общие параметры кластера, такие как версия управляющего слоя (control plane), сетевые параметры, параметры CRI и т.д.

  {% alert level="info" %}
  Использовать ресурс ClusterConfiguration в конфигурации необходимо только если при установке платформы нужно предварительно развернуть кластер Kubernetes. То есть `ClusterConfiguration` не нужен, если платформа устанавливается в существующем кластере Kubernetes.
  {% endalert %}

- [StaticClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#staticclusterconfiguration) — параметры кластера Kubernetes, разворачиваемого на серверах bare metal.

  {% alert level="info" %}
  Как и в случае с ресурсом ClusterConfiguration, ресурс StaticClusterConfiguration не нужен, если платформа устанавливается в существующем кластере Kubernetes.
  {% endalert %}

- [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig) — набор ресурсов, содержащих параметры конфигурации модулей платформы.

Например, при планировании параметров кластера были выбраны следующие значения:

- Подсети подов и сервисов — `10.88.0.0/16` и `10.99.0.0/16`;
- Узлы связаны между собой через подсеть `192.168.1.0/24`;
- Публичный wildcard-домен кластера `my-dvp-cluster.example.com`;
- Канал обновлений `early-access`.

{% offtopic title="Пример config.yaml для установки базовой платформы" %}

```yaml
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
podSubnetCIDR: 10.88.0.0/16
serviceSubnetCIDR: 10.99.0.0/16
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
  name: global
spec:
  version: 1
  settings:
    modules:
      publicDomainTemplate: "%s.my-dvp-cluster.example.com"
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  enabled: true
  version: 1
  settings:
    bundle: Default
    releaseChannel: EarlyAccess
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  enabled: true
  version: 2
  settings:
    controlPlaneConfigurator:
      dexCAMode: DoNotNeed
    # Включение доступа к Kubernetes API через Ingress.
    # https://deckhouse.ru/modules/user-authn/configuration.html#parameters-publishapi
    publishAPI:
      enabled: true
      https:
        mode: Global
        global:
          kubeconfigGeneratorMasterCA: ""
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cni-cilium
spec:
  enabled: true
  version: 1
  settings:
    tunnelMode: VXLAN
```

{% endofftopic %}

## Авторизация в container registry

В зависимости от выбранной редакции может потребоваться авторизация в container registry `registry.deckhouse.ru`:

- Для установки Community Edition авторизация не требуется.

- Для установки Enterprise Edition и выше необходимо выполнить авторизацию на **машине установки**  с использованием лицензионного ключа:

  ```shell
  docker login -u license-token registry.deckhouse.ru
  ```

## Запуск установщика платформы

### Выбор образа установщика

Установщик запускается в виде контейнера. Образ контейнера выбирается в зависимости от редакции и канала обновлений:

```shell
registry.deckhouse.ru/deckhouse/<REVISION>/install:<RELEASE_CHANNEL>
```

Где:

- `<REVISION>` — [редакция](../../../about/editions.html) платформы (например `ee` — для Enterprise Edition, `ce` — для Community Edition и т. д.)

- `<RELEASE_CHANNEL>` — [канал обновлений](../../../about/release-channels.html) платформы в kebab-case:
  - `alpha` — для канала обновлений *Alpha*;
  - `beta` — для канала обновлений *Beta*;
  - `early-access` — для канала обновлений *EarlyAccess*;
  - `stable` — для канала обновлений *Stable*;
  - `rock-solid` — для канала обновлений *RockSolid*.

### Установка с созданием кластера

1. Запустите контейнер, в который будут подмонтированы файл конфигурации и ключи для доступа к узлам.

   Например, для установки редакции `CE` из канала обновлений `Stable` следует использовать образ `registry.deckhouse.ru/deckhouse/ce/install:stable`. В этом случае контейнер можно запустить следующей командой:

   ```shell
   docker run -it --pull=always \
     -v "$PWD/config.yaml:/config.yaml" \
     -v "$HOME/.ssh/:/tmp/.ssh/" registry.deckhouse.ru/deckhouse/ce/install:stable bash
   ```

1. Запустите внутри контейнера установщик платформы с помощью команды `dhctl bootstrap`.

   Например, при подготовке узлов был создан пользователь `dvpinstall`, а master-узел имеет адрес `54.43.32.21`. В этом случае установку платформы можно запустить следующей командой:

   ```shell
   dhctl bootstrap \
     --ssh-host=54.43.32.21 \
     --ssh-user=dvpinstall --ssh-agent-private-keys=/tmp/.ssh/<SSH_PRIVATE_KEY_FILE> \
     --config=/config.yaml --ask-become-pass
   ```

{% alert level="info" %}
Замените здесь `<SSH_PRIVATE_KEY_FILE>` на имя вашего приватного ключа. Например, для ключа с RSA-шифрованием это может быть `id_rsa`, а для ключа с ED25519-шифрованием — `id_ed25519`.
{% endalert %}

Если для запуска `sudo` на сервере необходим пароль, то укажите его в ответ на запрос `[sudo] Password:`.
Параметр `--ask-become-pass` можно не указывать, если при подготовке узла был настроен запуск `sudo` без пароля.

Монтирование директории `$HOME/.ssh` позволяет установщику получить доступ к конфигурации SSH, поэтому в параметре `--ssh-host` можно указывать имена секций Host из конфигурационного файла SSH.

### Установка в существующем кластере

1. Запустите контейнер, в который будут подмонтированы файл конфигурации, ключи для доступа к узлам и файл для подключения к Kubernetes API.

   Например, для установки редакции `CE` из канала обновлений `Stable` будет использоваться образ `registry.deckhouse.ru/deckhouse/ce/install:stable`,  а для подключения к Kubernetes API будет использоваться файл конфигурации в `$HOME/.kube/config`.

   В этом случае контейнер можно запустить следующей командой:

   ```shell
   docker run -it --pull=always \
     -v "$PWD/config.yaml:/config.yaml" \
     -v "$HOME/.kube/config:/kubeconfig" registry.deckhouse.ru/deckhouse/ce/install:stable bash
   ```

1. Запустите внутри контейнера установщик платформы с помощью команды `dhctl bootstrap-phase install-deckhouse`.

   Если на **машине установки** настроен доступ к существующему кластеру, то запустить установку платформы можно командой:

   ```shell
   dhctl bootstrap-phase install-deckhouse \
     --config=/config.yaml \
     --kubeconfig=/kubeconfig
   ```

### Завершение установки

Время установки может варьироваться от 5 до 30 минут в зависимости от качества соединения между master-узлом и хранилищем образов.

{% offtopic title="Пример вывода при успешном окончании установки..." %}

```console
...

┌ Create deckhouse release for version v1.65.6
│ 🎉 Succeeded!
└ Create deckhouse release for version v1.65.6 (0.23 seconds)

┌ ⛵ ~ Bootstrap: Clear cache
│ ❗ ~ Next run of "dhctl bootstrap" will create a new Kubernetes cluster.
└ ⛵ ~ Bootstrap: Clear cache (0.00 seconds)

🎉 Deckhouse cluster was created successfully!
```

{% endofftopic %}

После успешной установки можно выйти из запущенного контейнера и перейти к [настройке доступа](access.html).

## Проверки, выполняемые перед началом установки

Список проверок, выполняемых инсталлятором перед началом установки платформы:

1. Общие проверки:
   - Значения параметров [`publicDomainTemplate`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate) и [`clusterDomain`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clusterdomain) не совпадают.
   - Данные аутентификации для хранилища образов контейнеров, указанные в конфигурации установки, корректны.
   - Имя хоста соответствует следующим требованиям:
     - Длина не более 63 символов;
     - Состоит только из строчных букв;
     - Не содержит спецсимволов (допускаются символы `-` и `.`, при этом они не могут быть в начале или в конце имени).
   - На сервере (ВМ) отсутствует установленный CRI (containerd).
   - Имя хоста должно быть уникальным в пределах кластера.

1. Проверки для установки статического и гибридного кластера:
   - Указан только один параметр `--ssh-host`. Для статической конфигурации кластера можно задать только один IP-адрес для настройки первого master-узла.
   - Должна быть возможность подключения по SSH с использованием указанных данных аутентификации.
   - Должна быть возможность установки SSH-туннеля до сервера (или виртуальной машины) master-узла.
   - Сервер (ВМ) удовлетворяет минимальным требованиям для настройки master-узла.
   - На сервере (ВМ) для master-узла установлен Python.
   - Хранилище образов контейнеров доступно через прокси (если настройки прокси указаны в конфигурации установки).
   - На сервере (ВМ) для master-узла и хосте инсталлятора свободны порты, необходимые для процесса установки.
   - DNS должен разрешать `localhost` в IP-адрес 127.0.0.1.
   - На сервере (ВМ) пользователю доступна команда `sudo`.

1. Проверки для установки облачного кластера:
   - Конфигурация виртуальной машины master-узла удовлетворяет минимальным требованиям.

{% offtopic title="Список флагов пропуска проверок..." %}

- `--preflight-skip-all-checks` — пропуск всех предварительных проверок.
- `--preflight-skip-ssh-forward-check`  — пропуск проверки проброса SSH.
- `--preflight-skip-availability-ports-check` — пропуск проверки доступности необходимых портов.
- `--preflight-skip-resolving-localhost-check` — пропуск проверки `localhost`.
- `--preflight-skip-deckhouse-version-check` — пропуск проверки версии Deckhouse.
- `--preflight-skip-registry-through-proxy` — пропуск проверки доступа к registry через прокси-сервер.
- `--preflight-skip-public-domain-template-check`  — пропуск проверки шаблона `publicDomain`.
- `--preflight-skip-ssh-credentials-check`   — пропуск проверки учетных данных SSH-пользователя.
- `--preflight-skip-registry-credential` — пропуск проверки учетных данных для доступа к registry.
- `--preflight-skip-containerd-exist` — пропуск проверки наличия containerd.
- `--preflight-skip-python-checks` — пропуск проверки наличия Python.
- `--preflight-skip-sudo-allowed` — пропуск проверки прав доступа для выполнения команды `sudo`.
- `--preflight-skip-system-requirements-check` — пропуск проверки системных требований.
- `--preflight-skip-one-ssh-host` — пропуск проверки количества указанных SSH-хостов.
  
Пример применения флага пропуска:

```shell
    dhctl bootstrap \
    --ssh-user=<SSH_USER> --ssh-agent-private-keys=/tmp/.ssh/<SSH_PRIVATE_KEY_FILE> \
    --config=/config.yml \
    --preflight-skip-all-checks 
```

{% alert level="info" %}
Замените здесь `<SSH_PRIVATE_KEY_FILE>` на имя вашего приватного ключа. Например, для ключа с RSA-шифрованием это может быть `id_rsa`, а для ключа с ED25519-шифрованием — `id_ed25519`.
{% endalert %}

{% endofftopic %}
