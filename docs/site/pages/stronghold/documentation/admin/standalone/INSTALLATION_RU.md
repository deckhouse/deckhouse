---
title: "Установка"
permalink: ru/stronghold/documentation/admin/standalone/installation.html
lang: ru
---

Stronghold поддерживает мультисерверный режим для обеспечения высокой доступности (HA). Этот режим автоматически включается при использовании совместимого хранилища данных и защищает систему от сбоев за счёт работы нескольких серверов Stronghold.

Чтобы проверить поддержку режима высокой доступности, запустите сервер и убедитесь, что рядом с информацией о хранилище выводится сообщение `HA available`. В этом случае Stronghold автоматически использует режим HA.

Для обеспечения высокой доступности один из узлов Stronghold получает блокировку в системе хранения данных и становится активным, а остальные узлы переходят в режим ожидания. Если резервные узлы получают запросы, они либо перенаправляют их, либо переадресовывают клиентов в соответствии с настройками и текущим состоянием кластера.

Для работы Stronghold в режиме высокой доступности (HA) с интегрированным хранилищем Raft требуется как минимум три сервера Stronghold. Это условие необходимо для достижения кворума — без него кластер не сможет работать с хранилищем.

Предварительные требования:

* На сервер установлена поддерживаемая ОС (Ubuntu, RedOS, Astra Linux).
* На сервер скопирован дистрибутив Stronghold.
* Создан systemd-unit для управления сервисом.
* Для каждого узла в кластере Raft выпущены индивидуальные сертификаты.
* Подготовлен сертификат корневого центра сертификации (CA).

## Предварительная подготовка инфраструктуры

Ниже приведён сценарий развёртывания кластера Stronghold, состоящего из трёх узлов: одного активного и двух резервных. Такой кластер обеспечивает режим высокой доступности (HA).

### Запуск через systemd-unit

{% alert level="warning" %}
Все примеры предполагают, что создан системный пользователь `stronghold`, и сервис запускается от его имени.
Если требуется использовать другого пользователя, замените `stronghold` на соответствующее имя.
{% endalert %}

