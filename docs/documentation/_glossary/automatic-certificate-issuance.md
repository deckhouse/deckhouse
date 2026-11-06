---
title: Automatic certificate issuance
description: Obtaining TLS certificates automatically with the [`cert-manager`](/modules/cert-manager/) module, either through a Certificate object that references a ClusterIssuer or through the tls-acme annotation on an Ingress.
lang: en
---

With a Certificate object, [`cert-manager`](/modules/cert-manager/) runs the domain ownership challenge by the method defined in the ClusterIssuer, for example `HTTP-01` or `DNS-01`, creates a temporary Ingress for the challenge if needed and stores the certificate in the Secret. The `tls-acme` annotation on an Ingress requests a certificate without a separate Certificate object. Deckhouse Platform supports certificates from Let's Encrypt, HashiCorp Vault and Venafi, self-signed certificates, automatic renewal and expiration monitoring.

For details, see [Configuration of using TLS certificates](user/security/tls.html).
