---
title: "Deckhouse Platform CE в существующем кластере: Добавление узла"
title_main: "Deckhouse Platform CE в существующем кластере"
title_sub: "Добавление узла"
permalink: ru/gs/existing/step5-ce.html
layout: page-nosidebar-notitle
revision: ce
platform_type: existing
platform_code: existing
platform_name: "существующем кластере"
lang: ru
toc: false
---

<link rel="stylesheet" type="text/css" href='{{ assets["getting-started.css"].digest_path }}' />

{::options parse_block_html="false" /}

<h1 class="docs__title">{{ page.title_main }}</h1>
{% include getting_started/steps.html steps=8 step=5 title=page.title_sub %}

{% include getting_started/STEP_NODE_ADD_RU.md %}

{% include getting_started/buttons.html step=5 nextStepCaption='workingWithModules' %}
