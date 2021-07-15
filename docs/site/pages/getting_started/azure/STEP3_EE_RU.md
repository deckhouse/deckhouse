---
title: "Deckhouse Platform EE в Microsoft Azure: Подготовка окружения"
title_main: "Deckhouse Platform EE в Microsoft Azure"
title_sub: "Подготовка окружения"
permalink: ru/gs/azure/step3-ee.html
layout: page-nosidebar-notitle
revision: ee
platform_code: azure
platform_name: Microsoft Azure
lang: ru
toc: false
---

<link rel="stylesheet" type="text/css" href='{{ assets["getting-started.css"].digest_path }}' />
{::options parse_block_html="false" /}

<h1 class="docs__title">{{ page.title_main }}</h1>
{% include getting_started/steps.html steps=9 step=3 title=page.title_sub %}

{% include getting_started/request_ee_access.md %}

{% include getting_started/azure/STEP_ENV_RU.md %}

{% include getting_started/buttons.html step=3 nextStepCaption='preparingConfig' %}
