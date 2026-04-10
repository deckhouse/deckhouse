---
title: "The service-with-healthchecks module: configuration"
---

{% alert level="info" %}

For the ServiceWithHealthchecks load balancers you create to work, the following conditions must be met:

* The network policy of the custom project in which the ServiceWithHealthchecks will be created must include a rule allowing incoming traffic from all pods in the `d8-service-with-healthchecks` namespace:
  
  ```yaml
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          kubernetes.io/metadata.name: d8-service-with-healthchecks
  ```

  For more information on network policies, see the [Configuring Network Policies](/products/kubernetes-platform/documentation/v1/admin/configuration/network/policy/configuration.html) section.

* The cluster role used in ClusterRoleBinding and RoleBinding when assigning permissions to users and service accounts for the ServiceWithHealthchecks resource must be extended with the following rules:

  * `get`
  * `list`
  * `watch`
  * `create`
  * `update`
  * `patch`
  * `delete`.

  For more details, see the section [Granting permissions to users and service accounts](/products/kubernetes-platform/documentation/latest/admin/configuration/access/authorization/granting.html).

{% endalert %}

<!-- SCHEMA -->