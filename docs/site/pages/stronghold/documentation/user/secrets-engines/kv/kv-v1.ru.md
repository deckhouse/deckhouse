---
title: "KV Version 1"
permalink: ru/stronghold/documentation/user/secrets-engines/kv/kv-v1.html
lang: ru
description: The KV secrets engine can store arbitrary secrets.
---

Механизм секретов `kv` используется для хранения произвольных секретов в пределах хранилища Stronghold.

При записи ключа в `kv` старое значение заменяется.

Имена ключей всегда должны быть строками. Если вы записываете нестроковые значения напрямую через CLI, они будут преобразованы в строки. Однако вы можете сохранить нестроковые значения, записав пары ключ/значение из JSON-файла или используя HTTP API.

Этот механизм секретов учитывает различие между операциями `create` и `update` в ACL-политиках.

{% alert %}Пути и имена ключей _не_ обфусцируются и не шифруются; шифруются только значения для ключей. Поэтому следует хранить конфиденциальную информацию как часть пути секрета.
{% endalert %}

## Как включить

Чтобы включить хранилище kv версии 1 выполните команду:

```shell-session
d8 stronghold secrets enable -version=1 kv
```

## Использование

Механизм секретов `kv` позволяет записывать ключи с произвольными значениями. Для этого потребуется токен с соответствующими правами

1. Запись произвольных данных:

   ```shell-session
   $ d8 stronghold kv put kv/my-secret my-value=s3cr3t
   Success! Data written to: kv/my-secret
   ```

1. Чтение произвольных данных:

   ```shell-session
   $ d8 stronghold kv get kv/my-secret
   Key                 Value
   ---                 -----
   my-value            s3cr3t
   ```

1. Получить список ключей:

   ```shell-session
   $ d8 stronghold kv list kv/
   Keys
   ----
   my-secret
   ```

1. Удалить ключ:

   ```shell-session
   $ d8 stronghold kv delete kv/my-secret
   Success! Data deleted (if it existed) at: kv/my-secret
   ```

   Вы также можете использовать механизм password policy для генерации произвольных значений.

1. Создать политику для паролей:

   ```shell-session
   $ d8 stronghold write sys/policies/password/example policy=-<<EOF
   
     length=20
   
     rule "charset" {
       charset = "abcdefghij0123456789"
       min-chars = 1
     }
   
     rule "charset" {
       charset = "!@#$%^&*STUVWXYZ"
       min-chars = 1
     }
   
   EOF
   ```

1. Сгенерировать пароль используя политику `example`:

   ```shell-session
   $ d8 stronghold kv put kv/my-generated-secret \
       password=$(d8 stronghold read -field password sys/policies/password/example/generate)
   ```

1. Прочитать сгенерированное значение секрета:

   ```shell-session
   $ d8 stronghold kv get kv/my-generated-secret
   ====== Data ======
   Key         Value
   ---         -----
   password    ^dajd609Xf8Zhac$dW24
   ```

## Время жизни ключей (TTL)

В отличие от других механизмов секретов, механизм секретов KV не применяет TTL для истечения срока действия. Вместо этого `lease_duration` является информацией для пользователя, как часто нужно проверять новое значение.

Если ключ имеет значение `ttl`, механизм секретов KV будет использовать это значение
в качестве продолжительности аренды:

Если ключ имеет значение `ttl`, движок будет использовать это значение в качестве продолжительности аренды:

```shell-session
$ d8 stronghold kv put kv/my-secret ttl=5s my-value=s3cr3t
Success! Data written to: kv/my-secret
```

Даже при установленном `ttl` движок secrets _никогда_ не удаляет данные самостоятельно. Ключ `ttl` носит лишь рекомендательный характер.

При чтении значения с `ttl`, как ключ `ttl`, так и интервал обновления будут отражать это значение:

```shell-session
$ d8 stronghold kv get kv/my-secret
Key                 Value
---                 -----
my-value            s3cr3t
ttl                 5s

curl -X 'GET' \
  'https://stronghold.example.com/v1/kv/my-secret' \
  -H 'X-Vault-Token: ***'

{
  "request_id": "3879d849-cb78-725a-c2eb-3ba9dfe8a1d3",
  "lease_id": "",
  "renewable": false,
  "lease_duration": 5,
  "data": {
    "my-value": "s3cr3t",
    "ttl": "5s"
  },
  "wrap_info": null,
  "warnings": null,
  "auth": null
}
```
