---
title: Установка DKP в закрытом окружении
permalink: ru/guides/private-environment.html
description: Руководство по установке Deckhouse Kubernetes Platform в закрытом окружении
lang: ru
layout: sidebar-guides
---

В этом руководстве описано, как развернуть кластер Deckhouse Kubernetes Platform в закрытом окружении без прямого доступа к хранилищу образов контейнеров DKP (`registry.deckhouse.ru`) и внешним репозиториям deb/rpm-пакетов, используемых на узлах [поддерживаемых операционных систем](../documentation/v1/reference/supported_versions.html#linux).

{% alert level="warning" %}
Обратите внимание, что установка DKP в закрытом окружении доступна в следующих редакциях: SE, SE+, EE, CSE Lite, CSE Pro.
{% endalert %}

## Особенности закрытого окружения

Установка в закрытом окружении практически не отличается от установки [на bare metal](../gs/bm/step2.html).

Ключевые особенности:

* Параметры прокси-сервера, задаваемые [в конфигурации кластера](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-proxy) при установке, автоматически транслируются в переменные окружения `HTTP_PROXY`, `HTTPS_PROXY` и `NO_PROXY` для **узлов кластера и компонентов DKP**.
  Пользовательские приложения (поды) не получают эти переменные из конфигурации кластера автоматически. Чтобы обеспечить им доступ в Интернет через прокси, необходимо явно задать переменные окружения (`HTTP_PROXY`, `HTTPS_PROXY` и, при необходимости, `NO_PROXY`) в манифестах. В зависимости от корпоративных политик доступ для приложений может быть организован и другими способами — например, через открытие прямого доступа для узлов.
* container registry с образами контейнеров DKP разворачивается отдельно с доступом изнутри контура, а в кластере настраивается его использование и необходимые права доступа.

Взаимодействие с узлами кластера, как правило, осуществляется через отдельный физический сервер или виртуальную машину — bastion-хост. Прокси для доступа к внешним ресурсам из внутреннего контура разворачивается в соответствии с сетевой политикой и архитектурой инфраструктуры. В зависимости от требований он может быть размещён как на bastion-хосте, так и на отдельной машине.
Приватный container registry рекомендуется размещать на отдельной виртуальной машине или сервере во внутренней сети. Совмещение registry с bastion-хостом в продуктивных средах не рекомендуется. Исключение могут составлять учебные или упрощённые стенды для ограниченных задач.

{% alert level="info" %}
В зависимости от принятых в компании политик безопасности доступ к внешним ресурсам может отсутствовать полностью. В таких случаях прокси-сервер для выхода во внешние сети не используется. Необходимые внешние зависимости (например, архив с образами контейнеров DKP) доставляются в контур на требуемую виртуальную машину любым разрешённым способом — например, с использованием съёмных носителей.
{% endalert %}

Общая схема закрытого окружения:

<img src="/images/guides/install_to_private_environment/private_environment-scheme_ru.png" alt="Схема развертывания Deckhouse Kubernetes Platform в закрытом окружении">

{% alert level="info" %}
На схеме также показан внутренний репозиторий пакетов ОС. Он используется для установки пакетов на узлы в случае, если доступ к официальным репозиториям отсутствует даже через прокси.
Во многих закрытых контурах уже развернуты собственные репозитории пакетов ОС, и установка выполняется из них — в этом случае прокси-сервер для работы с пакетами не требуется.
Прокси-сервер используется для других типов трафика:
- загрузка образов контейнеров с публичного registry DKP на bastion-хост;
- обращения компонентов DKP и узлов к внешним ресурсам (если такие обращения разрешены политикой безопасности);
- при необходимости — доступ приложений в подах к внешним сервисам.
{% endalert %}

## Выбор инфраструктуры

В данном руководстве описывается развёртывание в закрытом окружении кластера, состоящего из одного master-узла и одного worker-узла.

Для выполнения работ потребуются:

- персональный компьютер, с которого будут выполняться операции;
- отдельный физический сервер или виртуальная машина Bastion (bastion-хост);
- отдельный физический сервер или виртуальная машина под container registry;
- при необходимости физический сервер или виртуальная машина под прокси-сервер;
- два физических сервера или две виртуальные машины под узлы кластера.

Требования к серверам:

* **Bastion** — не менее 4 ядер CPU, 8 ГБ ОЗУ, 150 ГБ на быстром диске. Такой объём дискового пространства необходим, поскольку на bastion-хосте временно хранятся все образы DKP, используемые при установке. Перед загрузкой в приватный container registry образы скачиваются с публичного registry DKP на bastion-хост, после чего упаковываются в архивы. Эти операции требуют значительного объёма свободного места.
* **ВМ под приватный registry** — не менее 4 ядер CPU, 8 ГБ ОЗУ и не менее 150 ГБ на быстром диске для хранения образов DKP. Требуемый объём дискового пространства рекомендуется планировать с запасом, ориентируясь на размер бандла после выполнения команды `d8 mirror push`.
* **Узлы кластера** — [ресурсы под будущие узлы кластера](./hardware-requirements.html#выбор-ресурсов-для-узлов) выбираются исходя из требований к планируемой нагрузке. Для примера подойдёт минимально рекомендуемая конфигурация — 4 ядра CPU, 8 ГБ ОЗУ и 60 ГБ на быстром диске (400+ IOPS) на каждый узел.

## Подготовка приватного container registry

{% alert level="warning" %}
DKP поддерживает только Bearer token-схему авторизации в container registry.
{% endalert %}

В качестве приватного container registry можно использовать любой из поддерживаемых. Протестирована и гарантируется работа со следующими container registry — [Nexus](https://github.com/sonatype/nexus-public), [Harbor](https://github.com/goharbor/harbor), [Artifactory](https://jfrog.com/artifactory/), [Docker Registry](https://docs.docker.com/registry/), [Quay](https://quay.io/).

В рамках этого руководства будет для примера использован [Harbor](https://goharbor.io/). Он поддерживает настройку политик и управление доступом на основе ролей (RBAC), выполняет проверку образов на уязвимости и позволяет помечать доверенные артефакты. Harbor входит в состав проектов CNCF.

### Установка Harbor

Установите последнюю версию Harbor [из GitHub-репозитория](https://github.com/goharbor/harbor/releases) проекта. Для этого скачайте архив с установщиком из нужного релиза, выбрав вариант с `harbor-offline-installer` в названии.

<div style="text-align: center;">
<img src="/images/guides/install_to_private_environment/download-harbor-installer.png" alt="Скачивание установщика Harbor...">
</div>

Скопируйте адрес ссылки. Например, для версии `harbor-offline-installer-v2.14.1.tgz` ссылка будет выглядеть следующим образом: `https://github.com/goharbor/harbor/releases/download/v2.14.1/harbor-offline-installer-v2.14.1.tgz`.

Подключитесь по SSH к **виртуальной машине, на которой будет развёрнут Harbor**, и скачайте архив любым удобным способом.
Если у этой ВМ нет прямого доступа в Интернет, скачайте архив на рабочей машине или на bastion-хосте, а затем перенесите его на ВМ с Harbor.

{% offtopic title="Как скачать архив с помощью wget..." %}
Выполните команду (укажите актуальную ссылку):

```console
wget https://github.com/goharbor/harbor/releases/download/v2.14.1/harbor-offline-installer-v2.14.1.tgz
```

{% endofftopic %}

{% offtopic title="Как скачать архив с помощью curl..." %}
Выполните команду (укажите актуальную ссылку):

```console
curl -O https://github.com/goharbor/harbor/releases/download/v2.14.1/harbor-offline-installer-v2.14.1.tgz
```

{% endofftopic %}

Распакуйте скачанный архив (укажите имя архива):

```console
tar -zxf ./harbor-offline-installer-v2.14.1.tgz
```

В полученной директории `harbor` расположены файлы, необходимые для установки.

Установите на **эту же ВМ** [Docker](https://docs.docker.com/engine/install/) и плагин [Docker Compose](https://docs.docker.com/compose/install/#plugin-linux-only). Они потребуются для настройки доступа к registry по TLS, а также для запуска установщика Harbor.

Перед развёртыванием хранилища сгенерируйте самоподписанный (self-signed) TLS-сертификат.

{% alert level="info" %}
Из-за ограничений доступа в закрытом окружении невозможно получить сертификаты, например, от Let's Encrypt, так как сервис не сможет выполнить проверку, необходимую для выдачи сертификата.

Существует несколько способов генерации сертификатов. В этом руководстве приведён один из вариантов. При необходимости используйте другой подходящий способ или разместите уже имеющийся сертификат.
{% endalert %}

Создайте директорию `certs` в директории `harbor`:

```bash
cd harbor/
mkdir certs
```

Перейдите в созданную директорию и сгенерируйте сертификаты для внешнего доступа следующими командами:

```bash
openssl genrsa -out ca.key 4096
```

```bash
openssl req -x509 -new -nodes -sha512 -days 3650 -subj "/C=RU/ST=Moscow/L=Moscow/O=example/OU=Personal/CN=myca.local" -key ca.key -out ca.crt
```

Сгенерируйте сертификаты для внутреннего доменного имени `harbor.example`, чтобы внутри приватной сети обращаться к ВМ с Harbor по защищённому соединению.

{% alert level="warning" %}
В приведённых ниже командах замените `<INTERNAL_IP_ADDRESS>` на внутренний IP-адрес виртуальной машины с Harbor. Этот адрес используется узлами кластера и другими сервисами для доступа к container registry из закрытого контура.
{% endalert %}

```bash
openssl genrsa -out harbor.example.key 4096
```

```bash
openssl req -sha512 -new -subj "/C=RU/ST=Moscow/L=Moscow/O=example/OU=Personal/CN=harbor.example" -key harbor.example.key -out harbor.example.csr
```

```bash
cat > v3.ext <<-EOF
authorityKeyIdentifier=keyid, issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
IP.1=<INTERNAL_IP_ADDRESS>
DNS.1=harbor.example
EOF
```

```bash
openssl x509 -req -sha512 -days 3650 -extfile v3.ext -CA ca.crt -CAkey ca.key -CAcreateserial -in harbor.example.csr -out harbor.example.crt
```

```bash
openssl x509 -inform PEM -in harbor.example.crt -out harbor.example.cert
```

Проверьте, что все сертификаты созданы успешно:

```bash
ls -la
```

{% offtopic title="Пример вывода команды..." %}

```bash
$ ls -la
total 40
drwxrwxr-x 2 ubuntu ubuntu 4096 Dec  5 14:58 .
drwxrwxr-x 3 ubuntu ubuntu 4096 Dec  4 12:53 ..
-rw-rw-r-- 1 ubuntu ubuntu 2037 Dec  5 14:57 ca.crt
-rw------- 1 ubuntu ubuntu 3272 Dec  5 14:57 ca.key
-rw-rw-r-- 1 ubuntu ubuntu   41 Dec  5 14:58 ca.srl
-rw-rw-r-- 1 ubuntu ubuntu 2122 Dec  5 14:58 harbor.example.cert
-rw-rw-r-- 1 ubuntu ubuntu 2122 Dec  5 14:58 harbor.example.crt
-rw-rw-r-- 1 ubuntu ubuntu 1704 Dec  5 14:57 harbor.example.csr
-rw------- 1 ubuntu ubuntu 3268 Dec  5 14:57 harbor.example.key
-rw-rw-r-- 1 ubuntu ubuntu  247 Dec  5 14:58 v3.ext
```

{% endofftopic %}

Далее настройте Docker для работы с приватным container registry, доступ к которому выполняется по TLS. Для этого создайте директорию `harbor.example` в `/etc/docker/certs.d/`:

```bash
sudo mkdir -p /etc/docker/certs.d/harbor.example
```

> Параметр `-p` указывает утилите `mkdir` создать родительские директории, если они отсутствуют (в данном случае — директорию `certs.d`).

Скопируйте в неё созданные сертификаты:

```bash
cp ca.crt /etc/docker/certs.d/harbor.example/
cp harbor.example.cert /etc/docker/certs.d/harbor.example/
cp harbor.example.key /etc/docker/certs.d/harbor.example/
```

Эти сертификаты будут использоваться при обращении к registry по доменному имени `harbor.example`.

Скопируйте шаблон конфигурационного файла, который поставляется вместе с установщиком:

```bash
cp harbor.yml.tmpl harbor.yml
```

Измените в `harbor.yml` следующие параметры:

* `hostname` — укажите `harbor.example` (для него генерировались сертификаты);
* `certificate` — укажите путь к сгенерированному сертификату в директории `certs` (например, `/home/ubuntu/harbor/certs/harbor.example.crt`);
* `private_key` — укажите путь к приватному ключу (например, `/home/ubuntu/harbor/certs/harbor.example.key`);
* `harbor_admin_password` — задайте пароль для доступа в веб-интерфейс.

Сохраните файл.

{% offtopic title="Пример конфигурационного файла..." %}

```yaml
# Configuration file of Harbor

# The IP address or hostname to access admin UI and registry service.
# DO NOT use localhost or 127.0.0.1, because Harbor needs to be accessed by external clients.
hostname: harbor.example

# http related config
http:
  # port for http, default is 80. If https enabled, this port will redirect to https port
  port: 80

# https related config
https:
  # https port for harbor, default is 443
  port: 443
  # The path of cert and key files for nginx
  certificate: /home/ubuntu/harbor/certs/harbor.example.crt
  private_key: /home/ubuntu/harbor/certs/harbor.example.key
  # enable strong ssl ciphers (default: false)
  # strong_ssl_ciphers: false

# # Harbor will set ipv4 enabled only by default if this block is not configured
# # Otherwise, please uncomment this block to configure your own ip_family stacks
# ip_family:
#   # ipv6Enabled set to true if ipv6 is enabled in docker network, currently it affected the nginx related component
#   ipv6:
#     enabled: false
#   # ipv4Enabled set to true by default, currently it affected the nginx related component
#   ipv4:
#     enabled: true

# # Uncomment following will enable tls communication between all harbor components
# internal_tls:
#   # set enabled to true means internal tls is enabled
#   enabled: true
#   # put your cert and key files on dir
#   dir: /etc/harbor/tls/internal


# Uncomment external_url if you want to enable external proxy
# And when it enabled the hostname will no longer used
# external_url: https://reg.mydomain.com:8433

# The initial password of Harbor admin
# It only works in first time to install harbor
# Remember Change the admin password from UI after launching Harbor.
harbor_admin_password: Flant12345

# Harbor DB configuration
database:
  # The password for the user('postgres' by default) of Harbor DB. Change this before any production use.
  password: root123
  # The maximum number of connections in the idle connection pool. If it <=0, no idle connections are retained.
  max_idle_conns: 100
  # The maximum number of open connections to the database. If it <= 0, then there is no limit on the number of open connections.
  # Note: the default number of connections is 1024 for postgres of harbor.
  max_open_conns: 900
  # The maximum amount of time a connection may be reused. Expired connections may be closed lazily before reuse. If it <= 0, connections are not closed due to a connection's age.
  # The value is a duration string. A duration string is a possibly signed sequence of decimal numbers, each with optional fraction and a unit suffix, such as "300ms", "-1.5h" or "2h45m". Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".
  conn_max_lifetime: 5m
  # The maximum amount of time a connection may be idle. Expired connections may be closed lazily before reuse. If it <= 0, connections are not closed due to a connection's idle time.
  # The value is a duration string. A duration string is a possibly signed sequence of decimal numbers, each with optional fraction and a unit suffix, such as "300ms", "-1.5h" or "2h45m". Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".
  conn_max_idle_time: 0

# The default data volume
data_volume: /data

# Harbor Storage settings by default is using /data dir on local filesystem
# Uncomment storage_service setting If you want to using external storage
# storage_service:
#   # ca_bundle is the path to the custom root ca certificate, which will be injected into the truststore
#   # of registry's containers.  This is usually needed when the user hosts a internal storage with self signed certificate.
#   ca_bundle:

#   # storage backend, default is filesystem, options include filesystem, azure, gcs, s3, swift and oss
#   # for more info about this configuration please refer https://distribution.github.io/distribution/about/configuration/
#   # and https://distribution.github.io/distribution/storage-drivers/
#   filesystem:
#     maxthreads: 100
#   # set disable to true when you want to disable registry redirect
#   redirect:
#     disable: false

# Trivy configuration
#
# Trivy DB contains vulnerability information from NVD, Red Hat, and many other upstream vulnerability databases.
# It is downloaded by Trivy from the GitHub release page https://github.com/aquasecurity/trivy-db/releases and cached
# in the local file system. In addition, the database contains the update timestamp so Trivy can detect whether it
# should download a newer version from the Internet or use the cached one. Currently, the database is updated every
# 12 hours and published as a new release to GitHub.
trivy:
  # ignoreUnfixed The flag to display only fixed vulnerabilities
  ignore_unfixed: false
  # skipUpdate The flag to enable or disable Trivy DB downloads from GitHub
  #
  # You might want to enable this flag in test or CI/CD environments to avoid GitHub rate limiting issues.
  # If the flag is enabled you have to download the `trivy-offline.tar.gz` archive manually, extract `trivy.db` and
  # `metadata.json` files and mount them in the `/home/scanner/.cache/trivy/db` path.
  skip_update: false
  #
  # skipJavaDBUpdate If the flag is enabled you have to manually download the `trivy-java.db` file and mount it in the
  # `/home/scanner/.cache/trivy/java-db/trivy-java.db` path
  skip_java_db_update: false
  #
  # The offline_scan option prevents Trivy from sending API requests to identify dependencies.
  # Scanning JAR files and pom.xml may require Internet access for better detection, but this option tries to avoid it.
  # For example, the offline mode will not try to resolve transitive dependencies in pom.xml when the dependency doesn't
  # exist in the local repositories. It means a number of detected vulnerabilities might be fewer in offline mode.
  # It would work if all the dependencies are in local.
  # This option doesn't affect DB download. You need to specify "skip-update" as well as "offline-scan" in an air-gapped environment.
  offline_scan: false
  #
  # Comma-separated list of what security issues to detect. Possible values are `vuln`, `config` and `secret`. Defaults to `vuln`.
  security_check: vuln
  #
  # insecure The flag to skip verifying registry certificate
  insecure: false
  #
  # timeout The duration to wait for scan completion.
  # There is upper bound of 30 minutes defined in scan job. So if this `timeout` is larger than 30m0s, it will also timeout at 30m0s.
  timeout: 5m0s
  #
  # github_token The GitHub access token to download Trivy DB
  #
  # Anonymous downloads from GitHub are subject to the limit of 60 requests per hour. Normally such rate limit is enough
  # for production operations. If, for any reason, it's not enough, you could increase the rate limit to 5000
  # requests per hour by specifying the GitHub access token. For more details on GitHub rate limiting please consult
  # https://docs.github.com/rest/overview/resources-in-the-rest-api#rate-limiting
  #
  # You can create a GitHub token by following the instructions in
  # https://help.github.com/en/github/authenticating-to-github/creating-a-personal-access-token-for-the-command-line
  #
  # github_token: xxx

jobservice:
  # Maximum number of job workers in job service
  max_job_workers: 10
  # Maximum hours of task duration in job service, default 24
  max_job_duration_hours: 24
  # The jobLoggers backend name, only support "STD_OUTPUT", "FILE" and/or "DB"
  job_loggers:
    - STD_OUTPUT
    - FILE
    # - DB
  # The jobLogger sweeper duration (ignored if `jobLogger` is `stdout`)
  logger_sweeper_duration: 1 #days

notification:
  # Maximum retry count for webhook job
  webhook_job_max_retry: 3
  # HTTP client timeout for webhook job
  webhook_job_http_client_timeout: 3 #seconds

# Log configurations
log:
  # options are debug, info, warning, error, fatal
  level: info
  # configs for logs in local storage
  local:
    # Log files are rotated log_rotate_count times before being removed. If count is 0, old versions are removed rather than rotated.
    rotate_count: 50
    # Log files are rotated only if they grow bigger than log_rotate_size bytes. If size is followed by k, the size is assumed to be in kilobytes.
    # If the M is used, the size is in megabytes, and if G is used, the size is in gigabytes. So size 100, size 100k, size 100M and size 100G
    # are all valid.
    rotate_size: 200M
    # The directory on your host that store log
    location: /var/log/harbor

  # Uncomment following lines to enable external syslog endpoint.
  # external_endpoint:
  #   # protocol used to transmit log to external endpoint, options is tcp or udp
  #   protocol: tcp
  #   # The host of external endpoint
  #   host: localhost
  #   # Port of external endpoint
  #   port: 5140

#This attribute is for migrator to detect the version of the .cfg file, DO NOT MODIFY!
_version: 2.14.0

# Uncomment external_database if using external database.
# external_database:
#   harbor:
#     host: harbor_db_host
#     port: harbor_db_port
#     db_name: harbor_db_name
#     username: harbor_db_username
#     password: harbor_db_password
#     ssl_mode: disable
#     max_idle_conns: 2
#     max_open_conns: 0

# Uncomment redis if need to customize redis db
# redis:
#   # db_index 0 is for core, it's unchangeable
#   # registry_db_index: 1
#   # jobservice_db_index: 2
#   # trivy_db_index: 5
#   # it's optional, the db for harbor business misc, by default is 0, uncomment it if you want to change it.
#   # harbor_db_index: 6
#   # it's optional, the db for harbor cache layer, by default is 0, uncomment it if you want to change it.
#   # cache_layer_db_index: 7

# Uncomment external_redis if using external Redis server
# external_redis:
#   # support redis, redis+sentinel
#   # host for redis: <host_redis>:<port_redis>
#   # host for redis+sentinel:
#   #  <host_sentinel1>:<port_sentinel1>,<host_sentinel2>:<port_sentinel2>,<host_sentinel3>:<port_sentinel3>
#   host: redis:6379
#   password:
#   # Redis AUTH command was extended in Redis 6, it is possible to use it in the two-arguments AUTH <username> <password> form.
#   # there's a known issue when using external redis username ref:https://github.com/goharbor/harbor/issues/18892
#   # if you care about the image pull/push performance, please refer to this https://github.com/goharbor/harbor/wiki/Harbor-FAQs#external-redis-username-password-usage
#   # username:
#   # sentinel_master_set must be set to support redis+sentinel
#   #sentinel_master_set:
#   # tls configuration for redis connection
#   # only server-authentication is supported
#   # mtls for redis connection is not supported
#   # tls connection will be disable by default
#   tlsOptions:
#     enable: false
#   # if it is a self-signed ca, please set the ca path specifically.
#     rootCA:
#   # db_index 0 is for core, it's unchangeable
#   registry_db_index: 1
#   jobservice_db_index: 2
#   trivy_db_index: 5
#   idle_timeout_seconds: 30
#   # it's optional, the db for harbor business misc, by default is 0, uncomment it if you want to change it.
#   # harbor_db_index: 6
#   # it's optional, the db for harbor cache layer, by default is 0, uncomment it if you want to change it.
#   # cache_layer_db_index: 7

# Uncomment uaa for trusting the certificate of uaa instance that is hosted via self-signed cert.
# uaa:
#   ca_file: /path/to/ca

# Global proxy
# Config http proxy for components, e.g. http://my.proxy.com:3128
# Components doesn't need to connect to each others via http proxy.
# Remove component from `components` array if want disable proxy
# for it. If you want use proxy for replication, MUST enable proxy
# for core and jobservice, and set `http_proxy` and `https_proxy`.
# Add domain to the `no_proxy` field, when you want disable proxy
# for some special registry.
proxy:
  http_proxy:
  https_proxy:
  no_proxy:
  components:
    - core
    - jobservice
    - trivy

# metric:
#   enabled: false
#   port: 9090
#   path: /metrics

# Trace related config
# only can enable one trace provider(jaeger or otel) at the same time,
# and when using jaeger as provider, can only enable it with agent mode or collector mode.
# if using jaeger collector mode, uncomment endpoint and uncomment username, password if needed
# if using jaeger agent mode uncomment agent_host and agent_port
# trace:
#   enabled: true
#   # set sample_rate to 1 if you wanna sampling 100% of trace data; set 0.5 if you wanna sampling 50% of trace data, and so forth
#   sample_rate: 1
#   # # namespace used to differentiate different harbor services
#   # namespace:
#   # # attributes is a key value dict contains user defined attributes used to initialize trace provider
#   # attributes:
#   #   application: harbor
#   # # jaeger should be 1.26 or newer.
#   # jaeger:
#   #   endpoint: http://hostname:14268/api/traces
#   #   username:
#   #   password:
#   #   agent_host: hostname
#   #   # export trace data by jaeger.thrift in compact mode
#   #   agent_port: 6831
#   # otel:
#   #   endpoint: hostname:4318
#   #   url_path: /v1/traces
#   #   compression: false
#   #   insecure: true
#   #   # timeout is in seconds
#   #   timeout: 10

# Enable purge _upload directories
upload_purging:
  enabled: true
  # remove files in _upload directories which exist for a period of time, default is one week.
  age: 168h
  # the interval of the purge operations
  interval: 24h
  dryrun: false

# Cache layer configurations
# If this feature enabled, harbor will cache the resource
# `project/project_metadata/repository/artifact/manifest` in the redis
# which can especially help to improve the performance of high concurrent
# manifest pulling.
# NOTICE
# If you are deploying Harbor in HA mode, make sure that all the harbor
# instances have the same behaviour, all with caching enabled or disabled,
# otherwise it can lead to potential data inconsistency.
cache:
  # not enabled by default
  enabled: false
  # keep cache for one day by default
  expire_hours: 24

# Harbor core configurations
# Uncomment to enable the following harbor core related configuration items.
# core:
#   # The provider for updating project quota(usage), there are 2 options, redis or db,
#   # by default is implemented by db but you can switch the updation via redis which
#   # can improve the performance of high concurrent pushing to the same project,
#   # and reduce the database connections spike and occupies.
#   # By redis will bring up some delay for quota usage updation for display, so only
#   # suggest switch provider to redis if you were ran into the db connections spike around
#   # the scenario of high concurrent pushing to same project, no improvement for other scenes.
#   quota_update_provider: redis # Or db
```

{% endofftopic %}

Запустите скрипт установки:

```bash
./install.sh
```

Начнётся установка Harbor — будут подготовлены необходимые образы и запущены контейнеры.

{% offtopic title="Лог успешной установки..." %}

```console
...
[Step 5]: starting Harbor ...
[+] up 10/10
 ✔ Network harbor_harbor       Created 0.0s 
 ✔ Container harbor-log        Created 0.1s 
 ✔ Container registry          Created 0.1s 
 ✔ Container harbor-portal     Created 0.2s 
 ✔ Container redis             Created 0.1s 
 ✔ Container harbor-db         Created 0.1s 
 ✔ Container registryctl       Created 0.2s 
 ✔ Container harbor-core       Created 0.1s 
 ✔ Container nginx             Created 0.1s 
 ✔ Container harbor-jobservice Created 0.1s 
✔ ----Harbor has been installed and started successfully.----

```

{% endofftopic %}

Проверьте, что Harbor успешно запущен:

```bash
docker ps
```

{% offtopic title="Пример вывода команды..." %}

```console
CONTAINER ID   IMAGE                                 COMMAND                  CREATED         STATUS                   PORTS                                                                                NAMES
df1636bd1295   goharbor/nginx-photon:v2.14.1         "nginx -g 'daemon of…"   3 minutes ago   Up 3 minutes (healthy)   0.0.0.0:80->8080/tcp, [::]:80->8080/tcp, 0.0.0.0:443->8443/tcp, [::]:443->8443/tcp   nginx
15fe1abdf9b1   goharbor/harbor-jobservice:v2.14.1    "/harbor/entrypoint.…"   3 minutes ago   Up 3 minutes (healthy)                                                                                        harbor-jobservice
9b006f03821e   goharbor/harbor-core:v2.14.1          "/harbor/entrypoint.…"   3 minutes ago   Up 3 minutes (healthy)                                                                                        harbor-core
fbd35346573e   goharbor/registry-photon:v2.14.1      "/home/harbor/entryp…"   3 minutes ago   Up 3 minutes (healthy)                                                                                        registry
c199a232fdb6   goharbor/harbor-registryctl:v2.14.1   "/home/harbor/start.…"   3 minutes ago   Up 3 minutes (healthy)                                                                                        registryctl
a78d9a1a5b0b   goharbor/harbor-db:v2.14.1            "/docker-entrypoint.…"   3 minutes ago   Up 3 minutes (healthy)                                                                                        harbor-db
89d6c922b78a   goharbor/harbor-portal:v2.14.1        "nginx -g 'daemon of…"   3 minutes ago   Up 3 minutes (healthy)                                                                                        harbor-portal
ef18d7f24777   goharbor/redis-photon:v2.14.1         "redis-server /etc/r…"   3 minutes ago   Up 3 minutes (healthy)                                                                                        redis
9330bcce48be   goharbor/harbor-log:v2.14.1           "/bin/sh -c /usr/loc…"   3 minutes ago   Up 3 minutes (healthy)   127.0.0.1:1514->10514/tcp                                                            harbor-log
```

{% endofftopic %}

Добавьте в файл `/etc/hosts` на ВМ с Harbor ассоциацию доменного имени `harbor.example` с `localhost`, чтобы можно было обращаться к Harbor по этому имени с этой же машины:

```bash
127.0.0.1 localhost harbor.example
```

{% alert level="warning" %}
В некоторых облачных провайдерах (например, Yandex Cloud) изменения в `/etc/hosts` могут быть сброшены после перезагрузки виртуальной машины. Сообщение об этом обычно указано в начале файла `/etc/hosts`.

```text
# Your system has configured 'manage_etc_hosts' as True.
# As a result, if you wish for changes to this file to persist
# then you will need to either
# a.) make changes to the master file in /etc/cloud/templates/hosts.debian.tmpl
# b.) change or remove the value of 'manage_etc_hosts' in
#     /etc/cloud/cloud.cfg or cloud-config from user-data
```

Если у вашего провайдера действует такая схема, внесите соответствующие изменения также в файл шаблона, указанный в комментарии, чтобы настройки сохранялись после перезагрузки.
{% endalert %}

На этом установка Harbor завершена! 🎉

### Настройка Harbor

Создайте проект и пользователя, от имени которого будет выполняться работа с этим проектом.

Откройте веб-интерфейс Harbor по адресу `harbor.example`. Обратите внимание — доступ к этому интерфейсу из внешней сети закрыт, подключение возможно только с узла, имеющего доступ к внутреннему контуру.

<div style="text-align: center;">
<img src="/images/guides/install_to_private_environment/harbor_main_page_ru.png" alt="Главная страница Harbor...">
</div>

{% alert level="info" %}
Чтобы открыть Harbor по доменному имени `harbor.example` с рабочего компьютера, добавьте соответствующую запись в файл `/etc/hosts`, указав внутренний IP-адрес ВМ с Harbor.
{% endalert %}

Для входа в интерфейс воспользуйтесь логином и паролем, указанными в конфигурационном файле `harbor.yml`.

<div style="text-align: center;">
<img src="/images/guides/install_to_private_environment/harbor_main_dashboard_ru.png" alt="Главная страница Harbor...">
</div>

{% alert level="info" %}
Браузер может предупреждать о самоподписанном сертификате и считать соединение «небезопасным». Для закрытого окружения это ожидаемо и допустимо. При необходимости добавьте сертификат в хранилище доверенных сертификатов браузера или операционной системы, чтобы убрать предупреждения.
{% endalert %}

Создайте новый проект. Для этого нажмите на кнопку «Новый проект» и введите его название: `deckhouse`. Остальные настройки оставьте без изменений.

<div style="text-align: center;">
<img src="/images/guides/install_to_private_environment/harbor_new_project_ru.png" alt="Главная страница Harbor...">
</div>

Создайте [robot-account](https://goharbor.io/docs/1.10/working-with-projects/project-configuration/create-robot-accounts/) для этого проекта. Это специальный тип учетной записи, привязанный к проекту и предназначенный для автоматизации операций. Такой аккаунт не имеет доступа к веб-интерфейсу и используется исключительно для работы через Docker CLI или Helm CLI.

Перейдите в созданный проект и откройте вкладку «Аккаунты роботов». Нажмите кнопку «Создать новый аккаунт робота»:

<div style="text-align: center;">
<img src="/images/guides/install_to_private_environment/harbor_robot_account_ru.png" alt="Главная страница Harbor...">
</div>

Укажите имя аккаунта, краткое описание (если нужно) и срок жизни (можно выбрать в днях или поставить бессрочный):

<div style="text-align: center;">
<img src="/images/guides/install_to_private_environment/harbor_create_robot_account_ru.png" alt="Главная страница Harbor...">
</div>

Для обеспечения корректной работы необходимо предоставить аккаунту полный доступ в разделе «Repository». Остальные параметры можно настроить по желанию либо в соответствии с требованиями ИБ.

<div style="text-align: center;">
<img src="/images/guides/install_to_private_environment/harbor_robot_permissions_ru.png" alt="Главная страница Harbor...">
</div>

После создания аккаунта Harbor покажет секрет доступа (токен).

{% alert level="warning" %}
Сохраните секрет доступа сразу — позже Harbor больше не отобразит его, и получить его повторно будет невозможно.
{% endalert %}

<div style="text-align: center;">
<img src="/images/guides/install_to_private_environment/harbor_robot_created_ru.png" alt="Главная страница Harbor...">
</div>

На этом настройка Harbor завершена! 🎉

## Копирование образов DKP в приватный container registry

Следующим шагом необходимо скопировать образы компонентов DKP из публичного registry Deckhouse Kubernetes Platform в Harbor.

{% alert level="info" %}
Для дальнейших действий в этом разделе потребуется утилита Deckhouse CLI. Установите её на тот хост, с которого будут выполняться работы по переносу образов DKP в приватный registry. Для примера из этого руководства это bastion-хост. Инструкция по установке CLI — [в документации](../documentation/v1/cli/d8/#как-установить-deckhouse-cli).
{% endalert %}

{% alert level="warning" %}
Загрузка образов занимает продолжительное время. Чтобы не потерять прогресс при разрыве SSH-соединения, запускайте команды в сессии `tmux` или `screen`. В случае обрыва соединения вы сможете переподключиться к сессии и продолжить работу, не начиная процесс заново. Обе утилиты обычно доступны в репозиториях дистрибутивов Linux и устанавливаются через пакетный менеджер.

{% offtopic title="Как работать с tmux..." %}
* Запустите сессию командой `tmux`.
* Отсоединитесь от сессии сочетанием клавиш `Ctrl + b`, затем `d`. Сессия продолжит работать, а запущенные в ней процессы не остановятся. Для выхода из сессии используйте `Ctrl + d`.
* Просмотр запущенных сессий осуществляется командой `tmux ls`:

  ```console
  $ tmux ls
  0: 1 windows (created Thu Dec 11 13:52:41 2025)
  ```

* Подключение к запущенной сессии: `tmux attach -t <ИДЕНТИФИКАТОР СЕССИИ>`. Для примера выше `<ИДЕНТИФИКАТОР СЕССИИ>` будет `0`.
{% endofftopic %}

{% offtopic title="Как работать со screen..." %}
* Запустите сессию командой `screen`.
* Отсоединитесь от сессии сочетанием клавиш `Ctrl + a`, затем `d` (не отпуская `Ctrl`). Сессия продолжит работать, а запущенные процессы не остановятся. Для выхода из сессии используйте `Ctrl + d`.
* Просмотр запущенных сессий осуществляется командой `screen -r`:

  ```console
  $ screen -r
  There are several suitable screens on:
          1166154.pts-0.guide-bastion     (12/11/25 14:00:26)     (Detached)
          1165806.pts-0.guide-bastion     (12/11/25 13:59:35)     (Detached)
          1165731.pts-0.guide-bastion     (12/11/25 13:59:24)     (Detached)
          1165253.pts-0.guide-bastion     (12/11/25 13:58:16)     (Detached)
  Type "screen [-d] -r [pid.]tty.host" to resume one of them.
  ```

* Подключение к запущенной сессии: `screen -r <ИДЕНТИФИКАТОР СЕССИИ>`. Для примера выше `<ИДЕНТИФИКАТОР СЕССИИ>` будет `166154.pts-0.guide-bastion`.
{% endofftopic %}
{% endalert %}

Скачайте образы DKP в выделенную директорию, используя команду `d8 mirror pull`.

По умолчанию `d8 mirror pull` скачивает только актуальные версии DKP, базы данных сканера уязвимостей (если они входят в редакцию DKP) и официально поставляемых модулей.

Выполните следующую команду, чтобы скачать образы актуальных версий. Перед запуском подставьте вместо плейсхолдеров свои данные: `<EDITION>`, `<LICENSE_KEY>` и при необходимости путь к директории:

```shell
d8 mirror pull \
  --source='registry.deckhouse.ru/deckhouse/<EDITION>' \
  --license='<LICENSE_KEY>' /home/ubuntu/d8-bundle
```

где:

- `--source` — адрес хранилища образов Deckhouse;
- `<EDITION>` — код редакции Deckhouse Kubernetes Platform (например, `ee`, `se`, `se-plus`). По умолчанию используется значение `ee` (Enterprise Edition), поэтому параметр `--source` можно не указывать;
- `--license` — параметр для указания лицензионного ключа Deckhouse Kubernetes Platform для аутентификации в официальном хранилище образов;
- `<LICENSE_KEY>` — лицензионный ключ Deckhouse Kubernetes Platform;
- `/home/ubuntu/d8-bundle` — директория для размещения загруженных пакетов образов. Если она не существует, будет создана автоматически.

Если загрузка образов будет прервана, повторный вызов команды продолжит загрузку, если с момента её остановки прошло не более суток.

В зависимости от скорости интернет-соединения процесс может занять от 30 до 40 минут.

{% offtopic title="Пример успешного завершения процесса скачивания образов..." %}

Пример вывода успешной загрузки всех компонентов платформы:

```text
Feb 26 17:49:04.520 INFO  ║║ [822 / 824] Pulling registry.deckhouse.ru/deckhouse/ee@sha256:4e5c17098d2a884cc971676fa9a7980f0d784a787d21e113d28a72da96ea8b2b 
Feb 26 17:49:05.099 INFO  ║║ [823 / 824] Pulling registry.deckhouse.ru/deckhouse/ee@sha256:d229564f423a1ca7a59e0be28a71218e362cc8f07d979ce63a15bb505c6ccb40 
Feb 26 17:49:05.555 INFO  ║║ [824 / 824] Pulling registry.deckhouse.ru/deckhouse/ee@sha256:eb5ed1a71783f941addf75a29e9bca9328f9dcca41d70b24b70efd4995eb1ca1 
Feb 26 17:49:06.447 INFO  ║║ All required Deckhouse images are pulled!

```

Пример вывода успешной загрузки модулей:

```text
Feb 26 18:30:18.263 INFO  ║║ Deckhouse modules pulled!
Feb 26 18:30:18.263 INFO  ║╚ Pull images succeeded in 29m58.691782336s
Feb 26 18:30:18.265 INFO  ║ Processing image indexes
Feb 26 18:30:18.313 INFO  ║ Packing module-csi-hpe.tar
Feb 26 18:30:19.205 INFO  ║ Packing module-csi-netapp.tar
Feb 26 18:30:19.342 INFO  ║ Packing module-csi-nfs.tar
Feb 26 18:30:19.496 INFO  ║ Packing module-operator-argo.tar
Feb 26 18:30:19.666 INFO  ║ Packing module-runtime-audit-engine.tar
Feb 26 18:30:21.419 INFO  ║ Packing module-managed-memcached.tar
Feb 26 18:30:21.708 INFO  ║ Packing module-commander.tar
Feb 26 18:30:23.829 INFO  ║ Packing module-csi-s3.tar
Feb 26 18:30:25.199 INFO  ║ Packing module-csi-yadro-tatlin-unified.tar
Feb 26 18:30:25.889 INFO  ║ Packing module-operator-ceph.tar
Feb 26 18:30:27.075 INFO  ║ Packing module-operator-postgres.tar
Feb 26 18:30:28.901 INFO  ║ Packing module-pod-reloader.tar
Feb 26 18:30:28.929 INFO  ║ Packing module-sds-replicated-volume.tar
Feb 26 18:30:35.771 INFO  ║ Packing module-secrets-store-integration.tar
Feb 26 18:30:36.376 INFO  ║ Packing module-snapshot-controller.tar
Feb 26 18:30:36.458 INFO  ║ Packing module-payload-registry.tar
Feb 26 18:30:36.550 INFO  ║ Packing module-prompp.tar
Feb 26 18:30:37.318 INFO  ║ Packing module-code.tar
Feb 26 18:30:49.954 INFO  ║ Packing module-console.tar
Feb 26 18:30:50.489 INFO  ║ Packing module-csi-huawei.tar
Feb 26 18:30:53.224 INFO  ║ Packing module-storage-volume-data-manager.tar
Feb 26 18:30:53.280 INFO  ║ Packing module-operator-trivy.tar
Feb 26 18:30:54.042 INFO  ║ Packing module-sds-node-configurator.tar
Feb 26 18:30:54.249 INFO  ║ Packing module-virtualization.tar
Feb 26 18:30:58.367 INFO  ║ Packing module-commander-agent.tar
Feb 26 18:30:58.401 INFO  ║ Packing module-csi-scsi-generic.tar
Feb 26 18:31:00.045 INFO  ║ Packing module-development-platform.tar
Feb 26 18:31:00.120 INFO  ║ Packing module-sdn.tar
Feb 26 18:31:00.201 INFO  ║ Packing module-static-routing-manager.tar
Feb 26 18:31:00.228 INFO  ║ Packing module-stronghold.tar
Feb 26 18:31:01.160 INFO  ║ Packing module-sds-local-volume.tar
Feb 26 18:31:01.397 INFO  ║ Packing module-observability.tar
Feb 26 18:31:02.749 INFO  ║ Packing module-csi-ceph.tar
Feb 26 18:31:03.565 INFO  ║ Packing module-managed-postgres.tar
Feb 26 18:31:05.368 INFO  ║ Packing module-managed-valkey.tar
Feb 26 18:31:05.595 INFO  ║ Packing module-neuvector.tar
Feb 26 18:31:08.441 INFO  ║ Packing module-observability-platform.tar
Feb 26 18:31:17.443 INFO  ║ Packing module-state-snapshotter.tar
Feb 26 18:31:17.510 INFO  ╚ Pull Modules succeeded in 40m8.735435676s
```

{% endofftopic %}

Проверьте, что все архивы успешно созданы:

```console
$ ls -lh
итого 51G
-rw-rw-r-- 1 zhbert zhbert  4,8K фев 26 17:19 deckhousereleases.yaml
-rw-rw-r-- 1 zhbert zhbert  4,9G фев 26 18:30 module-code.tar
-rw-rw-r-- 1 zhbert zhbert   17M фев 26 18:30 module-commander-agent.tar
-rw-rw-r-- 1 zhbert zhbert 1008M фев 26 18:30 module-commander.tar
-rw-rw-r-- 1 zhbert zhbert  172M фев 26 18:30 module-console.tar
-rw-rw-r-- 1 zhbert zhbert  225M фев 26 18:31 module-csi-ceph.tar
-rw-rw-r-- 1 zhbert zhbert  1,1G фев 26 18:30 module-csi-hpe.tar
-rw-rw-r-- 1 zhbert zhbert  1,1G фев 26 18:30 module-csi-huawei.tar
-rw-rw-r-- 1 zhbert zhbert  170M фев 26 18:30 module-csi-netapp.tar
-rw-rw-r-- 1 zhbert zhbert  188M фев 26 18:30 module-csi-nfs.tar
-rw-rw-r-- 1 zhbert zhbert  555M фев 26 18:30 module-csi-s3.tar
-rw-rw-r-- 1 zhbert zhbert  544M фев 26 18:31 module-csi-scsi-generic.tar
-rw-rw-r-- 1 zhbert zhbert  207M фев 26 18:30 module-csi-yadro-tatlin-unified.tar
-rw-rw-r-- 1 zhbert zhbert   85M фев 26 18:31 module-development-platform.tar
-rw-rw-r-- 1 zhbert zhbert  146M фев 26 18:30 module-managed-memcached.tar
-rw-rw-r-- 1 zhbert zhbert  835M фев 26 18:31 module-managed-postgres.tar
-rw-rw-r-- 1 zhbert zhbert  113M фев 26 18:31 module-managed-valkey.tar
-rw-rw-r-- 1 zhbert zhbert  1,1G фев 26 18:31 module-neuvector.tar
-rw-rw-r-- 1 zhbert zhbert  3,4G фев 26 18:31 module-observability-platform.tar
-rw-rw-r-- 1 zhbert zhbert  600M фев 26 18:31 module-observability.tar
-rw-rw-r-- 1 zhbert zhbert  194M фев 26 18:30 module-operator-argo.tar
-rw-rw-r-- 1 zhbert zhbert  418M фев 26 18:30 module-operator-ceph.tar
-rw-rw-r-- 1 zhbert zhbert  705M фев 26 18:30 module-operator-postgres.tar
-rw-rw-r-- 1 zhbert zhbert  156M фев 26 18:30 module-operator-trivy.tar
-rw-rw-r-- 1 zhbert zhbert   60M фев 26 18:30 module-payload-registry.tar
-rw-rw-r-- 1 zhbert zhbert   15M фев 26 18:30 module-pod-reloader.tar
-rw-rw-r-- 1 zhbert zhbert  183M фев 26 18:30 module-prompp.tar
-rw-rw-r-- 1 zhbert zhbert 1022M фев 26 18:30 module-runtime-audit-engine.tar
-rw-rw-r-- 1 zhbert zhbert   78M фев 26 18:31 module-sdn.tar
-rw-rw-r-- 1 zhbert zhbert  179M фев 26 18:31 module-sds-local-volume.tar
-rw-rw-r-- 1 zhbert zhbert  157M фев 26 18:30 module-sds-node-configurator.tar
-rw-rw-r-- 1 zhbert zhbert  2,8G фев 26 18:30 module-sds-replicated-volume.tar
-rw-rw-r-- 1 zhbert zhbert  157M фев 26 18:30 module-secrets-store-integration.tar
-rw-rw-r-- 1 zhbert zhbert   51M фев 26 18:30 module-snapshot-controller.tar
-rw-rw-r-- 1 zhbert zhbert   37M фев 26 18:31 module-state-snapshotter.tar
-rw-rw-r-- 1 zhbert zhbert   24M фев 26 18:31 module-static-routing-manager.tar
-rw-rw-r-- 1 zhbert zhbert   41M фев 26 18:30 module-storage-volume-data-manager.tar
-rw-rw-r-- 1 zhbert zhbert  177M фев 26 18:31 module-stronghold.tar
-rw-rw-r-- 1 zhbert zhbert  1,5G фев 26 18:30 module-virtualization.tar
-rw-rw-r-- 1 zhbert zhbert   26G фев 26 17:50 platform.tar
-rw-rw-r-- 1 zhbert zhbert  1,3G фев 26 17:51 security.tar
```

Загрузите скачанные образы в приватный registry. В команде подставьте редакцию DKP и учётные данные robot-аккаунта Harbor:

* `<ROBOT_ACCOUNT_NAME>` — имя robot-аккаунта;
* `<PASSWORD>` — токен, выданный при создании robot-аккаунта.

```bash
d8 mirror push $(pwd)/d8-bundle 'harbor.example:443/deckhouse/<РЕДАКЦИЯ_DKP>' --registry-login='robot$<ROBOT_ACCOUNT_NAME>' --registry-password='<PASSWORD>' --tls-skip-verify
```

> Флаг `--tls-skip-verify` указывает утилите доверять сертификату registry и пропустить его проверку.

Архив будет распакован, после чего образы будут загружены в registry. Этот этап обычно выполняется быстрее, чем скачивание, так как работа идёт с локальным архивом. Как правило, он занимает около 15 минут.

{% offtopic title="Пример успешного завершения процесса заливки образов..." %}

```text
Dec 11 18:25:32.350 INFO  ║ Pushing harbor.example:443/deckhouse/ee/modules/virtualization/release
Dec 11 18:25:32.351 INFO  ║ [1 / 7] Pushing image harbor.example:443/deckhouse/ee/modules/virtualization/release:alpha
Dec 11 18:25:32.617 INFO  ║ [2 / 7] Pushing image harbor.example:443/deckhouse/ee/modules/virtualization/release:beta
Dec 11 18:25:32.760 INFO  ║ [3 / 7] Pushing image harbor.example:443/deckhouse/ee/modules/virtualization/release:early-access
Dec 11 18:25:32.895 INFO  ║ [4 / 7] Pushing image harbor.example:443/deckhouse/ee/modules/virtualization/release:rock-solid
Dec 11 18:25:33.081 INFO  ║ [5 / 7] Pushing image harbor.example:443/deckhouse/ee/modules/virtualization/release:stable
Dec 11 18:25:33.142 INFO  ║ [6 / 7] Pushing image harbor.example:443/deckhouse/ee/modules/virtualization/release:v1.1.3
Dec 11 18:25:33.213 INFO  ║ [7 / 7] Pushing image harbor.example:443/deckhouse/ee/modules/virtualization/release:v1.2.2
Dec 11 18:25:33.414 INFO  ║ Pushing module tag for virtualization
Dec 11 18:25:33.837 INFO  ╚ Push module: virtualization succeeded in 43.313801312s
Dec 11 18:25:33.837 INFO   Modules pushed: code, commander-agent, commander, console, csi-ceph, csi-hpe, csi-huawei, csi-netapp, csi-nfs, csi-s3, csi-scsi-generic, csi-yadro-tatlin-unified, development-platform, managed-postgres, neuvector, observability-platform, observability, operator-argo, operator-ceph, operator-postgres,
 payload-registry, pod-reloader, prompp, runtime-audit-engine, sdn, sds-local-volume, sds-node-configurator, sds-replicated-volume, secrets-store-integration, snapshot-controller, state-snapshotter, static-routing-manager, storage-volume-data-manager, stronghold, virtualization
```

{% endofftopic %}

Проверить, что образы загружены, можно в веб-интерфейсе Harbor: откройте проект `deckhouse` в веб-интерфейсе Harbor.

<div style="text-align: center;">
<img src="/images/guides/install_to_private_environment/harbor_state_with_images_ru.png" alt="Главная страница Harbor...">
</div>

Образы загружены и готовы к использованию! 🎉

## Вход в registry для запуска установщика

Выполните вход на хост, с которого будет запускаться установщик (в примере — bastion-хост). На этой машине имя `harbor.example` должно разрешаться в адрес ВМ с Harbor (через запись в `/etc/hosts` или DNS).
На этом же хосте настройте доверие Docker к TLS-реестру аналогично разделу про Harbor: создайте каталог `/etc/docker/certs.d/harbor.example/` и разместите в нём необходимые сертификаты. Их можно скопировать с ВМ с Harbor либо подготовить заново.

Выполните вход в registry Harbor, чтобы Docker получил доступ к образу установщика [dhctl](../documentation/v1/installing/):

```bash
docker login harbor.example
```

{% offtopic title="Пример успешного выполнения команды..." %}

```text
$ docker login harbor.example
Username: deckhouse
Password: 

WARNING! Your credentials are stored unencrypted in '/home/ubuntu/.docker/config.json'.
Configure a credential helper to remove this warning. See
https://docs.docker.com/go/credential-store/

Login Succeeded
```

{% endofftopic %}

## Подготовка ВМ для будущих узлов

### Требования к ВМ

{% alert level="warning" %}
Во время установки в качестве container runtime по умолчанию на узлах кластера используется `ContainerdV2`.
Для его работы узлы должны соответствовать следующим требованиям:

- поддержка `CgroupsV2`;
- systemd версии `244`;
- поддержка модуля ядра `erofs`.

Некоторые дистрибутивы (например, Astra Linux 1.7.4) не соответствуют этим требованиям, и ОС на узлах необходимо привести в соответствие требованиям перед установкой Deckhouse Kubernetes Platform. Подробнее — [в документации](../documentation/v1/reference/api/cr.html#clusterconfiguration-defaultcri).
{% endalert %}

Серверы для будущих узлов кластера должны соответствовать следующим требованиям:

- не менее 4 ядер CPU;
- не менее 8 ГБ RAM;
- не менее 60 ГБ дискового пространства на быстром диске (400+ IOPS);
- [поддерживаемая ОС](../documentation/v1/reference/supported_versions.html#linux);
- ядро Linux версии `5.8` или новее;
- **уникальный hostname** в пределах всех серверов кластера (физических серверов и виртуальных машин);
- наличие одного из пакетных менеджеров (`apt`/`apt-get`, `yum` или `rpm`).

  **Важно:** в РЕД ОС по умолчанию могут отсутствовать `yum` и `which`, поэтому их необходимо заранее установить;
- установленный Python;
- доступ к проксирующему registry или к приватному хранилищу образов контейнеров с образами Deckhouse;
- доступ к стандартным для используемой ОС репозиториям пакетов (через прокси-сервер или до внутреннего сервера-репозитория пакетов);
- SSH-доступ от сервера Bastion по ключу;
- сетевой доступ от сервера Bastion по порту <code>22/TCP</code>;
- на узле не должно быть установлено пакетов container runtime, например containerd или Docker.

{% alert level="warning" %}
Для правильного выбора ресурсов серверов ознакомьтесь с [рекомендациями по подготовке к production](/products/kubernetes-platform/guides/production.html) и [инструкцией](/products/kubernetes-platform/guides/hardware-requirements.html) по выбору типов и количества узлов кластера, а также ресурсов для них, в зависимости от ваших требований к эксплуатации будущего кластера.
{% endalert %}

### Сопоставление `harbor.example` с адресом ВМ с Harbor

Чтобы серверы, на которых будут разворачиваться master и worker-узлы, могли получить доступ к приватному registry, настройте на них соответствие доменного имени `harbor.example` внутреннему IP-адресу ВМ с Harbor в приватной сети.

Для этого по очереди подключитесь к каждому серверу и добавьте запись в `/etc/hosts` (а при необходимости также в облачный шаблон, если провайдер управляет этим файлом).

{% offtopic title="Как подключиться к серверу без внешнего доступа..." %}
Для SSH-подключения к серверу без внешнего доступа можно использовать Bastion как jump-хост.

Доступны два способа подключения:

1. *Подключение через jump-хост.* Выполните команду:

   ```bash
   ssh -J ubuntu@<BASTION_IP> ubuntu@<NODE_IP>
   ```

   В этом режиме сначала выполняется подключение к серверу Bastion, затем через него к целевому серверу с использованием того же SSH-ключа.
1. *Подключение в режиме агента.* Подключитесь к серверу Bastion командой:

   ```bash
   ssh -A ubuntu@<BASTION_IP>
   ```

   > Обратите внимание: для успешного выполнения команды может понадобиться предварительно запустить ssh-agent, выполнив команду `ssh-add` на том компьютере, с которого будет запускаться команда.

   После этого выполните подключение к целевым серверам:

   ```bash
   ssh ubuntu@<NODE_IP>
   ```

{% endofftopic %}

```console
<INTERNAL-IP-ADDRESS> harbor.example proxy.local
```

> Не забудьте заменить `<INTERNAL-IP-ADDRESS>` на реальный внутренний IP-адрес ВМ с Harbor.

### Создание пользователя для master-узла

Для выполнения установки DKP создайте на будущем master-узле пользователя, под которым будет выполняться подключение и установка платформы.

Выполните команды от `root` (подставьте публичную часть своего SSH-ключа):

```console
useradd deckhouse -m -s /bin/bash -G sudo
echo 'deckhouse ALL=(ALL) NOPASSWD: ALL' | sudo EDITOR='tee -a' visudo
mkdir /home/deckhouse/.ssh
export KEY='ssh-rsa AAAAB3NzaC1yc2EAAAADA...'
echo $KEY >> /home/deckhouse/.ssh/authorized_keys
chown -R deckhouse:deckhouse /home/deckhouse
chmod 700 /home/deckhouse/.ssh
chmod 600 /home/deckhouse/.ssh/authorized_keys
```

{% offtopic title="Как узнать публичную часть ключа..." %}
Узнать публичную часть ключа можно командой `cat ~/.ssh/id_rsa.pub`.
{% endofftopic %}

В результате этих команд:

* создаётся новый пользователь `deckhouse`, который добавляется в группу `sudo`;
* настраиваются права на повышение привилегий без ввода пароля;
* копируется публичная часть ключа, по которому можно будет войти на сервер под этим пользователем.

Проверьте подключение под новым пользователем:

```bash
ssh -J ubuntu@<BASTION_IP> deckhouse@<NODE_IP>
```

Если вход выполнен успешно, пользователь создан корректно.

### Создание пользователя для worker-узла

{% alert level="info" %}
Ниже описана подготовка узла для подключения через CAPS. Если вы предпочитаете добавлять статические узлы вручную с использованием bootstrap-скрипта, этот подраздел и последующие шаги с CAPS можно пропустить: создайте NodeGroup с типом `Static`, получите скрипт из секрета и выполните его на сервере, как описано в [документации (ручной способ)](../documentation/v1/admin/configuration/platform-scaling/node/bare-metal-node.html#добавление-узлов-в-bare-metal-кластере).
{% endalert %}

Сгенерируйте **на master-узле** SSH-ключ с пустой парольной фразой. Для этого выполните на master-узле следующую команду:

```bash
ssh-keygen -t rsa -f /dev/shm/caps-id -C "" -N ""
```

На подготовленном сервере для worker-узла создайте пользователя `caps`. Для этого выполните следующую команду, указав публичную часть SSH-ключа, полученную на предыдущем шаге:

```console
# Укажите публичную часть SSH-ключа пользователя.
export KEY='<SSH-PUBLIC-KEY>'
useradd -m -s /bin/bash caps
usermod -aG sudo caps
echo 'caps ALL=(ALL) NOPASSWD: ALL' | sudo EDITOR='tee -a' visudo
mkdir /home/caps/.ssh
echo $KEY >> /home/caps/.ssh/authorized_keys
chown -R caps:caps /home/caps
chmod 700 /home/caps/.ssh
chmod 600 /home/caps/.ssh/authorized_keys
```

{% offtopic title="Если у вас CentOS, Rocky Linux, ALT Linux, РОСА Сервер, РЕД ОС или МОС ОС..." %}
В операционных системах на базе RHEL (Red Hat Enterprise Linux) добавьте пользователя `caps` в группу `wheel`. Для этого выполните следующую команду, указав публичную часть SSH-ключа, полученную на предыдущем шаге:

```console
# Укажите публичную часть SSH-ключа пользователя.
export KEY='<SSH-PUBLIC-KEY>'
useradd -m -s /bin/bash caps
usermod -aG wheel caps
echo 'caps ALL=(ALL) NOPASSWD: ALL' | sudo EDITOR='tee -a' visudo
mkdir /home/caps/.ssh
echo $KEY >> /home/caps/.ssh/authorized_keys
chown -R caps:caps /home/caps
chmod 700 /home/caps/.ssh
chmod 600 /home/caps/.ssh/authorized_keys
```

{% endofftopic %}

{% offtopic title="Если у вас ОС из семейства Astra Linux..." %}
В операционных системах семейства Astra Linux, при использовании модуля мандатного контроля целостности Parsec, сконфигурируйте максимальный уровень целостности для пользователя `caps`:

```bash
pdpl-user -i 63 caps
```

{% endofftopic %}

## Подготовка конфигурационного файла

Конфигурационный файл для установки в закрытом окружении отличается от конфигурации для установки [на bare-metal](../gs/bm/step2.html) несколькими параметрами. Возьмите файл `config.yml` [из четвёртого шага](../gs/bm/step4.html) руководства по установке на bare-metal и внесите следующие изменения.

При необходимости доступа узлов кластера к внешним ресурсам через прокси прокси-сервер имеет смысл развернуть заранее. Для этого также рекомендуется использовать отдельную машину с доступом во внешнюю сеть.

{% offtopic title="Пример установки прокси-сервера на базе Squid..." %}

Прокси-сервер может быть нужен для доступа к внешним ресурсам из закрытого контура: загрузка образов контейнеров с публичного registry DKP на bastion, обращение компонентов DKP и узлов кластера к внешним URL (если это допускается политикой). Установка пакетов ОС на узлы при этом может выполняться из внутренних репозиториев — в таком случае прокси для пакетов не используется.

Разверните на отдельной машине прокси-сервер, через который будет обеспечиваться доступ к внешним ресурсам (при наличии такого доступа в вашем контуре).

Вы можете использовать любой прокси-сервер, соответствующий требованиям вашей инфраструктуры. В качестве примера приведен [Squid](https://www.squid-cache.org/).

Разверните Squid в контейнере:

```bash
docker run -d --name squid -p 3128:3128 ubuntu/squid
```

Пример успешного запуска:

```text
$ docker run -d --name squid -p 3128:3128 ubuntu/squid
Unable to find image 'ubuntu/squid:latest' locally
latest: Pulling from ubuntu/squid
1678e6c91c57: Pull complete 
040467b888ae: Pull complete 
18b9e99f4452: Pull complete 
Digest: sha256:6a097f68bae708cedbabd6188d68c7e2e7a38cedd05a176e1cc0ba29e3bbe029
Status: Downloaded newer image for ubuntu/squid:latest
059b21fddbd2aba33500920f3f6f0712fa7b23893d512a807397af5eec27fb37
```

Убедитесь, что контейнер запущен:

```console
059b21fddbd2   ubuntu/squid                          "entrypoint.sh -f /e…"   About a minute ago   Up About a minute     0.0.0.0:3128->3128/tcp, [::]:3128->3128/tcp                                          squid
```

В списке запущенных контейнеров должен быть контейнер с соответствующим именем (`squid`).

{% endofftopic %}

* В блоке `ClusterConfiguration` укажите настройки прокси-сервера (если в контуре используется прокси для доступа к внешним ресурсам).

  ```yaml
  # Настройки proxy-сервера.
  proxy:
    httpProxy: http://proxy.local:3128
    httpsProxy: https://proxy.local:3128
    noProxy: ["harbor.example", "proxy.local", "10.128.0.8", "10.128.0.32", "10.128.0.18"]
  ```

  Здесь указываются следующие параметры:
  * адреса HTTP и HTTPS прокси-сервера;
  * список доменов и IP-адресов, которые **не будут проксироваться** через прокси-сервер (внутренние доменные имена и внутренние IP-адреса всех серверов).
  
* В секции `InitConfiguration` добавьте параметры доступа к registry:

  ```yaml
  deckhouse:
    # Адрес Docker registry с образами Deckhouse (укажите редакцию DKP).
    imagesRepo: harbor.example/deckhouse/<РЕДАКЦИЯ_DKP>
    # Строка с ключом для доступа к Docker registry в формате Base64.
    registryDockerCfg: <DOCKER_CFG_BASE64>
    # Протокол доступа к registry (HTTP или HTTPS).
    registryScheme: HTTPS
    # Корневой сертификат, созданный ранее.
    # Получить его можно командой: `cat harbor/certs/ca.crt`.
    registryCA: |
      -----BEGIN CERTIFICATE-----
      ...
      -----END CERTIFICATE-----
  ```

  Здесь `<DOCKER_CFG_BASE64>` — строка авторизации из файла конфигурации Docker-клиента (в Linux обычно это `$HOME/.docker/config.json`) для доступа к стороннему container registry, закодированная в Base64.

  Например, для доступа к container registry `harbor.example` под пользователем `user` с паролем `P@ssw0rd` это будет `eyJhdXRocyI6eyJoYXJib3IuZXhhbXBsZSI6eyJhdXRoIjoiZFhObGNqcFFRSE56ZHpCeVpBPT0ifX19` (строка `{"auths":{"harbor.example":{"auth":"dXNlcjpQQHNzdzByZA=="}}}` в Base64).

* В параметре [releaseChannel](/modules/deckhouse/configuration.html#parameters-releasechannel) ModuleConfig `deckhouse` измените на `Stable` для использования стабильного [канала обновлений](../documentation/v1/reference/release-channels.html).
* В ModuleConfig [global](../documentation/v1/reference/api/global.html) укажите использование самоподписанных сертификатов для компонентов кластера и укажите шаблон доменного имени для системных приложений в параметре `publicDomainTemplate`:

  ```yaml
  settings:
  modules:
    # Шаблон, который будет использоваться для составления адресов системных приложений в кластере.
    # Например, Grafana для %s.test.local будет доступна на домене 'grafana.test.local'.
    # Домен НЕ ДОЛЖЕН совпадать с указанным в параметре clusterDomain ресурса ClusterConfiguration.
    # Можете изменить на свой сразу, либо следовать шагам руководства и сменить его после установки.
    publicDomainTemplate: "%s.test.local"
    # Способ реализации протокола HTTPS, используемый модулями Deckhouse.
    https:
      certManager:
        clusterIssuerName: selfsigned
  ```

  Параметр `settings.modules.https` в ModuleConfig/global поддерживает несколько [режимов](../documentation/v1/reference/api/global.html): `CertManager` — заказ сертификата у указанного `ClusterIssuer` (не обязательно `selfsigned`, можно задать свой издатель — корпоративный CA, HashiCorp Vault, Venafi и т. д., см. [обзор в документации по сертификатам](../documentation/v1/admin/configuration/security/certificates.html)); `CustomCertificate` — готовая пара «сертификат + ключ» в Secret формата `kubernetes.io/tls` в пространстве имён `d8-system`, при внешнем TLS-терминаторе возможен режим `OnlyInURI`. Сочетание `selfsigned` и отключение Let's Encrypt в блоке выше показывает простой пример использования HTTPS в изолированном контуре без ACME/Let's Encrypt.

* В ModuleConfig `user-authn` измените значение параметра [`dexCAMode`](/modules/user-authn/configuration.html#parameters-controlplaneconfigurator-dexcamode) на `FromIngressSecret`:

  ```yaml
  settings:
  controlPlaneConfigurator:
    dexCAMode: FromIngressSecret
  ```

* Добавьте включение и конфигурацию модуля [cert-manager](/modules/cert-manager/), в которой будет отключено использование Let's Encrypt:

  ```yaml
  apiVersion: deckhouse.io/v1alpha1
  kind: ModuleConfig
  metadata:
    name: cert-manager
  spec:
    version: 1
    enabled: true
    settings:
      disableLetsencrypt: true
  ```

* В параметре [internalNetworkCIDRs](../documentation/v1/reference/api/cr.html#staticclusterconfiguration-internalnetworkcidrs) StaticClusterConfiguration укажите подсеть внутренних IP-адресов узлов кластера. Например:

  ```yaml
  internalNetworkCIDRs:
  - 10.128.0.0/24
  ```

{% offtopic title="Пример полного конфигурационного файла..." %}

```yaml
# Общие параметры кластера.
# https://deckhouse.ru/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
# Адресное пространство подов кластера.
# Возможно, захотите изменить. Убедитесь, что не будет пересечений с serviceSubnetCIDR и internalNetworkCIDRs.
podSubnetCIDR: 10.111.0.0/16
# Адресное пространство сети сервисов кластера.
# Возможно, захотите изменить. Убедитесь, что не будет пересечений с podSubnetCIDR и internalNetworkCIDRs.
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "Automatic"
# Домен кластера.
clusterDomain: "cluster.local"
# Тип container runtime, используемый на узлах кластера (в NodeGroup’ах) по умолчанию.
defaultCRI: "ContainerdV2"
# Настройки proxy-сервера.
proxy:
  httpProxy: http://proxy.local:3128
  httpsProxy: https://proxy.local:3128
  noProxy: ["harbor.example", "proxy.local", "10.128.0.8", "10.128.0.32", "10.128.0.18"]
---
# Настройки первичной инициализации кластера Deckhouse.
# https://deckhouse.ru/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  # Адрес Docker registry с образами Deckhouse.
  imagesRepo: harbor.example/deckhouse/ee
  # Строка с ключом для доступа к Docker registry.
  registryDockerCfg: <DOCKER_CFG_BASE64>
  # Протокол доступа к registry (HTTP или HTTPS).
  registryScheme: HTTPS
  # Корневой сертификат, которым можно проверить сертификат registry (если registry использует самоподписанные сертификаты).
  registryCA: |
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
---
# Настройки модуля deckhouse.
# https://deckhouse.ru/modules/deckhouse/configuration.html
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
    logLevel: Info
---
# Глобальные настройки Deckhouse.
# https://deckhouse.ru/products/kubernetes-platform/documentation/v1/reference/api/global.html#%D0%BF%D0%B0%D1%80%D0%B0%D0%BC%D0%B5%D1%82%D1%80%D1%8B
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 2
  settings:
    modules:
      # Шаблон, который будет использоваться для составления адресов системных приложений в кластере.
      # Например, Grafana для %s.test.local будет доступна на домене 'grafana.test.local'.
      # Домен НЕ ДОЛЖЕН совпадать с указанным в параметре clusterDomain ресурса ClusterConfiguration.
      # Можете изменить на свой сразу, либо следовать шагам руководства и сменить его после установки.
      publicDomainTemplate: "%s.test.local"
      # Способ реализации протокола HTTPS, используемый модулями Deckhouse.
      https:
        certManager:
          # Использовать самоподписанные сертификаты для модулей Deckhouse.
          clusterIssuerName: selfsigned
---
# Настройки модуля user-authn.
# https://deckhouse.ru/modules/user-authn/configuration.html
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  version: 2
  enabled: true
  settings:
    controlPlaneConfigurator:
      dexCAMode: FromIngressSecret
    # Включение доступа к API-серверу Kubernetes через Ingress.
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
  name: cert-manager
spec:
  version: 1
  enabled: true
  settings:
    disableLetsencrypt: true
---
# Настройки модуля cni-cilium.
# https://deckhouse.ru/modules/cni-cilium/configuration.html
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cni-cilium
spec:
  version: 1
  # Включить модуль cni-cilium
  enabled: true
  settings:
    # Настройки модуля cni-cilium
    # https://deckhouse.ru/modules/cni-cilium/configuration.html
    tunnelMode: VXLAN
---
# Параметры статического кластера.
# https://deckhouse.ru/products/kubernetes-platform/documentation/v1/reference/api/cr.html#staticclusterconfiguration
apiVersion: deckhouse.io/v1
kind: StaticClusterConfiguration
# Список внутренних сетей узлов кластера (например, '10.0.4.0/24'), который
# используется для связи компонентов Kubernetes (kube-apiserver, kubelet...) между собой.
# Укажите, если используете модуль virtualization или узлы кластера имеют более одного сетевого интерфейса.
# Если на узлах кластера используется только один интерфейс, ресурс StaticClusterConfiguration можно не создавать.
internalNetworkCIDRs:
- 10.128.0.0/24
```

{% endofftopic %}

Конфигурационный файл для установки подготовлен.

## Установка DKP

Перенесите подготовленный конфигурационный файл на хост, с которого выполняется установка, например в директорию `~/deckhouse`. Перейдите в директорию и запустите установщик командой:

```bash
docker run --pull=always -it -v "$PWD/config.yml:/config.yml" -v "$HOME/.ssh/:/tmp/.ssh/" --network=host -v "$PWD/dhctl-tmp:/tmp/dhctl" harbor.example/deckhouse/<РЕДАКЦИЯ_DKP>/install:stable bash
```

{% offtopic title="Если появилась ошибка `509: certificate signed by unknown authority`..." %}
Даже при наличии сертификатов в `/etc/docker/certs.d/harbor.example/` Docker может выдавать ошибку о неизвестном центре сертификации (это типично для самоподписанных сертификатов). В таком случае, как правило, помогает добавление `ca.crt` в системное хранилище доверенных сертификатов с последующим перезапуском Docker.
{% endofftopic %}

{% alert level="info" %}
Если во внутренней сети нет локального DNS-сервера, и доменные имена прописаны в `/etc/hosts` на хосте, где запускается установщик, то обязательно укажите параметр `--network=host`, чтобы Docker смог ими воспользоваться.
{% endalert %}

После успешной загрузки и запуска контейнера вы увидите приглашение командной строки внутри контейнера:

```console
[deckhouse] root@guide-bastion / # 
```

Запустите установку DKP командой (укажите внутренний IP-адрес master-узла):

```bash
dhctl bootstrap --ssh-user=deckhouse --ssh-host=<master_ip> --ssh-agent-private-keys=/tmp/.ssh/id_rsa \
  --config=/config.yml \
  --ask-become-pass
```

Процесс установки может занять до 30 минут в зависимости от скорости сетевого соединения.

При успешном завершении установки вы увидите следующее сообщение:

```console
┌ ⛵ ~ Bootstrap: Run post bootstrap actions
│ ┌ Set release channel to deckhouse module config
│ │ 🎉 Succeeded!
│ └ Set release channel to deckhouse module config (0.09 seconds)
└ ⛵ ~ Bootstrap: Run post bootstrap actions (0.09 seconds)

┌ ⛵ ~ Bootstrap: Clear cache
│ ❗ ~ Next run of "dhctl bootstrap" will create a new Kubernetes cluster.
└ ⛵ ~ Bootstrap: Clear cache (0.00 seconds)

🎉 Deckhouse cluster was created successfully!
```

## Добавление узлов в кластер

Добавьте узел в кластер.

Для этого выполните следующие шаги:

* Настройте StorageClass [локального хранилища](../../../modules/local-path-provisioner/cr.html#localpathprovisioner), выполнив на master-узле следующую команду:

  ```console
  sudo -i d8 k create -f - << EOF
  apiVersion: deckhouse.io/v1alpha1
  kind: LocalPathProvisioner
  metadata:
    name: localpath
  spec:
    path: "/opt/local-path-provisioner"
    reclaimPolicy: Delete
  EOF
  ```

* Укажите, что созданный StorageClass должен использоваться как StorageClass по умолчанию. Для этого выполните на master-узле следующую команду:

  ```bash
  sudo -i d8 k patch mc global --type merge \
    -p "{\"spec\": {\"settings\":{\"defaultClusterStorageClass\":\"localpath\"}}}"
  ```

* Создайте NodeGroup `worker` и добавьте узел с помощью Cluster API Provider Static (CAPS):

  ```console
  sudo -i d8 k create -f - <<EOF
  apiVersion: deckhouse.io/v1
  kind: NodeGroup
  metadata:
    name: worker
  spec:
    nodeType: Static
    staticInstances:
      count: 1
      labelSelector:
        matchLabels:
          role: worker
  EOF
  ```

* Создайте в кластере ресурс [SSHCredentials](../../../../modules/node-manager/cr.html#sshcredentials). Для этого выполните на master-узле следующую команду:

  ```console
  sudo -i d8 k create -f - <<EOF
  apiVersion: deckhouse.io/v1alpha2
  kind: SSHCredentials
  metadata:
    name: caps
  spec:
    user: caps
    privateSSHKey: "`cat /dev/shm/caps-id | base64 -w0`"
  EOF
  ```

* Выведите публичную часть сгенерированного ранее SSH-ключа (он понадобится на следующем шаге). Для этого выполните на master-узле следующую команду:

  ```console
  cat /dev/shm/caps-id.pub
  ```

* Создайте [StaticInstance](../../../modules/node-manager/cr.html#staticinstance) для добавляемого узла. Для этого выполните на master-узле следующую команду, указав IP-адрес добавляемого узла:

  ```console
  # Укажите IP-адрес узла, который нужно подключить к кластеру.
  export NODE=<NODE-IP-ADDRESS>
  sudo -i d8 k create -f - <<EOF
  apiVersion: deckhouse.io/v1alpha2
  kind: StaticInstance
  metadata:
    name: d8cluster-worker
    labels:
      role: worker
  spec:
    address: "$NODE"
    credentialsRef:
      kind: SSHCredentials
      name: caps
  EOF
  ```

* Убедитесь, что все узлы кластера находятся в статусе `Ready`:

  ```console
  $ sudo -i d8 k get no
  NAME               STATUS   ROLES                  AGE    VERSION
  d8cluster          Ready    control-plane,master   30m   v1.23.17
  d8cluster-worker   Ready    worker                 10m   v1.23.17
  ```

  Запуск всех компонентов DKP после завершения установки может занять некоторое время.

## Настройка Ingress-контроллера и создание пользователя

### Установка ingress-контроллера

Убедитесь, что под Kruise controller manager модуля [ingress-nginx](../../../modules/ingress-nginx/) запустился и находится в статусе `Running`. Для этого выполните на master-узле следующую команду:

```bash
$ sudo -i d8 k -n d8-ingress-nginx get po -l app=kruise
NAME                                         READY   STATUS    RESTARTS    AGE
kruise-controller-manager-7dfcbdc549-b4wk7   3/3     Running   0           15m
```

Создайте на master-узле файл `ingress-nginx-controller.yml`, содержащий конфигурацию Ingress-контроллера:

```yaml
# Секция, описывающая параметры NGINX Ingress controller.
# https://deckhouse.ru/modules/ingress-nginx/cr.html
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: nginx
spec:
  # Имя Ingress-класса для обслуживания NGINX Ingress controller.
  ingressClass: nginx
  # Способ поступления трафика из внешнего мира.
  inlet: HostPort
  hostPort:
    httpPort: 80
    httpsPort: 443
  # Описывает, на каких узлах будет находиться компонент.
  # Возможно, захотите изменить.
  nodeSelector:
    node-role.kubernetes.io/control-plane: ""
  tolerations:
  - effect: NoSchedule
    key: node-role.kubernetes.io/control-plane
    operator: Exists
```

Примените его, выполнив на master-узле следующую команду:

```bash
sudo -i d8 k create -f $PWD/ingress-nginx-controller.yml
```

Запуск Ingress-контроллера после завершения установки DKP может занять некоторое время. Прежде чем продолжить, убедитесь, что Ingress-контроллер запустился (выполните на master-узле):

```console
$ sudo -i d8 k -n d8-ingress-nginx get po -l app=controller
NAME                                       READY   STATUS    RESTARTS   AGE
controller-nginx-r6hxc                     3/3     Running   0          5m
```

### Создание пользователя для доступа в веб-интерфейсы кластера

Создайте на master-узле файл `user.yml`, содержащий описание учётной записи пользователя и прав доступа:

```yaml
# Настройки RBAC и авторизации.
# https://deckhouse.ru/modules/user-authz/cr.html#clusterauthorizationrule
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: admin
spec:
  # Список учётных записей Kubernetes RBAC.
  subjects:
  - kind: User
    name: admin@deckhouse.io
  # Предустановленный шаблон уровня доступа.
  accessLevel: SuperAdmin
  # Разрешить пользователю делать kubectl port-forward.
  portForwarding: true
---
# Данные статического пользователя.
# https://deckhouse.ru/modules/user-authn/cr.html#user
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  # E-mail пользователя.
  email: admin@deckhouse.io
  # Это хеш пароля 3xqgv2auys, сгенерированного сейчас.
  # Сгенерируйте свой или используйте этот, но только для тестирования:
  # echo -n '3xqgv2auys' | htpasswd -BinC 10 "" | cut -d: -f2 | tr -d '\n' | base64 -w0; echo
  # Возможно, захотите изменить.
  password: 'JDJhJDEwJGtsWERBY1lxMUVLQjVJVXoxVkNrSU8xVEI1a0xZYnJNWm16NmtOeng5VlI2RHBQZDZhbjJH'
```

Примените его, выполнив на master-узле следующую команду:

```console
sudo -i d8 k create -f $PWD/user.yml
```

## Настройка DNS-записей

Для доступа к веб-интерфейсам кластера настройте соответствие следующих доменных имён внутреннему IP-адресу master-узла (используйте DNS-имена в соответствии с шаблоном DNS-имён, указанным в параметре [publicDomainTemplate](../documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate)). Например, можно прописать их в `/etc/hosts` на локальной машине для шаблона DNS-имён `%s.test.local`. Перед выполнением замените плейсхолдер `<MASTER_IP>` на внутренний IP-адрес master-узла:

```text
export PUBLIC_IP="<MASTER_IP>"
sudo -E bash -c "cat <<EOF >> /etc/hosts
$PUBLIC_IP api.test.local
$PUBLIC_IP code.test.local
$PUBLIC_IP commander.test.local
$PUBLIC_IP registry.test.local
$PUBLIC_IP console.test.local
$PUBLIC_IP dex.test.local
$PUBLIC_IP documentation.test.local
$PUBLIC_IP grafana.test.local
$PUBLIC_IP hubble.test.local
$PUBLIC_IP istio.test.local
$PUBLIC_IP istio-api-proxy.test.local
$PUBLIC_IP kubeconfig.test.local
$PUBLIC_IP openvpn-admin.test.local
$PUBLIC_IP prometheus.test.local
$PUBLIC_IP status.test.local
$PUBLIC_IP tools.test.local
$PUBLIC_IP upmeter.test.local
EOF
"
```

Проверить, что кластер корректно развёрнут и работает, можно в веб-интерфейсе Grafana, где отображается состояние кластера. Адрес Grafana формируется по шаблону `publicDomainTemplate`. Например, при значении `%s.test.local` интерфейс будет доступен по адресу `grafana.test.local`. Для входа используйте учётные данные пользователя, созданного ранее.

## Куда двигаться дальше?

Все установлено, настроено и работает! Теперь можно воспользоваться предоставляемыми веб-интерфейсами для управления кластером:

* **Веб-интерфейс Deckhouse** — управление кластером и основными компонентами. Адрес: **console.test.local**.
* **Документация** — документация по установленной в кластере версии DKP. Адрес: **documentation.test.local**.
* **Мониторинг** — дэшборды Grafana, поставляемые с DKP. Адрес: **grafana.test.local** (путь к Prometheus: **/prometheus/**).
  Подробнее [в документации](/products/kubernetes-platform/documentation/v1/admin/configuration/monitoring/).
* **Status page** — общий статус DKP и его компонентов. Адрес: **status.test.local**.
* **Upmeter** — контроль соблюдения SLA с детализацией по компонентам и периодам. Адрес: **upmeter.test.local**.
* **Подготовка к production** — проверьте готовность кластера к приёму трафика по [инструкции для подготовки к production](/products/kubernetes-platform/guides/production.html).

### Развёртывание первого приложения

* **Настройка CI/CD** — создайте ServiceAccount для развёртывания в кластере и выдайте ему права. В результате вы получите kubeconfig, который можно использовать в системах автоматизированного развёртывания в Kubernetes. Подробнее о настройке доступа для CI/CD в разделе [«Доступ для CI/CD»](/products/kubernetes-platform/documentation/v1/admin/configuration/access/authorization/ci_cd.html). Адрес: **kubeconfig.test.local**.
* **Направление трафика на приложение** — создайте Service и Ingress для приложения. Подробнее о возможностях сетевого взаимодействия в разделе [«Обработка входящего трафика»](/products/kubernetes-platform/documentation/v1/user/network/ingress/).
* **Мониторинг приложения** — добавьте к созданному Service аннотации `prometheus.deckhouse.io/custom-target: "my-app"` и `prometheus.deckhouse.io/port: "80"`. Подробнее о настройке мониторинга приложений в разделе [«Мониторинг приложений и инфраструктур»](/products/kubernetes-platform/documentation/v1/user/monitoring/).

### Что дальше?

Подробная информация о системе и компонентах Deckhouse Kubernetes Platform — в [документации](/products/kubernetes-platform/documentation/). По вопросам можно обратиться в [онлайн-сообщество](https://t.me/deckhouse_ru).
