---
title: "Установка"
permalink: ru/stronghold/documentation/admin/standalone/installation.html
lang: ru
---

Stronghold поддерживает мультисерверный режим для обеспечения высокой доступности (`HA`). Этот режим автоматически включается при использовании хранилища данных, которое его поддерживает, и защищает систему от сбоев за счёт работы нескольких серверов Stronghold.

Как определить, поддерживает ли ваше хранилище данных режим высокой доступности? Запустите сервер и проверьте, выводится ли сообщение `HA available` рядом с информацией о хранилище. Если да, то Stronghold будет автоматически использовать режим HA.

Для обеспечения высокой доступности один из узлов Stronghold получает блокировку в системе хранения данных. Затем этот узел становится активным, в то время как остальные узлы переходят в режим ожидания. Если резервные узлы получают запросы, они либо перенаправляют их, либо переадресовывают клиентов в соответствии с настройками и текущим состоянием кластера.

Для развёртывания Stronghold в режиме HA с интегрированным хранилищем Raft вам понадобятся как минимум три сервера Stronghold. В противном случае не получится достичь кворума и распечатать хранилище.

Предварительные требования:

* На сервер установлена поддерживаемая ОС (Ubuntu, RedOS, Astra Linux).
* На сервер скопирован дистрибутив Stronghold.
* Создан systemd-unit.
* Есть сертификаты для каждого узла в кластере Raft, а также сертификат корневого центра сертификации.

## Предварительная подготовка инфраструктуры

Сценарий ниже описывает процесс построения кластера Stronghold, который состоит из трёх узлов Stronghold — одного активного и двух резервных.

### Запуск через systemd-unit

{% alert level="warning" %}Все примеры предполагают, что существует пользователь `stronghold`, и сервис запущен под ним. Если вы хотите запустить сервис под другим пользователем, замените имя пользователя на необходимое.
{% endalert %}

Создайте файл `/etc/systemd/system/stronghold.service`:

```hcl
[Unit]
Description=Stronghold service
Documentation=https://deckhouse.ru/products/stronghold/
After=network.target

[Service]
Type=simple
ExecStart=/opt/stronghold/stronghold server -config=/opt/stronghold/config.hcl
ExecReload=/bin/kill -HUP $MAINPID
KillMode=process
Restart=on-failure
RestartSec=5
User=stronghold
Group=stronghold
LimitNOFILE=65536
CapabilityBoundingSet=CAP_IPC_LOCK
AmbientCapabilities=CAP_IPC_LOCK
SecureBits=noroot

[Install]
WantedBy=multi-user.target
```

Выполните команду `systemctl daemon-reload`.

Включите автозапуск сервиса `systemctl enable stronghold.service`.

Создайте каталог `/opt/stronghold/data` и установите права доступа на него:

```shell
mkdir -p /opt/stronghold/data
chown stronghold:stronghold /opt/stronghold/data
chmod 0700 /opt/stronghold/data
```

### Подготовка необходимых сертификатов

Для настройки TLS требуется описанный ниже набор сертификатов и ключей, размещённых в каталоге `/opt/stronghold/tls`.

Сертификат корневого центра сертификации, который подписал сертификат Stronghold TLS. В данном сценарии его имя — `stronghold-ca.pem`.

Сертификаты узлов Raft. В текущем сценарии в кластер будет добавлено три узла, для которых будут созданы такие сертификаты:

* node-1-cert.pem
* node-2-cert.pem
* node-3-cert.pem

Закрытые ключи сертификатов узлов:

* node-1-key.pem
* node-2-key.pem
* node-3-key.pem

В этом примере создадим корневой сертификат, а также набор самоподписанных сертификатов для каждого узла.

Хотя самоподписанные сертификаты и подходят для экспериментов с развёртыванием и запуском Stronghold, мы настоятельно рекомендуем использовать сертификаты, созданные и подписанные соответствующим центром сертификации.

### Порядок действий

На первом узле перейдите в каталог `/opt/stronghold/tls/`. Если каталог ещё не существует — создайте его:

