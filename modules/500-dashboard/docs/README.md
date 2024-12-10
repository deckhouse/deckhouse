---
title: "The dashboard module"
webIfaces:
- name: dashboard
---

This module installs Kubernetes Dashboard [Web UI](https://github.com/kubernetes/dashboard) that allows you to manage applications running in the cluster. It is integrated with [user-authn](../../modules/150-user-authn/) and [user-authz](../../modules/140-user-authz/) modules (access to the cluster is based on the user ID and credentials).

The module does not work over HTTP and will be disabled.

{% alert level="warning" %}
The `user-authz` module is required for the dashboard module to work.
{% endalert %}

{% alert level="warning" %}
The `dashboard` module requires the `user-authn` module enabled or the [`externalAuthentication`](examples.html) settings set.
{% endalert %}

Kubernetes Dashboard provides the following features:

- management of pods and other high-level resources;
- exec to containers via a web console for debugging purposes;
- viewing logs of individual containers.
