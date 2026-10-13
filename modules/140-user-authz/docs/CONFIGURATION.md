---
title: "The user-authz module: configuration"
---

{% alert level="warning" %}
Do not create Pods and ReplicaSets directly: these objects are secondary and should be created by controllers (Deployment, StatefulSet, Job, and others). The `User`, `PrivilegedUser`, `Editor`, and `ClusterEditor` access levels cannot create or modify Pods, and no access level below `SuperAdmin` can create or modify ReplicaSets.
{% endalert %}

In multi-tenancy mode, the `allowAccessToSystemNamespaces`, `namespaceSelector`, and `limitNamespaces` fields of ClusterAuthorizationRule are enforced by the authorization webhook. If the webhook is unavailable, the requests it has to answer are **denied**. The webhook does not check control plane components, kubelets, and the ServiceAccounts of `kube-system` and of the namespaces whose names start with `d8-`. For details, see [Basic role-based model](./#basic-role-based-model).

Access rights are configured with the roles of the [granular model](./#granular-role-based-model) in RoleBinding and ClusterRoleBinding objects, or with the [custom resources](cr.html) of the basic model.

<!-- SCHEMA -->