```shell
mkdir -p /opt/stronghold/tls
cd /opt/stronghold/tls/
```

Сгенерируйте ключ для корневого сертификата:

```shell
openssl genrsa 2048 > stronghold-ca-key.pem
```

Выпустите корневой сертификат:

```console
openssl req -new -x509 -nodes -days 3650 -key stronghold-ca-key.pem -out stronghold-ca.pem

Country Name (2 letter code) [XX]:RU
State or Province Name (full name) []:
Locality Name (eg, city) [Default City]:Moscow
Organization Name (eg, company) [Default Company Ltd]:MyOrg
Organizational Unit Name (eg, section) []:
Common Name (eg, your name or your server hostname) []:demo.tld
```

Атрибуты сертификата приведены для примера. Для выпуска сертификатов узлов создайте конфигурационные файлы, содержащие `subjectAltName` (SAN). Например, файл для узла raft-node-1 будет выглядеть так:

```shell
cat << EOF > node-1.cnf
[v3_ca]
subjectAltName = @alt_names
[alt_names]
DNS.1 = raft-node-1.demo.tld
IP.1 = 10.20.30.10
IP.2 = 127.0.0.1
EOF
```

Каждый узел должен иметь корректные FQDN и IP-адрес. Поле `subjectAltName` в сертификате должно содержать соответствующие значения для конкретного узла.

Также нужно создать конфигурационный файл для каждого узла, который вы планируете добавить в кластер.

Для каждого узла сформируйте файл запроса:

```shell
openssl req -newkey rsa:2048 -nodes -keyout node-1-key.pem -out node-1-csr.pem -subj "/CN=raft-node-1.demo.tld"
openssl req -newkey rsa:2048 -nodes -keyout node-2-key.pem -out node-2-csr.pem -subj "/CN=raft-node-2.demo.tld"
openssl req -newkey rsa:2048 -nodes -keyout node-3-key.pem -out node-3-csr.pem -subj "/CN=raft-node-3.demo.tld"
```

Выпустите сертификаты на основании запросов:

```shell
openssl x509 -req -set_serial 01 -days 3650 -in node-1-csr.pem -out node-1-cert.pem -CA stronghold-ca.pem -CAkey stronghold-ca-key.pem -extensions v3_ca -extfile ./node-1.cnf
openssl x509 -req -set_serial 01 -days 3650 -in node-2-csr.pem -out node-2-cert.pem -CA stronghold-ca.pem -CAkey stronghold-ca-key.pem -extensions v3_ca -extfile ./node-2.cnf
openssl x509 -req -set_serial 01 -days 3650 -in node-3-csr.pem -out node-3-cert.pem -CA stronghold-ca.pem -CAkey stronghold-ca-key.pem -extensions v3_ca -extfile ./node-3.cnf
```

Для автоматического подключения узлов скопируйте на каждый из них:

* Файл сертификата этого узла.
* Файл ключа этого узла.
* Файл корневого сертификата.

Например:

```shell
scp ./node-2-key.pem ./node-2-cert.pem ./stronghold-ca.pem  raft-node-2.demo.tld:/opt/stronghold/tls
scp ./node-3-key.pem ./node-3-cert.pem ./stronghold-ca.pem  raft-node-3.demo.tld:/opt/stronghold/tls
```

Если каталога `/opt/stronghold/tls` нет на целевых узлах — создайте его.

## Развёртывание кластера с Raft

Подключитесь к первому серверу, на котором будет выполняться инициализация кластера Stronghold.

Добавьте разрешающие правила на сетевом экране для TCP-портов 8200 и 8201\. Вот пример для firewalld:

```console
firewall-cmd --add-port=8200/tcp --permanent
firewall-cmd --add-port=8201/tcp --permanent
firewall-cmd --reload
```

Вы можете использовать и любые другие порты, указав их в конфигурационном файле `/opt/stronghold/config.hcl`.

