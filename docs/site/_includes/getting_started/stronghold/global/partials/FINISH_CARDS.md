<section class="cards-blocks">
<div class="cards-blocks__content">
<h2 class="cards-blocks__title text_h2">
Getting started with the cluster
</h2>
<div class="cards-blocks__cards">

{% if page.platform_code != 'existing' and page.platform_code != 'kind' %}
<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
📚 <span class="cards-item__title-text">Documentation</span>
</h3>
<div class="cards-item__text">
<p>Documentation for the Deckhouse Platform version installed in your cluster.</p>
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
<p>Explore Grafana dashboards bundled with Deckhouse Platform.</p>
<p>Web service name: {% include getting_started/global/partials/dns-template-title.html.liquid name="grafana" %}</p>
<p>To access Prometheus: {% include getting_started/global/partials/dns-template-title.html.liquid name="grafana" path="/prometheus/" onlyPath="true" %}</p>
<p>For details, see the <a href="/modules/prometheus/" target="_blank"><code>prometheus</code> module documentation</a>.</p>
</div>
</div>
{% endif %}

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
☸ <span class="cards-item__title-text">Dashboard</span>
</h3>
<div class="cards-item__text">
<p>Get access to the Kubernetes Dashboard.</p>
<p>Web service name: {% include getting_started/global/partials/dns-template-title.html.liquid name="dashboard" %}</p>
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
👌 <span class="cards-item__title-text">Status page</span>
</h3>
<div class="cards-item__text">
<p>Get information about the overall status of Deckhouse Platform and its components.<br />
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

Use the [production readiness checklist](/products/kubernetes-platform/documentation/v1/guides/production.html) to make sure you haven't forgotten anything.
</div>
</div>
{%- endif %}
</div>
</div>
</section>

<section class="cards-blocks">
<div class="cards-blocks__content">
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

For details, see the [`ingress-nginx` module documentation](/modules/ingress-nginx/).
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
🔍 <span class="cards-item__title-text">Monitoring your application</span>
</h3>
<div class="cards-item__text" markdown="1">
Add `prometheus.deckhouse.io/custom-target: "my-app"` and `prometheus.deckhouse.io/port: "80"` annotations to the Service created.

For details, see the [`monitoring-custom` module documentation](/modules/monitoring-custom/).
</div>
</div>

</div>
</div>
</section>

{% if page.platform_type == 'cloud' %}
<section class="cards-blocks">
<div class="cards-blocks__content">
<h2 class="cards-blocks__title text_h2">
Other features
</h2>
<div class="cards-blocks__cards">

<div class="cards-item cards-item_inverse" style="width: 100%">
<h3 class="cards-item__title text_h3">
⚖ <span class="cards-item__title-text">Managing nodes</span>
</h3>
<div class="cards-item__text" markdown="1">
Run the following command to list NodeGroups created in the cluster during the deployment process: `d8 k get nodegroups`. For details, see the [`node-manager` module documentation](/modules/node-manager/).

You only need to make changes to `minPerZone` and `maxPerZone` parameters to scale the existing groups. If these two parameters are not equal, Deckhouse will automatically launch an autoscaler.

You need to create a new
[InstanceClass](/modules/cloud-provider-{{ page.platform_code | regex_replace: "^(openstack)_.+$", "\1" | downcase }}/cr.html) and a
[NodeGroup](/modules/node-manager/cr.html#nodegroup) referring to it to create new groups.
</div>
</div>

</div>
</div>
</section>
{% endif %}

<div markdown="1">
## Next steps

Detailed information about the system and components is available in the [Stronghold documentation](/products/stronghold/documentation/admin/overview.html).

If you have any questions, contact the [Deckhouse online community](/community/about.html#online-community).
</div>
