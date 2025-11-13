---
title: "Руководство администратора модуля stronghold: Механизм секретов KV"
LinkTitle: "Руководство администратора: Механизм секретов KV"
description: "Руководство администратора: Механизм секретов KV в модуле stronghold."
---

## Механизм секретов KV (key-value / ключ-значение)

Механизм секретов `kv` представляет собой общее хранилище ключ-значение для хранения произвольных секретов в настроенном физическом хранилище для Stronghold. Этот механизм секретов может работать в двух режимах:

* kv версии 1, для хранения одного значения для ключа
* kv версии 2, с версионностью и возможностью хранения произвольно настроиваемового количества версий для каждого ключа.

### KV версия 1

При использовании механизма секретов `kv` хранилища в режиме без поддержки версионирования, сохраняется только последнее обновленное значение ключа. Основным преимуществом использования данного режима является уменьшение занимаемого пространства на хранилище для каждого ключа, так как не сохраняются дополнительные метаданные и история изменений. Кроме того, операции запроса к механизму секретов, настроенному таким образом, являются более производительными, так как для каждого конкретного запроса требуется меньше обращений к хранилищу данных и не возникает блокировки при изменении значения ключа.

### KV версия 2

При использовании версии 2 механизма секретов kv ключ может сохранять настраиваемое количество версий. По умолчанию это 10 версий. Метаданные и данные старых версий могут быть извлечены из каждой сохраненной версии. Кроме того, для предотвращения случайной перезаписи данных можно использовать операции Check-and-Set.

При удалении версии данные, лежащие в ее основе, не удаляются, а помечаются как удаленные. Удаление версии может быть отменено. Для окончательного удаления данных версии можно использовать консольную команду destroy или отправить запрос в соответствующий путь API. Кроме того, все версии и метаданные для ключа могут быть удалены командой `delete` по метаданным или конечной точкой API. На каждую из этих операций можно наложить различные ACL, ограничивающие права на мягкое удаление, удаление без удаления или полное удаление данных.

### Включение механизма секретов

Для начала работы с механизмом секретов KV, необходимо включить его по пути `kv`. Каждый путь полностью изолирован и не может взаимодействовать с другими путями. Например, механизм KV-секретов, включенный в foo, не может взаимодействовать с механизмом KV-секретов, включенным в bar.

```shell
vault secrets enable -path=kv kv
Success! Enabled the kv secrets engine at: kv/
```

Путь, по которому включен механизм секретов, по умолчанию равен имени механизма секретов. Таким образом, следующая команда эквивалентна выполнению приведенной выше команды.

```shell
vault secrets enable kv
```

Выполнение этой команды приведет к ошибке _path is already in use at kv/_.

Чтобы проверить успешность операции и получить дополнительные сведения о механизме управления секретами, используйте команду `vault secrets list`:

```shell
vault secrets list
Path Type Accessor Description
---- ---- -------- -----------
cubbyhole/ cubbyhole cubbyhole_78189996 per-token private secret storage
identity/ identity identity_ac07951e identity store
kv/ kv kv_15087625 n/a
secret/ kv kv_4b990c45 key/value secret storage
sys/ system system_adff0898 system endpoints used for control, policy and debugging
```

Это подтверждает наличие на сервере модуля stronghold пяти активных механизмов управления секретами. Можно увидеть тип такого механизма, соответствующий путь и необязательное описание (или «n/a», если оно не указано). При выполнении вышеуказанной команды с флагом `-detailed ` становится доступной информация о версии KV системы управления секретами, а также многое другое.

_Путь sys/ соответствует бэкенду системы. Эти пути взаимодействуют с основной системой Stronghold и не являются обязательными для новичков._

Потратьте несколько минут на чтение и запись некоторых данных в новый KV-механизм управления секретами, расположенный по адресу `kv/`. Ниже приведены несколько примеров для старта.

Для создания секретов используйте команду `kv put`.

```shell
vault kv put kv/hello target=world
Success! Data written to: kv/hello
```

Для чтения секретов, хранящихся в пути kv/hello, используйте команду `kv get`, как представлено на примере:

```shell
vault kv get kv/hello
===== Data =====
Key Value
--- -----
target world
```

