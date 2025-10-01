---
title: "Overview"
permalink: en/admin/configuration/access/authorization/
description: "Configure authorization and access control for Deckhouse Kubernetes Platform using RBAC. Manage user permissions, roles, and service accounts for secure cluster access."
---

In Deckhouse Kubernetes Platform (DKP),
authorization is based on the standard Kubernetes Role-Based Access Control (RBAC) mechanism.
This allows for flexible access control for different users, groups, and service accounts,
ensuring security and operational control in the cluster.

DKP supports two role models:

- [Current](../authorization/rbac-current.html): The end-to-end authorization subsystem extends the standard RBAC mechanism
  using custom resources — [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule) and [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule).
- [Experimental](../authorization/rbac-experimental.html): This model also relies on the standard RBAC mechanism.
  Access is configured by creating [RoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/role-binding-v1/) or [ClusterRoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/cluster-role-binding-v1/) resources.

Both models are supported by the [`user-authz`](/modules/user-authz/) module.
The choice of model depends on security requirements and usage scenarios.

## Who gets access and when

There are two main scenarios for granting access in DKP:

- Granting access to users for working via command-line clients, web UI,
  and other tools used for administration, development, and cluster management.
- Granting access to service accounts for automating tasks such as application deployment and updates
  (most commonly using the IaC approach).
  Examples of such services include CI/CD systems, monitoring systems, and others.

After successful authentication, users and service accounts are granted access to cluster resources
based on the configured authorization settings.

### User authentication

DKP supports multiple user authentication methods.
For details, refer to [User authentication](../authentication/).

### Service account authentication

In Kubernetes, service accounts (ServiceAccount) are special accounts used to automate tasks and interact with the cluster API.
They enable applications and services to securely communicate with the Kubernetes API.
In DKP, service accounts for external services are created in the `d8-service-accounts` namespace to maintain consistency.

Example manifest for creating a ServiceAccount:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: gitlab-runner-deploy
  namespace: d8-service-accounts
```

After creating the ServiceAccount, issue a token so the service can authenticate in the cluster.
This is done by creating a secret that contains the access token.

Example manifest for creating a secret with a ServiceAccount token:

```yaml
 apiVersion: v1
 kind: Secret
 metadata:
   name: gitlab-runner-deploy-token
   namespace: d8-service-accounts
   annotations:
     kubernetes.io/service-account.name: gitlab-runner-deploy
 type: kubernetes.io/service-account-token
```
