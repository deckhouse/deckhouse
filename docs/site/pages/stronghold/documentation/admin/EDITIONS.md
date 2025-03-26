---
title: "Platform editions"
permalink: en/stronghold/documentation/admin/editions.html
---

The Deckhouse Stronghold is available in Standart Edition (SE), Standart Edition Plus (SE+), Enterprise Edition (EE). Deckhouse Stronghold editions differ in their set of features and the level of available support.

The table below provides brief a comparison of editions listing its main features and functions:

{% capture coming_soon %}<img src="/images/icons/note.svg" title="{{ site.data.i18n.common.coming_soon[page.lang] }}" aria-expanded="false">{% endcapture %}
{% assign not_supported = '<img src="/images/icons/not_supported.svg">' %}
{% assign supported = '<img src="/images/icons/supported.svg">' %}

| Feature                                                          | SE                  | SE+               | EE |
|------------------------------------------------------------------|---------------------|-------------------|----|
| Deploying to an air-gapped environment                           | {{ supported }}     | {{ supported }} | {{ supported }} |
| Network policies (micro-segmentation)                            | {{ supported }}     | {{ supported }} | {{ supported }} |
| Extended monitoring                                              | {{ supported }}     | {{ supported }} | {{ supported }} |
| Traffic load balancing management                                | {{ supported }}     | {{ supported }} | {{ supported }} |
| Administrator interface                                          | {{ supported }}     | {{ supported }} | {{ supported }} |
| High Availability (HA) mode                                      | {{ supported }}     | {{ supported }} | {{ supported }} |
| Enterprise Security                                              | {{ not_supported }} | {{ supported }} | {{ supported }} |
| Online security scanner                                          | {{ not_supported }} | {{ supported }} | {{ supported }} |
| Runtime audit engine                                             | {{ not_supported }} | {{ supported }} | {{ supported }} |
| [Extended technical support](https://deckhouse.io/tech-support/) | {{ supported }} | {{ supported }}  | {{ supported }} |
| [Extended technical support](https://deckhouse.io/tech-support/) | {{ supported }} | {{ supported }}  | {{ supported }} |
