---
title: "Authentication"
permalink: en/admin/configuration/access/authentication/
description: "Configure authentication for Deckhouse Kubernetes Platform with local and external providers. Support for LDAP, OIDC, GitHub, GitLab, and more. Complete authentication setup guide."
---

**Authentication** is the process of verifying a user's identity, providing access control to all interfaces of the Deckhouse Kubernetes Platform (DKP) and cluster resources.  
The platform implements end-to-end authentication, allowing a unified mechanism to be applied both to internal components and user applications.

The core of the authentication mechanism is a federated OpenID Connect (OIDC) provider — `Dex`.  
Learn more about how authentication works in the [Architecture](../../../../architecture/authentication.html) section.

Depending on the configuration, DKP supports two authentication approaches:

- [Local authentication](./local.html) — users and groups are created directly in the cluster and stored as [User](/modules/user-authn/cr.html#user) and [Group](/modules/user-authn/cr.html#group) resources.  
  The User resource stores a hashed version of the password (bcrypt), not the plain-text password.
- [Integration with external providers](./external-authentication-providers.html) — enables connection to systems like LDAP, GitLab, GitHub, and others to support single sign-on across multiple DKP clusters.

From the perspective of a cluster user or application developer, the method chosen by the administrator to configure authentication in DKP does not matter — the authentication interface and integration steps are the same.

The platform also provides capabilities for:

- Enabling [authentication for any web application](./external-authentication-providers.html) in the cluster.
- Configuring [authenticated access to the Kubernetes API](./k8s-api-lb.html) via a load balancer.
