---
title: "Руководство администратора: Репликация KV1/KV2"
LinkTitle: "Руководство администратора: Репликация KV1/KV2"
description: "Руководство администратора: Репликация KV1/KV2"
---

## Описание механизма репликации KV1/KV2 в Stronghold

Под механизмом репликации подразумевается операция автоматического копирования секретов
между несколькими экземплярами Stronghold в режиме master-slave с использованием pull-модели.
Репликация поддерживается только для хранилищ KV1/KV2.
Операция синхронизации данных производится периодически по расписанию или по индивидуальным настройкам каждого хранилища KV1/KV2.
Для обеспечения работы механизма репликации необходимо обеспечить наличие сетевого соединения (сетевая связанность, сертификат TLS) с удаленным кластером Stronghold,
а также получить токен для доступа к нему. Токен должен обеспечивать доступ к хранилищам KV1/KV2 на удаленном кластере Stronghold
для выполнения операций list и read.

Для включения репликации необходимо задать настройки репликации при монтировании нового хранилища KV1/KV2.
Названия удаленного и локального mount_path могут не совпадать. Репликация может быть настроена на разных пространствах имен на локальном и удаленном хранилищах 
Несколько локальных хранилищ с разными именами, могу быть настроены для репликации к одному удаленному.

Если для локального хранилища KV1/KV2 настроена репликация, оно доступно только для операций чтения.
Запись, изменение и удаление секретов в таком хранилище невозможны — все изменения должны выполняться в исходном (мастер) хранилище.
Все внесенные изменения будут перенесены в локальное хранилище при следующем запуске репликации.
Если репликация для хранилища KV1/KV2 отключается, статус readonly снимается, и операции редактирования/удаления/добавления секретов становятся доступными.
При повторном включении репликации, все внесенные изменения будут удалены/переписаны.

## Настройка репликации KV1/KV2 в Stronghold

Настройка репликации производится на стороне потребителя (slave кластера Stronghold) путем задания настроек репликации
при монтировании нового хранилища KV1/KV2.

Настройки включают в себя следующие параметры:
- адрес удаленного кластера Stronghold (источник данных)
- токен для доступа к удаленному кластеру Stronghold (источнику данных)
- сертификат TLS или путь к сертификату TLS для подключения к удаленному кластеру Stronghold (источнику данных)
- имя namespace-path в котором находится хранилище KV1/KV2 на удаленном кластере Stronghold (по умолчанию root)
- имя mount-path хранилища KV1/KV2 на удаленном кластере Stronghold (источнике данных)
- список secret path для репликации (по умолчанию реплицируются все секреты)
- период запуска репликации данных (по умолчанию 1 минута)
- включение/выключение репликации. При создании нового хранилища KV1/KV2 репликация будет включена по умолчанию. Изменение
состояния возможно через редактирования настроек хранилища KV1/KV2.
- версия KV хранилища для монтирования и репликации.

**Внимание! Версия локального и удаленного KV хранилища должны совпадать**
Нельзя настроить репликацию kv1 в kv2 или kv2 в kv1


### Как создать токен для репликации

Токен для доступа к удаленному кластеру должен иметь права list и read для реплицируемых секретов. Если выданный токен позволяет продлевать самого себя, то Strongold будет 
автоматически продлевать токен на 30 дней, когда оставшийся TTL токена будет менее 7 дней и отсутствии превышения параметра maxTTL.

Пример, как можно создать политику и токен для репликации из mount <dev-secrets> находящегося в пространстве имен <ns_path_1>. Для этого на исходном сервере создайте политику и токен, привязанный к ней.

```shell
d8 stronghold policy write -namespace=ns_path_1 replicate-dev-secrets - <<EOF
# Allow token to list/read secrets from dev-secrets
path "dev-secrets/*" {
  capabilities = ["read", "list"]
}

# Allow token to read info about dev-secrets
path "sys/mounts/dev-secrets" {
  capabilities = ["read"]
}

# Allow token to look up own properties
path "auth/token/lookup-self" {
    capabilities = ["read"]
}

# Allow token to renew self
path "auth/token/renew-self" {
    capabilities = ["update"]
}
EOF

d8 stronghold token create -namespace=ns_path_1 -policy=replicate-dev-secrets -orphan=true -ttl=1h
```


