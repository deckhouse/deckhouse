---
title: "The multitenancy-manager module"
search: multitenancy
---

The module allows you to create isolated environments within a single kubernetes cluster based on the [user-authz](../../modules/140-user-authz) module and kubernetes resources (`NetworkPolicy`, `LimitRange`, `ResourceQuota`, etc.)

All customization is done using [Custom Resources](cr.html).

## Module capabilities

- User and group access control based on the Kubernetes RBAC mechanism (based on the [user-authz](../../modules/140-user-authz) module).
- Isolation level managing of the specific environments.
- Creation of templates for several environments and customizations with parameters according to the OpenAPI specification.
- Full compatibility with `helm` in resource templates.
