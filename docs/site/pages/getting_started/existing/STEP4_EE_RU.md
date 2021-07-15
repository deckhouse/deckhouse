---
title: "Deckhouse Platform EE в существующем кластере: Установка"
title_main: "Deckhouse Platform EE в существующем кластере"
title_sub: "Установка"
permalink: ru/gs/existing/step4-ee.html
layout: page-nosidebar-notitle
revision: ee
platform_type: existing
platform_code: existing
platform_name: "существующем кластере"
lang: ru
toc: false
---

<link rel="stylesheet" type="text/css" href='{{ assets["getting-started.css"].digest_path }}' />
<script type="text/javascript" src='{{ assets["getting-started.js"].digest_path }}'></script>

{::options parse_block_html="false" /}

<h1 class="docs__title">{{ page.title_main }}</h1>
{% include getting_started/steps.html steps=8 step=4 title=page.title_sub %}

{% include getting_started/STEP_INSTALL_RU.md %}

{% include getting_started/buttons.html step=4 nextStepCaption='addingNode' %}
