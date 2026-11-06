---
title: Static user
description: A user created in the cluster as a User object of local authentication, with an email and a hashed password, without an external authentication provider.
lang: en
---

Static users and groups are the basis of local authentication in Deckhouse Platform. A User object stores the user information and the password hash, the password is not stored in plain text. A Group object defines a list of users. Local authentication supports password policies and temporary users with a TTL. Two-factor authentication is also available.

The recommended way to create a static user is the `d8 iam user create` command.

For details, see [Local authentication](admin/configuration/access/authentication/local.html).
