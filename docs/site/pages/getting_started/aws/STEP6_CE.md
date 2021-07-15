---
title: "Deckhouse Platform CE in Amazon AWS: Adding a node"
title_main: "Deckhouse Platform CE in Amazon AWS"
title_sub: "Adding a node"
permalink: en/gs/aws/step6-ce.html
layout: page-nosidebar-notitle
revision: ce
platform_type: cloud
platform_code: aws
platform_name: Amazon AWS
lang: en
toc: false
---

<link rel="stylesheet" type="text/css" href='{{ assets["getting-started.css"].digest_path }}' />

{::options parse_block_html="false" /}

<h1 class="docs__title">{{ page.title_main }}</h1>
{% include getting_started/steps.html steps=9 step=6 title=page.title_sub %}

{% include getting_started/STEP_NODE_ADD.md %}

{% include getting_started/buttons.html step=6 nextStepCaption='workingWithModules' %}
