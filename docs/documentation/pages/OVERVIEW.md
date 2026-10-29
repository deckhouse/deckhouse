---
title: Deckhouse Platform documentation
permalink: en/
description: Deckhouse Platform Documentation.
rootLLMStxtURL: /products/kubernetes-platform/documentation/v1/llms.txt
---

{% include redirect-anchor.liquid anchors="deckhouse-configuration,modifying-cluster-configuration,viewing-current-configuration,configuring-the-module,enabling-and-disabling-the-module,accessing-documentation-for-the-current-version,managing-placement-of-deckhouse-components,module-features-that-depend-on-its-type,advanced-scheduling,module-bundles" to="admin/configuration/" %}

{% unless site.mode == "module" %}
<div style="text-align: right">
<a href="#" data-open-pdf-modal class="button button--outline">{{ site.data.i18n.common['pdf_download_button'][page.lang] }}</a>
</div>

{% include pdf_download_modal.html %}
{%- endunless %}

Welcome to the Deckhouse Platform documentation homepage. Deckhouse Platform is an AI-driven platform
for building hybrid distributed infrastructure.

Here you can find information about installing, configuring, and using the platform,
as well as descriptions of its features, components, and modules.

{% if site.mode != 'module' %}If you are new to Deckhouse Platform,
we recommend starting with the ["Getting Started"](/products/kubernetes-platform/gs/) section,
which provides step-by-step instructions for deploying the platform on any infrastructure.{% endif %}

To quickly find the information you need:

- Use search if you are looking for a specific Deckhouse Platform feature, parameter, or resource.
- Go to the [list of modules](/modules/) if you are looking for information about a specific module.
- Use the sidebar to navigate through the documentation sections.

{% if site.mode != 'module' %}The content of the Deckhouse Platform documentation may vary depending on the version.
Select the required version from the drop-down list at the top of the page.
The list contains currently supported documentation versions.{% endif %}

If you need help:

- Ask a question in the ["Deckhouse &#124; EN-community"]({{ site.social_links[page.lang]['telegram'] }}) Telegram channel.
- If you use a commercial [edition](glossary.html#edition) of Deckhouse Platform, contact technical support at [`support@deckhouse.io`](mailto:support@deckhouse.io).
