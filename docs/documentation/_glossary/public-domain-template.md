---
title: DNS name template
description: The cluster DNS name template from the modules.publicDomainTemplate parameter of the global [ModuleConfig](reference/api/cr.html#moduleconfig). Deckhouse Platform modules substitute a reserved name for the `%s` placeholder to build the DNS names of their service web interfaces.
lang: en
---

If the template is not set, the modules do not create Ingress resources. The domain in the template must not match or be a subdomain of the cluster domain from `clusterDomain`. With a wildcard DNS record one record covers all names. With a non-wildcard template, for example `%s-kube.company.my`, an A or CNAME record is needed for each reserved name, pointing to the public IP address of the [Ingress controller](glossary.html#ingress-controller). The template is a part of the [global settings](glossary.html#global-settings).

For details, see [List of reserved Deckhouse Platform DNS names](reference/public-domains.html).