Создайте секреты по пути `kv/my-secret`, как представлено на примере:

```shell
vault kv put kv/my-secret value="s3c(eT"
Success! Data written to: kv/my-secret
```

Читайте секреты по пути `kv/my-secret`, как представлено на примере:

```shell
vault kv get kv/my-secret
==== Data ====
Key Value
--- -----
value s3c(eT
```

Удалите секреты по адресу `kv/my-secret`, как представлено на примере:

```shell
vault kv delete kv/my-secret
Success! Data deleted (if it existed) at: kv/my-secret
```

Перечислите существующие ключи на пути `kv`, как представлено на примере:

```shell
vault kv list kv/
Keys
----
hello
```

### Отключение механизма секретов

Если необходимость в механизме управления секретами отпадает, его можно отключить. При отключении такого механизма все секреты удаляются, а соответствующие данные и настройки модуля stronghold уничтожаются.

```shell
vault secrets disable kv/
Success! Disabled the secrets engine (if it existed) at: kv/
```

> Обратите внимание, вышеприведенная команда принимает в качестве аргумента путь к механизму управления секретами, а не тип механизма управления секретами. Любые попытки маршрутизации данных по исходному пути привели бы к ошибке, однако теперь по этому пути может быть включен другой механизм управления секретами.

## Управление секретами

Механизм секретов Key/Value - это универсальное хранилище ключевых значений, используемое для хранения произвольных секретов в пределах настроенного физического хранилища модуля stronghold.

Секреты, записанные в модуле stronghold, шифруются и затем записываются во внутреннее хранилище. Внутренний механизм хранения данных не имеет доступа к незашифрованным значениям и не обладает средствами, необходимыми для их расшифровки без использования модуля stronghold.

Механизм секретов ключ/значение имеет версии 1 и 2. Разница в том, что v2 обеспечивает версионность секретов, а v1 - нет.

Для взаимодействия с механизмом секретов K/V используйте команду `vault kv <подкоманда> [options] [args]`.

Доступные подкоманды перечислены в следующей таблице:

| Подкоманда        | kv v1 | kv v2 | Описание                                                                       |
|-------------------|-------|-------|--------------------------------------------------------------------------------------|
| delete            | x     | x     | Удаление версий секретов, хранящихся в K/V                                           |
| destroy           |       | x     | Постоянное удаление одной или нескольких версий секретов                             |
| enable-versioning |       | x     | Включение версионности для существующего хранилища K/V v1                            |
| get               | x     | x     | Получение данных                                                                     |
| list              | x     | x     | Перечислить данные или секреты                                                       |
| metadata          |       | x     | Взаимодействие с хранилищем ключей-значений Stronghold                           |
| patch             |       | x     | Обновление секретов без перезаписи существующих секретов                             |
| put               | x     | x     | Установка или обновление секретов (при этом происходит замена существующих секретов) |
| rollback          |       | x     | Откат к предыдущей версии секретов                                                   |
| undelete          |       | x     | Восстановление удаленной версии секретов                                             |

### Получение справки по командам

Взаимодействовать с механизмом секретов ключ/значение можно с помощью команды `vault kv`.

Получите справку по команде:

```shell
vault kv -help
Usage: vault kv <subcommand> [options] [args]
This command has subcommands for interacting with Stronghold's key-value
store. Here are some simple examples, and more detailed examples are
available in the subcommands or the documentation.
Create or update the key named "foo" in the "secret" mount with the value
"bar=baz":
$ vault kv put -mount=secret foo bar=baz
Read this value back:
$ vault kv get -mount=secret foo
Get metadata for the key:
$ vault kv metadata get -mount=secret foo
Get a specific version of the key:
$ vault kv get -mount=secret -version=1 foo
The deprecated path-like syntax can also be used, but this should be avoided
for KV v2, as the fact that it is not actually the full API path to
the secret (secret/data/foo) can cause confusion:
$ vault kv get secret/foo
Please see the individual subcommand help for detailed usage information.
Subcommands:
delete Deletes versions in the KV store
destroy Permanently removes one or more versions in the KV store
enable-versioning Turns on versioning for a KV store
get Retrieves data from the KV store
list List data or secrets
metadata Interact with Stronghold's Key-Value storage
patch Sets or updates data in the KV store without overwriting
put Sets or updates data in the KV store
rollback Rolls back to a previous version of data
undelete Undeletes versions in the KV store
```

