---
title: "The cert-manager module: configuration"
---

The module does not have any mandatory parameters.

* `nodeSelector` — the same as the pods' `spec.nodeSelector` parameter in Kubernetes;
  * If the parameter is omitted of `false`, it will be determined [automatically](../../#advanced-scheduling).
* `tolerations` — the same as the pods' `spec.tolerations` parameter in Kubernetes.
  * If the parameter is omitted of `false`, it will be determined [automatically](../../#advanced-scheduling).
*  `cloudflareGlobalAPIKey` — the Cloudflare Global API key for managing DNS records (it allows you to verify that domains specified in the Certificate resource are managed by `cert-manager` and kept by the Cloudflare DNS provider. Verification is performed by adding special TXT records for the [ACME DNS01 Challenge Provider](https://cert-manager.io/docs/configuration/acme/dns01/) domain.
*  `cloudflareEmail` — the email used for accessing the Cloudflare platform.
*  `route53AccessKeyID` — the Access Key ID of the user with the attached [Amazon Route53 IAM Policy](https://cert-manager.io/docs/configuration/acme/dns01/route53/) for managing domain records.
*  `route53SecretAccessKey` — the Secret Access Key of the user with privileges to manage domain records.
*  `digitalOceanCredentials` — the Access Token for the Digital Ocean API (you can create it in the  `API` section).
*  `cloudDNSServiceAccount` — the Service Account for [Google Cloud](usage.html#issuing-a-dns-wildcard-certificate-using-google) for the same project that has the DNS Administrator role.
