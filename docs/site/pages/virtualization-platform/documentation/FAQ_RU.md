---
title: Частые вопросы
custom_class: "faq__container"
permalink: ru/virtualization-platform/documentation/faq.html
description: Часто задаваемые вопросы по настройке и использованию Deckhouse Virtualization Platform.
search: frequently asked questions, часто задаваемые вопросы, ЧАВО, faq, фак, вопросы и ответы
searchable: false
faqIndexPage: true
lang: ru
---

На странице представлены часто задаваемые вопросы по настройке и использованию Deckhouse Kubernetes Platform.

<button class="show__containers--expand">Развернуть все</button>
<button class="show__containers--collapse">Свернуть все</button>

{% include faq-list.liquid %}

{%- assign assetHash = site.time | date: "%Y-%m-%d %H:%M:%S" | sha256 -%}
<link href='/assets/css/faq.css?v={{ assetHash }}' rel='stylesheet' type='text/css' crossorigin="anonymous" />
<script type="text/javascript" src="/assets/js/faq.js?v={{ assetHash }}"></script>
