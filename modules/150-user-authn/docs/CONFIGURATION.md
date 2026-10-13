---
title: "The user-authn module: configuration"
---

<!-- SCHEMA -->

{% alert level="warning" %}
The module works only with HTTPS, because OpenID Connect over HTTP is a serious security threat and the Kubernetes API server does not support OIDC over HTTP. With `https.mode` set to `Disabled`, the module is turned off.
{% endalert %}

Set `https.mode` to a value other than `Disabled` at the cluster level ([`modules.https.mode`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-https-mode)) or in the module. If you set the `https` section of the module, set `mode` in it too. Without `mode`, `Disabled` applies.