### Настройка репликации через cli Stronghold

Для настройки репликации через cli Stronghold необходимо выполнить следующие команды:

Без использования TLS-соединения
```shell
d8 stronghold secrets enable \
 -path=<local_mount_path_name> \
 -src-address=<address_of_source_cluster> \
 -src-token=<token_of_source_cluster> \
 -src-namespace=<namespace_path_in_source_cluster> \
 -src-mount-path=<mount_path_in_source_cluster> \
 -version=<1/2> \
 -namespace=<namespace_path_in_local_cluster> \
 kv
```
С передачей настроек TLS-соединения

```shell
d8 stronghold secrets enable \
 -path=<local_mount_path_name> \
 -src-address=<address_of_source_cluster> \
 -src-token=<token_of_source_cluster> \
 -src-namespace=<namespace_path_in_source_cluster> \
 -src-mount-path=<mount_path_in_source_cluster> \
 -src-ca-cert=@<path_to_file_with_certificate> \
 -version=<1/2> \
 -namespace=<namespace_path_in_local_cluster> \
 kv
```

Описание параметров:

`-path` - имя mount-path локального хранилища KV1/KV2 в кластере Stronghold, куда будет выполнено копирование данных из источника.
Обязательный параметр. Например: "my-mount-kv2".

`-src-address` - адрес удаленного кластера Stronghold. Обязательный параметр. Пример: "127.0.0.1:8200", "vault.mycompany.tld:8200", "stronghold.mycompany.tld:443"

`-src-token` - токен для доступа к удаленному кластеру Stronghold (источнику данных). Обязательный параметр. Например: "z6VXjAi6F3vjaclHu99FLOcr".

`-src-namespace` - имя namespace-path в котором находится хранилище KV1/KV2 на удаленном кластере Stronghold. Необязательный параметр. По умолчанию: "root"

`-src-mount-path` - имя mount-path хранилища KV1/KV2 на удаленном кластере Stronghold. Обязательный параметр. Например: "remote-mount-kv2"

`-src-secret-path` - список secret paths для репликации. Необязательный параметр.

`-src-ca-cert` - сертификат CA для установки TLS - соединения. Eсли сертифкат в файле, то `-src-ca-cert=@ca-cert.pem`
Необязательный параметр

`-version` - версия KV хранилища для монтирования и репликации. **Внимание! Версия локального и удаленного KV хранилища должны совпадать**. Обязательный параметр

`-namespace` - имя namespace-path в котором создается хранилище KV1/KV2 на локальном кластере Stronghold. Необязательный параметр. По умолчанию: "root"

### Изменение настроек репликации через cli Stronghold

Для редактирования доступны следующие параметры:
- токен для доступа к удаленному кластеру Stronghold (источнику данных)
- сертификат TLS или путь к сертификату TLS для подключения к удаленному кластеру Stronghold (источнику данных)
- список secret path для репликации (параметр пока не используется, по умолчанию будут реплицироваться все секреты)
- период запуска репликации для данного хранилища (параметр пока не используется)
- включение/выключение репликации для данного хранилища

**Внимание! При изменении secret path в конфигурации репликации старый путь в локальном кластере останется неизменным, а новый будет добавлен.
Если secret path до изменения и после пересекаются, новые данные могут частично перезаписать существующие.
Например до изменения было `-src-secret-path=[first-secret/one, second-sercet/two]`,
а после изменения стало `-src-secret-path=[first-secret/two, second-sercet/two]`,
то данные в `"first-secret/one"` останутся прежними и больше не будут изменяться**


Для изменения настроек репликации через cli Stronghold необходимо выполнить следующие команды:

```shell
d8 stronghold secrets tune \
 -src-token=<token_of_source_cluster> \
 -src-secret-path=<list_of_secret_paths_in_source_cluster> \
 -src-ca-cert=@<path_to_file_with_certificate> \
 -sync-enable=true \
 -namespace=<namespace_path_in_local_cluster> \
 <local_mount_path_name>
```

