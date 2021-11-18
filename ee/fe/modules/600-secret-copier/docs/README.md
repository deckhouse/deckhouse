---
title: "The secret-copier module"
---

This module copies secrets to all namespaces.

It facilitates the copying of secrets required to pull images and to provision Ceph's RBDs as part of the CI process.

### How does it work?

This module monitors the secrets (with the `secret-copier.deckhouse.io/enabled: ""` label) in the `default` namespace for changes.
* The module copies such a secret to all namespaces after it is created;
* When a secret is changed, its new contents are also propagated to all namespaces;
* When a secret gets deleted, its copies are deleted from all namespaces;
* The module restores the original content of a copy of a secret in the application namespace if it gets modified;
* The module copies all the secrets (that have the `secret-copier.deckhouse.io/enabled: ""` label) of the default namespace to any newly created namespace;

Also, it synchronizes the secrets every night and makes sure they are identical to those in the default namespace.

### What do I need to configure?

All you need to do is to create a secret with the `secret-copier.deckhouse.io/enabled: ""` label in the default namespace.

### How to synchronize Secret to some selected namespaces instead of all namespaces?

Specify namespace label-selector in the value of the `secret-copier.deckhouse.io/target-namespace-selector` annotation. For example: `secret-copier.deckhouse.io/target-namespace-selector: "app=custom"`. The module will create a copy of that Secret in all namespaces that matches the label-selector. 
