---
title: "OmniAuth & LDAP"
menuTitle: OmniAuth and LDAP setup
force_searchable: true
description: guidelines on setting up OmniAuth and LDAP
permalink: en/code/documentation/user/oauth-and-ldap.html
lang: en
weight: 45
---

## OmniAuth configuration

Deckhouse Code supports OmniAuth configuration in accordance with the [GitLab official documentation](https://docs.gitlab.com/integration/omniauth/). Additionally, it provides extended functionality described below.

### OpenID connect (OIDC)

The following parameters are available for integrating with OIDC providers:

- `allowed_groups` — a list of groups whose users are allowed to log in. Users not in these groups will be denied access.  
  Default — `null` (all groups are allowed).

- `admin_groups` — a list of groups whose users are granted administrative privileges.  
  Default — `null` (no groups are granted admin rights).

- `groups_attribute` — the name of the attribute used to extract user group information.  
  Default — `'groups'`.

### OIDC configuration example

This configuration is set in the `spec.appConfig.omniauth.` section:

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

The same parameters are available for SAML providers:

- `allowed_groups` — a list of groups whose members are allowed to log in.  
  Default — `null` (all groups are allowed).

- `admin_groups` — groups whose members are granted administrative privileges.  
  Default — `null` (no groups are granted admin rights).

- `groups_attribute` — the name of the attribute that contains group information.  
  Default — `'Groups'`.

### SAML configuration example

This configuration is set in the `spec.appConfig.omniauth.` section:

```yaml
providers:
  - name: 'saml'
    allowed_groups:
      - 'gitlab'
    admin_groups:
      - 'admin'
    groups_attribute: 'gitlab_group'
```

> If a user belongs to `admin_groups` but is not listed in `allowed_groups`, access will be denied. In this case, administrative privileges will not be granted either.

## LDAP synchronization

Deckhouse Code supports synchronization of users, groups, and access rights with an LDAP server. Synchronization runs automatically every hour, or at a custom interval.

You can configure the synchronization interval using the `cronJobs` parameter in the `spec.appConfig.` section:

```yaml
cron_jobs:
  ldap_sync_worker:
    cron: "0 * * * *"
```

### LDAP server-side limitations

During synchronization, LDAP queries are executed for all users and groups defined in the configuration. Pagination is used automatically if necessary.  
If the LDAP server enforces limits on the number of returned entries, this may cause synchronization errors or lead to user access rights being removed.

### Example LDAP provider configuration

The configuration is defined in `spec.appConfig.ldap.`:

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
    name_mask: "(?<=-)[A-z0-9]*$",
    owner: "root",
    role_mapping: [
      { by_name: '.*-project_manager-.*', gitlab_role: 'maintainer' },
      { by_name: '.*-developer-.*', gitlab_role: 'developer' },
      { by_name: '.*-participant-.*', gitlab_role: 'reporter' }
    ]
  }
```

### Groups and access rights

LDAP groups are mapped to GitLab groups. You can assign roles to users based on group names.

Required parameters:

- `group_sync.base` — the DN from which LDAP group search starts.

Optional parameters:

- `group_sync.create_groups` — if `true`, groups will be created in Deckhouse Code.
- `group_sync.filter` — LDAP filter used to find groups.
- `group_sync.scope` — scope of group search (0 — Base, 1 — SingleLevel, 2 — WholeSubtree).
- `group_sync.prefix` — defines which attribute to use for determining the parent group name. If missing, the default value is used.
- `group_sync.top_level_group` — the top-level group to which all synchronized groups will be added.
- `group_sync.name_mask` — regular expression used to extract the group name from the CN attribute.
- `group_sync.owner` — name of the user to be assigned as group owner (default is `root`).

### `role_mapping` section

Assigns roles to users based on group names (`cn`):

- `role_mapping.by_name` — a regular expression; if the group name matches, the corresponding role is assigned to the user.
- `role_mapping.gitlab_role` — the role name in Deckhouse Code (e.g., `guest`, `reporter`, `developer`, `maintainer`, `owner`).

### Group membership resolution

Deckhouse Code supports the following attributes to determine group membership (all values must be arrays of user DNs):

- `member`
- `uniquemember`
- `memberof`
- `memberuid`
- `submember`

### User synchronization

During synchronization, usernames, email addresses, and account lock status are updated.

**Optional parameters:**

- `sync_name` — if `true`, the username will be updated based on LDAP data.

### Troubleshooting synchronization issues

#### Incorrect synchronization process

If a previous sync job was not completed successfully, Redis may retain a lock preventing the next job from starting (the default `concurrency` is set to 1).

To remove the lock:

1. Connect to Redis using the databases specified in `config/redis.shared_state.yml` and `config/redis.queues.yml`.
1. Delete the key `sidekiq:concurrency_limit:throttled_jobs:{ldap/sync_worker}` using the following commands:

   ```console
   keys *ldap*
   del "sidekiq:concurrency_limit:throttled_jobs:{ldap/sync_worker}"
   ```
   
### Manual synchronization run

To synchronize groups immediately after they are changed on the LDAP side, follow these steps:
1. Go to the LDAP synchronization worker page `/admin/sidekiq/cron/namespaces/default/jobs/ldap_sync_worker`.
2. In the upper-right corner, click the "Enqueue Now" button and confirm in the dialog.
   ![Ldap sync worker UI](/images/code/ldap_sync_worker_en.png)

To see how the triggered synchronization finished, open the metrics page for the LDAP synchronization task:
`/admin/sidekiq/metrics?substr=SyncWorker&period=8h`. The chart shows statistics for successful calls, and the table below shows counts of successful and failed LDAP synchronization runs.
![Ldap sync worker metrics](/images/code/ldap_sync_metrics.png)

To view the full synchronization logs:
1. On the worker page `/admin/sidekiq/cron/namespaces/default/jobs/ldap_sync_worker`, find the run events table named "History". The first row corresponds to the most recent run. Select the value in the JID (Job ID) column of the first row — you will need this identifier to search the logs.
   ![Ldap sync history table](/images/code/ldap_sync_history_en.png)
2. Log in to the virtual machine, then determine the name of the pod to collect logs from using `kubectl -n d8-code -l app.kubernetes.io/component=sidekiq get pod -o NAME`.
3. Using the synchronization process identifier (JID) and the pod name (POD_NAME), run the log collection command: `kubectl -n d8-code logs POD_NAME | jq 'select(.jid=="JID")'`.

> After some time, old logs are removed by rotation, and collecting them becomes impossible.
> If needed, you can rerun the synchronization process and collect the latest logs.
