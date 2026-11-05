---
title: "The secret-copier module"
description: "Automatic copying of Secrets across namespaces in the Deckhouse Platform cluster."
---

This module copies Secrets to all namespaces.

It facilitates the copying of Secrets required to pull images and to provision Ceph's RBDs as part of the CI process.

{% alert level="warning" %}
The `secret-copier` module cannot be used together with `multitenancy-manager`.

`multitenancy-manager` creates isolated environments for users within their projects, while `secret-copier` automatically distributes secrets across all namespaces.
This may lead to sensitive data leaks if important secrets end up in a user's isolated environment, violating the security model.

If you need to provide a shared certificate (e.g., a WC certificate for an internal environment) or a shared registry access token, do not use `secret-copier`.
Instead, place such secrets in the project template in `multitenancy-manager` — the cluster administrator should define them in the project configuration.
{% endalert %}

### How does it work?

This module monitors the Secrets (with the `secret-copier.deckhouse.io/enabled: ""` label) in the `default` namespace for changes.
* The module copies such a Secret to all namespaces after it is created.
* When a Secret is changed, its new contents are also propagated to all namespaces.
* When a Secret is deleted, its copies are deleted from all namespaces.
* The module restores the original content of a copy of a Secret in the application namespace if it gets modified.
* The module copies all the Secrets (that have the `secret-copier.deckhouse.io/enabled: ""` label) of the `default` namespace to any newly created namespace.
* The module never creates, updates or deletes the Secret `d8-system/cluster-key`, which holds the cluster key of Deckhouse Platform licensing.
* The module does not copy Secrets of the `bootstrap.kubernetes.io/token` type. The copies of such Secrets that earlier versions of the module made are deleted.
* The module updates and deletes only its own copies. If a namespace already has a Secret with the same name that the module did not create, the module leaves it as is and does not copy the Secret into that namespace.

Additionally, it synchronizes the Secrets every night, ensuring they are identical to those in the `default` namespace.

### What do I need to configure?

All you need to do is to create a Secret with the `secret-copier.deckhouse.io/enabled: ""` label in the `default` namespace.

> **Note!** The working namespace for the module is `default`, Secrets will be copied only from it. Secrets with the secret-copier.deckhouse.io/enabled: "" label created in other namespaces will be automatically deleted when the module is enabled.

### How to synchronize Secret to some selected namespaces instead of all namespaces?

Specify namespace label-selector in the value of the `secret-copier.deckhouse.io/target-namespace-selector` annotation. For example: `secret-copier.deckhouse.io/target-namespace-selector: "app=custom"`. The module will create a copy of that Secret in all namespaces that matches the label-selector.
