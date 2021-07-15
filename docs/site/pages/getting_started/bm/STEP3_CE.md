---
title: "Deckhouse Platform CE on bare metal: Preparing configuration"
title_main: "Deckhouse Platform CE on bare metal"
title_sub: "Preparing configuration"
permalink: en/gs/bm/step3-ce.html
layout: page-nosidebar-notitle
revision: ce
platform_type: baremetal
platform_code: bm
platform_name: "bare metal"
toc: false
---

<link rel="stylesheet" type="text/css" href='{{ assets["getting-started.css"].digest_path }}' />
<script type="text/javascript" src='{{ assets["getting-started.js"].digest_path }}'></script>

{::options parse_block_html="false" /}

<h1 class="docs__title">{{ page.title_main }}</h1>
{% include getting_started/steps.html steps=8 step=3 title=page.title_sub %}

{% include getting_started/bm/STEP_PREP_CONF.md %}

{% include getting_started/buttons.html step=3 nextStepCaption='installing' %}
