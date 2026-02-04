---
title: Введение в документацию
permalink: ru/
description: Документация Deckhouse Kubernetes Platform.
lang: ru
---

{% capture anchors %}
%D0%BE%D1%81%D0%BD%D0%BE%D0%B2%D1%8B-%D0%BA%D0%BE%D0%BD%D1%84%D0%B8%D0%B3%D1%83%D1%80%D0%B0%D1%86%D0%B8%D0%B8-deckhouse,%D0%B8%D0%B7%D0%BC%D0%B5%D0%BD%D0%B5%D0%BD%D0%B8%D0%B5-%D0%BA%D0%BE%D0%BD%D1%84%D0%B8%D0%B3%D1%83%D1%80%D0%B0%D1%86%D0%B8%D0%B8-%D0%BA%D0%BB%D0%B0%D1%81%D1%82%D0%B5%D1%80%D0%B0,%D0%BD%D0%B0%D1%81%D1%82%D1%80%D0%BE%D0%B9%D0%BA%D0%B0-%D0%BC%D0%BE%D0%B4%D1%83%D0%BB%D1%8F, %D0%B2%D0%BA%D0%BB%D1%8E%D1%87%D0%B5%D0%BD%D0%B8%D0%B5-%D0%B8-%D0%BE%D1%82%D0%BA%D0%BB%D1%8E%D1%87%D0%B5%D0%BD%D0%B8%D0%B5-%D0%BC%D0%BE%D0%B4%D1%83%D0%BB%D1%8F,%D0%BE%D1%81%D0%BE%D0%B1%D0%B5%D0%BD%D0%BD%D0%BE%D1%81%D1%82%D0%B8-%D1%80%D0%B0%D0%B1%D0%BE%D1%82%D1%8B-%D1%81-%D0%BD%D0%B0%D0%B1%D0%BE%D1%80%D0%BE%D0%BC-%D0%BC%D0%BE%D0%B4%D1%83%D0%BB%D0%B5%D0%B9-minimal,%D1%83%D0%BF%D1%80%D0%B0%D0%B2%D0%BB%D0%B5%D0%BD%D0%B8%D0%B5-%D1%80%D0%B0%D0%B7%D0%BC%D0%B5%D1%89%D0%B5%D0%BD%D0%B8%D0%B5%D0%BC-%D0%BA%D0%BE%D0%BC%D0%BF%D0%BE%D0%BD%D0%B5%D0%BD%D1%82%D0%BE%D0%B2-deckhouse,%D0%BE%D1%81%D0%BE%D0%B1%D0%B5%D0%BD%D0%BD%D0%BE%D1%81%D1%82%D0%B8-%D0%B0%D0%B2%D1%82%D0%BE%D0%BC%D0%B0%D1%82%D0%B8%D0%BA%D0%B8-%D0%B7%D0%B0%D0%B2%D0%B8%D1%81%D1%8F%D1%89%D0%B8%D0%B5-%D0%BE%D1%82-%D1%82%D0%B8%D0%BF%D0%B0-%D0%BC%D0%BE%D0%B4%D1%83%D0%BB%D1%8F,%D0%B2%D1%8B%D0%B4%D0%B5%D0%BB%D0%B5%D0%BD%D0%B8%D0%B5-%D1%83%D0%B7%D0%BB%D0%BE%D0%B2-%D0%BF%D0%BE%D0%B4-%D0%BE%D0%BF%D1%80%D0%B5%D0%B4%D0%B5%D0%BB%D0%B5%D0%BD%D0%BD%D1%8B%D0%B9-%D0%B2%D0%B8%D0%B4-%D0%BD%D0%B0%D0%B3%D1%80%D1%83%D0%B7%D0%BA%D0%B8,%D0%BD%D0%B0%D0%B1%D0%BE%D1%80%D1%8B-%D0%BC%D0%BE%D0%B4%D1%83%D0%BB%D0%B5%D0%B9
{% endcapture %}
{% assign anchors = anchors | strip %}

{% include redirect-anchor.liquid anchors=anchors to="admin/configuration/" %}

Приветствуем вас на главной странице документации Deckhouse Kubernetes Platform — платформы для управления Kubernetes-кластерами.
{% if site.mode != 'module' %}Если вы еще не использовали платформу, рекомендуем начать с раздела [Быстрый старт](/products/kubernetes-platform/gs/), где вы найдете пошаговые инструкции по развёртыванию платформы на любой инфраструктуре.{% endif %}

Как быстро найти то, что нужно:

- Если знаете, что вам необходимо — используйте поиск.
- Если нужен конкретный модуль — найдите его в [списке](reference/revision-comparison.html).
- Для поиска по области применения воспользуйтесь меню.

{% if site.mode != 'module' %}Документация по Deckhouse Kubernetes Platform разных версий может отличаться. Выберите нужную версию в выпадающем списке вверху страницы. В списке доступны актуальные версии документации.{% endif %}

Если возникнут вопросы, вы можете обратиться за помощью в наш [Telegram-канал]({{ site.social_links[page.lang]['telegram'] }}). Мы обязательно поможем и проконсультируем.

Если вы используете коммерческую редакцию, можете написать нам [на почту](mailto:support@deckhouse.ru), мы также окажем вам поддержку.

Хотите улучшить Deckhouse Kubernetes Platform? Можете завести [задачу](https://github.com/deckhouse/deckhouse/issues/), предложить свою [идею](https://github.com/deckhouse/deckhouse/discussions) или [решение](https://github.com/deckhouse/deckhouse/blob/main/CONTRIBUTING.md) на GitHub.

А если вам хочется большего, присоединяйтесь к нашей [команде](https://job.flant.ru/)! Мы рады новым специалистам.
