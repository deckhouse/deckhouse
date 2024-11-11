---
title: "Установка платформы"
permalink: ru/virtualization-platform/documentation/admin/install/steps/install.html
lang: ru
---

## Подготовка конфигурации

Для установки платформы нужно подготовить YAML-файл конфигурации установки и, при необходимости, YAML-файл ресурсов, которые нужно создать после успешной установки платформы.

### Файл конфигурации установки

TODO: поправить ссылки 

YAML-файл конфигурации установки содержит параметры нескольких ресурсов (манифесты):
- [InitConfiguration](configuration.html#initconfiguration) — начальные параметры [конфигурации платформы](../#конфигурация-deckhouse). С этой конфигурацией платформа запустится после установки.

  В этом ресурсе, в частности, указываются параметры, без которых платформа не запустится или будет работать некорректно. Например, параметры [размещения компонентов платформы](../deckhouse-configure-global.html#parameters-modules-placement-customtolerationkeys), используемый [storageClass](../deckhouse-configure-global.html#parameters-storageclass), параметры доступа к [container registry](configuration.html#initconfiguration-deckhouse-registrydockercfg), [шаблон используемых DNS-имен](../deckhouse-configure-global.html#parameters-modules-publicdomaintemplate) и другие.
  
- [ClusterConfiguration](configuration.html#clusterconfiguration) — общие параметры кластера, такие как версия control plane, сетевые параметры, параметры CRI и т. д.
  
  > Использовать ресурс `ClusterConfiguration` в конфигурации необходимо, только если при установке платформы нужно предварительно развернуть кластер Kubernetes. То есть `ClusterConfiguration` не нужен, если платформа устанавливается в существующем кластере Kubernetes.

- [StaticClusterConfiguration](configuration.html#staticclusterconfiguration) — параметры кластера Kubernetes, развертываемого на серверах bare metal или на виртуальных машинах в неподдерживаемых облаках.

  > Как и в случае с ресурсом `ClusterConfiguration`, ресурс`StaticClusterConfiguration` не нужен, если платформа устанавливается в существующем кластере Kubernetes.  

- `ModuleConfig` — набор ресурсов, содержащих параметры конфигурации [встроенных модулей платформы](../).

Если кластер изначально создается с узлами, выделенными под определенный вид нагрузки (системные узлы, узлы под мониторинг и т. п.), то для модулей использующих тома постоянного хранилища (например, для модуля `prometheus`), рекомендуется явно указывать соответствующий nodeSelector в конфигурации модуля. Например, для модуля `prometheus` это параметр [nodeSelector](../modules/300-prometheus/configuration.html#parameters-nodeselector).

TODO: поправить манифесты

{% offtopic title="Пример файла конфигурации установки..." %}

```yaml
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: Azure
  prefix: cloud-demo
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "Automatic"
clusterDomain: cluster.local
---
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  releaseChannel: Stable
---
apiVersion: deckhouse.io/v1
kind: AzureClusterConfiguration
layout: Standard
sshPublicKey: <SSH_PUBLIC_KEY>
vNetCIDR: 10.241.0.0/16
subnetCIDR: 10.241.0.0/24
masterNodeGroup:
  replicas: 3
  instanceClass:
    machineSize: Standard_D4ds_v4
    urn: Canonical:UbuntuServer:18.04-LTS:18.04.202010140
    enableExternalIP: true
provider:
  subscriptionId: <SUBSCRIPTION_ID>
  clientId: <CLIENT_ID>
  clientSecret: <CLIENT_SECRET>
  tenantId: <TENANT_ID>
  location: westeurope
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cni-flannel
spec:
  enabled: true
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  enabled: true
  settings:
    modules:
      publicDomainTemplate: "%s.k8s.example.com"
  version: 1
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: node-manager
spec:
  version: 1
  enabled: true
  settings:
    allowedBundles: ["ubuntu-lts"]
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: prometheus
spec:
  version: 2
  enabled: true
  # Укажите, в случае использование выделенных узлов для мониторинга. 
  # settings:
  #   nodeSelector:
  #     node.deckhouse.io/group: monitoring
```

{% endofftopic %}

### Файл ресурсов установки

Необязательный YAML-файл ресурсов установки содержит манифесты ресурсов Kubernetes, которые инсталлятор применит после успешной установки платформы.

Файл ресурсов может быть полезен для дополнительной настройки кластера после установки платформы: развертывание Ingress-контроллера, создание дополнительных групп узлов, ресурсов конфигурации, настройки прав и пользователей и т. д.

**Внимание!** В файле ресурсов установки нельзя использовать [ModuleConfig](../) для **встроенных** модулей. Для конфигурирования встроенных модулей используйте [файл конфигурации](#файл-конфигурации-установки).

{% offtopic title="Пример файла ресурсов..." %}

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: "nginx"
  controllerVersion: "1.1"
  inlet: "LoadBalancer"
  nodeSelector:
    node.deckhouse.io/group: worker
---
apiVersion: deckhouse.io/v1
kind: AzureInstanceClass
metadata:
  name: worker
spec:
  machineSize: Standard_F4
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  cloudInstances:
    classReference:
      kind: AzureInstanceClass
      name: worker
    maxPerZone: 3
    minPerZone: 1
    zones: ["1"]
  nodeType: CloudEphemeral
---
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: admin
spec:
  subjects:
  - kind: User
    name: admin@deckhouse.io
  accessLevel: SuperAdmin
  portForwarding: true
---
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  email: admin@deckhouse.io
  password: '$2a$10$isZrV6uzS6F7eGfaNB1EteLTWky7qxJZfbogRs1egWEPuT1XaOGg2'
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse-admin
spec:
  enabled: true
```

{% endofftopic %}

TODO: следующий раздел кажется избыточным

### Post-bootstrap-скрипт

После успешной установки платформы инсталлятор может запустить скрипт на одном из master-узлов. С помощью скрипта можно выполнять дополнительную настройку, собирать информацию о настройке и т. п.

Post-bootstrap-скрипт можно указать с помощью параметра `--post-bootstrap-script-path` при запуске инсталляции (см. далее).

{% offtopic title="Пример скрипта, выводящего IP-адрес балансировщика..." %}
Пример скрипта, который выводит IP-адрес балансировщика после развертывания кластера в облаке и установки платформы:

```shell
#!/usr/bin/env bash

set -e
set -o pipefail


INGRESS_NAME="nginx"


echo_err() { echo "$@" 1>&2; }

# declare the variable
lb_ip=""

# get the load balancer IP
for i in {0..100}
do
  if lb_ip="$(kubectl -n d8-ingress-nginx get svc "${INGRESS_NAME}-load-balancer" -o jsonpath='{.status.loadBalancer.ingress[0].ip}')"; then
    if [ -n "$lb_ip" ]; then
      break
    fi
  fi

  lb_ip=""

  sleep 5
done

if [ -n "$lb_ip" ]; then
  echo_err "The load balancer external IP: $lb_ip"
else
  echo_err "Could not get the external IP of the load balancer"
  exit 1
fi

outContent="{\"frontend_ips\":[\"$lb_ip\"]}"

if [ -z "$OUTPUT" ]; then
  echo_err "The OUTPUT env is empty. The result was not saved to the output file."
else
  echo "$outContent" > "$OUTPUT"
fi
```

{% endofftopic %}

## Установка платформы

> При установке платформы отличной от [редакции](../../editions.html) Community Edition из официального container registry `registry.deckhouse.io` необходимо предварительно авторизоваться с помощью лицензионного ключа:
>
> ```shell
> docker login -u license-token registry.deckhouse.io
> ```

Запуск контейнера инсталлятора из публичного container registry платформы в общем случае выглядит так:

```shell
docker run --pull=always -it [<MOUNT_OPTIONS>] registry.deckhouse.io/deckhouse/<DECKHOUSE_REVISION>/install:<RELEASE_CHANNEL> bash
```

где:
- `<DECKHOUSE_REVISION>` — [редакция](../../editions.html) платформы (например `ee` — для Enterprise Edition, `ce` — для Community Edition и т. д.)
- `<MOUNT_OPTIONS>` — параметры монтирования файлов в контейнер инсталлятора, таких как:
  - SSH-ключи доступа;
  - файл конфигурации;
  - файл ресурсов и т. д.
- `<RELEASE_CHANNEL>` — [канал обновлений](../update_channels.html) платформы в kebab-case. Должен совпадать с установленным в `config.yml`:
  - `alpha` — для канала обновлений *Alpha*;
  - `beta` — для канала обновлений *Beta*;
  - `early-access` — для канала обновлений *Early Access*;
  - `stable` — для канала обновлений *Stable*;
  - `rock-solid` — для канала обновлений *Rock Solid*.

Пример запуска контейнера инсталлятора платформы в редакции CE:

```shell
docker run -it --pull=always \
  -v "$PWD/config.yaml:/config.yaml" \
  -v "$PWD/resources.yml:/resources.yml" \
  -v "$PWD/dhctl-tmp:/tmp/dhctl" \
  -v "$HOME/.ssh/:/tmp/.ssh/" registry.deckhouse.io/deckhouse/ce/install:stable bash
```

Установка платформы запускается в контейнере инсталлятора с помощью команды `dhctl`:
- Для запуска установки платформы с развертыванием кластера (это все случаи, кроме установки в существующий кластер) используйте команду `dhctl bootstrap`.
- Для запуска установки платформы в существующем кластере используйте команду `dhctl bootstrap-phase install-deckhouse`.

> Для получения справки по параметрам выполните `dhctl bootstrap -h`.

Пример запуска установки платформы с развертыванием кластера в облаке:

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

Далее подключитесь к master-узлу по SSH (IP-адрес master-узла выводится инсталлятором по завершении установки):

```bash
ssh <USER_NAME>@<MASTER_IP>
```

Запуск Ingress-контроллера после завершения установки платформы может занять какое-то время. Прежде чем продолжить убедитесь что Ingress-контроллер запустился:

```bash
sudo d8 k -n d8-ingress-nginx get po
```

Дождитесь перехода Pod’ов в статус `Ready`.

Также дождитесь готовности балансировщика:

```bash
sudo d8 k -n d8-ingress-nginx get svc nginx-load-balancer
```

Значение `EXTERNAL-IP` должно быть заполнено публичным IP-адресом или DNS-именем.

## Настройка DNS

Для того чтобы получить доступ к веб-интерфейсам компонентов платформы, необходимо:

1. Настроить работу DNS.
2. Указать в параметрах платформы шаблон DNS-имен.

Шаблон DNS-имен используется для настройки Ingress-ресурсов системных приложений. Например, интерфейсу Grafana закреплено имя `grafana`. Тогда, для шаблона `%s.kube.company.my`, Grafana будет доступна по адресу `grafana.kube.company.my`, и т.д.

Чтобы упростить настройку, будет использоваться сервис `sslip.io`.

На master-узле выполните следующую команду, чтобы получить IP-адрес балансировщика и настроить шаблон DNS-имен сервисов платформы на использование `sslip.io`:

```bash
BALANCER_IP=$(sudo d8 k -n d8-ingress-nginx get svc nginx-load-balancer -o json | jq -r '.status.loadBalancer.ingress[0].ip') && \
echo "Balancer IP is '${BALANCER_IP}'." && sudo d8 k patch mc global --type merge \
  -p "{\"spec\": {\"settings\":{\"modules\":{\"publicDomainTemplate\":\"%s.${BALANCER_IP}.sslip.io\"}}}}" && echo && \
echo "Domain template is '$(sudo d8 k get mc global -o=jsonpath='{.spec.settings.modules.publicDomainTemplate}')'."
```

Команда также выведет установленный шаблон DNS-имен. Пример вывода:

```bash
Balancer IP is '1.2.3.4'.
moduleconfig.deckhouse.io/global patched

Domain template is '%s.1.2.3.4.sslip.io'.
```

## Установка систем хранения

Для корректного функционирования платформы необходимо установить одну или несколько систем хранения, для:

- постоянного хранения системных данных платформы (метрики, логи, образы)
- хранения дисков виртуальных машин

Описание перечня поддерживаемых систем хранения описано в разделе ["Настройка хранилищ"](../platform-management/storage/)