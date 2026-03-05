---
title: Installing DKP in a private environment
permalink: en/guides/private-environment.html
description: A guide to installing the Deckhouse Kubernetes Platform in a private environment
lang: en
layout: sidebar-guides
---

This guide describes how to deploy a Deckhouse Kubernetes Platform cluster in a private environment with no direct access to the DKP container image registry (`registry.deckhouse.io`) and to external deb/rpm package repositories used on nodes running [supported operating systems](../documentation/v1/reference/supported_versions.html#linux).

{% alert level="warning" %}
Note that installing DKP in a private environment is available in the following editions: SE, SE+, EE, CSE Lite (1.67), CSE Pro (1.67).
{% endalert %}

## Private environment specifics

Deploying in a private environment is almost the same as deploying [on bare metal](../gs/bm/step2.html).

Key specifics:

* Internet access for applications deployed in the private environment is provided through a proxy server, whose parameters must be specified in the cluster configuration.
* A container registry with DKP container images is deployed separately and must be reachable from within the private environment. The cluster is configured to use it and is granted the required access permissions.

All interactions with external resources are performed via a dedicated physical server or virtual machine called a Bastion host. The container registry and proxy server are deployed on the Bastion host, and all cluster management operations are performed from it.

Overall private environment diagram:

<img src="/images/gs/private-env-schema.png" alt="Deckhouse Kubernetes Platform deployment diagram in a private environment">

{% alert level="info" %}
The diagram also shows an internal OS package repository. It is required to install `curl` on the future cluster nodes if access to the official repositories is not available even through the proxy server.
{% endalert %}

## Infrastructure selection

This guide describes deploying a cluster in a private environment consisting of one master node and one worker node.

To perform the deployment, you will need:

- a personal computer from which the operations will be performed;
- a dedicated physical server or virtual machine (Bastion host) where the container registry and related components will be deployed;
- two physical servers or two virtual machines for the cluster nodes.

Server requirements:

* **Bastion**: at least 4 CPU cores, 8 GB RAM, and 150 GB on fast storage. This capacity is required because the Bastion host stores all DKP images needed for installation. Before being pushed to the private registry, the images are pulled from the public DKP registry to the Bastion host.
* **Cluster nodes**: the [resources for the future cluster nodes](./hardware-requirements.html#deciding-on-the-amount-of-resources-needed-for-nodes) should be selected based on the expected workload. For example, the minimum recommended configuration is 4 CPU cores, 8 GB RAM, and 60 GB on fast storage (400+ IOPS) per node.

## Preparing a private container registry

{% alert level="warning" %}
DKP supports only the Bearer token authentication scheme for container registries.

Compatibility has been tested and is guaranteed for the following container registries: [Nexus](https://github.com/sonatype/nexus-public), [Harbor](https://github.com/goharbor/harbor), [Artifactory](https://jfrog.com/artifactory/), [Docker Registry](https://docs.docker.com/registry/), and [Quay](https://quay.io/).
{% endalert %}

### Installing Harbor

In this guide, [Harbor](https://goharbor.io/) is used as the private registry. It supports policy configuration and role-based access control (RBAC), scans images for vulnerabilities, and allows you to mark trusted artifacts. Harbor is a CNCF project.

Install the latest Harbor release from the project’s [GitHub releases page](https://github.com/goharbor/harbor/releases). Download the installer archive from the desired release, selecting the asset with `harbor-offline-installer` in its name.

<div style="text-align: center;">
<img src="/images/guides/install_to_private_environment/download-harbor-installer.png" alt="Downloading the Harbor installer...">
</div>

Copy the download URL. For example, for `harbor-offline-installer-v2.14.1.tgz` it will look like this: `https://github.com/goharbor/harbor/releases/download/v2.14.1/harbor-offline-installer-v2.14.1.tgz`.

Connect to the Bastion host via SSH and download the archive using any convenient method.

{% offtopic title="How to download the archive with wget..." %}
Run the command (use the current URL):

```console
wget https://github.com/goharbor/harbor/releases/download/v2.14.1/harbor-offline-installer-v2.14.1.tgz
```

{% endofftopic %}

{% offtopic title="How to download the archive with curl..." %}
Run the command (use the current URL):

```console
curl -O https://github.com/goharbor/harbor/releases/download/v2.14.1/harbor-offline-installer-v2.14.1.tgz
```

{% endofftopic %}

Extract the downloaded archive (specify the archive name):

```console
tar -zxf ./harbor-offline-installer-v2.14.1.tgz
```

The extracted `harbor` directory contains the files required for installation.

Before deploying the registry, generate a self-signed TLS certificate.

{% alert level="info" %}
Due to access restrictions in a private environment, it is not possible to obtain certificates from services such as Let's Encrypt, since the service will not be able to perform the validation required to issue a certificate.

There are several ways to generate certificates. This guide describes one of them. If needed, use any other suitable approach or provide an existing certificate.
{% endalert %}

Create the `certs` directory inside the `harbor` directory:

```bash
cd harbor/
mkdir certs
```

Generate certificates for external access using the following commands:

```bash
openssl ecparam -name prime256v1 -genkey -out ca.key 4096
```

```bash
openssl req -x509 -new -nodes -sha512 -days 3650 -subj "/C=US/ST=California/L=SanFrancisco/O=example/OU=Personal/CN=myca.local" -key ca.key -out ca.crt
```

Generate certificates for the internal domain name `harbor.local` so that the Bastion host can be accessed securely from within the private network:

```bash
openssl ecparam -name prime256v1 -genkey -out harbor.local.key
```

```bash
openssl req -sha512 -new -subj "/C=US/ST=California/L=SanFrancisco/O=example/OU=Personal/CN=harbor.local" -key harbor.local.key -out harbor.local.csr
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

{% alert level="warning" %}
Do not forget to replace `<INTERNAL_IP_ADDRESS>` with the Bastion host’s internal IP address. This address will be used to access the container registry from within the private network. The `harbor.local` domain name is also associated with this address.
{% endalert %}

```bash
openssl x509 -req -sha512 -days 3650 -extfile v3.ext -CA ca.crt -CAkey ca.key -CAcreateserial -in harbor.local.csr -out harbor.local.crt
```

```bash
openssl x509 -inform PEM -in harbor.local.crt -out harbor.local.cert
```

Verify that all certificates were created successfully:

```bash
ls -la
```

{% offtopic title="Example command output..." %}

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

Next, configure Docker to work with the private container registry over TLS. To do this, create the `harbor.local` directory under `/etc/docker/certs.d/`:

```bash
sudo mkdir -p /etc/docker/certs.d/harbor.local
```

> The `-p` option tells `mkdir` to create parent directories if they do not exist (in this case, the `certs.d` directory).

Copy the generated certificates into it:

```bash
cp ca.crt /etc/docker/certs.d/harbor.local/
cp harbor.local.cert /etc/docker/certs.d/harbor.local/
cp harbor.local.key /etc/docker/certs.d/harbor.local/
```

These certificates will be used when accessing the registry via the `harbor.local` domain name.

Copy the configuration file template that comes with the installer:

```bash
cp harbor.yml.tmpl harbor.yml
```

Update the following parameters in `harbor.yml`:

* `hostname`: set to `harbor.local` (the certificates were generated for this name);
* `certificate`: specify the path to the generated certificate in the `certs` directory (for example, `/home/ubuntu/harbor/certs/harbor.local.crt`);
* `private_key`: specify the path to the private key (for example, `/home/ubuntu/harbor/certs/harbor.local.key`);
* `harbor_admin_password`: set a password for accessing the web UI.

Save the file.

{% offtopic title="Example configuration file..." %}

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

Install [Docker](https://docs.docker.com/engine/install/) and the [Docker Compose](https://docs.docker.com/compose/install/#plugin-linux-only) plugin on the Bastion host.

Run the installation script:

```bash
./install.sh
```

Harbor installation will start: the required images will be prepared and the containers will be started.

{% offtopic title="Successful installation log..." %}

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

Verify that Harbor is running successfully:

```bash
docker ps
```

{% offtopic title="Example command output..." %}

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

Add an entry to `/etc/hosts` that maps the `harbor.local` domain name to the Bastion host’s localhost address so that you can access Harbor by this name from the Bastion host itself:

```bash
127.0.0.1 localhost harbor.local
```

{% alert level="warning" %}
In some cloud providers (for example, Yandex Cloud), changes to `/etc/hosts` may be reverted after a virtual machine reboot. A note about this is typically shown at the beginning of the `/etc/hosts` file.

```text
# Your system has configured 'manage_etc_hosts' as True.
# As a result, if you wish for changes to this file to persist
# then you will need to either
# a.) make changes to the master file in /etc/cloud/templates/hosts.debian.tmpl
# b.) change or remove the value of 'manage_etc_hosts' in
#     /etc/cloud/cloud.cfg or cloud-config from user-data
```

If your provider uses the same mechanism, apply the corresponding changes to the template file referenced in the comment so that the settings persist after reboot.
{% endalert %}

Harbor installation is now complete! 🎉

### Configuring Harbor

Create a project and a user that will be used to work with this project.

Open the Harbor web UI at `harbor.local`:

<div style="text-align: center;">
<img src="/images/guides/install_to_private_environment/harbor_main_page.png" alt="Harbor main page...">
</div>

{% alert level="info" %}
To access Harbor by the `harbor.local` domain name from your workstation, add a corresponding entry to `/etc/hosts` and point it to the Bastion host IP address.
{% endalert %}

To sign in, use the username and password specified in the `harbor.yml` configuration file.

<div style="text-align: center;">
<img src="/images/guides/install_to_private_environment/harbor_main_dashboard.png" alt="Harbor dashboard...">
</div>

{% alert level="info" %}
Your browser may warn about the self-signed certificate and mark the connection as “not secure”. In a private environment this is expected and acceptable. If needed, add the certificate to the trusted certificate store of your browser or operating system to remove the warning.
{% endalert %}

Create a new project. Click **New Project** and enter `deckhouse` as the project name. Leave the remaining settings unchanged.

<div style="text-align: center;">
<img src="/images/guides/install_to_private_environment/harbor_new_project.png" alt="Creating a project in Harbor...">
</div>

Create a new user for this project. In the left menu, go to **Users** and click **New User**:

<div style="text-align: center;">
<img src="/images/guides/install_to_private_environment/harbor_create_new_user.png" alt="Creating a user in Harbor...">
</div>

Specify the username, email address, and password:

<div style="text-align: center;">
<img src="/images/guides/install_to_private_environment/harbor_creating_user.png" alt="Entering user details in Harbor...">
</div>

Add the created user to the `deckhouse` project: go back to **Projects**, open the `deckhouse` project, then open the **Members** tab and click **User** to add a member.

<div style="text-align: center;">
<img src="/images/guides/install_to_private_environment/harbor_adding_user_to_project.png" alt="Adding a user to a Harbor project...">
</div>

Keep the default role: **Project Admin**.

<div style="text-align: center;">
<img src="/images/guides/install_to_private_environment/harbor_new_project_user.png" alt="Selecting a user role in a Harbor project...">
</div>

Harbor configuration is now complete! 🎉

## Copying DKP images to a private container registry

The next step is to copy DKP component images from the public Deckhouse Kubernetes Platform registry to Harbor.

{% alert level="info" %}
To proceed with the steps in this section, you need the Deckhouse CLI utility. Install it on the Bastion host according to the [documentation](../documentation/v1/cli/d8/).
{% endalert %}

{% alert level="warning" %}
Downloading images takes a significant amount of time. To avoid losing progress if the SSH connection is interrupted, run the commands in a `tmux` or `screen` session. If the connection drops, you can reattach to the session and continue without starting over. Both utilities are typically available in Linux distribution repositories and can be installed using the package manager.

{% offtopic title="How to use tmux..." %}
* Start a session with `tmux`.
* Detach from the session by pressing `Ctrl + b`, then `d`. The session will keep running, and the processes started in it will continue to run. To exit the session, use `Ctrl + d`.
* List running sessions with `tmux ls`:

  ```console
  $ tmux ls
  0: 1 windows (created Thu Dec 11 13:52:41 2025)
  ```

* Reattach to a running session: `tmux attach -t <SESSION_ID>`. In the example above, the `<SESSION_ID>` is `0`.
{% endofftopic %}

{% offtopic title="How to use screen..." %}
* Start a session with `screen`.
* Detach from the session by pressing `Ctrl + a`, then `d` (while holding `Ctrl`). The session will keep running, and the processes started in it will continue to run. To exit the session, use `Ctrl + d`.
* List running sessions with `screen -r`:

  ```console
  $ screen -r
  There are several suitable screens on:
          1166154.pts-0.guide-bastion     (12/11/25 14:00:26)     (Detached)
          1165806.pts-0.guide-bastion     (12/11/25 13:59:35)     (Detached)
          1165731.pts-0.guide-bastion     (12/11/25 13:59:24)     (Detached)
          1165253.pts-0.guide-bastion     (12/11/25 13:58:16)     (Detached)
  Type "screen [-d] -r [pid.]tty.host" to resume one of them.
  ```

* Reattach to a running session: `screen -r <SESSION_ID>`. In the example above, the `<SESSION_ID>` is `166154.pts-0.guide-bastion`.
{% endofftopic %}
{% endalert %}

Run the following command to download all required images and pack them into the `d8.tar` archive. Specify your license key and the DKP edition (for example, `se-plus` for Standard Edition+, `ee` for Enterprise Edition, and so on):

```bash
d8 mirror pull --source="registry.deckhouse.io/deckhouse/<DKP_EDITION>" --license="<LICENSE_KEY>" $(pwd)/d8.tar
```

Depending on your Internet connection speed, the process may take 30 to 40 minutes.

{% offtopic title="Example of a successful image download completion..." %}

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

Verify that the archive has been created:

```console
$ ls -lh
total 650M
drwxr-xr-x 2 ubuntu ubuntu 4.0K Dec 11 15:08 d8.tar
```

Push the downloaded images to the private registry (specify the DKP edition and the credentials of the user created in Harbor):

```bash
d8 mirror push $(pwd)/d8.tar 'harbor.local:443/deckhouse/<DKP_EDITION>' --registry-login='deckhouse' --registry-password='<PASSWORD>' --tls-skip-verify
```

> The `--tls-skip-verify` flag tells the utility to trust the registry certificate and skip its verification.

The archive will be unpacked and the images will be pushed to the registry. This step is usually faster than downloading because it operates on a local archive. It typically takes about 15 minutes.

{% offtopic title="Example of a successful image push completion..." %}

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
Dec 11 18:25:33.837 INFO   Modules pushed: code, commander-agent, commander, console, csi-ceph, csi-hpe, csi-huawei, csi-netapp, csi-nfs, csi-s3, csi-scsi-generic, csi-yadro-tatlin-unified, development-platform, managed-postgres, neuvector, observability-platform, observability, operator-argo, operator-ceph, operator-postgres,
 payload-registry, pod-reloader, prompp, runtime-audit-engine, sdn, sds-local-volume, sds-node-configurator, sds-replicated-volume, secrets-store-integration, snapshot-controller, state-snapshotter, static-routing-manager, storage-volume-data-manager, stronghold, virtualization
```

{% endofftopic %}

To verify that the images have been pushed, open the `deckhouse` project in the Harbor web UI.

<div style="text-align: center;">
<img src="/images/guides/install_to_private_environment/harbor_state_with_images.png" alt="Harbor project page...">
</div>

The images are now available and ready to use! 🎉

## Installing a proxy server

To allow the future cluster nodes located in the private environment to access external package repositories (to install packages required for DKP), deploy a proxy server on the Bastion host through which this access will be provided.

You can use any proxy server that meets your requirements. In this example, we will use [Squid](https://www.squid-cache.org/).

Deploy Squid on the Bastion host as a container:

```bash
docker run -d --name squid -p 3128:3128 ubuntu/squid
```

{% offtopic title="Example of a successful command execution..." %}

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

Verify that the container is running:

```console
059b21fddbd2   ubuntu/squid                          "entrypoint.sh -f /e…"   About a minute ago   Up About a minute     0.0.0.0:3128->3128/tcp, [::]:3128->3128/tcp                                          squid
```

The list of running containers must include a container named `squid`.

## Signing in to the registry to run the installer

Sign in to the Harbor registry so that Docker can pull the [dhctl](../documentation/v1/installing/) installer image from it:

```bash
docker login harbor.local
```

{% offtopic title="Example of a successful command execution..." %}

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

## Preparing VMs for the future nodes

### VM requirements

{% alert level="warning" %}
During installation, `ContainerdV2` is used as the default container runtime on cluster nodes. To use it, the nodes must meet the following requirements:

- `CgroupsV2` support;
- systemd version `244`;
- support for the `erofs` kernel module.

Some distributions do not meet these requirements. In this case, you must bring the OS on the nodes into compliance before installing Deckhouse Kubernetes Platform. For details, see the [documentation](../documentation/v1/reference/api/cr.html#clusterconfiguration-defaultcri).
{% endalert %}

Servers intended for future cluster nodes must meet the following requirements:

- at least 4 CPU cores;
- at least 8 GB RAM;
- at least 60 GB of disk space on fast storage (400+ IOPS);
- a [supported OS](../documentation/v1/reference/supported_versions.html#linux);
- Linux kernel version `5.8` or later;
- a unique hostname across all cluster servers (physical servers and virtual machines);
- one of the package managers available (`apt`/`apt-get`, `yum`, or `rpm`).

{% alert level="warning" %}
For proper resource sizing, refer to the [production preparation recommendations](/products/kubernetes-platform/guides/production.html) and the [hardware requirements guide](/products/kubernetes-platform/guides/hardware-requirements.html) for selecting node types, the number of nodes, and node resources based on your expected workload and operational requirements.
{% endalert %}

### Configuring access to the Bastion host

To allow the servers where the master and worker nodes will be deployed to access the private registry, configure them to resolve the `harbor.local` domain name to the Bastion host’s internal IP address in the private network.

To do this, connect to each server one by one and add an entry to `/etc/hosts` (and, if necessary, also to the cloud template file if your provider manages `/etc/hosts`).

{% offtopic title="How to connect to a server without external access..." %}
To connect via SSH to a server without external access, you can use the Bastion host as a jump host.

There are two ways to connect:

1. *Connect via a jump host.* Run the command:

   ```bash
   ssh -J ubuntu@<BASTION_IP> ubuntu@<NODE_IP>
   ```

   In this mode, you first connect to the Bastion host, and then connect through it to the target server using the same SSH key.
1. *Connect with agent forwarding.* Connect to the Bastion host using:

   ```bash
   ssh -A ubuntu@<BASTION_IP>
   ```

   > Note: for this to work, you may need to start ssh-agent and add your key with `ssh-add` on the workstation from which you run the command.

   Then connect to the target servers:

   ```bash
   ssh ubuntu@<NODE_IP>
   ```

{% endofftopic %}

```console
<INTERNAL-IP-ADDRESS> harbor.local proxy.local
```

> Do not forget to replace `<INTERNAL-IP-ADDRESS>` with the Bastion host’s actual internal IP address.

### Creating a user for the master node

To install DKP, create a user on the future master node that will be used to connect to the node and perform the platform installation.

Run the commands as `root` (substitute the public part of your SSH key):

```console
useradd deckhouse -m -s /bin/bash -G sudo
echo 'deckhouse ALL=(ALL) NOPASSWD: ALL' | sudo EDITOR='tee -a' visudo
mkdir /home/deckhouse/.ssh
export KEY='ssh-ed25519 AAAAB3NzaC1yc2EAAAADA...'
echo $KEY >> /home/deckhouse/.ssh/authorized_keys
chown -R deckhouse:deckhouse /home/deckhouse
chmod 700 /home/deckhouse/.ssh
chmod 600 /home/deckhouse/.ssh/authorized_keys
```

{% offtopic title="How to obtain the public part of the key..." %}
You can get the public part of the key by running `cat ~/.ssh/<SSH_PUBLIC_KEY_FILE>`.

Replace `<SSH_PUBLIC_KEY_FILE>` here with the name of your public key. For example, for a key with RSA encryption, it will be `id_rsa.pub`, and for a key with ED25519 encryption, it will be with `id_ed25519.pub`.
{% endofftopic %}

As a result of these commands:

* a new `deckhouse` user is created and added to the `sudo` group;
* passwordless privilege escalation is configured;
* the public SSH key is added so you can log in to the server as this user.

Verify that you can connect as the new user:

```bash
ssh -J ubuntu@<BASTION_IP> deckhouse@<NODE_IP>
```

If the login succeeds, the user has been created correctly.

## Preparing the configuration file

The configuration file for deployment in a private environment differs from the configuration used for [bare metal](../gs/bm/step2.html) installation in a few parameters. Take the `config.yml` file from [step 4](../gs/bm/step4.html) of the bare metal installation guide and make the following changes:

* In the `deckhouse` section of the `ClusterConfiguration` block, change the container registry settings from the public Deckhouse Kubernetes Platform registry to your private registry:

  ```yaml
  # Proxy server settings.
  proxy:
    httpProxy: http://proxy.local:3128
    httpsProxy: https://proxy.local:3128
    noProxy: ["harbor.local", "proxy.local", "10.128.0.8", "10.128.0.32", "10.128.0.18"]
  ```

   The following parameters are specified here:
  * the HTTP and HTTPS proxy server addresses deployed on the Bastion host;
  * the list of domains and IP addresses that will **not** be routed through the proxy server (internal domain names and internal IP addresses of all servers).

* In the `InitConfiguration` section, add the parameters required to access the registry:

  ```yaml
  deckhouse:
    # Docker registry address that hosts Deckhouse images (specify the DKP edition).
    imagesRepo: harbor.local/deckhouse/<DKP_EDITION>
    # Base64-encoded Docker registry credentials.
    # You can obtain it by running: `cat .docker/config.json | base64`.
    registryDockerCfg: <DOCKER_CFG_BASE64>
    # Registry access scheme (HTTP or HTTPS).
    registryScheme: HTTPS
    # The root CA certificate created earlier.
    # You can obtain it by running: `cat harbor/certs/ca.crt`.
    registryCA: |
      -----BEGIN CERTIFICATE-----
      ...
      -----END CERTIFICATE-----
  ```

* In the [releaseChannel](/modules/deckhouse/configuration.html#parameters-releasechannel) parameter of the `deckhouse` ModuleConfig, set the value to `Stable` to use the stable [update channel](../documentation/v1/reference/release-channels.html).
* In the [global](../documentation/v1/reference/api/global.html) ModuleConfig, configure the use of self-signed certificates for cluster components and specify the domain name template for system applications using the `publicDomainTemplate` parameter:

  ```yaml
  settings:
    modules:
      # A template used to construct the addresses of system applications in the cluster.
      # For example, with %s.example.com, Grafana will be available at 'grafana.example.com'.
      # The domain MUST NOT match the value specified in the clusterDomain parameter of the ClusterConfiguration resource.
      # You can set your own value right away, or follow the guide and change it after installation.
      publicDomainTemplate: "%s.test.local"
      # The HTTPS implementation method used by Deckhouse modules.
      https:
        certManager:
          # Use self-signed certificates for Deckhouse modules.
          clusterIssuerName: selfsigned
  ```

* In the `user-authn` ModuleConfig, set the [dexCAMode](/modules/user-authn/configuration.html#parameters-controlplaneconfigurator-dexcamode) parameter to `FromIngressSecret`:

  ```yaml
  settings:
    controlPlaneConfigurator:
      dexCAMode: FromIngressSecret
  ```

* Enable and configure the [cert-manager](/modules/cert-manager/) module and disable the use of Let's Encrypt:

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

* In the [internalNetworkCIDRs](../documentation/v1/reference/api/cr.html#staticclusterconfiguration-internalnetworkcidrs) parameter of StaticClusterConfiguration, specify the subnet for the cluster nodes’ internal IP addresses. For example:

  ```yaml
  internalNetworkCIDRs:
    - 10.128.0.0/24
  ```

{% offtopic title="Full configuration file example..." %}

```yaml
# Cluster-wide settings.
# https://deckhouse.io/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
# Cluster Pod address space.
# You may want to change this. Make sure it does not overlap with serviceSubnetCIDR and internalNetworkCIDRs.
podSubnetCIDR: 10.111.0.0/16
# Cluster Service address space.
# You may want to change this. Make sure it does not overlap with podSubnetCIDR and internalNetworkCIDRs.
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "Automatic"
# Cluster domain.
clusterDomain: "cluster.local"
# The default container runtime type used on cluster nodes (in NodeGroups).
defaultCRI: "ContainerdV2"
# Proxy server settings.
proxy:
  httpProxy: http://proxy.local:3128
  httpsProxy: https://proxy.local:3128
  noProxy: ["harbor.local", "proxy.local", "10.128.0.8", "10.128.0.32", "10.128.0.18"]
---
# Initial cluster bootstrap settings for Deckhouse.
# https://deckhouse.io/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  # Docker registry address that hosts Deckhouse images.
  imagesRepo: harbor.local/deckhouse/ee
  # Docker registry credentials string.
  registryDockerCfg: <DOCKER_CFG_BASE64>
  # Registry access scheme (HTTP or HTTPS).
  registryScheme: HTTPS
  # Root CA certificate used to validate the registry certificate (if the registry uses a self-signed certificate).
  registryCA: |
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
---
# deckhouse module settings.
# https://deckhouse.io/modules/deckhouse/configuration.html
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
# Global Deckhouse settings.
# https://deckhouse.io/products/kubernetes-platform/documentation/v1/reference/api/global.html#%D0%BF%D0%B0%D1%80%D0%B0%D0%BC%D0%B5%D1%82%D1%80%D1%8B
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 2
  settings:
    modules:
      # A template used to construct the addresses of system applications in the cluster.
      # For example, with %s.test.local, Grafana will be available at 'grafana.test.local'.
      # The domain MUST NOT match the value specified in the clusterDomain parameter of the ClusterConfiguration resource.
      # You can set your own value right away, or follow the guide and change it after installation.
      publicDomainTemplate: "%s.test.local"
      # The HTTPS implementation method used by Deckhouse modules.
      https:
        certManager:
          # Use self-signed certificates for Deckhouse modules.
          clusterIssuerName: selfsigned
---
# user-authn module settings.
# https://deckhouse.io/modules/user-authn/configuration.html
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
    # Enable access to the Kubernetes API server via Ingress.
    # https://deckhouse.io/modules/user-authn/configuration.html#parameters-publishapi
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
# cni-cilium module settings.
# https://deckhouse.io/modules/cni-cilium/configuration.html
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cni-cilium
spec:
  version: 1
  # Enable the cni-cilium module.
  enabled: true
  settings:
    # cni-cilium module settings.
    # https://deckhouse.io/modules/cni-cilium/configuration.html
    tunnelMode: VXLAN
---
# Static cluster settings.
# https://deckhouse.io/products/kubernetes-platform/documentation/v1/reference/api/cr.html#staticclusterconfiguration
apiVersion: deckhouse.io/v1
kind: StaticClusterConfiguration
# A list of internal node networks (for example, '10.0.4.0/24') used for communication between Kubernetes components
# (kube-apiserver, kubelet, etc.).
# Specify this if you use the virtualization module or if cluster nodes have more than one network interface.
# If cluster nodes use only one interface, you can omit the StaticClusterConfiguration resource.
internalNetworkCIDRs:
  - 10.128.0.0/24
```

{% endofftopic %}

The installation configuration file is ready.

## Installing DKP

Copy the prepared configuration file to the Bastion host (for example, to `~/deckhouse`). Change to that directory and start the installer using the following command:

```bash
docker run --pull=always -it -v "$PWD/config.yml:/config.yml" -v "$HOME/.ssh/:/tmp/.ssh/" --network=host -v "$PWD/dhctl-tmp:/tmp/dhctl" harbor.local/deckhouse/<DKP_EDITION>/install:stable bash
```

{% offtopic title="If you get the `509: certificate signed by unknown authority` error..." %}
Even if the certificates are present in `/etc/docker/certs.d/harbor.local/`, Docker may still report that the certificate is signed by an unknown certificate authority (which is typical for self-signed certificates). In most cases, adding `ca.crt` to the system trusted certificate store and restarting Docker resolves the issue.
{% endofftopic %}

{% alert level="info" %}
If there is no local DNS server in the internal network and the domain names are configured in the Bastion host’s `/etc/hosts`, make sure to specify `--network=host` so that Docker can use those name resolutions.
{% endalert %}

After the image is pulled and the container starts successfully, you will see a shell prompt inside the container:

```console
[deckhouse] root@guide-bastion / #
```

Start the DKP installation with the following command (specify the master node’s internal IP address):

```bash
dhctl bootstrap --ssh-user=deckhouse --ssh-host=<master_ip> --ssh-agent-private-keys=/tmp/.ssh/<SSH_PRIVATE_KEY_FILE> \
  --config=/config.yml \
  --ask-become-pass
```

> Replace `<SSH_PRIVATE_KEY_FILE>` here with the name of your private key. For example, for a key with RSA encryption it can be `id_rsa`, and for a key with ED25519 encryption it can be `id_ed25519`.

The installation process may take up to 30 minutes depending on the network speed.

If the installation completes successfully, you will see the following message:

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

## Adding nodes to the cluster

Add a node to the cluster (for details on adding a static node, see the [documentation](../modules/node-manager/examples.html#adding-a-static-node-to-a-cluster)).

To do this, follow these steps:

* Configure a StorageClass for [local storage](../../../modules/local-path-provisioner/cr.html#localpathprovisioner) by running the following command on the master node:

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

* Set the created StorageClass as the default StorageClass. To do this, run the following command on the master node:

  ```bash
  sudo -i d8 k patch mc global --type merge \
    -p "{\"spec\": {\"settings\":{\"defaultClusterStorageClass\":\"localpath\"}}}"
  ```

* Create the `worker` NodeGroup and add a node using Cluster API Provider Static (CAPS):

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

* Generate an SSH key with an empty passphrase. Run the following command on the master node:

  ```bash
  ssh-keygen -t ed25519 -f /dev/shm/caps-id -C "" -N ""
  ```

* Create an [SSHCredentials](../../../../modules/node-manager/cr.html#sshcredentials) resource in the cluster. To do this, run the following command on the master node:

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

* Print the public part of the SSH key generated earlier (you will need it in the next step). Run the following command on the master node:

  ```console
  cat /dev/shm/caps-id.pub
  ```

* On the prepared server for the worker node, create the `caps` user. Run the following command, specifying the public SSH key obtained in the previous step:

  ```console
  # Specify the public part of the user's SSH key.
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

{% offtopic title="If you are using CentOS, Rocky Linux or ALT Linux..." %}
On RHEL-based operating systems (Red Hat Enterprise Linux), add the `caps` user to the `wheel` group. Run the following command, specifying the public SSH key obtained in the previous step:

```console
# Specify the public part of the user's SSH key.
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

* Create a [StaticInstance](../../../modules/node-manager/cr.html#staticinstance) for the node being added. Run the following command on the master node, specifying the IP address of the node you want to add:

  ```console
  # Specify the IP address of the node to be added to the cluster.
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

* Make sure all cluster nodes are in the `Ready` status:

  ```console
  $ sudo -i d8 k get no
  NAME               STATUS   ROLES                  AGE    VERSION
  d8cluster          Ready    control-plane,master   30m   v1.23.17
  d8cluster-worker   Ready    worker                 10m   v1.23.17
  ```

  It may take some time for all DKP components to start after the installation completes.

## Configuring the Ingress controller and creating a user

### Installing the ingress controller

Make sure the Kruise controller manager Pod of the [ingress-nginx](../../../modules/ingress-nginx/) module is running and in the `Running` status. To do this, run the following command on the master node:

```bash
$ sudo -i d8 k -n d8-ingress-nginx get po -l app=kruise
NAME                                         READY   STATUS    RESTARTS    AGE
kruise-controller-manager-7dfcbdc549-b4wk7   3/3     Running   0           15m
```

Create the `ingress-nginx-controller.yml` file on the master node containing the Ingress controller configuration:

```yaml
# NGINX Ingress controller parameters.
# https://deckhouse.io/modules/ingress-nginx/cr.html
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: nginx
spec:
  # The name of the IngressClass served by the NGINX Ingress controller.
  ingressClass: nginx
  # How traffic enters from outside the cluster.
  inlet: HostPort
  hostPort:
    httpPort: 80
    httpsPort: 443
  # Defines which nodes will run the component.
  # You may want to adjust this.
  nodeSelector:
    node-role.kubernetes.io/control-plane: ""
  tolerations:
    - effect: NoSchedule
      key: node-role.kubernetes.io/control-plane
      operator: Exists
```

Apply it by running the following command on the master node:

```bash
sudo -i d8 k create -f $PWD/ingress-nginx-controller.yml
```

Starting the Ingress controller after DKP installation may take some time. Before you proceed, make sure the Ingress controller is running (run the following command on the master node):

```console
$ sudo -i d8 k -n d8-ingress-nginx get po -l app=controller
NAME                                       READY   STATUS    RESTARTS   AGE
controller-nginx-r6hxc                     3/3     Running   0          5m
```

### Creating a user to access the cluster web-interface

Create the `user.yml` file on the master node containing the user account definition and access rights:

```yaml
# RBAC and authorization settings.
# https://deckhouse.io/modules/user-authz/cr.html#clusterauthorizationrule
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: admin
spec:
  # List of Kubernetes RBAC subjects.
  subjects:
    - kind: User
      name: admin@deckhouse.io
  # A predefined access level template.
  accessLevel: SuperAdmin
  # Allow the user to use kubectl port-forward.
  portForwarding: true
---
# Static user data.
# https://deckhouse.io/modules/user-authn/cr.html#user
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  # User email.
  email: admin@deckhouse.io
  # This is the password hash for 3xqgv2auys, generated just now.
  # Generate your own or use this one for testing purposes only:
  # echo -n '3xqgv2auys' | htpasswd -BinC 10 "" | cut -d: -f2 | tr -d '\n' | base64 -w0; echo
  # You may want to change it.
  password: 'JDJhJDEwJGtsWERBY1lxMUVLQjVJVXoxVkNrSU8xVEI1a0xZYnJNWm16NmtOeng5VlI2RHBQZDZhbjJH'
```

Apply it by running the following command on the master node:

```console
sudo -i d8 k create -f $PWD/user.yml
```

## Configuring DNS records

To access the cluster web UIs, configure the following domain names to resolve to the master node’s internal IP address (use DNS names according to the template specified in the [publicDomainTemplate](../documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate) parameter). Example for the `%s.example.com` DNS name template:

```text
api.example.com
code.example.com
commander.example.com
console.example.com
dex.example.com
documentation.example.com
grafana.example.com
hubble.example.com
istio.example.com
istio-api-proxy.example.com
kubeconfig.example.com
openvpn-admin.example.com
prometheus.example.com
status.example.com
tools.example.com
upmeter.example.com
```

You can do this either on an internal DNS server or by adding the mappings to `/etc/hosts` on the required computers.

To verify that the cluster has been deployed correctly and is functioning properly, open the Grafana web UI, which shows the cluster status. The Grafana address is generated from the `publicDomainTemplate` value. For example, if `%s.example.com` is used, Grafana will be available at `grafana.example.com`. Sign in using the credentials of the user you created earlier.
