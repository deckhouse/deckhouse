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

To create a static user, create a [User](/modules/user-authn/cr.html#user) resource.

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

## Adding a user to a group

{% alert level="warning" %}
It is forbidden to use users and groups with the `system:` prefix.  
Authentication attempts by such users or members of such groups will be rejected, and a corresponding warning will appear in the `kube-apiserver` logs.
{% endalert %}

To group static users together, create a [Group](/modules/user-authn/cr.html#group) resource.

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

Once the group is created and includes all necessary users, proceed by configuring [authorization](../../access/authorization/).

## Configuring password policy

Password policy allows controlling password complexity, rotation, and user lockout.

To set up a password policy, use the [`passwordPolicy`](/modules/user-authn/configuration.html#parameters-passwordpolicy) field in the configuration of the `user-authn` module:

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

Field description:

* `complexityLevel`: Password complexity level.
* `passwordHistoryLimit`: Number of previous passwords stored in the system to prevent their reuse.
* `lockout`: Lockout settings after exceeding the limit of failed login attempts:
  * `lockout.maxAttempts`: Limit of allowed failed login attempts.
  * `lockout.lockDuration`: User lockout duration.
* `rotation`: Password rotation settings:
  * `rotation.interval`: Period for mandatory password change.

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
