---
title: "Local authentication"
permalink: en/admin/configuration/access/authentication/local.html
---

In addition to external authentication providers, DKP also supports local authentication.

Local authentication involves creating User and Group resources in the cluster for static users and groups:

- A User object stores user information, including email and a hashed password (the password is not stored in plain text).
- A Group object defines a list of users grouped together.

## Creating a static user

To create a static user, create a User resource.

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

To group static users together, create a Group resource.

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

Members: List of users included in the group (specified as `kind`: User and username).

Once the group is created and includes all necessary users, proceed by configuring [authorization](../../access/authorization/).
