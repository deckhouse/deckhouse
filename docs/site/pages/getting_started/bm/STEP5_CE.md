---
title: "Deckhouse Platform CE on bare metal: Adding a node"
title_main: "Deckhouse Platform CE on bare metal"
title_sub: "Adding a node"
permalink: en/gs/bm/step5-ce.html
layout: page-nosidebar-notitle
revision: ce
platform_type: baremetal
platform_code: bm
platform_name: "bare metal"
toc: false
---

<link rel="stylesheet" type="text/css" href='{{ assets["getting-started.css"].digest_path }}' />

{::options parse_block_html="false" /}

<h1 class="docs__title">{{ page.title_main }}</h1>
{% include getting_started/steps.html steps=8 step=5 title=page.title_sub %}

{% include getting_started/STEP_NODE_ADD.md %}

{% include getting_started/buttons.html step=5 nextStepCaption='workingWithModules' %}
