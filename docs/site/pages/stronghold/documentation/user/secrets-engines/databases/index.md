---
title: "Databases"
permalink: en/stronghold/documentation/user/secrets-engines/databases/overview.html
lang: en
description: |-
  The database secrets engine generates database credentials dynamically based
  on configured roles. It works with a number of different databases through a
  plugin interface. There are a number of built-in database types and an exposed
  framework for running custom database types for extendability.
---

{% raw %}

## Databases

The database secrets engine generates database credentials dynamically based on
configured roles. It works with a number of different databases through a plugin
interface. There are a number of built-in database types, and an exposed framework
for running custom database types for extendability. This means that services
that need to access a database no longer need to hardcode credentials: they can
request them from Stronghold, and use Stronghold's [leasing mechanism](../../concepts/lease.html)
to more easily roll keys. These are referred to as "dynamic roles" or "dynamic
secrets".

Since every service is accessing the database with unique credentials, it makes
auditing much easier when questionable data access is discovered. You can track
it down to the specific instance of a service based on the SQL username.

Stronghold makes use of its own internal revocation system to ensure that users
become invalid within a reasonable time of the lease expiring.

### Static roles

With dynamic secrets, Stronghold generates a unique username and password pair for
each unique credential request. Stronghold also supports **static roles** for
some database secrets engines. Static roles are a 1-to-1 mapping of Stronghold roles
to usernames in a database. With static roles, Stronghold stores, and automatically
rotates, passwords for the associated database user based on a configurable
period of time.

When a client requests credentials for the static role, Stronghold
returns the current password for whichever database user is mapped to the
requested role. With static roles, anyone with the proper Stronghold policies can
access the associated user account in the database.

{% endraw %}
{% alert level="warning" %}[Do not use static roles for root database credentials]
   Do not manage the same root database credentials that you provide to Stronghold in
   <tt>config/</tt> with static roles.

   Stronghold does not distinguish between standard credentials and root credentials
   when rotating passwords. If you assign your root credentials to a static
   role, any dynamic or static users managed by that database configuration will
   fail after rotation because the password for <tt>config/</tt> is no longer
   valid.

   If you need to rotate root credentials, use the
   `rotate-root-credentials` API endpoint.

{% endalert %}
{% raw %}

Refer to the [database capabilities table](#database-capabilities) to determine
if your chosen database backend supports static roles.

## Setup

Most secrets engines must be configured in advance before they can perform their
functions. These steps are usually completed by an operator or configuration
management tool.

1. Enable the database secrets engine:

   ```shell-session
   $ d8 stronghold secrets enable database
   Success! Enabled the database secrets engine at: database/
   ```

   By default, the secrets engine will enable at the name of the engine. To
   enable the secrets engine at a different path, use the `-path` argument.

1. Configure Stronghold with the proper plugin and connection information:

   ```shell-session
   $ d8 stronghold write database/config/my-database \
       plugin_name="..." \
       connection_url="..." \
       allowed_roles="..." \
       username="..." \
       password="..." \
   ```

{% endraw %}
{% alert level="warning" %}

 It is highly recommended a user within the database is created
   specifically for Stronghold to use. This user will be used to manipulate
   dynamic and static users within the database. This user is called the
   "root" user within the documentation.
{% endalert %}
{% raw %}

   Stronghold will use the user specified here to create/update/revoke database
   credentials. That user must have the appropriate permissions to perform
   actions upon other database users (create, update credentials, delete, etc.).

   This secrets engine can configure multiple database connections. For details
   on the specific configuration options, please see the database-specific
   documentation.

1. After configuring the root user, it is highly recommended you rotate that user's
   password such that the stronghold user is not accessible by any users other than
   Stronghold itself:

   ```shell-session
   d8 stronghold write -force database/rotate-root/my-database
   ```

{% endraw %}
{% alert level="critical" %}
When this is done, the password for the user specified in the previous step
   is no longer accessible. Because of this, it is highly recommended that a
   user is created specifically for Stronghold to use to manage database
   users.

{% endalert %}
{% raw %}

1. Configure a role that maps a name in Stronghold to a set of creation statements to
   create the database credential:

   ```shell-session
   $ d8 stronghold write database/roles/my-role \
       db_name=my-database \
       creation_statements="..." \
       default_ttl="1h" \
       max_ttl="24h"
   Success! Data written to: database/roles/my-role
   ```

   The `{{username}}` and `{{password}}` fields will be populated by the plugin
   with dynamically generated values. In some plugins the `{{expiration}}` field is also supported.

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
    password           FSREZ1S0kFsZtLat-y94
    username           v-strongholduser-e2978cd0-ugp7iqI2hdlff5hfjylJ-1602537260
    ```

## Database capabilities

All databases support dynamic roles and static roles. All plugins support rotating
the root user's credentials.

| Database                                                   | Root Credential Rotation | Dynamic Roles | Static Roles | Username Customization | Credential Types |
|------------------------------------------------------------|--------------------------|---------------|--------------|------------------------|------------------|
| [MySQL/MariaDB](mysql.html) | Yes                      | Yes           | Yes          | Yes                    | password         |
| [PostgreSQL](postgresql.html)     | Yes                      | Yes           | Yes          | Yes                    | password         |

## Credential types

Database systems support a variety of authentication methods and credential types.
The database secrets engine supports management of credentials alternative to usernames
and passwords. The `credential_type`
and `credential_config` parameters
of dynamic and static roles configure the credential that Stronghold will generate and
make available to database plugins. See the documentation of individual database
plugins for the credential types they support and usage examples.

## Password generation

Passwords are generated via password policies.
Databases can optionally set a password policy for use across all roles or at the
individual role level for that database. For example, each time you call
`d8 stronghold write database/config/my-database` you can specify a password policy for all
roles using `my-database`. Each database has a default password policy defined as:
20 characters with at least 1 uppercase character, at least 1 lowercase character,
at least 1 number, and at least 1 dash character.

The default password generation can be represented as the following password policy:

```hcl
length = 20

rule "charset" {
 charset = "abcdefghijklmnopqrstuvwxyz"
 min-chars = 1
}
rule "charset" {
 charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
 min-chars = 1
}
rule "charset" {
 charset = "0123456789"
 min-chars = 1
}
rule "charset" {
 charset = "-"
 min-chars = 1
}
```

## Disable character escaping

You can specify the option `disable_escaping` with a value of `true` in some
secrets engines to prevent Stronghold from escaping special characters in the
username and password fields. This is necessary for some alternate connection
string formats.

For example, when the password contains URL-escaped characters like `#` or `%` they will
remain as so instead of becoming `%23` and `%25` respectively.

```shell-session
$ d8 stronghold write database/config/my-mysql-database \
plugin_name="mysql-database-plugin" \
connection_url='server=localhost;port=3306;user id={{username}};password={{password}};database=mydb;' \
username="root" \
password='your#StrongPassword%' \
disable_escaping="true"
```

{% endraw %}
