---
title: "PostgreSQL"
permalink: ru/stronghold/documentation/user/secrets-engines/databases/postgresql.html
lang: ru
description: |-
  PostgreSQL is one of the supported plugins for the database secrets engine.
  This plugin generates database credentials dynamically based on configured
  roles for the PostgreSQL database.
---

{% raw %}
PostgreSQL это один из поддерживаемых плагинов для механизма секретов баз данных. Этот плагин генерирует
учетные данные базы данных динамически на основе настроенных ролей для базы данных PostgreSQL, а также
поддерживает Static Roles.

## Возможности

| Имя плагина                  | Изменение Root учетной записи | Динамические роли | Статические роли | Кастомизация имени пользователя |
|------------------------------|-------------------------------|-------------------|------------------|---------------------------------|
| `postgresql-database-plugin` | Да                            | Да                | Да               | Да                              |

## Установка

Включите механизм секретов базы данных, если он еще не включен:

```shell-session
$ d8 stronghold secrets enable database
Success! Enabled the database secrets engine at: database/
```

По умолчанию механизм секретов будет включаться на основе его имени.
Чтобы включить механизм секретов по другому пути, используйте аргумент `-path`.

Настройте Stronghold с помощью соответствующего плагина и информации о подключении:

```shell-session
$ d8 stronghold write database/config/my-postgresql-database \
  plugin_name="postgresql-database-plugin" \
  allowed_roles="my-role" \
  connection_url="postgresql://{{username}}:{{password}}@localhost:5432/database-name" \
  username="strongholduser" \
  password="strongholdpass" \
  password_authentication="scram-sha-256"
```

Настройте роль, которая сопоставляет имя в Stronghold SQL-запросом,
выполняемым для создания учетной записи базы данных:

```shell-session
$ d8 stronghold write database/roles/my-role \
  db_name="my-postgresql-database" \
  creation_statements="CREATE ROLE \"{{name}}\" WITH LOGIN PASSWORD '{{password}}' VALID UNTIL '{{expiration}}'; \
      GRANT SELECT ON ALL TABLES IN SCHEMA public TO \"{{name}}\";" \
  default_ttl="1h" \
  max_ttl="24h"
Success! Data written to: database/roles/my-role
```

## Использование

После того как механизм секретов настроен и у пользователя/машины есть токен Stronghold с
соответствующими правами, он может генерировать учетные данные.

Сгенерируйте новую учетную запись, используя `/creds` и имя роли:

```shell-session
$ d8 stronghold read database/creds/my-role
Key                Value
---                -----
lease_id           database/creds/my-role/2f6a614c-4aa2-7b19-24b9-ad944a8d4de6
lease_duration     1h
lease_renewable    true
password           SsnoaA-8Tv4t34f41baD
username           v-strongholduse-my-role-x
```

{% endraw %}
