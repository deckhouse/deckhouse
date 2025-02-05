---
title: "mTLS"
permalink: en/admin/network/mtls.html
---

In Deckhouse Kubernetes Platform, mTLS is implemented by Istio tools (`istio` module).

<!-- Transferred with minor modifications from https://deckhouse.io/products/kubernetes-platform/documentation/latest/modules/istio/#mutual-tls -->

Mutual TLS is the main method of mutual service authentication.
It is based on the fact that all outgoing requests are verified using the server certificate, and all incoming requests are verified using the client certificate.
After the verification is complete, the sidecar-proxy can identify the remote node and use these data for authorization or auxiliary purposes.

Each service gets its own identifier of the following format: `<TrustDomain>/ns/<Namespace>/sa/<ServiceAccount>` where `TrustDomain` is the cluster domain in our case.
You can assign your own ServiceAccount to each service or use the regular “default” one.
The service ID can be used for authorization and other purposes.
This is the identifier used as a name to validate against in TLS certificates.

You can redefine this settings at the Namespace level.
