---
title: "Local authentication"
permalink: en/admin/configuration/access/authentication/local.html
description: "Configure local authentication for Deckhouse Platform with password policies, 2FA support, and group management."
---

In addition to external authentication providers, DP also supports local authentication.

Local authentication provides user verification and access management with support for configurable password policies, two-factor authentication (2FA), and group management.
In the module parameters, you can configure password complexity, password history, password rotation, lockout after failed sign-in attempts and per-IP rate limiting of failed attempts. No external authentication system is required.

Local authentication involves creating User and Group resources in the cluster for [static users](../../../../glossary.html#static-user) and groups:

- A [User](/modules/user-authn/cr.html#user) object stores user information, including email and a hashed password (the password is not stored in plain text).
- A [Group](/modules/user-authn/cr.html#group) object defines a list of users grouped together.

## Creating a static user

The recommended way to create a local user is the [`d8 iam user create`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-iam-user-create) command. It supports interactive password entry, automatic password generation, group assignment, and TTL for temporary users.

Examples:

Interactive password prompt (default when stdin is a terminal):

```shell
d8 iam user create anton --email anton@abc.com
```

Automatically generate a password (the generated password is shown in the command output once):

```shell
d8 iam user create anton --email anton@abc.com --generate-password
```

Read the password from stdin (for CI/CD pipelines):

```shell
echo "s3cret" | d8 iam user create anton --email anton@abc.com --password-stdin
```

Create the user and add them to groups (auto-creating groups if missing):

```shell
d8 iam user create anton --email anton@abc.com --generate-password --member-of admins --create-groups
```

Create a temporary user with a TTL:

```shell
d8 iam user create anton --email anton@abc.com --generate-password --ttl 24h
```

As an alternative, you can create a [User](/modules/user-authn/cr.html#user) resource manually.

Example resource definition (note that the example includes a [ttl](/modules/user-authn/cr.html#user-v1-spec-ttl)):

```yaml
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  email: admin@yourcompany.com
  password: $2a$10$etblbZ9yfZaKgbvysf1qguW3WULdMnxwWFrkoKpRH1yeWa5etjjAa
  ttl: 24h
```

Come up with a password and specify it in the `password` field as a raw bcrypt hash or a Base64-encoded bcrypt hash.
To generate a Base64-encoded bcrypt hash, use the following command:

```shell
echo -n '<PASSWORD>' | htpasswd -BinC 10 "" | cut -d: -f2 | tr -d '\n' | base64 -w0; echo
```

{% alert level="info" %}
If the `htpasswd` command is not available, install the appropriate package:

* `apache2-utils` — for Debian-based distributions.
* `httpd-tools` — for CentOS-based distributions.
* `apache2-htpasswd` — for ALT Linux.
{% endalert %}

{% alert level="warning" %}
You cannot create a User whose `spec.email` matches a `User` subject of an existing ClusterAuthorizationRule or AuthorizationRule: the new user would get the permissions of the rule unnoticed. If the match is intentional, add the `user-authz.deckhouse.io/allow-authorization-rule-collision: "true"` annotation to the User. For details, see [How rules are applied](../authorization/rbac-current.html#how-rules-are-applied).
{% endalert %}

## Deleting a user

To delete a local user, use the [`d8 iam user delete`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-iam-user-delete) command. By default, it also removes the user from all [Group](/modules/user-authn/cr.html#group) resources they belong to.

Examples:

Delete the user (and automatically remove them from all groups):

```shell
d8 iam user delete anton
```

Delete the user but keep their references in groups:

```shell
d8 iam user delete anton --keep-memberships
```

## Local user operations

Password reset, forced password change, 2FA reset, user lock/unlock and logout operations are performed via the [UserOperation](/modules/user-authn/cr.html#useroperation) resource. The `initiatorType` field indicates who initiated the operation: an administrator (`admin`), the system (`system`), or the user (`self`).

### Administrative operations

Use the [`d8 iam user`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-iam) commands for administrative actions on local users. They create a UserOperation resource with `initiatorType: admin`, wait for the operation to complete, and print the result.

{% alert level="info" %}
Deleting, recreating, or running a `ResetPassword`, `Reset2FA`, `Lock`, or `Unlock` operation on a user is allowed only if you already have permissions covering the roles that the user receives through their email or group membership, or you are explicitly allowed to assign those roles. The value of the `initiatorType` field does not affect this check: setting `initiatorType: self` does not bypass it. For details, see the [`user-authn` module documentation](/modules/user-authn/usage.html#administrative-operations). The same rule applies to DexProvider objects, see [Limiting the identities of a provider](external-authentication-providers.html#limiting-the-identities-of-a-provider).
{% endalert %}

The `ResetPassword`, `Reset2FA`, `Lock` and `Logout` operations end the sessions of the user at once, and the user has to authenticate again. For what ends and how fast, see [UserOperation lifecycle and side effects](/modules/user-authn/usage.html#useroperation-lifecycle-and-side-effects). Only `Reset2FA` removes the user's 2FA secret.

After an administrator resets the password with `ResetPassword`, the user signs in with that password once. Dex then asks the user to set a new password that complies with the password policy.

Examples of using the [`d8 iam user`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-iam) commands:

- Interactive password reset:

  ```shell
  d8 iam user reset-password admin
  ```

- Reading the new password from stdin:

  ```shell
  echo "N3wPa$$wo#d" | d8 iam user reset-password admin --password-stdin
  ```

- Generating a new password automatically:

  ```shell
  d8 iam user reset-password admin --generate-password
  ```

- Reset the password in hashed form (if the password is hashed, provide the bcrypt hash without Base64 encoding):

  ```shell
  d8 iam user reset-password admin --password-hash '$2y$10$abcdef...'
  ```

- 2FA reset:

  ```shell
  d8 iam user reset2fa admin
  ```

- Locking a user for 30 minutes:

  ```shell
  d8 iam user lock admin 30m
  ```

  The `d8 iam user lock` command locks a local user and accepts a duration in seconds, minutes or hours, for example `30s`, `10m` or `1h`. To lock a user for several days (`lock.for: 7d`) or permanently (`lock.for: permanent`), create a UserOperation as described in [Creating UserOperation manually](#creating-useroperation-manually).

- User unlock:

  ```shell
  d8 iam user unlock admin
  ```

By default, commands wait for the operation to complete. To only create a UserOperation and print its name, use the `--wait=false` flag.

### Self-service operations

In the Deckhouse web interface, a user can sign out on all devices, and a local user can change their password. The web interface creates a UserOperation with `initiatorType: self` for the user who is signed in:

- Signing out on all devices creates a `Logout` operation. It ends every session of the user, including kubeconfig files.
- Changing the password creates a `RequirePasswordChange` operation. At the next sign-in, Dex asks for the current password and a new one that complies with the [password policy](#configuring-password-policy). The operation ends no sessions.

Changing the password is available only for local accounts (the built-in `Local` connector). Users who sign in through external authentication providers change their password in the corresponding system.

For user-facing password change and reset scenarios, see [Configuring authentication for applications](../../../../user/access/authentication.html#changing-and-resetting-a-local-users-password).

### Creating UserOperation manually

When running the `d8 iam user` CLI command is unavailable (e.g. in CI/CD, GitOps, or automation), you can create a [UserOperation](/modules/user-authn/cr.html#useroperation) resource directly. Example — reset a local user's password (the `newPasswordHash` contains a bcrypt hash without Base64 encoding; the module encodes it automatically):

```yaml
apiVersion: deckhouse.io/v1
kind: UserOperation
metadata:
  name: reset-password-admin
spec:
  user: admin
  type: ResetPassword
  initiatorType: admin
  resetPassword:
    newPasswordHash: "$2y$10$..."
```

For users of external providers of any type (LDAP, Atlassian Crowd, OIDC, SAML, GitHub, GitLab, Bitbucket Cloud), use [`spec.target`](/modules/user-authn/cr.html#useroperation-v1-spec-target) instead of `spec.user`. The `Lock`, `Unlock` and `Logout` operations are supported for external users. Example — lock an external user for 30 minutes:

```yaml
apiVersion: deckhouse.io/v1
kind: UserOperation
metadata:
  name: lock-external-user
spec:
  target:
    connectorID: my-ldap
    email: jane.doe@example.org
  type: Lock
  initiatorType: admin
  lock:
    for: "30m"
```

A lock applies only to sign-ins through the provider set in `connectorID`. It does not block a local user or a user of another provider with the same email. The operation fails if the user has never signed in through this provider.

A UserOperation is a **single-use** object: after creation, the module processes it and writes the result to `status.phase` (`Succeeded` or `Failed`). A UserOperation runs once, and changes to its spec after creation are ignored. Completed operations are **automatically deleted** after 24 hours.

{% alert level="warning" %}
The `ResetPassword`, `Reset2FA`, `Lock` and `Logout` operations end the sessions of the user at once, and the user has to authenticate again. For what ends and how fast, see [UserOperation lifecycle and side effects](/modules/user-authn/usage.html#useroperation-lifecycle-and-side-effects). `ResetPassword`, `Lock` and `Logout` keep the user's 2FA secret.
{% endalert %}

### Viewing account state

The `user-authn` module keeps a [UserAccount](/modules/user-authn/cr.html#useraccount) object for each local user and for each user who has signed in through an external provider. It shows whether the account is locked, until when, and how many failed sign-in attempts were counted. Do not create or edit UserAccount objects.

To list the accounts, run:

```shell
d8 k get useraccounts
```

## Adding a user to a group

{% alert level="warning" %}
It is forbidden to use users and groups with the `system:` prefix.  
Authentication attempts by such users or members of such groups will be rejected, and a corresponding warning will appear in the `kube-apiserver` logs.
{% endalert %}

{% alert level="warning" %}
You cannot create a Group whose `spec.name` matches a `Group` subject of an existing ClusterAuthorizationRule or AuthorizationRule: the members of the new group would get the permissions of the rule unnoticed. If the match is intentional, add the `user-authz.deckhouse.io/allow-authorization-rule-collision: "true"` annotation to the Group. For details, see [How rules are applied](../authorization/rbac-current.html#how-rules-are-applied).
{% endalert %}

The recommended way to manage groups is the [`d8 iam group`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-iam-group) command. Examples:

Create a group:

```shell
d8 iam group create admins
```

Add a user to a group:

```shell
d8 iam group add-member admins user anton
```

Remove a user from a group:

```shell
d8 iam group remove-member admins user anton
```

Delete a group:

```shell
d8 iam group delete admins
```

As an alternative, you can create a [Group](/modules/user-authn/cr.html#group) resource manually.

Example resource definition:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: Group
metadata:
  name: admins
spec:
  name: admins
  members:
    - kind: User
      name: admin
```

Where `members` is a list of users belonging to the group.

Once the group is created and includes all necessary users, proceed by configuring [authorization](../authorization/).

## Configuring password policy

Password policy allows controlling password complexity, rotation, and automatic lockout after consecutive failed login attempts.

An administrator can lock or unlock a user with [`d8 iam user lock` / `unlock`](../../../../cli/d8/reference/#d8-iam) (or a UserOperation) without enabling `passwordPolicy.lockout`. The `lockout` section only turns on automatic lockout after the configured number of failed attempts.

Dex stores an automatic lock itself and does not create a UserOperation for it. Local users are always covered. LDAP and Atlassian Crowd users are locked only if their provider type is listed in [`passwordPolicy.lockout.applyToConnectors`](/modules/user-authn/configuration.html#parameters-passwordpolicy-lockout-applytoconnectors). To limit failed sign-in attempts per client IP address, use the [`rateLimit`](/modules/user-authn/configuration.html#parameters-ratelimit) parameter.

To set up a password policy, use the [`passwordPolicy`](/modules/user-authn/configuration.html#parameters-passwordpolicy) field in the configuration of the `user-authn` module.

Examples of policies:

{% tabs Examples of password policies%}
{% tab "Without custom complexity rules" %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  version: 3
  enabled: true
  settings:
    passwordPolicy:
      complexityLevel: Fair
      passwordHistoryLimit: 10
      lockout:
        lockDuration: 15m
        maxAttempts: 3
      rotation:
        interval: "30d"
```

{% endtab %}
{% tab "With custom complexity rules" %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  version: 3
  enabled: true
  settings:
    passwordPolicy:
      complexityLevel: Custom
      custom:
        minLength: 10
        specialCharacters: true
        numbers: false
        capitalized: true
        repeatedChars: false
      passwordHistoryLimit: 10
```

{% endtab %}
{% endtabs %}

## Configuring two-factor authentication (2FA)

2FA increases security by requiring a code from a TOTP authenticator application (for example, Google Authenticator) during login.

To set up 2FA, use the [`staticUsers2FA`](/modules/user-authn/configuration.html#parameters-staticusers2fa) field in the configuration of the `user-authn` module:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  version: 3
  enabled: true
  settings:
    staticUsers2FA:
      enabled: true
      issuerName: "awesome-app"
```

Field description:

* `enabled`: Enables or disables 2FA for all [static users](../../../../glossary.html#static-user).
* `issuerName`: Name displayed in the authenticator application when adding an account.

{% alert level="info" %}
After enabling 2FA, each user must register in the authenticator application during their first login.
{% endalert %}
