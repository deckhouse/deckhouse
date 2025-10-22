---
title: "KV Version 2"
permalink: ru/stronghold/documentation/user/secrets-engines/kv/kv-v2.html
lang: ru
description: The KV secrets engine can store arbitrary secrets.
---

Механизм секретов `kv` используется для хранения произвольных секретов в пределах хранилища Stronghold.

Имена ключей всегда должны быть строками. Если вы записываете нестроковые значения напрямую через CLI, они будут преобразованы в строки. Однако вы можете сохранить нестроковые значения, записав пары ключ/значение из JSON-файла или используя HTTP API.

Этот механизм секретов учитывает различие между операциями `create` и `update` в ACL-политиках. Также поддерживается функция `patch`, которая используется для выполнения частичных обновлений, в то время во время операции `update` выполняется полная перезапись.

## Настройка

Большинство механизмов секретов должны быть предварительно настроены. Настройка обычно выполняются оператором или с помощью инструментов управления конфигурацией, таких как Terraform.

Механизм секретов v2 `kv` может быть включен с помощью команды:

```shell-session
d8 stronghold secrets enable -version=2 kv
```

Или вы можете передать `kv-v2` в качестве типа механизма секретов:

```shell-session
d8 stronghold secrets enable kv-v2
```

## Обновление с версии 1 на версию 2

Существующее хранилище kv версии 1 можно обновить до хранилища kv версии 2 с помощью CLI или API. Во время процесса миграции хранилище будет недоступно. Это может занять много времени, поэтому планируйте обновление заранее.

