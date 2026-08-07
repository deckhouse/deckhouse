---
title: "Local authentication"
permalink: en/admin/configuration/access/authentication/local.html
description: "Configure local authentication for Deckhouse Kubernetes Platform with password policies, 2FA support, and group management. OWASP-compliant security implementation."
---

In addition to external authentication providers, DKP also supports local authentication.

Local authentication provides user verification and access management with support for configurable password policies, two-factor authentication (2FA), and group management.
The implementation complies with OWASP recommendations, ensuring reliable protection of access to the cluster and applications without requiring integration with external authentication systems.

Local authentication involves creating User and Group resources in the cluster for static users and groups:

- A [User](/modules/user-authn/cr.html#user) object stores user information, including email and a hashed password (the password is not stored in plain text).
- A [Group](/modules/user-authn/cr.html#group) object defines a list of users grouped together.

## Creating a static user

The recommended way to create a local user is the [`d8 iam user create`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-iam-user-create) command. It supports interactive password entry, automatic password generation, group assignment, and TTL for temporary users:

```shell
# Interactive password prompt (default when stdin is a terminal)
d8 iam user create anton --email anton@abc.com

# Automatically generate a password (shown once)
d8 iam user create anton --email anton@abc.com --generate-password

# Read the password from stdin (for CI/CD pipelines)
echo "s3cret" | d8 iam user create anton --email anton@abc.com --password-stdin

# Create the user and add to groups (auto-creating groups if missing)
d8 iam user create anton --email anton@abc.com --generate-password --member-of admins --create-groups

# Create a temporary user with a TTL
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

Come up with a password and specify its hashed value in the `password` field. The password is stored in encrypted form (bcrypt).  
You can generate the hash using the following command:

```shell
echo "$password" | htpasswd -BinC 10 "" | cut -d: -f2 | base64 -w0
```

{% alert level="info" %}
If `htpasswd` command not found, you need to install `apache2-utils` package for Debian-based distribution and `httpd-utils` for CentOS-based distribution.
If the `htpasswd` command is not available, install the appropriate package:

* `apache2-utils` — for Debian-based distributions.
* `httpd-tools` — for CentOS-based distributions.
* `apache2-htpasswd` — for ALT Linux.
{% endalert %}

## Deleting a user

To delete a local user, use the [`d8 iam user delete`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-iam-user-delete) command. By default, it also removes the user from all [Group](/modules/user-authn/cr.html#group) resources they belong to:

```shell
# Delete the user and remove them from all groups
d8 iam user delete anton

# Delete the user but keep their references in groups
d8 iam user delete anton --keep-memberships
```

## Local user operations

Password reset, 2FA reset, and user lock/unlock operations are performed via the [UserOperation](/modules/user-authn/cr.html#useroperation) resource. The `initiatorType` field indicates who initiated the operation: an administrator (`admin`), the system (`system`), or the user (`self`).

### Administrative operations

Use the [`d8 iam user`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-iam) commands for administrative actions on local users. They create a UserOperation resource with `initiatorType: admin`, wait for the operation to complete, and print the result.

The `ResetPassword`, `Reset2FA`, and `Lock` operations delete the user's Dex OfflineSessions and RefreshToken objects. This terminates the user's active offline sessions and requires re-authentication.

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

- User unlock:

  ```shell
  d8 iam user unlock admin
  ```

By default, commands wait for the operation to complete. To only create a UserOperation and print its name, use the `--wait=false` flag.

### Self-service password reset

A local user can reset their own password in the DKP authentication interface. This creates a UserOperation resource with `type: ResetPassword` and `initiatorType: self`.

Self-service password reset is available only for local accounts (the built-in `Local` connector). Users who sign in through external authentication providers must contact the administrator of the corresponding system.

When a user resets their password:

- The new password must comply with the [password policy](#configuring-password-policy).
- The user's active sessions are terminated and re-authentication is required.

For user-facing password change and reset scenarios, see [Configuring authentication for applications](../../../../user/access/authentication.html#changing-and-resetting-a-local-users-password).

### Creating UserOperation manually

When the `d8 iam user` CLI is unavailable (e.g. in CI/CD, GitOps, or automation), you can create a [UserOperation](/modules/user-authn/cr.html#useroperation) resource directly. Example — reset a local user's password (the `newPasswordHash` contains a bcrypt hash without Base64 encoding; the hook encodes it automatically):

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

For users authenticated through external providers (LDAP, Atlassian Crowd), use `spec.target` instead of `spec.user`. Only `Lock` and `Unlock` are supported for external users. Example — lock an external user for 30 minutes:

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

A UserOperation is a **single-use**, **immutable** object: after creation, the hook processes it and writes the result to `status.phase` (`Succeeded` or `Failed`). Completed operations are **automatically deleted** after 24 hours.

{% alert level="warning" %}
The `ResetPassword`, `Reset2FA`, and `Lock` operations terminate all active sessions of the user (they delete the user's Dex OfflineSessions and RefreshToken objects). The user will be forced to re-authenticate.
{% endalert %}

## Adding a user to a group

{% alert level="warning" %}
It is forbidden to use users and groups with the `system:` prefix.  
Authentication attempts by such users or members of such groups will be rejected, and a corresponding warning will appear in the `kube-apiserver` logs.
{% endalert %}

The recommended way to manage groups is the [`d8 iam group`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-iam-group) command:

```shell
# Create a group
d8 iam group create admins

# Add a user to a group
d8 iam group add-member admins user anton

# Remove a member from a group
d8 iam group remove-member admins user anton

# Delete a group
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

Password policy allows controlling password complexity, rotation, and user lockout.

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
  version: 2
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
  version: 2
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
  version: 2
  enabled: true
  settings:
    staticUsers2FA:
      enabled: true
      issuerName: "awesome-app"
```

Field description:

* `enabled`: Enables or disables 2FA for all static users.
* `issuerName`: Name displayed in the authenticator application when adding an account.

{% alert level="info" %}
After enabling 2FA, each user must register in the authenticator application during their first login.
{% endalert %}
