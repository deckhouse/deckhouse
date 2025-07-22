---
title: "The dashboard module"
description: "Kubernetes Dashboard Web UI for managing applications in the Deckhouse Kubernetes Platform cluster."
webIfaces:
- name: dashboard
---

This module installs Kubernetes Dashboard [Web UI](https://github.com/kubernetes/dashboard) that allows you to manage applications running in the cluster. It is integrated with [user-authn](../../modules/user-authn/) and [user-authz](../../modules/user-authz/) modules (access to the cluster is based on the user ID and credentials).

Kubernetes Dashboard provides the following features:

- Managing pods and other high-level resources.
- Accessing containers via a web console for debugging.
- Viewing logs of individual containers.

{% alert level="warning" %}
The module does not work over HTTP.
{% endalert %}

For the module to work, it is necessary to:

1. Enable the [user-authz](../user-authz/) module;
1. Enable the [user-authn](../user-authn/) module or enable external authentication (see the [externalAuthentication](configuration.html#parameters-auth-externalauthentication) module parameters section).