После обновления до версии 2 прежние пути, по которым можно было получить доступ к данным, больше не будут доступны. Вам нужно будет по новому настроить политики пользователей, чтобы восстановить доступ, как это описано в разделе [Правила ACL](#правила-acl). Аналогично, пользователям/приложениям необходимо будет обновить пути, по которым они взаимодействуют с данными kv после их обновления до версии 2.

Существующее хранилище kv версии 1 можно обновить до хранилища kv версии 2 с помощью команды CLI:

```shell-session
$ d8 stronghold kv enable-versioning secret/
Success! Tuned the secrets engine at: secret/
```

## Правила ACL

Хранилище kv версии 2 использует API с префиксом, который отличается от API версии 1. Перед обновлением с версии 1 kv необходимо изменить правила ACL. Кроме того, различные пути в API версии 2 могут быть по-разному защищены ACL.

Пути для операций чтения и записи имеют префикс `data/`. Например, следующую политику для kv-v1:

```plaintext
path "secret/dev/team-1/*" {
  capabilities = ["create", "update", "read"]
}
```

Нужно заменить на:

```plaintext
path "secret/data/dev/team-1/*" {
  capabilities = ["create", "update", "read"]
}
```

Для kv-v2 существуют различные уровни удаления данных. Чтобы предоставить права на удаление последней версии ключа создайте такую политику:

```plaintext
path "secret/data/dev/team-1/*" {
  capabilities = ["delete"]
}
```

Чтобы разрешить удалять любую версию ключа:

```plaintext
path "secret/delete/dev/team-1/*" {
  capabilities = ["update"]
}
```

Чтобы разрешить восстанавливать удаленные версии:

```plaintext
path "secret/undelete/dev/team-1/*" {
  capabilities = ["update"]
}
```

Чтобы разрешить уничтожить значения ключей (без возможности восстановления):

```plaintext
path "secret/destroy/dev/team-1/*" {
  capabilities = ["update"]
}
```

Это политика, позволяющая получить список ключей:

```plaintext
path "secret/metadata/dev/team-1/*" {
  capabilities = ["list"]
}
```

Политика, позволяющая просматривать метаданные ключей:

```plaintext
path "secret/metadata/dev/team-1/*" {
  capabilities = ["read"]
}
```

Разрешить навсегда удалить все версии и метаданные ключа:

```plaintext
path "secret/metadata/dev/team-1/*" {
  capabilities = ["delete"]
}
```

Поля `allowed_parameters`, `denied_parameters` и `required_parameters` не поддерживаются для политик, используемых с хранилищем kv версии 2. Описание этих параметров см. в разделе [Политики](../../concepts/policy.html).

## Использование

После того как механизм секретов включен и у пользователя/машины есть токен Stronghold с соответствующими правами, он может взаимодействовать с секретами.
Синтаксис KV-v1, похожий на путь для ссылки на секрет (`secret/foo`), по-прежнему можно использовать в KV-v2, но мы рекомендуем использовать синтаксис с флагом `-mount=secret`, чтобы не перепутать его с реальным путем к секрету (реальный путь - `secret/data/foo`).

### Запись/чтение произвольных данных

 Запись ключей:

```shell-session
$ d8 stronghold kv put -mount=secret my-secret foo=a bar=b
Key              Value
---              -----
created_time     2024-06-19T17:20:22.985303Z
custom_metadata  <nil>
deletion_time    n/a
destroyed        false
version          1
```

 Чтение ключей:

```shell-session
$ d8 stronghold kv get -mount=secret my-secret
====== Metadata ======
Key              Value
---              -----
created_time     2024-06-19T17:20:22.985303Z
custom_metadata  <nil>
deletion_time    n/a
destroyed        false
version          1

====== Data ======
Key         Value
---         -----
foo         a
bar         b
   ```

- Запишите другую версию, при этом предыдущая версия будет по-прежнему доступна. Опционально может быть передан флаг `-cas` (`check-and-set)` для выполнения проверки, что ключ существует . Если флаг не установлен, запись будет разрешена. Если же флаг `cas` установлен, то для того чтобы запись была успешной, его значение должно соответствовать текущей версию секрета. Если установлено значение 0, запись будет разрешена только в том случае, если ключ не существует, так как неустановленные ключи не имеют информации о версии. Также помните, что удаление "версии" не удаляет из хранилища информацию о версиях. Таким образом, для записи в секрет, у которого были удаленные версии, параметр cas должен соответствовать текущей версии секрета.

```shell-session
$ d8 stronghold kv put -mount=secret -cas=1 my-secret foo=aa bar=bb
Key              Value
---              -----
created_time     2024-06-19T17:22:23.369372Z
custom_metadata  <nil>
deletion_time    n/a
destroyed        false
version          2
```

 Чтение вернет самую свежую версию данных:

```shell-session
$ d8 stronghold kv get -mount=secret my-secret
====== Metadata ======
Key              Value
---              -----
created_time     2024-06-19T17:22:23.369372Z
custom_metadata  <nil>
deletion_time    n/a
destroyed        false
version          2

====== Data ======
Key         Value
---         -----
foo         aa
bar         bb
```

С помощью команды `d8 stronghold kv patch`  может быть выполнено частичное обновление секрета. Команда первоначально попытается выполнить HTTP-запрос `PATCH`, который требует наличия ACL-возможности `patch`. Запрос `PATCH` будет неудачным, если используемый токен связан с политикой, которая не содержит возможности `patch`. В этом случае команда выполнит чтение, локальное обновление и последующую запись, для которых требуются возможности ACL `read` и `update`.
Опционально может быть передан флаг `-cas`  для выполнения проверки, что ключ существует. Он будет использоваться только в случае начального запроса `PATCH`. Вариант с последовательными чтением и записью будет использовать значение `version` из секрета, возвращенного при чтении, для выполнения проверки `cas` при последующей записи.

```shell-session
$ d8 stronghold kv patch -mount=secret -cas=2 my-secret bar=bbb
Key              Value
---              -----
created_time     2024-06-19T17:23:49.199802Z
custom_metadata  <nil>
deletion_time    n/a
destroyed        false
version          3

  ```

Команда `d8 stronghold kv patch` также поддерживает флаг `-method`, который можно использовать чтобы указать, какой метод использовать, `patch` или `rw`.

Выполнить обновление секрета используя `patch`:

```shell-session
$ d8 stronghold kv patch -mount=secret -method=patch -cas=2 my-secret bar=bbb
Key              Value
---              -----
created_time     2024-06-19T17:23:49.199802Z
custom_metadata  <nil>
deletion_time    n/a
destroyed        false
version          3
```

Выполнить обновление, используя `rw`, то есть сначала прочитать значение, а потом записать новую измененную версию:

```shell-session
$ d8 stronghold kv patch -mount=secret -method=rw my-secret bar=bbb
Key              Value
---              -----
created_time     2024-06-19T17:23:49.199802Z
custom_metadata  <nil>
deletion_time    n/a
destroyed        false
version          3
```

Чтение вернет самую новую версию, в которой были обновлены только заданные значения:

```shell-session
$ d8 stronghold kv get -mount=secret my-secret
====== Metadata ======
Key              Value
---              -----
created_time     2024-06-19T17:23:49.199802Z
custom_metadata  <nil>
deletion_time    n/a
destroyed        false
version          3

====== Data ======
Key         Value
---         -----
foo         aa
bar         bbb
```

Предыдущие версии секретов можно получить используя флаг `-version`:

```shell-session
$ d8 stronghold kv get -mount=secret -version=1 my-secret
====== Metadata ======
Key              Value
---              -----
created_time     2024-06-19T17:20:22.985303Z
custom_metadata  <nil>
deletion_time    n/a
destroyed        false
version          1

====== Data ======
Key         Value
---         -----
foo         a
bar         b
```

Также вы можете использовать политику генерации паролей, чтобы создавать секреты.

Создать политику:

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

Создать секрет, используя политику `example`:

```shell-session
$ d8 stronghold kv put -mount=secret my-generated-secret \
    password=$(d8 stronghold read -field password sys/policies/password/example/generate)
```

```plaintext
========= Secret Path =========
secret/data/my-generated-secret

======= Metadata =======
Key                Value
---                -----
created_time       2024-06-10T14:32:32.37354939Z
custom_metadata    <nil>
deletion_time      n/a
destroyed          false
version            1
```

Прочитать созданный секрет:

```shell-session
$ d8 stronghold kv get -mount=secret my-generated-secret
========= Secret Path =========
secret/data/my-generated-secret

======= Metadata =======
Key                Value
---                -----
created_time       2024-06-10T14:32:32.37354939Z
custom_metadata    <nil>
deletion_time      n/a
destroyed          false
version            1

====== Data ======
Key         Value
---         -----
password    !hh&be1e4j16dVc0ggae
```

### Удаление (delete) и уничтожение (destroy) секретов

При удалении команда `d8 stronghold kv delete` будет выполнять «мягкое» удаление. Она пометит версию как удаленную и заполнит значение `deletion_time` в метаданных секрета. Мягкое удаление не удаляет данные версии из хранилища, и секрет можно восстановить с помощью команды `d8 stronghold kv undelete`.

Версия секрета удаляется навсегда только в том случае, если секрета имеет больше версий, чем разрешено настройкой max-versions, или при использовании команды `d8 stronghold kv destroy`. При использовании команды destroy данные версии будут удалены, а метаданные будут помечены как уничтоженные. Если версия очищена из-за превышения количества версий, метаданные версии также будут удалены.

Примеры:

Последняя версия ключа может быть удалена с помощью команды delete, которая также   принимает флаг `-versions` для удаления предыдущих версий:

```shell-session
 $ d8 stronghold kv delete -mount=secret my-secret
 Success! Data deleted (if it existed) at: secret/data/my-secret
```

Версии могут быть восстановлены:

```shell-session
 $ d8 stronghold kv undelete -mount=secret -versions=2 my-secret
 Success! Data written to: secret/undelete/my-secret

 $ d8 stronghold kv get -mount=secret my-secret
 ====== Metadata ======
 Key              Value
 ---              -----
 created_time     2024-06-19T17:23:21.834403Z
 custom_metadata  <nil>
 deletion_time    n/a
 destroyed        false
 version          2

 ====== Data ======
 Key         Value
 ---         -----
 my-value    short-lived-s3cr3t
```

Уничтожение версии полностью удаляет все данные:

```shell-session
$ d8 stronghold kv destroy -mount=secret -versions=2 my-secret
Success! Data written to: secret/destroy/my-secret
```

### Метаданные

Все версии и метаданные ключа можно посмотреть с помощью команды metadata или с помощью API. Удаление ключа metadata приведет к тому, что все метаданные и версии для этого ключа будут удалены навсегда.

Примеры:

Можно просмотреть все метаданные и версии для ключа:

```shell-session
$ d8 stronghold kv metadata get -mount=secret my-secret
========== Metadata ==========
Key                     Value
---                     -----
cas_required            false
created_time            2024-06-19T17:20:22.985303Z
current_version         2
custom_metadata         <nil>
delete_version_after    0s
max_versions            0
oldest_version          0
updated_time            2024-06-19T17:22:23.369372Z

====== Version 1 ======
Key              Value
---              -----
created_time     2024-06-19T17:20:22.985303Z
deletion_time    n/a
destroyed        false

====== Version 2 ======
Key              Value
---              -----
created_time     2024-06-19T17:22:23.369372Z
deletion_time    n/a
destroyed        true
```

Можно настроить параметры:

```shell-session
$ d8 stronghold kv metadata put -mount=secret -max-versions 2 -delete-version-after="3h25m19s" my-secret
Success! Data written to: secret/metadata/my-secret
```

   Настройка `delete-version-after` будет применяться только к новым версиям, параметр `max-versions` будет применен при следующей операции записи.

```shell-session
$ d8 stronghold kv put -mount=secret my-secret my-value=newer-s3cr3t
Key              Value
---              -----
created_time     2024-06-19T17:31:16.662563Z
custom_metadata  <nil>
deletion_time    2024-06-19T20:56:35.662563Z
destroyed        false
version          4
```

   Если у ключа больше версий, чем `max-versions`б самые старые версии уничтожаются:

```shell-session
$ d8 stronghold kv metadata get -mount=secret my-secret
========== Metadata ==========
Key                     Value
---                     -----
cas_required            false
created_time            2024-06-19T17:20:22.985303Z
current_version         4
custom_metadata         <nil>
delete_version_after    3h25m19s
max_versions            2
oldest_version          3
updated_time            2024-06-19T17:31:16.662563Z

====== Version 3 ======
Key              Value
---              -----
created_time     2024-06-19T17:23:21.834403Z
deletion_time    n/a
destroyed        true

====== Version 4 ======
Key              Value
---              -----
created_time     2024-06-19T17:31:16.662563Z
deletion_time    2024-06-19T20:56:35.662563Z
destroyed        false
```

   Метаданные ключа секрета могут содержать пользовательские метаданные, используемые для описания секрета, в виде пар ключ-значение. Флаг `-custom-metadata` можно указать несколько раз, чтобы добавить несколько пар ключ-значение.

   Команда `d8 stronghold kv metadata put` может быть использована для полной перезаписи значения `custom_metadata`:

```shell-session
$ d8 stronghold kv metadata put -mount=secret -custom-metadata=foo=abc -custom-metadata=bar=123 my-secret
Success! Data written to: secret/metadata/my-secret

$ d8 stronghold kv get -mount=secret my-secret
====== Metadata ======
Key              Value
---              -----
created_time     2024-06-19T17:22:23.369372Z
custom_metadata  map[bar:123 foo:abc]
deletion_time    n/a
destroyed        false
version          2

====== Data ======
Key         Value
---         -----
foo         aa
bar         bb
```

   Команда `d8 stronghold kv metadata patch` может быть использована для частичной перезаписи значения `custom_metadata`. Следующий вызов обновит поле `custom_metadata` `foo`, но оставит `bar` нетронутым:

```shell-session
$ d8 stronghold kv metadata patch -mount=secret -custom-metadata=foo=def my-secret
Success! Data written to: secret/metadata/my-secret
```

```shell-session
$ d8 stronghold kv get -mount=secret my-secret
====== Metadata ======
Key              Value
---              -----
created_time     2024-06-19T17:22:23.369372Z
custom_metadata  map[bar:123 foo:def]
deletion_time    n/a
destroyed        false
version          2

====== Data ======
Key         Value
---         -----
foo         aa
bar         bb
```

Полное уничтожение всех метаданных и версий для ключа:

```shell-session
$ d8 stronghold kv metadata delete -mount=secret my-secret
Success! Data deleted (if it existed) at: secret/metadata/my-secret
```