Создайте файл `/opt/stronghold/config.hcl` для конфигурации Raft. Если каталог `/etc/stronghold/` не существует, создайте его. Добавьте в файл следующее содержимое, заменив значения соответствующих параметров своими:

```hcl
ui = true
cluster_addr  = "https://10.20.30.10:8201"
api_addr      = "https://10.20.30.10:8200"
disable_mlock = true

listener "tcp" {
  address       = "0.0.0.0:8200"
  tls_cert_file      = "/opt/stronghold/tls/node-1-cert.pem"
  tls_key_file       = "/opt/stronghold/tls/node-1-key.pem"
}

storage "raft" {
  path = "/opt/stronghold/data"
  node_id = "raft-node-1"

  retry_join {
    leader_tls_servername   = "raft-node-1.demo.tld"
    leader_api_addr         = "https://10.20.30.10:8200"
    leader_ca_cert_file     = "/opt/stronghold/tls/stronghold-ca.pem"
    leader_client_cert_file = "/opt/stronghold/tls/node-1-cert.pem"
    leader_client_key_file  = "/opt/stronghold/tls/node-1-key.pem"
  }
  retry_join {
    leader_tls_servername   = "raft-node-2.demo.tld"
    leader_api_addr         = "https://10.20.30.11:8200"
    leader_ca_cert_file     = "/opt/stronghold/tls/stronghold-ca.pem"
    leader_client_cert_file = "/opt/stronghold/tls/node-1-cert.pem"
    leader_client_key_file  = "/opt/stronghold/tls/node-1-key.pem"
  }
  retry_join {
    leader_tls_servername   = "raft-node-3.demo.tld"
    leader_api_addr         = "https://10.20.30.12:8200"
    leader_ca_cert_file     = "/opt/stronghold/tls/stronghold-ca.pem"
    leader_client_cert_file = "/opt/stronghold/tls/node-1-cert.pem"
    leader_client_key_file  = "/opt/stronghold/tls/node-1-key.pem"
  }
}
```

Выполните запуск:

```shell
systemctl start stronghold
```

Выполните инициализацию:

```shell
stronghold operator init -ca-cert /opt/stronghold/tls/stronghold-ca.pem
```

Вы можете передать параметры `-key-shares` и `-key-threshold`, чтобы определить, на сколько частей будет разбит ключ и сколько из них будет достаточно для распечатывания хранилища. По умолчанию `key-shares=5`, а `key-threshold=3`.

{% alert level="warning" %}После завершения инициализации в терминал будут выведены все части ключа и корневой токен. Обязательно сохраните эти данные в надёжном месте. Части ключа и начальный корневой токен крайне важны. Если вы потеряете часть ключа, то не сможете получить доступ к данным Stronghold.{% endalert %}

Дальше нужно распечатать кластер. Для этого выполните необходимое количество раз команду:

```shell
stronghold operator unseal -ca-cert /opt/stronghold/tls/stronghold-ca.pem
```

и введите ключи распечатки, полученные на предыдущем шаге. Если вы не меняли параметр `-key-threshold`, то ввести нужно 3 части ключа.

Повторите настройку на остальных узлах кластера. Для этого укажите в файле `/opt/stronghold/config.hcl` в параметрах `cluster_addr` и `api_addr` соответствующие IP-адреса узлов. Пропустите шаг с инициализацией и сразу переходите к шагу распечатки кластера.

Остаётся только проверить работу кластера:

```console
stronghold status -ca-cert /opt/stronghold/tls/stronghold-ca.pem
Key                     Value
---                     -----
Seal Type               shamir
Initialized             true
Sealed                  false
Total Shares            5
Threshold               3
Version                 1.15.2
Build Date              2025-03-07T16:10:46Z
Storage Type            raft
Cluster Name            stronghold-cluster-a3fcc270
Cluster ID              f682968d-5e6c-9ad4-8303-5aecb259ca0b
HA Enabled              true
HA Cluster              https://10.20.30.10:8201
HA Mode                 active
Active Node Address     https://10.20.30.10:8200
Raft Committed Index    40
Raft Applied Index      40
```
