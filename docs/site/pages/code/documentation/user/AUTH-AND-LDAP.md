---
title: "OmniAuth & LDAP"
menuTitle: OmniAuth and LDAP setup
force_searchable: true
description: guidelines on setting up OmniAuth and LDAP
permalink: en/code/documentation/user/oauth-and-ldap.html
lang: en
weight: 45
---

## OmniAuth Configuration

Configuration mostly relies on the one documented in the [official documentation](https://docs.gitlab.com/integration/omniauth/). However, Deckhouse Code brings some extension over it described in sections below.

### OpenID Connect (OIDC)

For OIDC integration, the following new parameters have been added:

- **`allowed_groups`**: An array of groups permitted to log in. Users who do not belong to these groups will be blocked and unable to log in.  
  **Default:** `null` (allows all groups).

- **`admin_groups`**: An array of groups permitted to log in as administrators. Users belonging to these groups will be granted an admin role.  
  **Default:** `null` (no groups are granted admin privileges).

- **`groups_attribute`**: The attribute name used to retrieve user groups.  
  **Default:** `'groups'`.

### Example OIDC Configuration

Section is under `spec.appConfig.omniauth.`

```yaml
providers:
  - name: 'openid_connect'
    allowed_groups:
      - 'gitlab'
    admin_groups:
      - 'admin'
    groups_attribute: 'gitlab_group'
```

## SAML

For SAML integration, the following new parameters have been added:

- **`allowed_groups`**: An array of groups permitted to log in. Users who do not belong to these groups will be blocked and unable to log in.  
  **Default:** `null` (allows all groups).

- **`admin_groups`**: An array of groups permitted to log in as administrators. Users belonging to these groups will be granted an admin role.  
  **Default:** `null` (no groups are granted admin privileges).

- **`groups_attribute`**: The attribute name used to retrieve user groups.  
  **Default:** `'Groups'`.

### Example SAML Configuration

Section is under `spec.appConfig.omniauth.`

```yaml
providers:
  - name: 'saml'
    allowed_groups:
      - 'gitlab'
    admin_groups:
      - 'admin'
    groups_attribute: 'gitlab_group'
```

> **Note:** for oidc and SAML If a user belongs to `admin_groups` but is not present in `allowed_groups`, they will not be able to log in. In such cases, `admin_groups` will not be considered, and the user will not be granted administrative privileges.

## LDAP Synchronization

Performs synchronization of users, groups, and group access rights with the LDAP server. By default it happens once per hour.

You can configure the synchronization schedule via `cronJobs` param (at `spec.appConfig.` section):

```yaml
cron_jobs:
  ldap_sync_worker:
    cron: "0 * * * *"
```

### LDAP Server-Side Limits

During synchronization, queries are made for all users and groups specified in the configuration file.
The synchronization task automatically uses pagination if needed.
However, if the LDAP server has a limit on the maximum number of records returned, it may lead to unexpected user access being blocked or removed.

### Example LDAP Provider Configuration

Section is located under `spec.appConfig.ldap.`

```yaml
main:
  label: ldap
  host: 127.0.0.1
  port: 3389
  bind_dn: 'uid=viewer,ou=People,dc=example,dc=com'
  base: 'ou=People,dc=example,dc=com'
  uid: 'cn'
  password: 'viewer123'
  sync_name: true
  group_sync: {
    create_groups: true,
    base: 'ou=Groups,dc=example,dc=org',
    filter: '(objectClass=groupOfNames)',
    prefix: {
      attribute: 'businessCategory',
      default: 'default-program',
    },
    top_level_group: "LdapGroups",
    name_mask: "(?<=-)[A-z0-9А-я]*$",
    owner: "root",
    role_mapping: [
      { by_name: '.*-project_manager-.*', gitlab_role: 'maintainer' },
      { by_name: '.*-developer-.*', gitlab_role: 'developer' },
      { by_name: '.*-participant-.*', gitlab_role: 'reporter' }
    ]
  }
```

### Group and Access Rights Synchronization

Creates GitLab groups and assigns user roles based on records retrieved from the LDAP server.  
Can be configured with the following parameters:

Required Parameters:

- **group_sync.base** — The base DN from which the search begins.

Optional Parameters:

- **group_sync.create_groups** — If `true`, groups will be created in Deckhouse Code.
- **group_sync.filter** — LDAP filter for groups.
- **group_sync.scope** — Search scope (0 — Base, 1 — SingleLevel, 2 — WholeSubtree).
- **group_sync.prefix** — Defines which attribute to use for the parent group name. If the attribute is missing, the default value is used.
- **group_sync.top_level_group** — The name of the top-level group to which all synchronized groups will be added.
- **group_sync.name_mask** — A regular expression to extract the group name from the `cn` attribute.
- **group_sync.owner** — The username to be added as the group owner (default is `root`).

### ``role_mapping` section

Assigns access rights to users based on the group name (`cn`):

- **role_mapping.by_name** — A regular expression; if the group name matches, the corresponding `gitlab_role` is assigned to the user.
- **role_mapping.gitlab_role** — The Deckhouse Code role name (e.g., `guest`, `reporter`, `developer`, `maintainer`, `owner`).

> Deckhouse Code leverages basic roles from Gitlab

#### How group membership is defined

Group members are determined from the following group attributes. The value of each attribute is expected to be an array of user DNs:

- `member`
- `uniquemember`
- `memberof`
- `memberuid`
- `submember`

### User Synchronization

Locks and unlocks users and updates their name and email based on data from the LDAP server.

Optional parameters:

`sync_name` - if `true`, the user's name will be updated from LDAP data

#### Troubleshooting synchronization

If a previous synchronization job did not complete correctly, Redis may retain a record indicating the job is still running.  
This prevents a new job from starting because concurrency is set to 1.

To fix perform following steps:

1. Connect to Redis using the databases specified in `config/redis.shared_state.yml` and `config/redis.queues.yml`.
2. Delete the following key:  `sidekiq:concurrency_limit:throttled_jobs:{ldap/sync_worker}` by executing:
- `keys *ldap*` - makes sure the key exists
- `del "sidekiq:concurrency_limit:throttled_jobs:{ldap/sync_worker}"` - deletes the key
