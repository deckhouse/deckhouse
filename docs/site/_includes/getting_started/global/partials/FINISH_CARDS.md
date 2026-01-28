<section class="cards-blocks">
<div class="cards-blocks__content container">
<h2 class="cards-blocks__title text_h2">
Essentials
</h2>
<div class="cards-blocks__cards">

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" fill="none" viewBox="0 0 52 52" style="display: inline-block; vertical-align: middle; margin-right: 8px;"><g id="sign"><path id="Vector" fill="#0064FF" d="M27.43 33.1h-7.06V18.9h7.06a7.1 7.1 0 0 1 0 14.2Z"/><g id="Group"><g id="Group_2" fill="#00003C"><path id="Vector_2" d="m14.32 19.73-3.79-3.8L.5 26l10.04 10.07 3.78-3.9-1.94-1.85v-8.64l1.94-1.95Z"/><path id="Vector_3" d="m42.28 15.93-3.8 3.8 1.85 1.95v8.64l-1.84 1.85 3.79 3.9L52.3 26 42.28 15.93Z"/><path id="Vector_4" d="m16.37 10.07 3.79 3.8 1.95-1.95h8.6l1.84 1.95 3.9-3.8L26.4 0 16.37 10.07Z"/><path id="Vector_5" d="m34.5 39.98-1.95-1.85-1.84 1.85h-8.6l-1.95-1.85-1.84 1.85-1.95 1.95L26.41 52l10.03-10.07-1.94-1.95Z"/></g></g></g></svg>
<span class="cards-item__title-text">Deckhouse web UI</span>
</h3>
<div class="cards-item__text">
<p>Try the <a href="/products/kubernetes-platform/documentation/v1/user/web/ui.html" target="_blank">web UI</a> for managing the cluster and its main components.</p>
<p>Web service name: {% include getting_started/global/partials/dns-template-title.html.liquid name="console" %}</p>
</div>
</div>

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

Use our [checklist](/products/kubernetes-platform/guides/production.html) to make sure you haven't forgotten anything.
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

<section class="cards-blocks">
<div class="cards-blocks__content container">
<h2 class="cards-blocks__title text_h2">
Deploying your first application
</h2>
<div class="cards-blocks__cards">

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
⟳ <span class="cards-item__title-text">Setting up a CI/CD system</span>
</h3>
<div class="cards-item__text" markdown="1">
[Create](/modules/user-authz/usage.html#creating-a-serviceaccount-for-a-machine-and-granting-it-access)
a ServiceAccount to use for deploying to the cluster and grant it all the necessary privileges.

You can use the generated `kubeconfig` file in Kubernetes with any deployment system.
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
🔀 <span class="cards-item__title-text">Routing traffic</span>
</h3>
<div class="cards-item__text" markdown="1">
Create a `Service` and `Ingress` for your application.

[Learn more](/modules/ingress-nginx/) about the capabilities of the `ingress-nginx` module.
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
🔍 <span class="cards-item__title-text">Monitoring your application</span>
</h3>
<div class="cards-item__text" markdown="1">
Add `prometheus.deckhouse.io/custom-target: "my-app"` and `prometheus.deckhouse.io/port: "80"` annotations to the Service created.

For more information, see the `monitoring-custom` module's [documentation](/modules/monitoring-custom/).
</div>
</div>

</div>
</div>
</section>

{% if page.platform_type == 'cloud' %}
<section class="cards-blocks">
<div class="cards-blocks__content container">
<h2 class="cards-blocks__title text_h2">
Other features
</h2>
<div class="cards-blocks__cards">

<div class="cards-item cards-item_inverse" style="width: 100%">
<h3 class="cards-item__title text_h3">
⚖ <span class="cards-item__title-text">Managing nodes</span>
</h3>
<div class="cards-item__text" markdown="1">
Run the following command to list nodegroups created in the cluster during the deployment process: `kubectl get nodegroups`. For more information, see the node-manager's [documentation](/modules/node-manager/).

You only need to make changes to `minPerZone` and `maxPerZone` parameters to scale the existing groups. If these two parameters are not equal, Deckhouse will automatically launch an autoscaler.

You need to create a new
[InstanceClass](/modules/cloud-provider-{{ page.platform_code | regex_replace: "^(openstack)_.+$", "\1" | replace: "dvp-provider", "dvp" | downcase }}/cr.html) and a
[NodeGroup](/modules/node-manager/cr.html#nodegroup) referring to it to create new groups.
</div>
</div>

</div>
</div>
</section>
{% endif %}

<div markdown="1">
## What's next?

Detailed information about the system and the Deckhouse Kubernetes Platform components is available in the [documentation](/products/kubernetes-platform/documentation/v1/).

Please, reach us via our [online community](/community/about.html#online-community) if you have any questions.
</div>
