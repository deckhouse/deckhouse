---
title: "ClickHouse"
permalink: en/stronghold/documentation/user/secrets-engines/databases/clickhouse.html
lang: en
description: |-
  ClickHouse is one of the supported plugins for the database secrets engine.
  This plugin generates database credentials dynamically based on configured
  roles for the ClickHouse database.
---

{% raw %}

## ClickHouse database secrets engine

ClickHouse is one of the supported plugins for the database secrets engine. This
plugin generates database credentials dynamically based on configured roles for
the ClickHouse database, and also supports Static
Roles.

## Capabilities

| Plugin Name                  | Root Credential Rotation | Dynamic Roles | Static Roles | Username Customization |
| ---------------------------- | ------------------------ | ------------- | ------------ | ---------------------- |
| `clickhouse-database-plugin` | Yes                      | Yes           | Yes          | Yes                    |

## Setup

1. Enable the database secrets engine if it is not already enabled:

    ```shell-session
    $ d8 stronghold secrets enable database
    Success! Enabled the database secrets engine at: database/
    ```

    By default, the secrets engine will enable at the name of the engine. To
    enable the secrets engine at a different path, use the `-path` argument.

1. Configure Stronghold with the proper plugin and connection information:

   ```shell-session
   $ d8 stronghold write database/roles/my-role \
        db_name="my-clickhouse-database" \
        creation_statements="CREATE USER '{{name}}' IDENTIFIED BY '{{password}}' ON CLUSTER 'my_cluster'; \
            GRANT readonly TO '{{name}}' ON CLUSTER 'my_cluster'; \
            SET DEFAULT ROLE readonly TO '{{name}}';" \
        default_ttl="1h" \
        max_ttl="24h"
     Success! Data written to: database/roles/my-role
   ```

2. Configure a role that maps a name in Stronghold to an SQL statement to execute to
    create the database credential.
    The example assumes that the `readonly` role has been created in the `my_cluster` database cluster.

   ```shell-session
   $ d8 stronghold write database/roles/my-role \
        db_name="my-clickhouse-database" \
        creation_statements="CREATE USER '{{name}}' IDENTIFIED BY '{{password}}' ON CLUSTER 'my_cluster'; \
            GRANT readonly TO '{{name}}' ON CLUSTER 'my_cluster'; \
            SET DEFAULT ROLE readonly TO '{{name}}';" \
        default_ttl="1h" \
        max_ttl="24h"
    Success! Data written to: database/roles/my-role
    ```

## Usage

After the secrets engine is configured and a user/machine has an Stronghold token with
the proper permission, it can generate credentials.

1. Generate a new credential by reading from the `/creds` endpoint with the name
    of the role:

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
