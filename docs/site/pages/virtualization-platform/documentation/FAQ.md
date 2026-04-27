---
title: Frequently Asked Questions
custom_class: "faq__container"
permalink: en/virtualization-platform/documentation/faq.html
description: Frequently Asked Questions about configuring and using the Deckhouse Virtualization Platform.
search: faq
searchable: false
faqIndexPage: true
lang: en
---

The page contains frequently asked questions about configuring and using the Deckhouse Kubernetes Platform.

<button class="show__containers--expand">Expand all</button>
<button class="show__containers--collapse">Collapse all</button>

{% include faq-list.liquid %}

{%- assign assetHash = site.time | date: "%Y-%m-%d %H:%M:%S" | sha256 -%}
<link href='/assets/css/faq.css?v={{ assetHash }}' rel='stylesheet' type='text/css' crossorigin="anonymous" />
<script type="text/javascript" src="/assets/js/faq.js?v={{ assetHash }}"></script>
