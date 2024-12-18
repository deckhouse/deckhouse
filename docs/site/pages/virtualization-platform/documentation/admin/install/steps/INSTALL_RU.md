---
title: "Установка платформы"
permalink: ru/virtualization-platform/documentation/admin/install/steps/install.html
lang: ru
---

## Подготовка конфигурации

Для установки платформы нужно подготовить YAML-файл конфигурации установки. При необходимости, добавьте YAML-файл для ресурсов, которые будут созданы после успешной установки платформы.

### Файл конфигурации установки

YAML-файл конфигурации установки содержит параметры нескольких ресурсов (манифесты):

- [InitConfiguration](../../../../reference/cr/initconfiguration.html) — начальные параметры конфигурации платформы. С этой конфигурацией платформа запустится после установки.

  В этом ресурсе, в частности, указываются параметры, без которых платформа не запустится или будет работать некорректно. Например, параметры [размещения компонентов платформы](../../../../reference/mc.html#global-parameters-modules-placement-customtolerationkeys), используемый [storageClass](../../../../reference/mc.html#global-parameters-modules-storageclass), параметры доступа к [container registry](../../../../reference/cr/initconfiguration.html#initconfiguration-deckhouse-registrydockercfg), [шаблон используемых DNS-имен](../../../../reference/mc.html#global-parameters-modules-publicdomaintemplate) и другие.

- [ClusterConfiguration](../../../../reference/cr/clusterconfiguration.html) — общие параметры кластера, такие как версия control plane, сетевые параметры, параметры CRI и т.д.

  > Использовать ресурс ClusterConfiguration в конфигурации необходимо, только если при установке платформы нужно предварительно развернуть кластер Kubernetes. То есть `ClusterConfiguration` не нужен, если платформа устанавливается в существующем кластере Kubernetes.

- [StaticClusterConfiguration](../../../../reference/cr/staticclusterconfiguration.html) — параметры кластера Kubernetes, разворачиваемого на серверах bare metal.

  > Как и в случае с ресурсом `ClusterConfiguration`, ресурс`StaticClusterConfiguration` не нужен, если платформа устанавливается в существующем кластере Kubernetes.

- [ModuleConfig](../../../../reference/cr/moduleconfig.html) — набор ресурсов, содержащих параметры конфигурации встроенных модулей платформы.

  Если кластер изначально создается с узлами, выделенными под определенный вид нагрузки (системные узлы, узлы под мониторинг и т. п.), то для модулей, использующих тома постоянного хранилища (например, для модуля `prometheus`), рекомендуется явно указать соответствующий nodeSelector в конфигурации модуля. Например, для модуля `prometheus` это параметр [nodeSelector](/products/virtualization-platform/reference/mc.html#prometheus-parameters-nodeselector).

{% offtopic title="Пример файла конфигурации установки (config.yaml)..." %}

```yaml
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
podSubnetCIDR: 10.88.0.0/16
serviceSubnetCIDR: 10.99.0.0/16
kubernetesVersion: "Automatic"
clusterDomain: "cluster.local"
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  enabled: true
  settings:
    releaseChannel: Stable
    bundle: Default
    logLevel: Info
  version: 1
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  enabled: true
  settings:
    modules:
      publicDomainTemplate: "%s.dvp.example.com"
  version: 1
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
```

{% endofftopic %}

### Файл ресурсов установки

YAML-файл ресурсов установки содержит манифесты платформы, которые инсталлятор применит после ее успешной установки.

Файл необязателен, но может быть полезен для дополнительной настройки кластера после установки платформы. С его помощью можно создать Ingress-контроллер, дополнительные группы узлов, ресурсы конфигурации, настройки прав и пользователей и т.д.

**Внимание!** В файле ресурсов установки нельзя использовать [ModuleConfig](../../../../reference/cr/moduleconfig.html) для **встроенных** модулей. Используйте для них [файл конфигурации](#файл-конфигурации-установки).

{% offtopic title="Пример файла ресурсов (resources.yaml)..." %}

```yaml
# Создать группу из двух рабочих узлов
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  disruptions:
    approvalMode: Manual
  nodeType: Static
  staticInstances:
    count: 2
---
# SSH-ключ, для доступа к рабочим узлам для автоматизированной установки
apiVersion: deckhouse.io/v1alpha1
kind: SSHCredentials
metadata:
  name: worker-key
spec:
  # Имя технического пользователя, созданного на этапе подготовки узлов платформы
  user: install-user
  # Закрытый ключ, созданный на этапе подготовки узлов платформы, кодированный в base64 формате
  privateSSHKey: ZXhhbXBsZQo=
---
apiVersion: deckhouse.io/v1alpha1
kind: StaticInstance
metadata:
  name: worker-01
  labels:
    role: worker
spec:
  # Адрес первого рабочего узла
  address: 192.88.99.10
  credentialsRef:
    kind: SSHCredentials
    name: worker-key
---
apiVersion: deckhouse.io/v1alpha1
kind: StaticInstance
metadata:
  name: worker-01
  labels:
    role: worker
spec:
  # Адрес второго рабочего узла
  address: 192.88.99.20
  credentialsRef:
    kind: SSHCredentials
    name: worker-key
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: virtualization
spec:
  enabled: true
  settings:
    dvcr:
      storage:
        persistentVolumeClaim:
          size: 10G
        type: PersistentVolumeClaim
    virtualMachineCIDRs:
      - 192.168.10.0/24
  version: 1
---
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  inlet: HostPort
  enableIstioSidecar: true
  ingressClass: nginx
  hostPort:
    httpPort: 80
    httpsPort: 443
  nodeSelector:
    node-role.kubernetes.io/master: ''
  tolerations:
    - effect: NoSchedule
      operator: Exists
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
```

{% endofftopic %}

## Установка платформы

> При установке платформы, отличной от [редакции](../../editions.html) Community Edition, из официального container registry `registry.deckhouse.io` необходимо предварительно авторизоваться с помощью лицензионного ключа:
>
> ```shell
> docker login -u license-token registry.deckhouse.io
> ```

Пример запуска контейнера инсталлятора из публичного container registry платформы:

```shell
docker run --pull=always -it [<MOUNT_OPTIONS>] registry.deckhouse.io/deckhouse/<REVISION>/install:<RELEASE_CHANNEL> bash
```

где:

- `<REVISION>` — [редакция](../../editions.html) платформы (например `ee` — для Enterprise Edition, `ce` — для Community Edition и т. д.)
- `<MOUNT_OPTIONS>` — параметры монтирования файлов в контейнер инсталлятора, таких как:
  - SSH-ключи доступа;
  - файл конфигурации;
  - файл ресурсов и т. д.
- `<RELEASE_CHANNEL>` — [канал обновлений](../../update-channels.html) платформы в kebab-case. Должен совпадать с установленным в `config.yaml`:
  - `alpha` — для канала обновлений *Alpha*;
  - `beta` — для канала обновлений *Beta*;
  - `early-access` — для канала обновлений *Early Access*;
  - `stable` — для канала обновлений *Stable*;
  - `rock-solid` — для канала обновлений *Rock Solid*.

Пример запуска контейнера инсталлятора платформы в редакции CE:

```shell
docker run -it --pull=always \
  -v "$PWD/config.yaml:/config.yaml" \
  -v "$PWD/resources.yaml:/resources.yaml" \
  -v "$HOME/.ssh/:/tmp/.ssh/" registry.deckhouse.io/deckhouse/ce/install:stable bash
```

Установка платформы запускается в контейнере инсталлятора с помощью команды `dhctl`:

- Для запуска установки платформы с развертыванием кластера (это все случаи, кроме установки в существующий кластер) используйте команду `dhctl bootstrap`.
- Для запуска установки платформы в существующем кластере используйте команду `dhctl bootstrap-phase install-deckhouse`.

> Для получения справки по параметрам выполните `dhctl bootstrap -h`.

Пример запуска установки платформы:

```shell
dhctl bootstrap \
  --ssh-user=<SSH_USER> --ssh-agent-private-keys=/tmp/.ssh/id_rsa \
  --config=/config.yaml --config=/resources.yaml
```

где:

- `/config.yaml` — файл конфигурации установки;
- `/resources.yaml` — файл манифестов ресурсов;
- `<SSH_USER>` — пользователь на сервере для подключения по SSH;
- `--ssh-agent-private-keys` — файл приватного SSH-ключа для подключения по SSH.

Далее подключитесь к master-узлу по SSH (IP-адрес master-узла выводится инсталлятором по завершении установки):

```bash
ssh <USER_NAME>@<MASTER_IP>
```

Запуск Ingress-контроллера после завершения установки платформы может занять какое-то время. Прежде чем продолжить, убедитесь что Ingress-контроллер запустился:

```bash
d8 k -n d8-ingress-nginx get po
```

Дождитесь перехода подов в статус `Ready`.

Также дождитесь готовности балансировщика:

```bash
d8 k -n d8-ingress-nginx get svc nginx-load-balancer
```

Значение `EXTERNAL-IP` должно быть заполнено публичным IP-адресом или DNS-именем.

## Настройка DNS

Для того чтобы получить доступ к веб-интерфейсам компонентов платформы, необходимо:

1. Настроить работу DNS.
2. Указать в параметрах платформы шаблон DNS-имен.

Шаблон DNS-имен используется для настройки Ingress-ресурсов системных приложений. Например, интерфейсу Grafana закреплено имя `grafana`. Тогда, для шаблона `%s.kube.company.my`, Grafana будет доступна по адресу `grafana.kube.company.my`, и т.д.

Чтобы упростить настройку, будет использоваться сервис `sslip.io`.

Чтобы получить IP-адрес балансировщика и настроить шаблон DNS-имен сервисов платформы на использование `sslip.io`, выполните команду на master-узле:

```bash
BALANCER_IP=$(d8 k -n d8-ingress-nginx get svc nginx-load-balancer -o json | jq -r '.status.loadBalancer.ingress[0].ip') && \
echo "Balancer IP is '${BALANCER_IP}'." && d8 k patch mc global --type merge \
  -p "{\"spec\": {\"settings\":{\"modules\":{\"publicDomainTemplate\":\"%s.${BALANCER_IP}.sslip.io\"}}}}" && echo && \
echo "Domain template is '$(d8 k get mc global -o=jsonpath='{.spec.settings.modules.publicDomainTemplate}')'."
```

Команда также выведет установленный шаблон DNS-имен. Пример вывода:

```bash
Balancer IP is '1.2.3.4'.
moduleconfig.deckhouse.io/global patched

Domain template is '%s.1.2.3.4.sslip.io'.
```

## Установка систем хранения

Для корректного функционирования платформы необходимо установить одну или несколько систем хранения. Они предоставляют возможности:

- постоянного хранения системных данных платформы (метрики, логи, образы);
- хранения дисков и образов виртуальных машин.

Описание поддерживаемых систем хранения и инструкция по их подключению приведены в разделе [Настройка хранилищ](../../platform-management/storage/supported-storage.html).

## Установка модуля Сilium

Для получения информации по установке и настройке модуля, обратитесь к разделу [Настройки Cilium](/products/virtualization-platform/reference/mc.html#cni-cilium).

## Установка модуля виртуализации

Для обеспечения возможностей виртуализации (создание виртуальных машин, образов, дисков и так далее), необходимо включить
модуль виртуализации. Чтобы сделать это, создайте ресурс ModuleConfig `virtualization`, предварительно указав, какой
StorageClass следует использовать:

```shell
# Укажите имя своего ресурса StorageClass.
STORAGE_CLASS_NAME=replicated-storage-class

# Создайте ModuleConfig `virtualization`.
d8 k apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
 name: virtualization
spec:
 enabled: false
 settings:
   dvcr:
     storage:
       type: PersistentVolumeClaim
       persistentVolumeClaim:
         size: 50G
         storageClassName: ${STORAGE_CLASS_NAME}
   virtualMachineCIDRs:
     - 10.66.10.0/24
     - 10.66.20.0/24
     - 10.66.30.0/24
 version: 1
EOF
```

После создания ресурса ModuleConfig `virtualization` дождитесь выполнения заданий из очереди:

```shell
d8 k -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue main

# Queue 'main': length 0, status: 'waiting for task 1m1s'
```

Если все выполнено правильно, после включения модуля появится Namespase `d8-virtualization`:

```bash
d8 k get ns d8-virtualization

# NAME                STATUS   AGE
# d8-virtualization   Active   1h
```
