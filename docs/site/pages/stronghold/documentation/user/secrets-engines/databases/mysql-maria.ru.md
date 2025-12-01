---
title: "Механизм секретов баз данных MySQL/MariaDB"
permalink: ru/stronghold/documentation/user/secrets-engines/databases/mysql.html
lang: ru
description: |-
  MySQL is one of the supported plugins for the database secrets engine. This
  plugin generates database credentials dynamically based on configured roles
  for the MySQL database.
---

{% raw %}
MySQL - один из поддерживаемых плагинов для механизма секретов баз данных.
Этот плагин генерирует учетные данные базы данных динамически на основе
настроенных ролей для базы данных MySQL, а также поддерживает статические роли.

Этот плагин имеет несколько различных экземпляров, встроенных в Stronghold,
каждый из которых предназначен для немного разных драйверов MySQL. Единственное
различие между этими плагинами заключается в длине имен пользователей,
генерируемых плагином, так как разные версии mysql принимают разные длины.
Доступны следующие плагины:

- mysql-database-plugin
- mysql-aurora-database-plugin
- mysql-rds-database-plugin
- mysql-legacy-database-plugin

## Возможности

| Имя плагина                     | Изменение Root учетной записи | Динамические роли | Статические роли | Кастомизация имени пользователя |
|---------------------------------|-------------------------------|-------------------|------------------|---------------------------------|
| Может меняться                  | Да                            | Да                | Да               | Да                              |

## Установка

1. Включите механизм секретов базы данных, если он еще не включен:

```text
$ d8 stronghold secrets enable database
Success! Enabled the database secrets engine at: database/
```

   По умолчанию механизм секретов будет включаться на основе его имени.
   Чтобы включить механизм секретов по другому пути, используйте аргумент `-path`.

1. Настройте Stronghold с помощью соответствующего плагина и информации о подключении:

```text
$ d8 stronghold write database/config/my-mysql-database \
    plugin_name=mysql-database-plugin \
    connection_url="{{username}}:{{password}}@tcp(127.0.0.1:3306)/" \
    allowed_roles="my-role" \
    username="strongholduser" \
    password="strongholdpass"
```

1. Настройте роль, которая сопоставляет имя в Stronghold с SQL запросом,
   выполняемым для создания учетной записи базы данных:

```text
$ d8 stronghold write database/roles/my-role \
    db_name=my-mysql-database \
    creation_statements="CREATE USER '{{name}}'@'%' IDENTIFIED BY '{{password}}';GRANT SELECT ON *.* TO '{{name}}'@'%';" \
    default_ttl="1h" \
    max_ttl="24h"
Success! Data written to: database/roles/my-role
```

## Использование

После того как механизм секретов настроен и у пользователя/машины есть токен Stronghold с
соответствующими правами, он может генерировать учетные данные.

1. Сгенерируйте новую учетную запись, используя `/creds` и имя роли:

```text
$ d8 stronghold read database/creds/my-role
Key                Value
---                -----
lease_id           database/creds/my-role/2f6a614c-4aa2-7b19-24b9-ad944a8d4de6
lease_duration     1h
lease_renewable    true
password           yY-57n3X5UQhxnmFRP3f
username           v_strongholduser_my-role_crBWVqVh2Hc1
```

## Проверка подлинности сертификата клиента x509

Этот плагин поддерживает использование MySQL's [x509 Client-side Certificate Authentication](https://dev.mysql.com/doc/refman/8.0/en/using-encrypted-connections.html#using-encrypted-connections-client-side-configuration)

Чтобы использовать этот механизм аутентификации, настройте плагин:

```shell-session
$ d8 stronghold write database/config/my-mysql-database \
    plugin_name=mysql-database-plugin \
    allowed_roles="my-role" \
    connection_url="user:password@tcp(localhost:3306)/test" \
    tls_certificate_key=@/path/to/client.pem \
    tls_ca=@/path/to/client.ca
```

Примечание: `tls_certificate_key` и `tls_ca` соответствуют [`ssl-cert (combined with ssl-key)`](https://dev.mysql.com/doc/refman/8.0/en/connection-options.html#option_general_ssl-cert)
и [`ssl-ca`](https://dev.mysql.com/doc/refman/8.0/en/connection-options.html#option_general_ssl-ca) настройкам конфигурации из MySQL, за исключением того, что параметры
Stronghold - это содержимое этих файлов, а не имена файлов. Таким образом, эти два
параметра не зависят друг от друга. См. раздел [MySQL Connection Options](https://dev.mysql.com/doc/refman/8.0/en/connection-options.html)
для получения дополнительной информации.

## Примеры

### Использование шаблонов в grant statements

MySQL поддерживает использование шаблонов в grant statements. Это иногда необходимо приложениям,
которые ожидают доступа к большому количеству баз данных внутри MySQL.
Это можно реализовать, используя шаблоны в grant statements. Например, если
вы хотите, чтобы пользователь, созданный Stronghold, имел доступ ко всем
базам данных, начинающимся с `fooapp_`, вы можете использовать следующий оператор создания:

```text
CREATE USER '{{name}}'@'%' IDENTIFIED BY '{{password}}'; GRANT SELECT ON `fooapp\_%`.* TO '{{name}}'@'%';
```

MySQL ожидает, что часть, в которой должны быть помещены шаблоны, будет находиться
внутри кавычек. Если вы хотите добавить этот стейтмент создания в Stronghold
через Stronghold CLI, вы не можете просто вставить вышеприведенный оператор в
CLI, потому что shell интерпретирует текст между кавычками
как нечто, что должно быть выполнено. Самый простой способ обойти это - закодировать
стейтмент создания в Base64 и передать его в Stronghold.
Например:

```shell-session
$ d8 stronghold write database/roles/my-role \
    db_name=mysql \
    creation_statements="Q1JFQVRFIFVTRVIgJ3t7bmFtZX19J0AnJScgSURFTlRJRklFRCBCWSAne3twYXNzd29yZH19JzsgR1JBTlQgU0VMRUNUIE9OIGBmb29hcHBcXyVgLiogVE8gJ3t7bmFtZX19J0AnJSc7" \
    default_ttl="1h" \
    max_ttl="24h"
```

### Изменение root учетных данных in MySQL 5.6

По умолчанию для MySQL используется синтаксис `ALTER USER`, присутствующий в MySQL 5.7 и выше.
Для MySQL 5.6, `root_rotation_statements`
должны быть настроены на использование старого синтаксиса `SET PASSWORD`.
Например:

```shell-session
$ d8 stronghold write database/config/my-mysql-database \
    plugin_name=mysql-database-plugin \
    connection_url="{{username}}:{{password}}@tcp(127.0.0.1:3306)/" \
    root_rotation_statements="SET PASSWORD = PASSWORD('{{password}}')" \
    allowed_roles="my-role" \
    username="root" \
    password="mysql"
```

{% endraw %}
