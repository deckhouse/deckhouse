---
title: "The cert-manager module"
---

This module installs the reliable and highly available cert-manager v1.12.9 [release](https://github.com/cert-manager/cert-manager).

The installation process automatically takes into account cluster aspects:
- the component (webhook) that the `kube-apiserver` is accessing is installed on master nodes;
- if the webhook is unavailable, the `apiservice` is temporary deleted so that the unavailability of *cert-manager* does not block regular cluster operation.

The module itself is updated automatically (including the migration of cert-manager resources).

## Features of the cert-manager module (with the changes made)

The module has all the features of the original cert-manager, including:
- Provisioning certificates of all the supported CA such as *Let’s Encrypt*, *HashiCorp Vault*, *Venafi*;
- Issuing self-signed certificates;
- Keeping certificates up-to-date, reissuing them automatically, etc.

Changes to the original [cert-manager](https://github.com/cert-manager/cert-manager) were made so that the `cm-acme-http-solver` Pods could run on master and dedicated nodes.

## Monitoring

The module can expose metrics in the Prometheus format, allowing you to monitor:
- certificate validity;
- correctness of the certificate reissue.

## Module roles

The module has several well-thought-out roles for managing resources:
- `User` – has read-only access to Certificate & Issuers resources in the permitted namespaces and to the global clusterIssues;
- `Editor` – manages Certificate and Issuer resources in the permitted namespaces;
- `ClusterEditor` – manages Certificate & Issuer resources in all namespaces;
- `SuperAdmin` – manages internal service objects.