`-src-token` - токен для доступа к удаленному кластеру Stronghold (источнику данных). Обязательный параметр. Например: "z6VXjAi6F3vjaclHu99FLOcr"

`-src-ca-cert` - сертификат CA для установки TLS - соединения. Необязательный параметр

`-src-secret-path` - список secret paths для репликации. Необязательный параметр.

`-src-ca-cert` - сертификат CA для установки TLS - соединения. Eсли сертифкат в файле, то `-src-ca-cert=@ca-cert.pem`
Необязательный параметр

`-sync-enable` - включение или выключение репликации для данного локального mount_path. Обязательный параметр

`-namespace` - имя namespace-path в котором создается хранилище KV1/KV2 на локальном кластере Stronghold. Необязательный параметр. По умолчанию: "root"

Для отключения репликации заданного хранилища достаточно выполнить операцию:

```shell
d8 stronghold secrets tune -sync-enable=false -namespace=<namespace_path_in_local_cluster> <local_mount_path_name>
```

Если будут переданы остальные параметры настройки репликации, то они будут проигнорированы.

Для включения репликации необходимо выполнить команду:

```shell
d8 stronghold secrets tune -sync-enable=true -namespace=<namespace_path_in_local_cluster> <local_mount_path_name>
```

В данном случае также можно передавать и остальные параметры настройки репликации, они будут учитываться

Для чтения настроек репликации необходимо выполнить команду:

```shell
d8 stronghold read -namespace=<namespace_path_in_local_cluster> sys/mounts/<mount_path>/tune
```

### Настройка через API Stronghold

Для настройки репликации через API Stronghold необходимо выполнить обращение к API
создания mount и добавить в тело запроса конфигурацию для репликации:

```shell
curl --header "X-Vault-Token: <token_for_local_cluster>" \
     --header "X-Vault-Namespace: <namespace_path_in_local_cluster>" \
     --request POST \
     --data '{
  "type" : "<kv-v1>/<kv-v2>",
  "config" : {
    "replication_config" : {
      "src_address" : "<address_of_source_cluster>",
      "src_token" : "<token_of_source_cluster>",
      "src_ca_cert" : "<tls_cert_for_source_cluster>",
      "src_namespace" : "<namespace_path_in_source_cluster>",
      "src_mount_path" : "<mount_path_in_source_cluster>",
      "src_secret_path" : [ "<list_of_secret_paths_in_source_cluster>" ],
    }
  }
}’ <local_stronghold_address>/v1/sys/mounts/<local_mount_path_name>
```

Если удаленный кластер источника данных не поддерживает протокол tls, то параметр `"src_ca_cert"` передавать не нужно.
По умолчанию параметр `"src_secret_path"` равен `"*"`, что означает, что реплицироваться будут все secret paths.

Описание параметров:

`local_stronghold_address` - адрес локального стронгхолда, на котором настраивается репликация.

`token_for_local_cluster` - токен к кластеру репликации чтобы был доступ к созданию mount.

`namespace_path_in_local_cluster` - имя namespace-path в котором создается хранилище KV1/KV2 на локальном кластере Stronghold. Необязательный параметр. По умолчанию: "root"

`local_mount_path_name` - имя mount-path локального хранилища KV1/KV2 в кластере Stronghold, куда будет выполнено копирование данных из источника.
Обязательный параметр. Например: "my-mount-kv2".

`src_address` - адрес удаленного кластера Stronghold. Обязательный параметр. Пример: "127.0.0.1:8200", "vault.mycompany.tld:8200", "stronghold.mycompany.tld:443"

`src_token` - токен для доступа к удаленному кластеру Stronghold (источнику данных). Обязательный параметр. Например: "z6VXjAi6F3vjaclHu99FLOcr"

`src_namespace` - имя namespace-path в котором находится хранилище KV1/KV2 на удаленном кластере Stronghold. Необязательный параметр. По умолчанию: "root"

`src_mount_path` - имя mount-path хранилища KV1/KV2 на удаленном кластере Stronghold. Обязательный параметр. Например: "remote-mount-kv2"

`src_secret_path` - список secret paths для репликации. Необязательный параметр.

`src_ca_cert` - сертификат CA для установки TLS - соединения. Необязательный параметр

