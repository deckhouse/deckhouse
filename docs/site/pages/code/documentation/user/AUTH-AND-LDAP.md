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

Deckhouse Code supports signing in through external authentication providers (OmniAuth), including OpenID Connect (OIDC) and SAML. The sections below describe the general OmniAuth parameters, the provider parameters, and the extended functionality of Deckhouse Code.

### Supported providers

The `name` parameter of a provider entry accepts one of the following values:

- `openid_connect` — OpenID Connect (described below);
- `saml` — SAML (described below);
- `oauth2_generic` — any OAuth 2.0 provider;
- `jwt` — JWT authentication;
- `github` — GitHub;
- `gitlab` — GitLab.com;
- `google_oauth2` — Google;
- `azure_activedirectory_v2` — Microsoft Entra ID (Azure AD);
- `atlassian_oauth2` — Atlassian;
- `crowd` — Atlassian Crowd;
- `auth0` — Auth0;
- `alicloud` — AliCloud;
- `salesforce` — Salesforce;
- `shibboleth` — Shibboleth.

Signing in through LDAP is configured separately, in the `spec.appConfig.ldap.` section (see [LDAP synchronization](#ldap-synchronization)).

### General OmniAuth parameters

The parameters are set in the `spec.appConfig.omniauth.` section:

- `enabled` — allows signing in through external providers.
  Default — `true`.
- `providers` — the list of providers users are allowed to sign in through.
  Default — `[]`.
- `allow_single_sign_on` — the list of providers for which an account is created automatically on the first sign-in (for example, `['openid_connect']`). Also accepts `true` (all providers) and `false`. If automatic creation is disabled, the user must first get a Deckhouse Code account and then link it to the provider.
  Default — `false`.
- `block_auto_created_users` — if `true`, automatically created accounts are blocked pending administrator approval.
  Default — `true`.
- `auto_link_ldap_user` — links the account to an LDAP account on the first sign-in (see [Linking OIDC accounts to LDAP](#linking-oidc-accounts-to-ldap)).
  Default — `false`.
- `auto_link_user` — links a sign-in through a provider to an existing Deckhouse Code account by email address. Accepts a list of providers or the `true` and `false` values.
  Default — `false`.
- `auto_sign_in_with_provider` — the name of the provider whose sign-in page the user is redirected to automatically, bypassing the Deckhouse Code sign-in page.
  Default — `false`.
- `external_providers` — the list of providers whose accounts are created as external.
  Default — `[]`.
- `allow_bypass_two_factor` — the list of providers that do not require two-factor authentication on sign-in. Also accepts the `true` and `false` values.
  Default — `false`.
- `sync_profile_from_provider` — the list of providers whose data updates the user profile on every sign-in. Also accepts the `true` and `false` values.
  Default — `false`.
- `sync_profile_attributes` — the list of profile attributes to update during synchronization: `name`, `email`, `location`. The synchronized attributes become read-only.
  Default — `['email']`.

### OpenID Connect (OIDC)

Providers are listed in the `providers` parameter of the `spec.appConfig.omniauth.` section. The following parameters are available for an OIDC provider:

- `name` — the provider type. For OIDC, it is always `'openid_connect'`.
- `label` — the sign-in button label.
  Default — `'Openid Connect'`.
- `icon` — the address of the image shown on the sign-in button.
- `args` — the provider connection parameters:
  - `name` — the OmniAuth strategy name, matching the value of the provider `name` parameter;
  - `scope` — the list of requested scopes, for example `['openid', 'profile', 'email']`;
  - `response_type` — the OAuth 2.0 response type. For the Authorization Code flow, it is `'code'`;
  - `issuer` — the OIDC provider address;
  - `discovery` — if `true`, the provider settings are retrieved automatically from `<issuer>/.well-known/openid-configuration`;
  - `client_auth_method` — the client authentication method at the token endpoint: `'basic'` or `'query'`;
  - `uid_field` — the field from the user data used as the account `uid` (for example, `preferred_username`). If the parameter is not set or the field is missing, the `sub` field is used;
  - `send_scope_to_token_endpoint` — whether to pass the `scope` parameter in requests to the token endpoint. Set it to `false` if the provider does not accept this parameter.
    Default — `true`;
  - `pkce` — enables Proof Key for Code Exchange (PKCE);
  - `client_options`:
    - `identifier` — the identifier of the client registered with the provider;
    - `secret` — the client secret;
    - `redirect_uri` — the address of your Deckhouse Code installation with the `/users/auth/openid_connect/callback` path. The same address must be specified in the client settings on the provider side.

Additionally, Deckhouse Code supports the following parameters. They are set at the top level of the provider entry, next to the `name` parameter:

- `allowed_groups` — a list of groups whose users are allowed to log in. Users not in these groups will be denied access.
  Default — `null` (all groups are allowed).

- `admin_groups` — a list of groups whose users are granted administrative privileges.
  Default — `null` (no groups are granted admin rights).

- `auditor_groups` — a list of groups whose users are granted the auditor role: read-only access to all groups and projects, without access to the admin area.
  Default — `null` (no groups are granted the auditor role).

- `groups_attribute` — the name of the attribute used to extract user group information.
  Default — `'groups'`.

{% alert level="info" %}
The `admin_groups` and `auditor_groups` parameters are taken into account only if the `allowed_groups` parameter is set. If a user belongs to both `admin_groups` and `auditor_groups`, administrative privileges are granted.
{% endalert %}

### OIDC configuration example

This configuration is set in the `spec.appConfig.omniauth.` section:

```yaml
providers:
  - name: 'openid_connect'   # Do not change this value.
    label: 'Keycloak'        # Sign-in button label.
    allowed_groups:
      - 'gitlab'
    admin_groups:
      - 'admin'
    auditor_groups:
      - 'audit'
    groups_attribute: 'gitlab_group'
    args:
      name: 'openid_connect'
      scope:
        - 'openid'
        - 'profile'
        - 'email'
      response_type: 'code'
      issuer: 'https://keycloak.example.com/realms/example'
      discovery: true
      client_auth_method: 'query'
      uid_field: 'preferred_username'
      send_scope_to_token_endpoint: false
      pkce: true
      client_options:
        identifier: '<client_id>'
        secret: '<client_secret>'
        redirect_uri: 'https://code.example.com/users/auth/openid_connect/callback'
```

## SAML

The same parameters are available for SAML providers:

- `allowed_groups` — a list of groups whose members are allowed to log in.  
  Default — `null` (all groups are allowed).

- `admin_groups` — groups whose members are granted administrative privileges.  
  Default — `null` (no groups are granted admin rights).

- `auditor_groups` — groups whose members are granted the auditor role: read-only access to all groups and projects.  
  Default — `null` (no groups are granted the auditor role).

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

{% alert level="info" %}
If a user belongs to `admin_groups` but is not listed in `allowed_groups`, access will be denied. In this case, administrative privileges will not be granted either.
{% endalert %}

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

### Multiple LDAP servers

The `spec.appConfig.ldap.` section can describe several LDAP servers. The key of an entry is the server name; the provider name is derived from it as `ldap<key>` in lowercase. For example, the `main` key corresponds to the `ldapmain` provider. This name is stored in the account identity and appears in the `server` field of the synchronization log entries.

Each server has its own connection parameters. The synchronization parameters — the `group_sync` section with everything in it, including `role_mapping` — are taken into account only for the server under the `main` key; on the other servers the section is ignored (see [Synchronization scope](#synchronization-scope)):

```yaml
main:
  label: 'Head office'
  host: ldap-main.example.com
  base: 'ou=People,dc=example,dc=com'
  uid: 'cn'
  # Other connection parameters.
  group_sync: {
    base: 'ou=Groups,dc=example,dc=com',
    role_mapping: [
      { by_name: '.*-developer-.*', gitlab_role: 'developer' }
    ]
  }
contractors:
  label: 'Contractors'
  host: ldap-contractors.example.com
  base: 'ou=People,dc=contractors,dc=example,dc=com'
  uid: 'cn'
  # Other connection parameters.
```

Signing in works through any of the described servers: the sign-in page and the admin area sign-in page show a separate tab for each server. No extra setting is needed to enable this.

Which servers are used depends on the scenario:

| Scenario | Servers used |
| -------- | ------------ |
| Signing in through the LDAP form in the web interface | All servers, one tab each |
| Looking up the account when linking to OIDC (`auto_link_ldap_user`) | All servers, one by one until the first match |
| Synchronization of users, groups, and access rights | The `ldapmain` server only |
| Periodic access re-check | `ldapmain`, if the account has its identity; otherwise, the directories of the account's own identities |

#### Synchronization scope

Synchronization of users, groups, and access rights covers a single server — the one whose provider name is `ldapmain`, that is, the server under the `main` key. The other servers remain a source of sign-in, but not a source of groups and rights. The limitation applies both to the scheduled synchronization task and to the synchronization that runs when a user signs in.

{% alert level="warning" %}
The `ldapmain` name is fixed and cannot be configured. If there is no server under the `main` key in the section, synchronization does not run for any of the described servers.
{% endalert %}

As a result:

- a user who exists only in a non-main directory can sign in to Deckhouse Code but receives no groups or rights from LDAP — they have to be assigned manually;
- for a user with two identities (the same primary email address in both directories), groups and rights always come from `ldapmain`, no matter which server they signed in through;
- the `group_sync` section of non-main servers is not read, so an error in their `group_sync.filter` parameter does not prevent the application from starting. An error in the filter of the `main` server is still detected at startup and keeps the application down.

When synchronization reaches a non-main server, it skips the server and writes a single `INFO` level entry about it, with the name of the skipped server in the `server` field:

```console
Server is not synchronized: users, groups and permissions come from 'ldapmain' only
```

#### User identification with multiple servers

A user is looked up first by account identity, then by primary email address:

- if the primary address is the same in both directories, signing in through the second server adds a second LDAP identity to the existing account; no new account is created;
- if the addresses differ, two separate accounts are created.

The list of account identities is available in the admin area, on the `/admin/users/<username>/identities` page.

#### Access decision with multiple identities

Deckhouse Code periodically, at most once an hour, re-checks the access of an LDAP user: it queries the directory and blocks the account if the entry is not found there, or unblocks it if the entry is found again. If the account has several LDAP identities, the directory for the check is chosen as follows:

- if the `ldapmain` identity is present, only it decides, and the other directories are not queried at all. If the user exists in `ldapmain` and is not disabled there, access is granted; if the user is missing or disabled there, access is denied regardless of what happens in the other directories;
- if the `ldapmain` identity is absent, meaning the user exists only in non-main directories, the user is checked against their own identities, and access is kept as long as the entry is found in at least one of the corresponding directories.

"The entry is found" means that it exists in the directory and is not disabled through Active Directory. The latter is checked only for directories configured as AD.

The rule is aligned with the synchronization scope: synchronization blocks users based on the `ldapmain` data, and the access re-check blocks exactly the same users. Because of that, signing in through another directory does not lift a block set by synchronization.

{% alert level="warning" %}
When moving a user from the `ldapmain` directory to another directory, remove the stale `ldapmain` identity from their account on the `/admin/users/<username>/identities` page. As long as that identity is in place, the user stays blocked even if they exist in another directory. The identity is not removed automatically.
{% endalert %}

Removing or disabling a user in a non-main directory does not revoke access on its own while the user remains in `ldapmain`. Access has to be revoked in the `ldapmain` directory.

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
- `group_sync.name_mask` — regular expression used to extract the group name from the CN (Common Name) attribute.
- `group_sync.owner` — name of the user to be assigned as group owner (default is `root`).

### `role_mapping` section

Assigns roles to users based on group names (`cn`):

- `role_mapping.by_name` — a regular expression; if the group name matches, the corresponding role is assigned to the user.
- `role_mapping.gitlab_role` — the name of a role available on the instance.

The list of available roles comes from the standard role catalog — the same one used to decide which roles can be granted to a group or project member. The catalog is read anew on every synchronization run, so a role disabled at the instance level is not in it and cannot be used in `role_mapping`.

Currently the catalog provides seven names:

| `gitlab_role` | Role | Access level |
| ------------- | ---- | ------------ |
| `guest` | Guest | 10 |
| `planner` | Planner | 15 |
| `reporter` | Reporter | 20 |
| `security_manager` | Security Manager | 25 |
| `developer` | Developer | 30 |
| `maintainer` | Maintainer | 40 |
| `owner` | Owner | 50 |

The role name is written exactly as shown in the `gitlab_role` column: lowercase, with words separated by underscores. Case and spaces are not normalized, so `Security Manager`, for example, counts as an unrecognized name.

The `security_manager` role is present in the catalog only if the role is enabled on the instance (the `GITLAB_SECURITY_MANAGER_ROLE` environment variable, enabled by default). If the role is disabled, its name counts as unrecognized.

If several rules match an LDAP group name, the numerically highest access level is assigned. For example, if a group matches both a rule with the `security_manager` role (level 25) and a rule with the `developer` role (level 30), its members get the Developer role.

#### Role name validation

Role names are validated once, at the start of membership distribution — including the rules that no LDAP group matched in that run. This also catches a dormant typo that would otherwise show up only once a matching group appears.

The validation does not interrupt membership distribution:

- LDAP groups whose matching rules are all recognized are processed in full and receive memberships;
- an unrecognized rule does not take part in the access level calculation;
- if only unrecognized rules match an LDAP group, no access level is determined for it, and the group is skipped entirely in that run: its current members are neither recalculated nor removed.

The run itself, however, ends with an error — after the groups, the users, and the valid memberships have been written. As a result:

- the run counts as abnormally finished on the metrics page of the synchronization task, and the successful synchronization mark is not updated (see [Manual synchronization run](#manual-synchronization-run));
- an `ERROR` level entry is written to the logs, listing both the unrecognized names and the full set of valid ones:

  ```console
  Unknown gitlab_role in group_sync.role_mapping: 'security_manger'. Available roles: guest, planner, reporter, security_manager, developer, maintainer, owner
  ```

Memberships are also distributed when a user signs in through an LDAP provider. On that path the same error is only written to the logs: the sign-in works as usual, and the remaining memberships are assigned.

### Group membership resolution

{% alert level="warning" %}
LDAP Sync does not support transitivity for nested groups. See [Nested groups and transitivity](#nested-groups-and-transitivity) section for details and workarounds.
{% endalert %}

Deckhouse Code supports the following attributes to determine group membership (all values are arrays of DNs):

- `member`
- `uniquemember`
- `memberof`
- `memberuid`
- `submember`

### User synchronization

During synchronization, usernames, email addresses, and account lock status are updated.

**Optional parameters:**

- `sync_name` — if `true`, the username will be updated based on LDAP data.

#### Blocking users based on LDAP data

If a user is removed from LDAP, the next scheduled synchronization blocks their account. If the user is restored in LDAP, the next synchronization unblocks the account automatically.

Blocking always happens and requires no additional settings: LDAP remains the source of truth for the account status. A blocked user will be denied access even if they sign in through an OIDC provider where the account is still active.

If your directory does not delete users but blocks them using an attribute, exclude blocked users from LDAP results using the `user_filter` parameter. Specify a single filter matching your directory:

```yaml
user_filter: '(!(pwdAccountLockedTime=*))'                        # OpenLDAP ppolicy
user_filter: '(!(nsAccountLock=TRUE))'                            # 389-DS
user_filter: '(!(userAccountControl:1.2.840.113556.1.4.803:=2))'  # Active Directory
user_filter: '(!(employeeType=blocked))'                          # custom attribute
```

### Linking OIDC accounts to LDAP

If users sign in through an OIDC provider (for example, Keycloak) while permissions are granted based on LDAP groups, enable automatic linking of the OIDC account to the LDAP account. This configuration is set in the `spec.appConfig.omniauth.` section:

```yaml
auto_link_ldap_user: true
```

#### How the LDAP account is found

On the first sign-in through OIDC, Deckhouse Code looks the user up in LDAP. The search uses two values from the OIDC provider data:

- `uid` — the value of the field specified in the provider `uid_field` parameter (for example, `preferred_username`);
- email — the user email address.

The configured LDAP servers are queried one by one. On each server, up to four search attempts are made, until the first match:

| Value being searched for | LDAP attribute searched |
| ------------------------ | ----------------------- |
| `uid`                    | The attribute specified in the LDAP server `uid` parameter (for example, `cn`) |
| `uid`                    | Mail attributes: `mail`, `email`, `userPrincipalName` |
| email                    | The same mail attributes |
| `uid`                    | DN — if the `uid` value is a DN itself |

If the user is found, an LDAP identity with the discovered DN is added to their account. If none of the attempts succeeds, the account is created without an LDAP link.

This means linking works only if the `uid` or email from the OIDC provider matches the value of the corresponding attribute in LDAP.

#### First and subsequent sign-ins

The first successful sign-in links the account to LDAP. On subsequent sign-ins:

- LDAP is not queried — the previously established link is used;
- access and administrative privileges are re-evaluated based on the groups provided by the OIDC provider (the `allowed_groups` and `admin_groups` parameters). If the user is removed from an allowed group, the account is blocked;
- group and project memberships, as well as roles in them, do not change on an OIDC sign-in — they are updated by the scheduled background LDAP synchronization.

As a result, right after the first sign-in a user can log in but has no group or project memberships yet: they appear after the next synchronization. To avoid waiting for it, run the synchronization manually (see [Manual synchronization run](#manual-synchronization-run)).

{% alert level="info" %}
Group and membership synchronization on sign-in runs only when a user signs in through an LDAP provider (a provider whose name starts with `ldap`). An OIDC sign-in does not trigger it, even if the account is already linked to LDAP.
{% endalert %}

#### LDAP as the source of truth

To allow users found in LDAP to sign in immediately and require administrator approval for everyone else, set the following parameters in the `spec.appConfig.` section:

```yaml
omniauth:
  auto_link_ldap_user: true
  # The user is not found in LDAP — the account is created blocked,
  # pending administrator approval.
  block_auto_created_users: true
ldap:
  main:
    # The user is found in LDAP — sign-in is allowed immediately.
    block_auto_created_users: false
```

#### Linking specifics

- If a user is blocked by renaming their `cn` in the directory, on the first sign-in they may still be found by email and linked to the LDAP account. To block users, remove them from the directory or use the `user_filter` parameter (see [Blocking users based on LDAP data](#blocking-users-based-on-ldap-data)).
- If a user was not linked to LDAP and was blocked by the `Ldap::BlockNonLdapUsersWorker` job, automatic unblocking will not work. Such a user must be unblocked manually and linked to an LDAP account.

### Troubleshooting synchronization issues

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
1. In the upper-right corner, click the "Enqueue Now" button and confirm in the dialog.
   ![Ldap sync worker UI](/images/code/ldap_sync_worker_en.png)

To see how the triggered synchronization finished, open the metrics page for the LDAP synchronization task:
`/admin/sidekiq/metrics?substr=SyncWorker&period=8h`. The chart displays call statistics; the table below shows the number of successful and failed LDAP synchronization runs.

![Ldap sync worker metrics](/images/code/ldap_sync_metrics.png)

To view the full synchronization logs:

1. On the worker page `/admin/sidekiq/cron/namespaces/default/jobs/ldap_sync_worker`, find the run events table named "History". The first row corresponds to the most recent run. Copy the value in the JID (Job ID) column — you will need it to search the logs.

   ![Ldap sync history table](/images/code/ldap_sync_history_en.png)

1. Connect to the cluster and determine the Sidekiq pod name:
   `d8 k -n d8-code -l app.kubernetes.io/component=sidekiq get pod -o NAME`

1. Run the log collection command, substituting the copied JID and pod name (POD_NAME):
   `d8 k -n d8-code logs POD_NAME | jq 'select(.jid=="JID")'`

{% alert level="info" %}
Old logs are removed by rotation over time, so they may become unavailable. If needed, rerun the synchronization and collect the latest logs.
{% endalert %}

### LDAP Sync behavior

#### Synchronization algorithm

LDAP Sync uses a flat, non-recursive synchronization algorithm:

1. Group retrieval. An LDAP query retrieves all groups based on the configured `base`, `filter`, and `scope` parameters.
1. Member extraction. For each discovered group, LDAP Sync reads the membership attributes: `member`, `uniquemember`, `memberof`, `memberuid`, `submember`.
1. User matching. Each DN from the membership attributes is matched against `Identity.extern_uid` in the database.
1. Ignoring unknown DNs. If a DN does not match a known user, it is skipped. For example, this may be the DN of a nested group.

#### Cyclic group dependencies

Cyclic dependencies in the LDAP group hierarchy do not cause synchronization errors. LDAP Sync processes groups in a flat way, according to the configured filter, and does not attempt to reconstruct the LDAP tree structure.

Because recursive traversal of nested groups is not performed, cycles do not affect the synchronization result.

#### Nested groups and transitivity

{% alert level="warning" %}
LDAP Sync does not support transitivity for nested groups.
{% endalert %}

During synchronization, LDAP Sync processes only the direct values of a group's membership attributes and does not recursively traverse nested groups.

If one LDAP group contains another group as a member, users from the nested group are not automatically added to the parent group.

For synchronization to work correctly, all required user DNs must be present directly in the group's membership attributes.

If nested groups must be taken into account, this must be implemented on the LDAP server side. For example, the `submember` attribute can be populated with the full list of transitive members.

This approach simplifies synchronization and avoids issues related to recursive group processing.

#### Creating a local account when LDAP synchronization is enabled

Local accounts can still be created and used even when LDAP synchronization is enabled.

To allow such users to sign in through the web interface, enable the "Enable password and passkey authentication for the web interface" setting (`password_authentication_enabled_for_web`) in the admin area, on the Settings → General → Sign-in restrictions page.

## Configuration example: signing in through OIDC with permissions from LDAP

The steps below describe a setup where users sign in through an OIDC provider (for example, Keycloak), while groups, memberships, and roles come from LDAP.

### Prerequisites

- An OIDC provider.
- An LDAP directory with the same users and with groups whose names allow determining the role.
- An LDAP service account with read access to the directory (the `bind_dn` and `password` parameters).

{% alert level="info" %}
The `uid` or email value in the OIDC provider must match the value of the corresponding attribute in LDAP, otherwise linking will not work (see [How the LDAP account is found](#how-the-ldap-account-is-found)).
{% endalert %}

### Step 1. Configure the LDAP provider

The configuration is defined in `spec.appConfig.ldap.`:

```yaml
main:
  label: ldap
  host: ldap.example.com
  port: 3389
  bind_dn: 'uid=viewer,ou=People,dc=example,dc=com'
  password: 'viewer123'
  base: 'ou=People,dc=example,dc=com'
  uid: 'cn'
  sync_name: true
  # Ignore users blocked in the directory (optional).
  user_filter: '(!(nsAccountLock=TRUE))'
  # The user is found in LDAP — sign-in is allowed immediately.
  block_auto_created_users: false
  group_sync: {
    create_groups: true,
    base: 'ou=Groups,dc=example,dc=com',
    filter: '(objectClass=groupOfNames)',
    top_level_group: "LdapGroups",
    name_mask: "(?<=-)[A-z0-9]*$",
    owner: "root",
    role_mapping: [
      { by_name: '.*-maintainer-.*', gitlab_role: 'maintainer' },
      { by_name: '.*-developer-.*', gitlab_role: 'developer' },
      { by_name: '.*-participant-.*', gitlab_role: 'reporter' }
    ]
  }
```

### Step 2. Configure the OIDC provider and enable linking to LDAP

The configuration is set in the `spec.appConfig.omniauth.` section:

```yaml
auto_link_ldap_user: true
providers:
  - name: 'openid_connect'   # Do not change this value.
    label: 'Keycloak'        # Sign-in button label.
    groups_attribute: 'gitlab_group'
    args:
      name: 'openid_connect'
      scope:
        - 'openid'
        - 'profile'
        - 'email'
      response_type: 'code'
      issuer: 'https://keycloak.example.com/realms/example'
      discovery: true
      client_auth_method: 'query'
      uid_field: 'preferred_username'
      send_scope_to_token_endpoint: false
      pkce: true
      client_options:
        identifier: '<client_id>'
        secret: '<client_secret>'
        redirect_uri: 'https://code.example.com/users/auth/openid_connect/callback'
```

Pay attention to the `uid_field` parameter: the field it points to becomes the account `uid`, and this value is used when looking the user up in LDAP. It must match the value of the attribute specified in the LDAP server `uid` parameter (`cn` in this example), or the email address — for details, see [How the LDAP account is found](#how-the-ldap-account-is-found).

The remaining provider parameters are described in the [OpenID Connect (OIDC)](#openid-connect-oidc) section.

### Step 3. Verify linking on the first sign-in

1. Sign in with a test user through the OIDC provider.
1. Open the user page in the admin area (`/admin/users/<username>/identities`). A linked account must have two identities: `openid_connect` and `ldapmain` (the LDAP identity name consists of the `ldap` prefix and the LDAP server name, `main` in this example).

If the LDAP identity is missing, check the following:

- the `uid` or email values match in the OIDC provider and in LDAP;
- the user falls within the `base` search scope and is not filtered out by `user_filter`;
- the `auto_link_ldap_user` parameter is enabled.

### Step 4. Wait for the permissions to be synchronized

Right after the first sign-in, the user has an account but no group or project memberships. Wait for the next synchronization or run it manually (see [Manual synchronization run](#manual-synchronization-run)), then check that:

- the groups are created inside the group specified in `group_sync.top_level_group`;
- the user is added to them with the role matching `role_mapping`.
