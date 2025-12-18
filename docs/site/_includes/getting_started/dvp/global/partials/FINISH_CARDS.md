<section class="cards-blocks">
<div class="cards-blocks__content container">
<h2 class="cards-blocks__title text_h2">
Essentials
</h2>
<div class="cards-blocks__cards">

{% if page.platform_code != 'existing' and page.platform_code != 'kind' %}
<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
📚 <span class="cards-item__title-text">Documentation</span>
</h3>
<div class="cards-item__text">
<p>The documentation for the installed in your cluster version of Deckhouse.</p>
<p>Web service name: {% include getting_started/global/partials/dns-template-title.html.liquid name="documentation" %}</p>
</div>
</div>
{% endif %}

{% if page.platform_code != 'kind' %}
<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
📊 <span class="cards-item__title-text">Monitoring</span>
</h3>
<div class="cards-item__text">
<p>Explore Grafana dashboards bundled with Deckhouse.</p>
<p>Web service name: {% include getting_started/global/partials/dns-template-title.html.liquid name="grafana" %}</p>
<p>To access Prometheus: {% include getting_started/global/partials/dns-template-title.html.liquid name="grafana" path="/prometheus/" onlyPath="true" %}</p>
<a href="/modules/prometheus/" target="_blank">Learn more</a> about the <code>monitoring</code> module.
</div>
</div>
{% endif %}

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
👌 <span class="cards-item__title-text">Status page</span>
</h3>
<div class="cards-item__text">
<p>Get information about the overall status of Deckhouse and its components.<br />
Web service name: {% include getting_started/global/partials/dns-template-title.html.liquid name="status" %}</p>

<p>Get detailed SLA statistics for each component and time frame.<br />
Web service name: {% include getting_started/global/partials/dns-template-title.html.liquid name="upmeter" %}</p>
</div>
</div>

{% if page.platform_code != 'kind' %}
<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
🏭 <span class="cards-item__title-text">Going to production</span>
</h3>
<div class="cards-item__text" markdown="1">
Prepare your cluster to receive traffic.

Use our [checklist](/products/virtualization-platform/guides/production.html) to make sure you haven't forgotten anything.
</div>
</div>

<div style="width: 30%">&nbsp;</div>
{%- endif %}

{% if page.platform_code == 'kind' %}
<div style="width: 30%">&nbsp;</div>
{%- endif %}
</div>
</div>
</section>

<div markdown="1">
## What's next?

Detailed information about the system and components is available in the [documentation](/products/virtualization-platform/documentation/admin/overview.html).

Please, reach us via our [online community](/community/about.html#online-community) if you have any questions.
</div>
