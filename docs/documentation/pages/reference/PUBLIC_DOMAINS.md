---
title: List of reserved Deckhouse Platform DNS names
permalink: en/reference/public-domains.html
description: List of reserved DNS names that Deckhouse Platform uses. Hosts file data generator
editionsNotAvailable: [cse]
search: public domains DP
---

This page lists the reserved DNS names that Deckhouse Platform (DP) uses. The names are given in the [table below](#list-of-reserved-dns-names).

Use the list if the cluster [DNS name template](../glossary.html#public-domain-template) (the [`modules.publicDomainTemplate`](api/global.html#parameters-modules-publicdomaintemplate) parameter of the `global` ModuleConfig) is not a wildcard template (for example, `%s-kube.company.my`). In this case, add an A or CNAME record for each name. The record value is the public IP address of the [Ingress controller](../glossary.html#ingress-controller) (the HTTP/HTTPS traffic load balancer).

To get the command for adding the DNS names to the local hosts file, use the hosts file data generator.

## Hosts file data generator

To access the cluster services without DNS records (for example, from a workstation), add entries for the required names from the [table below](#list-of-reserved-dns-names) to the hosts file. Enter your DNS name template (from the [`modules.publicDomainTemplate`](api/global.html#parameters-modules-publicdomaintemplate) parameter of the `global` ModuleConfig) and the IP address of the Ingress controller in the generator. The generator returns a ready-to-run command:

{% include public_domains_form.liquid %}

## List of reserved DNS names

The following table lists the DNS names reserved by DP and the services that use them.

When you add A or CNAME records, use the name from the table in place of `%s` in `publicDomainTemplate`. For example, for the `%s.example.com` template, the `grafana` service gets the name `grafana.example.com`, and for `%s-kube.company.my` — `grafana-kube.company.my`.

{% include domains_list.liquid %}
