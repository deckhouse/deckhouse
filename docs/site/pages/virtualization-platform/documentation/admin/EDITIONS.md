---
title: "Platform editions"
permalink: en/virtualization-platform/documentation/admin/editions.html
---

The Deckhouse Virtualization Platform is available in Community Edition (CE) and Enterprise Edition (EE). DVP editions differ in their set of features and the level of available support.

The table below provides brief a comparison of editions listing its main features and functions:

{% capture coming_soon %}<img src="/images/icons/note.svg" title="{{ site.data.i18n.common.coming_soon[page.lang] }}" aria-expanded="false">{% endcapture %}
{% assign not_supported = '<img src="/images/icons/not_supported.svg">' %}
{% assign supported = '<img src="/images/icons/supported.svg">' %}

| Feature                                                        | CE                  | EE |
|------------------------------------------------------------------|---------------------|----|
| Deploying to an air-gapped environment                           | {{ not_supported }} | {{ supported }} |
| Network policies (micro-segmentation)                            | {{ supported }}     | {{ supported }} |
| Extended monitoring                                              | {{ supported }}     | {{ supported }} |
| Traffic load balancing management                                | {{ supported }}     | {{ supported }} |
| Support for NFS                                                  | {{ supported }}     | {{ supported }} |
| Built-in SDS                                                     | {{ supported }}     | {{ supported }} |
| Support for hardware storage systems                             | {{ not_supported }} | {{ supported }} |
| Public LUN                                                       | {{ coming_soon }}   | {{ coming_soon }} |
| Administrator interface                                          | {{ not_supported }} | {{ supported }} |
| Changing VM parameters without stopping it                       | {{ not_supported }} | {{ supported }} |
| High Availability (HA) mode of virtual machines                  | {{ coming_soon }}   | {{ coming_soon }} |
| Disaster resilience (inter-cluster replication)                  | {{ coming_soon }}   | {{ coming_soon }} |
| [Extended technical support](/tech-support/) | {{ not_supported }} | {{ supported }}  |
| [Extended technical support](/tech-support/) | {{ not_supported }} | {{ supported }}  |
