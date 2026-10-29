---
title: End-to-end authentication
description: Authentication implemented in Deckhouse Platform so that a user is verified when accessing any interface of the platform or any cluster resource. Users can also enable it for their own applications.
lang: en
---

Depending on the configuration, authentication uses either the internal user database ([static users](glossary.html#static-user)) or external authentication providers such as LDAP, GitLab or GitHub. An external provider lets users keep their existing credentials and use them in several Deckhouse Platform clusters. For a cluster user or an application developer the interface and the authentication methods for applications stay the same whichever way the administrator configured authentication.

For details, see [Configuring authentication for applications](user/access/authentication.html).