### Записывание секрета

Перед началом работы ознакомьтесь со справкой по команде:

```shell
vault kv put -help
```

В справке приведены примеры команд, а также необязательные параметры, которые можно использовать.

Запишите ключ-значение `secret` в путь `hello`, с ключом `foo` и значением `world`, используя команду `vault kv put` против пути `mount path secret`, на котором установлен механизм управления секретами KV v2. Эта команда создаст новую версию секрета и заменит все ранее существовавшие данные по указанному пути, если они существуют.

```shell
vault kv put -mount=secret hello foo=world
== Secret Path ==
secret/data/hello
======= Metadata =======
Key Value
--- -----
created_time 2022-06-15T19:36:54.389113Z
custom_metadata <nil>
deletion_time n/a
destroyed false
version 1
```

Важно, чтобы путь монтирования к механизму секретов KV v2 был указан с параметром `-mount=secret`, иначе данный пример не будет работать. Путь монтирования `secret` (который был автоматически задан при запуске сервера модуля stronghold в режиме `-dev`) - это место, где можно читать и записывать произвольные секреты.

С помощью kv put можно записывать несколько фрагментов данных.

```shell
vault kv put -mount=secret hello foo=world excited=yes
== Secret Path ==
secret/data/hello
======= Metadata =======
Key Value
--- -----
created_time 2022-06-15T19:49:06.761365Z
custom_metadata <nil>
deletion_time n/a
destroyed false
version 2
```


>Обратите внимание, что версия теперь равна 2.
В примерах этого руководства для отправки секретов в Stronghold используется ввод &lt;ключ>=&lt;значение>. Однако отправка данных в составе команды CLI часто попадает в историю оболочки в незашифрованном виде.

### Чтение секрета

Cекреты могут быть получены с помощью _vault kv get_.

```shell
vault kv get -mount=secret hello
== Secret Path ==
secret/data/hello
======= Metadata =======
Key Value
--- -----
created_time 2022-01-15T01:40:09.888293Z
custom_metadata <nil>
deletion_time n/a
destroyed false
version 2
===== Data =====
Key Value
--- -----
excited yes
foo world
```

Модуль stronghold возвращает последнюю версию (в данном случае версию 2) секретов по адресу `secret/hello`.

Чтобы вывести только значение заданного поля, используйте флаг -field=&lt;имя_ключа>.

```shell
vault kv get -mount=secret -field=excited hello
yes
```

Необязательный JSON-вывод может быть очень полезен для скриптов. Например, с помощью jq можно получить значение извлеченного секрета:

```shell
vault kv get -mount=secret -format=json hello | jq -r .data.data.excited
yes
```

### Удаление секрета

Удалить секрет можно с помощью команды _vault kv delete_.

```shell
vault kv delete -mount=secret hello
Success! Data deleted (if it existed) at: secret/data/hello
```

Для проверки, попробуйте прочитать секрет, который только что удалили:

```shell
vault kv get -mount=secret hello
== Secret Path ==
secret/data/hello
======= Metadata =======
Key Value
--- -----
created_time 2022-01-15T01:40:09.888293Z
custom_metadata <nil>
deletion_time 2022-01-15T01:40:41.786995Z
destroyed false
version 2
```

>На выходе отображаются только метаданные со временем удаления (deletion_time). Сами данные после удаления недоступны. Обратите внимание, что параметр `destroyed` со значением `false` указывает на возможность восстановления удаленных данных, если удаление произошло случайно.

```shell
vault kv undelete -mount=secret -versions=2 hello
Success! Data written to: secret/undelete/hello
```

Теперь данные восстановлены, как представлен на примере:

```shell
vault kv get -mount=secret hello
======= Metadata =======
Key Value
--- -----
created_time 2022-01-15T01:40:09.888293Z
custom_metadata <nil>
deletion_time n/a
destroyed false
version 2
===== Data =====
Key Value
--- -----
excited yes
foo world
```
