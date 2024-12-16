---
title: "The dashboard module"
webIfaces:
- name: dashboard
---

This module installs Kubernetes Dashboard [Web UI](https://github.com/kubernetes/dashboard) that allows you to manage applications running in the cluster. It is integrated with [user-authn](../../modules/150-user-authn/) and [user-authz](../../modules/140-user-authz/) modules (access to the cluster is based on the user ID and credentials).

Kubernetes Dashboard provides the following features:

- management of pods and other high-level resources;
- exec to containers via a web console for debugging purposes;
- viewing logs of individual containers.

{% alert level="warning" %}
The module does not work over HTTP.
{% endalert %}

For the module to work, it is necessary to:
- enable the [user-authz](../user-authz/) module;
- either enable the [user-authn](../user-authn/) module or enable external authentication (see the [externalAuthentication](configuration.html#parameters-auth-externalauthentication) module parameters section).
