---
title: "Установка базовой платформы"
permalink: ru/virtualization-platform/documentation/admin/install/steps/base-cluster.html
lang: ru
---

## Подготовка конфигурации

Для установки платформы нужно подготовить YAML-файл конфигурации установки.

### Файл конфигурации установки

YAML-файл конфигурации установки содержит параметры нескольких ресурсов (манифесты):

- [ClusterConfiguration](../../../../reference/cr/clusterconfiguration.html) — общие параметры кластера, такие как версия control plane, сетевые параметры, параметры CRI и т.д.

  > Использовать ресурс ClusterConfiguration в конфигурации необходимо, только если при установке платформы нужно предварительно развернуть кластер Kubernetes. То есть `ClusterConfiguration` не нужен, если платформа устанавливается в существующем кластере Kubernetes.

- [StaticClusterConfiguration](../../../../reference/cr/staticclusterconfiguration.html) — параметры кластера Kubernetes, разворачиваемого на серверах bare metal.

  > Как и в случае с ресурсом `ClusterConfiguration`, ресурс`StaticClusterConfiguration` не нужен, если платформа устанавливается в существующем кластере Kubernetes.

- ModuleConfig — набор ресурсов, содержащих параметры конфигурации [встроенных модулей платформы](../).

{% offtopic title="Дополнительная конфигурация" %}
- [InitConfiguration](../../../../reference/cr/initconfiguration.html) — начальные параметры [конфигурации платформы](../#конфигурация-deckhouse). С этой конфигурацией платформа запустится после установки.

  В этом ресурсе, в частности, указываются параметры, без которых платформа не запустится или будет работать некорректно. Например, параметры [размещения компонентов платформы](../../../../reference/cr/deckhouse-configure-global.html#parameters-modules-placement-customtolerationkeys), используемый [storageClass](../deckhouse-configure-global.html#parameters-storageclass), параметры доступа к [container registry](configuration.html#initconfiguration-deckhouse-registrydockercfg), [шаблон используемых DNS-имен](/products/virtualization-platform/reference/mc.html#global-parameters-modules-publicdomaintemplate) и другие.

Если кластер изначально создается с узлами, выделенными под определенный вид нагрузки (системные узлы, узлы под мониторинг и т. п.), то для модулей, использующих тома постоянного хранилища (например, для модуля `prometheus`), рекомендуется явно указать соответствующий nodeSelector в конфигурации модуля. Например, для модуля `prometheus` это параметр [nodeSelector](/products/virtualization-platform/reference/mc.html#prometheus-parameters-nodeselector).

{% endofftopic %}

Например, при планировании параметров кластера были выбраны следующие значения:
- Подсети подов и сервисов — `10.10.0.0/16` и `10.11.0.0./16`;
- Узлы связаны между собой через подсеть `192.168.1.0/24`;
- Публичный wildcard-домен кластера `my-dvp-cluster.example.com`;
- Канал обновлений `early-access`.

`config.yaml` для установки базовой платформы будет выглядеть так:

```yaml
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
podSubnetCIDR: 10.10.0.0/16
serviceSubnetCIDR: 10.11.0.0/16
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
    # https://deckhouse.io/products/kubernetes-platform/documentation/v1/modules/150-user-authn/configuration.html#parameters-publishapi
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


## Авторизация в container registry

В зависимости от выбранной редакции может потребоваться авторизация в container registry `registry.deckhouse.io`. 

- Для установки Community Edition авторизация не требуется.

- Для Enterprise Edition и выше на **машине установки** нужно авторизоваться с помощью лицензионного ключа:

  ```shell
  docker login -u license-token registry.deckhouse.io
  ```

## Запуск установщика платформы

### Выбор образа установщика

Установщик запускается в виде docker контейнера. Образ контейнера выбирается в зависимости от редакции и канала обновлений:

```shell
registry.deckhouse.io/deckhouse/<REVISION>/install:<RELEASE_CHANNEL>
```

где:
- `<REVISION>` — [редакция](../../editions.html) платформы (например `ee` — для Enterprise Edition, `ce` — для Community Edition и т. д.)

- `<RELEASE_CHANNEL>` — [канал обновлений](../update_channels.html) платформы в kebab-case. Должен совпадать с указанным в `config.yaml`:
  - `alpha` — для канала обновлений *Alpha*;
  - `beta` — для канала обновлений *Beta*;
  - `early-access` — для канала обновлений *EarlyAccess*;
  - `stable` — для канала обновлений *Stable*;
  - `rock-solid` — для канала обновлений *RockSolid*.

### Установка с созданием кластера

Первым шагом нужно запустить контейнер, куда будет подмонтирован файл с конфигурацией и ключи для доступа к узлам. 

Например, для установки редакции `CE` из канала обновлений `Stable` нужно использовать образ `registry.deckhouse.io/deckhouse/ce/install:stable`. Тогда контейнер может быть запущен такой командой:

```shell
docker run -it --pull=always \
  -v "$PWD/config.yaml:/config.yaml" \
  -v "$HOME/.ssh/:/tmp/.ssh/" registry.deckhouse.io/deckhouse/ce/install:stable bash
```

Следующим шагом внутри контейнера запускается установщик платформы с помощью команды `dhctl bootstrap`.

Например, при подготовке узлов был создан пользователь `dvpinstall`, а master-узел имеет адрес `54.43.32.21`.
Команда для запуска установки платформы будет иметь вид:

```shell
dhctl bootstrap \
  --ssh-host=54.43.32.21 \
  --ssh-user=dvpinstall --ssh-agent-private-keys=/tmp/.ssh/id_rsa \
  --config=/config.yaml --ask-become-pass
```

Если для запуска sudo на сервере необходим пароль, то укажите его в ответ на запрос `[sudo] Password:`
Параметр `--ask-become-pass` можно не указывать, если при подготовке узла был настроен запуск sudo без пароля.

Благодаря монтированию директории `$HOME/.ssh` установщику доступен конфиг ssh, поэтому в параметр --ssh-host можно передавать имена секций Host.

### Установка в существующий кластер

Первым шагом нужно запустить контейнер, куда будет подмонтирован файл с конфигурацией, ключи для доступа к узлам и файл для подключения к Kubernetes API.

Например:
- Выбрана установка редакции `CE` из канала обновлений `Stable`, будет использоваться образ `registry.deckhouse.io/deckhouse/ce/install:stable`;
- Подключение к кластеру настроено в `$HOME/.kube/config`. 

Тогда контейнер может быть запущен такой командой:

```shell
docker run -it --pull=always \
  -v "$PWD/config.yaml:/config.yaml" \
  -v "$HOME/.kube/config:/kubeconfig" registry.deckhouse.io/deckhouse/ce/install:stable bash
```

Следующим шагом внутри контейнера запускается установщик платформы с помощью команды `dhctl bootstrap-phase install-deckhouse`.

Если на **машине установки** настроен доступ к существующему кластеру, то команда для запуска установки платформы будет иметь вид:

```shell
dhctl bootstrap-phase install-deckhouse \
  --config=/config.yaml \
  --kubeconfig=/kubeconfig
```

### Завершение установки

Процесс установки может занять от 5 до 30 минут, в зависимости от качества соединения между master-узлом и хранилищем образов.

Пример вывода при успешном окончании установки:
```
...

┌ Create deckhouse release for version v1.65.6
│ 🎉 Succeeded!
└ Create deckhouse release for version v1.65.6 (0.23 seconds)

┌ ⛵ ~ Bootstrap: Clear cache
│ ❗ ~ Next run of "dhctl bootstrap" will create a new Kubernetes cluster.
└ ⛵ ~ Bootstrap: Clear cache (0.00 seconds)

🎉 Deckhouse cluster was created successfully!
```

После успешной установки можно выйти из запущенного контейнера и проверить статус master-узла: 

```bash
ssh dvpinstall@54.43.32.21
d8 k get no

NAME           STATUS   ROLES                  AGE     VERSION
master-0       Ready    control-plane,master   5m      v1.29.10
```