`type` - версия KV хранилища для монтирования и репликации. **Внимание! Версия локального и удаленного KV хранилища должны совпадать**. Обязательный параметр

### Изменение настроек репликации через API Stronghold
Для редактирования доступны следующие параметры:
- токен для доступа к удаленному кластеру Stronghold (источнику данных)
- сертификат TLS или путь к сертификату TLS для подключения к удаленному кластеру Stronghold (источнику данных)
- список secret path для репликации (по умолчанию реплицируются все секреты)
- период запуска репликации для данного хранилища (параметр пока не используется)
- включение/выключение репликации для данного хранилища


**Внимание!**
При изменении secret path в конфигурации репликации старый путь в локальном кластере останется неизменным, а новый будет добавлен.
Если secret path до изменения и после пересекаются, новые данные могут частично перезаписать существующие.
Например до изменения было `"src_secret_path"=["first-secret/one", "second-sercet/two"]`,
а после изменения стало `"src_secret_path"=["first-secret/two", "second-sercet/two"]`,
то данные в `"first-secret/one"` останутся прежними и больше не будут изменяться.

Для изменения настроек репликации через API Stronghold необходимо выполнить обращение к API
изменения mount и добавить в тело запроса новую конфигурацию для репликации:

```shell
curl --header "X-Vault-Token: <token_for_local_cluster>" \
     --header "X-Vault-Namespace: <namespace_path_in_local_cluster>" \
     --request POST \
     --data '{
        "replication_config" : {
          "src_token" : "<token_of_source_cluster>",
          "src_ca_cert" : "<tls_cert_for_source_cluster>",
          "src_secret_path" : [ "<list_of_secret_paths_in_source_cluster>" ],
          "sync_enable" : true
        }
    }’
    <local_stronghold_address>/v1/sys/mounts/<local_mount_path_name>/tune
```

`local_stronghold_address` - адрес локального stronghold, на котором настраивается репликация.

`token_for_local_cluster` - токен к кластеру репликации чтобы был доступ к редактированию mount.

`namespace_path_in_local_cluster` - имя namespace-path в котором создается хранилище KV1/KV2 на локальном кластере Stronghold. Необязательный параметр. По умолчанию: "root"

`local_mount_path_name` - имя mount-path локального хранилища KV1/KV2 в кластере Stronghold, куда будет выполнено копирование данных из источника.
Обязательный параметр. Например: "my-mount-kv2".

`src_token` - токен для доступа к удаленному кластеру Stronghold (источнику данных). Обязательный параметр. Например: "z6VXjAi6F3vjaclHu99FLOcr"

`src_ca_cert` - сертификат CA для установки TLS - соединения. Необязательный параметр

`sync_enable` - включение или выключение репликации для данного локального mount_path. Обязательный параметр

`src_secret_path` - список secret paths для репликации. Необязательный параметр.


Для отключения репликации заданного хранилища достаточно выполнить операцию:

```shell
curl --header "X-Vault-Token: <token_for_local_cluster>" \
     --header "X-Vault-Namespace: <namespace_path_in_local_cluster>" \
     --request POST \
     --data '{
        "replication_config" : {
          "sync_enable" : false
        }
    }’
    <local_stronghold_address>/v1/sys/mounts/<local_mount_path_name>/tune
```

Если будут переданы остальные параметры настройки репликации, то они будут проигнорированы.

Для включения репликации необходимо выполнить команду:

```shell
curl --header "X-Vault-Token: <token_for_local_cluster>" \
     --header "X-Vault-Namespace: <namespace_path_in_local_cluster>" \
     --request POST \
     --data '{
        "replication_config" : {
          "sync_enable" : enable
        }
    }’
    <local_stronghold_address>/v1/sys/mounts/<local_mount_path_name>/tune
```

В данном случае также можно передавать и остальные параметры настройки репликации, они будут учитываться

Для чтения настроек репликации необходимо выполнить команду:

```shell
curl -X GET \
     -H "X-Vault-Token: <token_for_local_cluster>" \
     -H "X-Vault-Namespace: <namespace_path_in_local_cluster>" \
     <local_stronghold_address>/v1/sys/mounts/<local_mount_path_name>/tune
```
