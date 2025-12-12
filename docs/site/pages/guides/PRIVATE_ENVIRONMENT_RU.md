---
title: Установка DKP в закрытом окружении
permalink: ru/guides/private-environment.html
description: Руководство по установке Deckhouse Kubernetes Platform в закрытом окружении
lang: ru
layout: sidebar-guides
---

В приведённом ниже руководстве рассказывается, как развернуть кластер под управлением Deckhouse Kubernetes Platform в закрытом окружении, из которого нет прямого доступа к хранилищу образов контейнеров DKP (`registry.deckhouse.ru`) и к внешним репозиториям deb/rpm-пакетов установленных на узлах [операционных систем](../documentation/v1/reference/supported_versions.html#linux).

{% alert level="warning" %}
Обратите внимание, что установка DKP в закрытое окружения доступна в следующих редакциях: SE, SE+, EE, CSE Lite (1.67), CSE Pro (1.67).
{% endalert %}

## Особенности приватного окружения

Установка в закрытое окружение практически не отличается от установки [на bare metal](../gs/bm/step2.html). 

Главные особенности:

* доступ в Интернет для приложений, развёрнутых в закрытом контуре, осуществляется через прокси-сервер, который необходимо указать в конфигурации кластера;
* container registry с образами контейнеров Deckhouse также разворачиваеся отдельно с доступом изнутри контура, а в кластере настраивается его использование и необходимые права доступа.

Все взаимоотношения с внешними ресурсами осуществляются через отдельную машину (назовё её «бастион»). На ней же разворачиваются container registry и прокси-сервер, а также с неё производятся все операции с кластером.

Общая схема закрытого окружения представлена на рисунке:

<img src="/images/gs/private-env-schema-RU.svg" alt="Схема развертывания Deckhouse в закрытом окружении">

{% alert level="info" %}
На схеме также показан внутренний репозиторий пакетов ОС, который необходим для установки curl на узлах будущего кластера при отсутствии возможности доступа к официальным репозиториям через прокси-сервер.
{% endalert %}

## Выбор инфраструктуры

В этом руководстве мы будем разворачивать в закрытом окружении кластер, состоящий из одного master-узла и одного worker-узла.

На понядобятся персональный компьютер, с которого будут выполняться работы, отдельная машина под бастион, на котором будет развёрнут container registry и сопутствующие компоненты, а также две машины под узлы кластера.

Требования к машинам следующие:

* Бастион — 4 ядра, 8 ГБ ОЗУ, 150 ГБ на быстром диске. Такой объем объясняется тем, что на этой машине будут храниться все образы Deckhouse, необходиимые для установки, причем скачиваться с registry Фланта перед заливкой в приватный registry, они будут так же на эту же машину;
* [Ресурсы под будущие узлы кластера](./hardware-requirements.html#выбор-ресурсов-для-узлов) нужно выбрать исходя из ваших требований к будущей нагрузке в кластере. Для примера возьмём минимально рекомендуемые 4 ядра, 8 ГБ ОЗУ, 60 ГБ на быстром диске (400+ IOPS) для каждого из узов.

## Подготовка приватного container registry

### Установка Harbor

В качестве приватного registry будем использовать [Harbor](https://goharbor.io/). Это container registry с открытым исходным кодом, который имеет возможности настройки политик и управления доступом на основе ролей, проверяет образы на наличие уязвимостей и помечает их как надежные. Harbor входит в состав проектов CNCF.

Устанавливать мы будем latest-версию [из GitHub-репозитория](https://github.com/goharbor/harbor/releases) проекта. Для этого нужно скачать архив с установщиком из нужного релиза, выбрав вариант с `harbor-offline-installer` в названии.

<div style="text-align: center;">
<img src="/images/guides/install_to_private_environment/download-harbor-installer.png" alt="Скачивание установщика Harbor...">
</div>

Скопируйте адрес ссылки. Для версии `harbor-offline-installer-v2.14.1.tgz` ссылка будет выглядеть следующим образом: `https://github.com/goharbor/harbor/releases/download/v2.14.1/harbor-offline-installer-v2.14.1.tgz`.

Подключитель к вашему бастиону по SSH и скачайте файл любым удобным вам способом.

{% offtopic title="Как скачать файл с помощью wget..." %}
Выполните команду:
```console
wget https://github.com/goharbor/harbor/releases/download/v2.14.1/harbor-offline-installer-v2.14.1.tgz
```

Пример выполнени команды:
```text
$ wget https://github.com/goharbor/harbor/releases/download/v2.14.1/harbor-offline-installer-v2.14.1.tgz
--2025-12-04 12:38:42--  https://github.com/goharbor/harbor/releases/download/v2.14.1/harbor-offline-installer-v2.14.1.tgz
Resolving github.com (github.com)... 140.82.121.4
Connecting to github.com (github.com)|140.82.121.4|:443... connected.
HTTP request sent, awaiting response... 302 Found
Location: https://release-assets.githubusercontent.com/github-production-release-asset/50613991/01508bef-5c2c-40bb-bc66-f40e34ad2cae?sp=r&sv=2018-11-09&sr=b&spr=https&se=2025-12-04T13%3A28%3A53Z&rscd=attachment%3B+filename%3Dharbor-offline-installer-v2.14.1.tgz&rsct=application%2Foctet-stream&skoid=96c2d410-5711-43a1-aedd-ab1947aa7ab0&sktid=398a6654-997b-47e9-b12b-9515b896b4de&skt=2025-12-04T12%3A28%3A27Z&ske=2025-12-04T13%3A28%3A53Z&sks=b&skv=2018-11-09&sig=bUJ%2Bo6Bx7brkGAvaf2Pq9cXHah1aPJi9PDlc7G3WwS0%3D&jwt=eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJnaXRodWIuY29tIiwiYXVkIjoicmVsZWFzZS1hc3NldHMuZ2l0aHVidXNlcmNvbnRlbnQuY29tIiwia2V5Ijoia2V5MSIsImV4cCI6MTc2NDg1NTUyMiwibmJmIjoxNzY0ODUxOTIyLCJwYXRoIjoicmVsZWFzZWFzc2V0cHJvZHVjdGlvbi5ibG9iLmNvcmUud2luZG93cy5uZXQifQ.DHH2Fz_xRutTNEnN5p3uxxG_T03dhqppICQhc0gJQq0&response-content-disposition=attachment%3B%20filename%3Dharbor-offline-installer-v2.14.1.tgz&response-content-type=application%2Foctet-stream [following]
--2025-12-04 12:38:42--  https://release-assets.githubusercontent.com/github-production-release-asset/50613991/01508bef-5c2c-40bb-bc66-f40e34ad2cae?sp=r&sv=2018-11-09&sr=b&spr=https&se=2025-12-04T13%3A28%3A53Z&rscd=attachment%3B+filename%3Dharbor-offline-installer-v2.14.1.tgz&rsct=application%2Foctet-stream&skoid=96c2d410-5711-43a1-aedd-ab1947aa7ab0&sktid=398a6654-997b-47e9-b12b-9515b896b4de&skt=2025-12-04T12%3A28%3A27Z&ske=2025-12-04T13%3A28%3A53Z&sks=b&skv=2018-11-09&sig=bUJ%2Bo6Bx7brkGAvaf2Pq9cXHah1aPJi9PDlc7G3WwS0%3D&jwt=eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJnaXRodWIuY29tIiwiYXVkIjoicmVsZWFzZS1hc3NldHMuZ2l0aHVidXNlcmNvbnRlbnQuY29tIiwia2V5Ijoia2V5MSIsImV4cCI6MTc2NDg1NTUyMiwibmJmIjoxNzY0ODUxOTIyLCJwYXRoIjoicmVsZWFzZWFzc2V0cHJvZHVjdGlvbi5ibG9iLmNvcmUud2luZG93cy5uZXQifQ.DHH2Fz_xRutTNEnN5p3uxxG_T03dhqppICQhc0gJQq0&response-content-disposition=attachment%3B%20filename%3Dharbor-offline-installer-v2.14.1.tgz&response-content-type=application%2Foctet-stream
Resolving release-assets.githubusercontent.com (release-assets.githubusercontent.com)... 185.199.108.133, 185.199.109.133, 185.199.110.133, ...
Connecting to release-assets.githubusercontent.com (release-assets.githubusercontent.com)|185.199.108.133|:443... connected.
HTTP request sent, awaiting response... 200 OK
Length: 680961237 (649M) [application/octet-stream]
Saving to: ‘harbor-offline-installer-v2.14.1.tgz’

harbor-offline-installer-v2.14.1.tgz                         100%[=============================================================================================================================================>] 649.42M  77.2MB/s    in 8.2s

2025-12-04 12:38:50 (79.4 MB/s) - ‘harbor-offline-installer-v2.14.1.tgz’ saved [680961237/680961237]
```
{% endofftopic %}

{% offtopic title="Как скачать файл с помощью curl..." %}
Выполните команду:
```console
curl -O https://github.com/goharbor/harbor/releases/download/v2.14.1/harbor-offline-installer-v2.14.1.tgz
```

Пример выполнени команды:
```text
$ curl -O https://github.com/goharbor/harbor/releases/download/v2.14.1/harbor-offline-installer-v2.14.1.tgz
  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                 Dload  Upload   Total   Spent    Left  Speed
  0     0    0     0    0     0      0      0 --:--:-- --:--:-- --:--:--     0
```
{% endofftopic %}

Распакуйте скачанный файл:

```console
tar -zxf ./harbor-offline-installer-v2.14.1.tgz
```

В полученном каталоге `harbor` расположены файлы, необходимые для установки.

Прежде чем разворачивать хранилище нужно сгенерировать сертификаты, т.к. в закрытом окружении невозможно получить сертификаты от Let's Encrypt (он не сможет достучаться до внутренний ресурсов при проверке доступности).

{% alert level="info" %}
Способов для генерации существует несколько, мы для примера выбрали один из них. Вы можете следовать своему пути.
{% endalert %}

Создадим каталог `certs` в каталоге `harbor`:

```bash
cd harbor/
mkdir certs
``` 

Сгенерируем сертификаты для внешнего доступа командами:

```bash
openssl genrsa -out ca.key 4096
```

```bash
openssl req -x509 -new -nodes -sha512 -days 3650 -subj "/C=RU/ST=Moscow/L=Moscow/O=example/OU=Personal/CN=myca.local" -key ca.key -out ca.crt
```

Сгенерируем сертификаты для внутреннего доменного имени `harbor.local`, чтобы внутри приватной сети обращаться к бастиону также по защищённому соединению:

```bash
openssl genrsa -out harbor.local.key 4096
```

```bash
openssl req -sha512 -new -subj "/C=RU/ST=Moscow/L=Moscow/O=example/OU=Personal/CN=harbor.local" -key harbor.local.key -out harbor.local.csr
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
DNS.1=harbor.local
EOF
```

**Важно!** Не забудьте заменить в этой команде `<INTERNAL_IP_ADDRESS>` на внутренниый серый IP-адрес бастиона. По нему будет происходить обращение к container registry изнутри закрытого контура. С этим же адресом будет связано доменное имя `harbor.local`.


```bash
openssl x509 -req -sha512 -days 3650 -extfile v3.ext -CA ca.crt -CAkey ca.key -CAcreateserial -in harbor.local.csr -out harbor.local.crt
```

```bash
openssl x509 -inform PEM -in harbor.local.crt -out harbor.local.cert
```

Проверим, что все сертификаты созданы успешно:

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
-rw-rw-r-- 1 ubuntu ubuntu 2122 Dec  5 14:58 harbor.local.cert
-rw-rw-r-- 1 ubuntu ubuntu 2122 Dec  5 14:58 harbor.local.crt
-rw-rw-r-- 1 ubuntu ubuntu 1704 Dec  5 14:57 harbor.local.csr
-rw------- 1 ubuntu ubuntu 3268 Dec  5 14:57 harbor.local.key
-rw-rw-r-- 1 ubuntu ubuntu  247 Dec  5 14:58 v3.ext
```
{% endofftopic %}

Следующим шагом нужно настроить Docker на работу с нашим новым приватным container registry, обращение к которому будет по SSL. Для этого создадим каталог:

```bash
sudo mkdir -p /etc/docker/certs.d/harbor.local
```

> Ключик `-p` здесь указывает утилите mkdir, что нужно создать и сам каталог `certs.d`, если он отсутствует.

И затем скопируем в него созданные сертификаты:

```bash
$ cp ca.crt /etc/docker/certs.d/harbor.local/
$ cp harbor.local.cert /etc/docker/certs.d/harbor.local/
$ cp harbor.local.key /etc/docker/certs.d/harbor.local/
```

Эти сертификаты будут использоваться при обращении к regitry по доменному имени `harbor.local`.

Скопируйте шаблон конфигурационного файла, предоставленный разработчиками в архиве с установщиком:

```bash
cp harbor.yml.tmpl harbor.yml
```

Измените в нём следующие параметры:

* `hostname` — задайте `harbor.local`, для которого мы генерировали сертификаты;
* `certificate` — укажите путь к сгенерированному сертификату в каталоге `certs` (например, `/home.ubuntu/harbor/certs/harbor.local.crt`);
* `private_key` — укажите путь к приватному ключу (например, `/home.ubuntu/harbor/certs/harbor.local.key`);
* `harbor_admin_password` — задайте пароль для доступа в веб-интерфейс.

Сохраните файл.

{% offtopic title="Пример конфигурационного файла..." %}
```yaml
# Configuration file of Harbor

# The IP address or hostname to access admin UI and registry service.
# DO NOT use localhost or 127.0.0.1, because Harbor needs to be accessed by external clients.
hostname: harbor.local

# http related config
http:
  # port for http, default is 80. If https enabled, this port will redirect to https port
  port: 80

# https related config
https:
  # https port for harbor, default is 443
  port: 443
  # The path of cert and key files for nginx
  certificate: /home.ubuntu/harbor/certs/harbor.local.crt
  private_key: /home.ubuntu/harbor/certs/harbor.local.key
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
# if using jaeger agetn mode uncomment agent_host and agent_port
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

{% alert level="info" %}
Обратитите внимание, что перед запуском установки нужно установить на машину [Docker](https://docs.docker.com/engine/install/) и docker-compose. При их отсутствии скрипт установки покажет ошибку с требованиям предварительно установить нужный софт.
{% offtopic title="Ошибка в случае их отсутвтия..." %}
```console
$ ./install.sh 

[Step 0]: checking if docker is installed ...
✖ Need to install docker(20.10.10+) first and run this script again.
```
{% endofftopic %}
{% endalert %}

Начнется установка Harbor — будут выкачаны все нужные образы и запущены соответствующие контейнеры.

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

Проверим, что Harbor успешно запущен:

```bash
$ docker ps
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

Добавьте в файл `/etc/hosts` ассоциацию доменного имени `harbor.local` с localhost машины-бастиона, чтобы можно было обращаться к Harbor по этому имени с этой же машины:

```bash
127.0.0.1 localhost harbor.local
```
{% alert level="warning" %}
**Обратите внимание!** В некоторых облачных провайерах, например в Yandex Cloud, исправления в `/etc/hosts` будут откачены к дефолтным значениям после перезагрузки виртуальной машины. Предупреждение об этом написано в самом начале этого файла:
```text
# Your system has configured 'manage_etc_hosts' as True.
# As a result, if you wish for changes to this file to persist
# then you will need to either
# a.) make changes to the master file in /etc/cloud/templates/hosts.debian.tmpl
# b.) change or remove the value of 'manage_etc_hosts' in
#     /etc/cloud/cloud.cfg or cloud-config from user-data
```
Если у вашего провайдера такая же схема, внесите соответствующие изменения и в предложенный файл шаблона, чтобы после перезагрузки настройки не пострадали.
{% endalert %}

На этом установка Harbor завершена! 🎉

### Настройка Harbor

Теперь нужно настроить Harbor: создать проект и пользователя, под которым будет выполняться работа в этом проекте.

Перейдём на главную страницу Harbor по адресу `harbor.local`:

<div style="text-align: center;">
<img src="/images/guides/install_to_private_environment/harbor_main_page.png" alt="Главная страница Harbor...">
</div>

{% alert level="info" %}
Обратитите внимание, что для доступа по этому доменному имени с рабочей машины нужно также добавить доменное имя `harbor.local` в `/etc/hosts`, указав в качестве назначения IP-адрес машины-бастиона.
{% endalert %}

Для входа в интерфейс воспользуйтесь логином и паролем, указанными в конфигурационном файле `harbor.yml`.

<div style="text-align: center;">
<img src="/images/guides/install_to_private_environment/harbor_main_dashboard.png" alt="Главная страница Harbor...">
</div>

{% alert level="info" %}
Обратитите внимание, что браузер может ругаться на самоподписанный сертификат, считая его «небезопасным». Это не так, таким сертификатом можно пользоваться.
{% endalert %}

Создайте новый проект. Для этого нажмите на кнопку «Новый проект» и введите его название: `deckhouse`. Остальные настройки трогать не нужно.

<div style="text-align: center;">
<img src="/images/guides/install_to_private_environment/harbor_new_project.png" alt="Главная страница Harbor...">
</div>

Создайте нового пользователя для этого проекта. Перейдите на вкладку «Пользователи» в левом меню и нажмите «Новый пользователь»:

<div style="text-align: center;">
<img src="/images/guides/install_to_private_environment/harbor_create_new_user.png" alt="Главная страница Harbor...">
</div>

Задайте ему имя, адрес электронной почты и пароль:

<div style="text-align: center;">
<img src="/images/guides/install_to_private_environment/harbor_creating_user.png" alt="Главная страница Harbor...">
</div>

Добавьте созданного пользователя к созданный ранее проект. Перейдите обратно на вкладку «Проекты», выберите проект «deckhouse», перейдите на вкладку «Участники» и добавьте туда созданного пользователя, нажав на кнопку «Пользователь».

<div style="text-align: center;">
<img src="/images/guides/install_to_private_environment/harbor_adding_user_to_project.png" alt="Главная страница Harbor...">
</div>

Роль пользователя оставьте предложенную по умолчанию: «Администратор проекта»:

<div style="text-align: center;">
<img src="/images/guides/install_to_private_environment/harbor_new_project_user.png" alt="Главная страница Harbor...">
</div>

На этом настройка Harbor завершена! 🎉

## Копироване образов DKP в приватный container registry

Следующим шагом будет копирование образов компонентов DKP из registry Фланта в Harbor.

{% alert level="info" %}
Для дальнейших действий в этом разделе понадобится установленна утилита Deckhouse CLI. Установите на бастион-машину, следуя инструции по установке из [официальной документации](../documentation/v1/cli/d8/#как-установить-deckhouse-cli).
{% endalert %}

{% alert level="warning" %}
Процесс загрузки образов занимает довольно продолжительное время, поэтому чтобы избежать проблем от прерывания сетевого соединения рекомендуется запускать его в сессии tmux или screen, чтобы в случае обырыва соедиения по SSH с бастионом можно было просто подключиться к сессии, а не запускать всё заново. Обе утилиты обычно присутствуют в репозиториях дистрибутивов Linux, и спокойно устанавливаются с помощью пакетного менеджера.

{% offtopic title="Как работать с tmux..." %}
* Запустить сессию можно командой `tmux`.
* Выход из сессии осущесвляется сочетанием клавиш `Ctrl + b` и затем `d`. Сессия остаётся запущенной, выполняемые в ней процесс продолжают работать. Для полного выхода из сессии используется сочетание `Ctrl + d`, как и для любой терминальной сессии.
* Просмотр запущенных сессий осуществляется командой `tmux ls`:
  ```console
  $ tmux ls
  0: 1 windows (created Thu Dec 11 13:52:41 2025)
  ```
* Подключение к запущенной сесии: `tmux attach -t <ИДЕНТИФИКАТОР СЕССИИ>`. Для примера выше `<ИДЕНТИФИКАТОР СЕССИИ>` будет `0`.
{% endofftopic %}

{% offtopic title="Как работать со screen..." %}
* Запустить сессию можно командой `screen`.
* Выход из сессии осущесвляется сочетанием клавиш `Ctrl + a` и затем `d`, не отпуская `Ctrl`. Сессия остаётся запущенной, выполняемые в ней процесс продолжают работать. Для полного выхода из сессии используется сочетание `Ctrl + d`, как и для любой терминальной сессии.
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
* Подключение к запущенной сесии: `screen -r <ИДЕНТИФИКАТОР СЕССИИ>`. Для примера выше `<ИДЕНТИФИКАТОР СЕССИИ>` будет `166154.pts-0.guide-bastion`.
{% endofftopic %}
{% endalert %}

Выполните команду:

```bash
d8 mirror pull --source="registry.deckhouse.ru/deckhouse/ee" --license="<ЛИЦЕНЗИОННЫЙ КЛЮЧ>" $(pwd)/d8.tar
```

Не забудьте изменить в команде следующие параметры:

* `<ЛИЦЕНЗИОННЫЙ КЛЮЧ>` — ваш лицензионный ключ;
* в адресе registry путь к вашей редакции DKP:
  * `be` — для Basic Edition;
  * `se` — для Standart Edition;
  * `se-plus` — для Standart Edition+;
  * `ee` — для Enterprise Edition;

В зависимости от скорости интернет-соединения процесс может занять до 30-40 минут.

{% offtopic title="Пример успешного завершения процесса скачивания образов..." %}
```text
Dec 11 15:06:42.280 INFO  ║ Packing module-csi-scsi-generic.tar
Dec 11 15:06:56.770 INFO  ║ Packing module-operator-ceph.tar
Dec 11 15:07:04.748 INFO  ║ Packing module-secrets-store-integration.tar
Dec 11 15:07:11.936 INFO  ║ Packing module-stronghold.tar
Dec 11 15:07:18.426 INFO  ║ Packing module-development-platform.tar
Dec 11 15:07:20.280 INFO  ║ Packing module-sdn.tar
Dec 11 15:07:24.318 INFO  ║ Packing module-prompp.tar
Dec 11 15:07:27.777 INFO  ║ Packing module-storage-volume-data-manager.tar
Dec 11 15:07:28.354 INFO  ║ Packing module-sds-node-configurator.tar
Dec 11 15:07:29.115 INFO  ║ Packing module-sds-replicated-volume.tar
Dec 11 15:08:00.529 INFO  ║ Packing module-csi-yadro-tatlin-unified.tar
Dec 11 15:08:07.376 INFO  ║ Packing module-neuvector.tar
Dec 11 15:08:30.766 INFO  ╚ Pull Modules succeeded in 27m55.883250757s
```
{% endofftopic %}

Проверьте, что образы скачались:

```console
$ ls -lh
total 650M
drwxr-xr-x 2 ubuntu ubuntu 4.0K Dec 11 15:08 d8.tar
```

Теперь запустим команду, которая зальёт скачанные образы в созданный ранее приватный registry:

```bash
d8 mirror push $(pwd)/d8.tar 'harbor.local:443/deckhouse/ee' --registry-login='deckhouse' --registry-password='Flant12345678' --tls-skip-verify
```

> Ключ `--tls-skip-verify` указывает утилите, что мы доверяем сертификату и пропускаем его проверку.

Скачанный архив распакуется и зальётся в registry. Процесс пройдёт чуть быстрее, чем выкачиваение образов на шаге ранее, т.к. мы работаем с локальным репозиторием, в нашем случае она заняла где-то 15 минут.

{% offtopic title="Пример успешного завершения процесса заливки образов..." %}
```text
Dec 11 18:25:32.350 INFO  ║ Pushing harbor.local:443/deckhouse/ee/modules/virtualization/release
Dec 11 18:25:32.351 INFO  ║ [1 / 7] Pushing image harbor.local:443/deckhouse/ee/modules/virtualization/release:alpha
Dec 11 18:25:32.617 INFO  ║ [2 / 7] Pushing image harbor.local:443/deckhouse/ee/modules/virtualization/release:beta
Dec 11 18:25:32.760 INFO  ║ [3 / 7] Pushing image harbor.local:443/deckhouse/ee/modules/virtualization/release:early-access
Dec 11 18:25:32.895 INFO  ║ [4 / 7] Pushing image harbor.local:443/deckhouse/ee/modules/virtualization/release:rock-solid
Dec 11 18:25:33.081 INFO  ║ [5 / 7] Pushing image harbor.local:443/deckhouse/ee/modules/virtualization/release:stable
Dec 11 18:25:33.142 INFO  ║ [6 / 7] Pushing image harbor.local:443/deckhouse/ee/modules/virtualization/release:v1.1.3
Dec 11 18:25:33.213 INFO  ║ [7 / 7] Pushing image harbor.local:443/deckhouse/ee/modules/virtualization/release:v1.2.2
Dec 11 18:25:33.414 INFO  ║ Pushing module tag for virtualization
Dec 11 18:25:33.837 INFO  ╚ Push module: virtualization succeeded in 43.313801312s
Dec 11 18:25:33.837 INFO   Modules pushed: code, commander-agent, commander, console, csi-ceph, csi-hpe, csi-huawei, csi-netapp, csi-nfs, csi-s3, csi-scsi-generic, csi-yadro-tatlin-unified, development-platform, managed-postgres, neuvector, observability-platform, observability, operator-argo, operator-ceph, operator-postgres, payload-registry, pod-reloader, prompp, runtime-audit-engine, sdn, sds-local-volume, sds-node-configurator, sds-replicated-volume, secrets-store-integration, snapshot-controller, state-snapshotter, static-routing-manager, storage-volume-data-manager, stronghold, virtualization
```
{% endofftopic %}

Можно проверить, что все образы залиты в registry, открыв проект `deckhouse` в веб-интерфейсе Harbor:

<div style="text-align: center;">
<img src="/images/guides/install_to_private_environment/harbor_state_with_images.png" alt="Главная страница Harbor...">
</div>

Образы закачаны и готовы к использованию! 🎉

## Установка прокси-сервера

Для того, чтобы находящиеся в закрытом окружении ВМ будущих узлов кластера могли достучаться до внешних репозиториев пакетов (чтобы установить необходимые для работы DKP пакеты), нужно поднять на машине-бастионе прокси-сервер, через который будет осуществляться этот доступ.

Можно использовать любой прокси-сервер, подходящий под ваши требования или пристрастия. Мы для примера воспользуемся [Squid](https://www.squid-cache.org/).

Развернуть его на машине можно также в контейнере, выполнив команду:

```bash
docker run -d --name squid -p 3128:3128 ubuntu/squid
```
{% offtopic title="Пример успешного выполнения команды..." %}
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
{% endofftopic %}

Убедимся, что Squid запустился:

```console
059b21fddbd2   ubuntu/squid                          "entrypoint.sh -f /e…"   About a minute ago   Up About a minute     0.0.0.0:3128->3128/tcp, [::]:3128->3128/tcp                                          squid
```

В списке запущенных контейнеров должен быть контейнер с соответствующем именем.

## Вход в registry для запуска установщика

Теперь нужно залогиниться в наш registry, чтобы docker смог выкачать из него образ установщика [dhctl](../documentation/v1/installing/):

```bash
docker login harbor.local
```

{% offtopic title="Пример успешного выполнения команды..." %}
```text
$ docker login harbor.local
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
Во время установки в качестве container runtime по умолчанию на узлах кластера будет использоваться `ContainerdV2`.
Чтобы использовать `ContainerdV2` в качестве container runtime на узлах кластера, они должны соответствовать следующим требованиям:

- поддержка `CgroupsV2`;
- systemd версии `244`;
- поддержка модуля ядра `erofs`.

Некоторые дистрибутивы (например, Astra Linux 1.7.4) не соответствуют этим требованиям, и ОС на узлах необходимо привести в соответствие требованиям перед установкой Deckhouse Kubernetes Platform. Подробнее — [в документации](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-defaultcri).
{% endalert %}

Виртуальные машины под будущие узлы кластера должны соответствовать следующиюм требованиям:

- не менее 4 ядер CPU;
- не менее 8 ГБ RAM;
- не менее 60 ГБ дискового пространства на быстром диске (400+ IOPS);
- [поддерживаемая ОС](../documentation/v1/reference/supported_versions.html#linux);
- ядро Linux версии `5.8` или новее;
- **уникальный hostname** в пределах серверов (виртуальных машин) кластера;
- наличие одного из пакетных менеджеров (`apt`/`apt-get`, `yum` или `rpm`).

  **Важно.** — в РЕД ОС по умолчанию могут отсутствовать `yum` и `which`, поэтому их следует заранее установить;
- установленный Python;
- доступ до проксирующего registry или до приватного хранилища образов контейнеров с образами контейнеров Deckhouse;
- доступ к стандартным для используемой ОС репозиториям пакетов (через прокси-сервер или до внутреннего сервера-репозитория пакетов);
- SSH-доступ от машины-бастиона по ключу;
- сетевой доступ от машины-бастиона по порту <code>22/TCP</code>;
- на узле не должно быть установлено пакетов container runtime, например containerd или Docker.

{% alert level="warning" %}
Для правильного выбора ресурсов виртуальных машин знакомьтесь с [рекомендациями по подготовке к production](/products/kubernetes-platform/guides/production.html) и [инструкцией](/products/kubernetes-platform/guides/hardware-requirements.html) по выбору типов и количества узлов кластера, а также ресурсов для них, в зависимости от ваших требований к эксплуатации будущего кластера.
{% endalert %}

### Настройка доступа к бастиону

Для того, чтобы ВМ, на которых будут разворачиваться master и worker-узлы, могли получить доступ к созданному приватному хранилищу, необходимо настроить на них соответствие доменного имени `harbor.local` внутреннему IP-адресу бастион-машины в приватной сети.

Для этого поотчередно подлючитесь к каждой из машин и добавьте соответствующую запись в файлы `/etc/hosts` и файл облачного шаблона:

{% offtopic title="Как подключиться к машине без внешнего доступа..." %}
Для подключения по SSH к машине, к которой нет внешнего доступа, можно воспользоваться всё тем же бастион-хостом.

Есть два способа подключения:

1. *Подключение через джамп-хост.* Для этого выполните команду:
   ```bash
   ssh -J ubuntu@<BASTION_IP> ubuntu@<VM_IP>
   ```
   В таком режиме сначала будет выполнено подключение к бастион хосту, а затем с него к следующей машине с использованием того же ключа.
2. *Подключение в режиме агента.* Подключитеси к бастион-хосту командой:
   ```bash
   ssh -A ubuntu@<BASTION_IP>
   ```
   Дальше можно выполнять подключение к следующим машинам:
   ```bash
   ssh ubuntu@<VM_IP>
   ```
{% endofftopic %}

```console
<INTERNAL-IP-ADDRESS> harbor.local proxy.local
```

> Не забудьте заменить в команде `<INTERNAL-IP-ADDRESS>` на реальный внутренний адрес бастион-машины.

### Создание пользователя

Для выполнения установки DKP нужно создать на машинах пользователя, под которым будет выполняться подключение и установка платформы.

Выполните команды на каждоый из ВМ (выполнять команды нужно от root-пользователя:

```console
useradd deckhouse -m -s /bin/bash -G sudo
echo 'deckhouse ALL=(ALL) NOPASSWD: ALL' | sudo EDITOR='tee -a' visudo
mkdir /home/deckhouse/.ssh
export KEY='ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQCs1N5nn7gcGKps2OLeTCH6HN2p3TIpHcy1C9CQBu2yh7k+pn0i03SMadFEPoe0so4G3ZwmwGpV9GKcmrnITX/18ZC1STLGJimHBGXimev37qI6/D5OabJ86Eq/p0ixqCdfBErJ7/H/ozLy3X1CKThn/5iotibP3vw+jzFWnmeLfybgZ3003q9T4A1U4Z9kUtLkUMyxz3a0ZxRVrV0/iASdow2Lckc6B92BMAdvII0JRT8eFnpC9tQirSUAUbtlXKGM29xAaQlYQfOm+8RuXkca07BJ1d39Yhrhj6A21gHdcUKYMT0iRs6R83URz36vc5FW0FU1e+DIvXUQ/1QZRQ6l39FsHTOAbNfCCGMJu2MIanrjgJAI0Wew00t+kPwHK/GgtzYG8Bx7YLJaDgfAH1ZBKsl9KxPD2kddt5S0xeYDo2l5/j7P3wmZ/x4yOhvmlCWsuuOIr3wpVXzdwZKU9gUQQRg3mUMxAxVazDrBDvhdUqoVubyqRUTfFWHyOlCw6hc= zhbert@MacBook-Pro-Konstantin.local'
echo $KEY >> /home/deckhouse/.ssh/authorized_keys
chown -R deckhouse:deckhouse /home/deckhouse
chmod 700 /home/deckhouse/.ssh
chmod 600 /home/deckhouse/.ssh/authorized_keys
```

