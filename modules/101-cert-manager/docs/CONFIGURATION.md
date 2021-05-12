---
title: "The cert-manager module: configuration"
---

The module does not have any mandatory parameters.

* `nodeSelector` — the same as the pods' `spec.nodeSelector` parameter in Kubernetes;
    * If the parameter isn't set, then the `{"node-role.deckhouse.io/cert-manager":""}` or `{"node-role.deckhouse.io/system":""}` value is used given that such nodes exist in the cluster. Otherwise, the parameter stays empty. 
    * You can set it to `false` to avoid adding any nodeSelector.
* `tolerations` — the same as the pods' `spec.tolerations` parameter in Kubernetes.
    * If the parameter isn't set, then the `[{"key":"dedicated.deckhouse.io","operator":"Equal","value":"cert-manager"},{"key":"dedicated.deckhouse.io","operator":"Equal","value":"system"}]` value is used.
    * You can set it to `false` to avoid adding any tolerations.
*  `cloudflareGlobalAPIKey` — the Cloudflare Global API key for managing DNS records (it allows you to verify that domains specified in the Certificate resource are managed by `cert-manager` and kept by the Cloudflare DNS provider. Verification is performed by adding special TXT records for the [ACME DNS01 Challenge Provider](https://cert-manager.io/docs/configuration/acme/dns01/) domain.
*  `cloudflareEmail` — the email used for accessing the Cloudflare platform.
*  `route53AccessKeyID` — the Access Key ID of the user with the attached [Amazon Route53 IAM Policy](https://cert-manager.io/docs/configuration/acme/dns01/route53/) for managing domain records.
*  `route53SecretAccessKey` — the Secret Access Key of the user with privileges to manage domain records.
*  `digitalOceanCredentials` — the Access Token for the Digital Ocean API (you can create it in the  `API` section).
*  `cloudDNSServiceAccount` — the Service Account for [Google Cloud](usage.html#issuing-a-dns-wildcard-certificate-using-google) for the same project that has the DNS Administrator role.