1. Создайте файл `/etc/systemd/system/stronghold.service` со следующим содержимым:

   ```console
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

1. Примените изменения в конфигурации systemd:

   ```shell
   systemctl daemon-reload
   ```

1. Включите автозапуск сервиса с помощью команды:

   ```shell
   systemctl enable stronghold.service
   ```

1. Создайте каталог `/opt/stronghold/data` и установите права доступа на него:

   ```shell
   mkdir -p /opt/stronghold/data
   chown stronghold:stronghold /opt/stronghold/data
   chmod 0700 /opt/stronghold/data
   ```

### Подготовка необходимых сертификатов

Для настройки TLS требуется набор сертификатов и ключей, которые должны быть размещены в каталоге `/opt/stronghold/tls`:

- Сертификат корневого центра сертификации (CA).`stronghold-ca.pem` — сертификат, которым подписаны TLS-сертификаты Stronghold.
- Сертификаты узлов Raft. В текущем сценарии в кластер будет добавлено три узла, для которых будут созданы сертификаты:
  - `node-1-cert.pem`;
  - `node-2-cert.pem`;
  - `node-3-cert.pem`.
- Закрытые ключи сертификатов узлов:
  - `node-1-key.pem`;
  - `node-2-key.pem`;
  - `node-3-key.pem`.

В данном примере создаётся корневой сертификат, а также набор самоподписанных сертификатов для каждого узла.

{% alert level="warning" %}
Самоподписанные сертификаты подходят только для тестовых сценариев и экспериментов.
Для эксплуатации в production настоятельно рекомендуется использовать сертификаты, созданные и подписанные доверенным центром сертификации (CA).
{% endalert %}

### Порядок действий

1. На первом узле создайте каталог для хранения сертификатов (если он ещё не существует) и перейдите в него:

   ```shell
   mkdir -p /opt/stronghold/tls
   cd /opt/stronghold/tls/
   ```

1. Сгенерируйте ключ для корневого сертификата:

   ```shell
   openssl genrsa 2048 > stronghold-ca-key.pem
   ```

1. Выпустите корневой сертификат:

   ```console
   openssl req -new -x509 -nodes -days 3650 -key stronghold-ca-key.pem -out stronghold-ca.pem

   Country Name (2 letter code) [XX]:RU
   State or Province Name (full name) []:
   Locality Name (eg, city) [Default City]:Moscow
   Organization Name (eg, company) [Default Company Ltd]:MyOrg
   Organizational Unit Name (eg, section) []:
   Common Name (eg, your name or your server hostname) []:demo.tld
   ```

   > Атрибуты сертификата приведены в качестве примера.

1. Для выпуска сертификатов узлов создайте конфигурационные файлы, содержащие параметр `subjectAltName` (SAN). Например, для узла `raft-node-1`:

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

   Каждый узел должен иметь корректные FQDN и IP-адреса.
   Поле `subjectAltName` в сертификате должно содержать значения, актуальные для конкретного узла. Аналогично создайте отдельный конфигурационный файл для каждого узла.

1. Сформируйте запросы на сертификаты (CSR) и ключи для узлов:

   ```shell
   openssl req -newkey rsa:2048 -nodes -keyout node-1-key.pem -out node-1-csr.pem -subj "/CN=raft-node-1.demo.tld"
   openssl req -newkey rsa:2048 -nodes -keyout node-2-key.pem -out node-2-csr.pem -subj "/CN=raft-node-2.demo.tld"
   openssl req -newkey rsa:2048 -nodes -keyout node-3-key.pem -out node-3-csr.pem -subj "/CN=raft-node-3.demo.tld"
   ```

1. Выпустите сертификаты на основе созданных CSR:

   ```shell
   openssl x509 -req -set_serial 01 -days 3650 -in node-1-csr.pem -out node-1-cert.pem -CA stronghold-ca.pem -CAkey stronghold-ca-key.pem -extensions v3_ca -extfile ./node-1.cnf
   openssl x509 -req -set_serial 01 -days 3650 -in node-2-csr.pem -out node-2-cert.pem -CA stronghold-ca.pem -CAkey stronghold-ca-key.pem -extensions v3_ca -extfile ./node-2.cnf
   openssl x509 -req -set_serial 01 -days 3650 -in node-3-csr.pem -out node-3-cert.pem -CA stronghold-ca.pem -CAkey stronghold-ca-key.pem -extensions v3_ca -extfile ./node-3.cnf
   ```

   > Рекомендуется использовать уникальные значения `-set_serial` для каждого сертификата.

1. Скопируйте на каждый узел необходимые файлы:

   - Сертификат узла;
   - Закрытый ключ узла;
   - Корневой сертификат.

     Например, для узлов `raft-node-2` и `raft-node-3`:

     ```shell
     scp ./node-2-key.pem ./node-2-cert.pem ./stronghold-ca.pem  raft-node-2.demo.tld:/opt/stronghold/tls
     scp ./node-3-key.pem ./node-3-cert.pem ./stronghold-ca.pem  raft-node-3.demo.tld:/opt/stronghold/tls
     ```

     > Если каталога `/opt/stronghold/tls` нет на целевых узлах — создайте его.

## Развёртывание кластера с Raft

1. Подключитесь к первому серверу, на котором будет выполняться инициализация кластера Stronghold.

1. Разрешите сетевые подключения для TCP-портов `8200` и `8201`. Пример для `firewalld`:

   ```shell
   firewall-cmd --add-port=8200/tcp --permanent
   firewall-cmd --add-port=8201/tcp --permanent
   firewall-cmd --reload
   ```

   > При необходимости можно использовать другие порты, указав их в конфигурационном файле `/opt/stronghold/config.hcl`.

1. Создайте конфигурационный файл `/opt/stronghold/config.hcl` для Raft. Если каталог `/etc/stronghold/` не существует, создайте его:

   ```console
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

1. Запустите сервис Stronghold:

   ```shell
   systemctl start stronghold
   ```

1. Инициализируйте кластер:

   ```shell
   stronghold operator init -ca-cert /opt/stronghold/tls/stronghold-ca.pem
   ```

   При необходимости можно задать параметры:

   - `-key-shares` — количество частей ключа (по умолчанию 5);
   - `-key-threshold` — минимальное число частей, достаточных для распечатывания хранилища (по умолчанию 3).

     {% alert level="warning" %}
     После инициализации в терминале будут показаны все части ключа и корневой токен.
     Обязательно сохраните их в надёжном месте.
     Без достаточного числа ключевых частей доступ к данным Stronghold будет невозможен.
     {% endalert %}

1. Распечатайте кластер. Выполните команду несколько раз, вводя ключи распечатки:

   ```shell
   stronghold operator unseal -ca-cert /opt/stronghold/tls/stronghold-ca.pem
   ```

   > Если параметр `-key-threshold` не менялся, нужно ввести 3 части ключа.

1. Настройте остальные узлы:

   - Укажите в `/opt/stronghold/config.hcl` свои значения `cluster_addr` и `api_addr`.
   - Пропустите шаг инициализации.
   - Сразу выполните распечатку кластера (operator unseal).

1. Проверьте работу кластера:

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
