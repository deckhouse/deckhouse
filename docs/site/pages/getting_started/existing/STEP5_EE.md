---
title: "Deckhouse Platform EE in the existing cluster: Adding a node"
title_main: "Deckhouse Platform EE in the existing cluster"
title_sub: "Adding a node"
permalink: en/gs/existing/step5-ee.html
layout: page-nosidebar-notitle
revision: ee
platform_type: existing
platform_code: existing
platform_name: "the existing cluster"
toc: false
---

<link rel="stylesheet" type="text/css" href='{{ assets["getting-started.css"].digest_path }}' />

{::options parse_block_html="false" /}

<h1 class="docs__title">{{ page.title_main }}</h1>
{% include getting_started/steps.html steps=8 step=5 title=page.title_sub %}

{% include getting_started/STEP_NODE_ADD.md %}

{% include getting_started/buttons.html step=5 nextStepCaption='workingWithModules' %}
