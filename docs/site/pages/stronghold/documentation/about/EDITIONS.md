---
title: "Editions"
permalink: en/stronghold/documentation/about/editions.html
---

Deckhouse Stronghold is available in Community Edition (CE) and Enterprise Edition (EE).

Deckhouse Stronghold Community Edition is available for use in any Deckhouse Kubernetes Platform editions.

Deckhouse Stronghold Enterprise Edition is licensed separately and available for use in any **commercial edition** of the Deckhouse Kubernetes Platform.

The table below provides brief a comparison of editions listing its main features and functions:

{% capture coming_soon %}<img src="/images/icons/note.svg" title="{{ site.data.i18n.common.coming_soon[page.lang] }}" aria-expanded="false">{% endcapture %}
{% capture techsupport_notice_ce %}<img src="/images/icons/intermediate_v2.svg" title="{{ site.data.i18n.common.tech_support_stronghold_notice_ce[page.lang] }}" aria-expanded="false">{% endcapture %}
{% capture techsupport_notice_commercial %}<img src="/images/icons/intermediate_v2.svg" title="{{ site.data.i18n.common.tech_support_stronghold_notice_commercial[page.lang] }}" aria-expanded="false">{% endcapture %}

{% assign not_supported = '<img src="/images/icons/not_supported_v2.svg">' %}
{% assign supported = '<img src="/images/icons/supported_v2.svg">' %}

| Feature                                                                                               | CE                                 | EE                                          |
|-------------------------------------------------------------------------------------------------------|------------------------------------|---------------------------------------------|
| Secure management of the lifecycle of secrets (storage, creation, delivery, revocation, and rotation) | {{ supported }}                    | {{ supported }}                             |
| The ability to use IaC automation tools Ansible and Terraform                                         | {{ supported }}                    | {{ supported }}                             |
| Support for authentication methods                                                                    | JWT, OIDC, Kubernetes, LDAP, Token | JWT, OIDC, Kubernetes, LDAP, Token |
| Support for Secret Engines KV, Kubernetes, Database, SSH, PKI                                         | {{ supported }}                    | {{ supported }}                             |
| Deploying to an air-gapped environment                                                                | {{ supported }}                    | {{ supported }}                             |
| Administrator web interface                                                                           | {{ supported }}                    | {{ supported }}                             |
| Role and access policy management through a web interface                                             | {{ not_supported }}                | {{ supported }}                             |
| Support for namespaces                                                                                | {{ not_supported }}                | {{ supported }}                             |
| Built-in automatic unsealing of the vault                                                             | {{ not_supported }}                | {{ supported }}                             |
| Data replication                                                                                      | {{ not_supported }}                | KV1/KV2                                     |
| Automatic backup creation on a schedule                                                               | {{ not_supported }}                | {{ supported }}                             |
| Audit logging support                                                                                 | {{ not_supported }}                | {{ supported }}                             |
| Delivery as standalone executable file                                                                | {{ not_supported }}                | {{ supported }}                             |
| Launching in Deckhouse Kubernetes Platform Community Edition                                          | {{ supported }}                    | {{ not_supported }}                         |
| [Technical support «Standard»](https://deckhouse.io/tech-support/)                                    | {{ techsupport_notice_ce }}        | {{ techsupport_notice_commercial }}         |
| [Technical support «Standard +»](https://deckhouse.io/tech-support/)                                  | {{ techsupport_notice_ce }}        | {{ techsupport_notice_commercial }}         |
