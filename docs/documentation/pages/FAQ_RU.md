---
title: Частые вопросы
custom_class: "faq__container"
permalink: ru/faq.html
description: Часто задаваемые вопросы по настройке и использованию Deckhouse Kubernetes Platform.
search: frequently asked questions, часто задаваемые вопросы, ЧАВО, faq, фак, вопросы и ответы
searchable: false
faqIndexPage: true
lang: ru
---

На странице представлены часто задаваемые вопросы по настройке и использованию Deckhouse Kubernetes Platform.

{% alert level="info" %}
Дополнительные материалы и разбор типовых сценариев администрирования представлены в курсе [«Администрирование Deckhouse Kubernetes Platform»](https://deckhouse.ru/courses/basics-administration-deckhouse-kubernetes-platform/) в [Deckhouse Академии](https://deckhouse.ru/academy/).
{% endalert %}

<button class="show__containers--expand">Развернуть все</button>
<button class="show__containers--collapse">Свернуть все</button>

{% include faq-list.liquid %}

{%- assign assetHash = site.time | date: "%Y-%m-%d %H:%M:%S" | sha256 -%}
<link href='/assets/css/faq.css?v={{ assetHash }}' rel='stylesheet' type='text/css' crossorigin="anonymous" />
<script type="text/javascript" src="/assets/js/faq.js?v={{ assetHash }}"></script>
