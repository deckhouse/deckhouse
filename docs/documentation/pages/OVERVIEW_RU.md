---
title: Документация Deckhouse Platform
permalink: ru/
description: Документация Deckhouse Platform.
rootLLMStxtURL: /products/kubernetes-platform/documentation/v1/llms.txt
lang: ru
---

{% capture anchors %}
%D0%BE%D1%81%D0%BD%D0%BE%D0%B2%D1%8B-%D0%BA%D0%BE%D0%BD%D1%84%D0%B8%D0%B3%D1%83%D1%80%D0%B0%D1%86%D0%B8%D0%B8-deckhouse,%D0%B8%D0%B7%D0%BC%D0%B5%D0%BD%D0%B5%D0%BD%D0%B8%D0%B5-%D0%BA%D0%BE%D0%BD%D1%84%D0%B8%D0%B3%D1%83%D1%80%D0%B0%D1%86%D0%B8%D0%B8-%D0%BA%D0%BB%D0%B0%D1%81%D1%82%D0%B5%D1%80%D0%B0,%D0%BD%D0%B0%D1%81%D1%82%D1%80%D0%BE%D0%B9%D0%BA%D0%B0-%D0%BC%D0%BE%D0%B4%D1%83%D0%BB%D1%8F, %D0%B2%D0%BA%D0%BB%D1%8E%D1%87%D0%B5%D0%BD%D0%B8%D0%B5-%D0%B8-%D0%BE%D1%82%D0%BA%D0%BB%D1%8E%D1%87%D0%B5%D0%BD%D0%B8%D0%B5-%D0%BC%D0%BE%D0%B4%D1%83%D0%BB%D1%8F,%D0%BE%D1%81%D0%BE%D0%B1%D0%B5%D0%BD%D0%BD%D0%BE%D1%81%D1%82%D0%B8-%D1%80%D0%B0%D0%B1%D0%BE%D1%82%D1%8B-%D1%81-%D0%BD%D0%B0%D0%B1%D0%BE%D1%80%D0%BE%D0%BC-%D0%BC%D0%BE%D0%B4%D1%83%D0%BB%D0%B5%D0%B9-minimal,%D1%83%D0%BF%D1%80%D0%B0%D0%B2%D0%BB%D0%B5%D0%BD%D0%B8%D0%B5-%D1%80%D0%B0%D0%B7%D0%BC%D0%B5%D1%89%D0%B5%D0%BD%D0%B8%D0%B5%D0%BC-%D0%BA%D0%BE%D0%BC%D0%BF%D0%BE%D0%BD%D0%B5%D0%BD%D1%82%D0%BE%D0%B2-deckhouse,%D0%BE%D1%81%D0%BE%D0%B1%D0%B5%D0%BD%D0%BD%D0%BE%D1%81%D1%82%D0%B8-%D0%B0%D0%B2%D1%82%D0%BE%D0%BC%D0%B0%D1%82%D0%B8%D0%BA%D0%B8-%D0%B7%D0%B0%D0%B2%D0%B8%D1%81%D1%8F%D1%89%D0%B8%D0%B5-%D0%BE%D1%82-%D1%82%D0%B8%D0%BF%D0%B0-%D0%BC%D0%BE%D0%B4%D1%83%D0%BB%D1%8F,%D0%B2%D1%8B%D0%B4%D0%B5%D0%BB%D0%B5%D0%BD%D0%B8%D0%B5-%D1%83%D0%B7%D0%BB%D0%BE%D0%B2-%D0%BF%D0%BE%D0%B4-%D0%BE%D0%BF%D1%80%D0%B5%D0%B4%D0%B5%D0%BB%D0%B5%D0%BD%D0%BD%D1%8B%D0%B9-%D0%B2%D0%B8%D0%B4-%D0%BD%D0%B0%D0%B3%D1%80%D1%83%D0%B7%D0%BA%D0%B8,%D0%BD%D0%B0%D0%B1%D0%BE%D1%80%D1%8B-%D0%BC%D0%BE%D0%B4%D1%83%D0%BB%D0%B5%D0%B9
{% endcapture %}
{% assign anchors = anchors | strip %}

{% include redirect-anchor.liquid anchors=anchors to="admin/configuration/" %}

{% unless site.mode == "module" %}
<div style="text-align: right">
<a href="#" data-open-pdf-modal class="button button--outline">{{ site.data.i18n.common['pdf_download_button'][page.lang] }}</a>
</div>

{% include pdf_download_modal.html %}
{%- endunless %}

Добро пожаловать на главную страницу документации Deckhouse Platform — платформы для построения
гибридной распределённой инфраструктуры, усиленной искусственным интеллектом.

Здесь вы найдёте информацию об установке, настройке и использовании платформы,
а также описание её возможностей, компонентов и модулей.

{% if site.mode != 'module' %}Если вы только начинаете знакомство с Deckhouse Platform,
рекомендуем начать [с раздела «Быстрый старт»](/products/kubernetes-platform/gs/) с пошаговыми инструкциями
по развёртыванию платформы на любой инфраструктуре.{% endif %}

Чтобы быстро найти нужную информацию:

- воспользуйтесь поиском, если вас интересует конкретная возможность Deckhouse Platform, параметр или ресурс;
- перейдите [к списку модулей](/modules/), если ищете информацию о конкретном модуле;
- используйте боковое меню для навигации по разделам документации.

{% if site.mode != 'module' %}Содержание документации Deckhouse Platform может отличаться в зависимости от версии.
Выберите нужную версию в выпадающем списке в верхней части страницы. В списке доступны актуальные версии документации.{% endif %}

Если у вас возникли вопросы или вы хотите поделиться опытом работы с Deckhouse Platform,
[присоединяйтесь к Deckhouse User Community](https://deckhouse.ru/community/).
В сообществе можно обсудить технические вопросы с другими пользователями и разработчиками,
получить рекомендации и предложить улучшения платформы.

Если вам нужна помощь:

- задайте вопрос [в Telegram-канале «Deckhouse &#124; RU-сообщество»]({{ site.social_links[page.lang]['telegram'] }});
- если вы используете коммерческую [редакцию](glossary.html#edition) Deckhouse Platform, обратитесь в поддержку по адресу [`support@deckhouse.ru`](mailto:support@deckhouse.ru).
