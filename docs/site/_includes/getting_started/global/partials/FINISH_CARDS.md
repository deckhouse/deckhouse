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
<a href="/documentation/v1/modules/300-prometheus/" target="_blank">Learn more</a> about the <code>monitoring</code> module.
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
<p>Get information about the overall status of Deckhouse and its components.<br />
Web service name: {% include getting_started/global/partials/dns-template-title.html.liquid name="status" %}</p>

<p>Get detailed SLA statistics for each component and time frame.<br />
Web service name: {% include getting_started/global/partials/dns-template-title.html.liquid name="upmeter" %}</p>
</div>
</div>

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
[Create](/documentation/v1/modules/140-user-authz/usage.html#creating-a-serviceaccount-for-a-machine-and-granting-it-access)
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

[Learn more](/documentation/v1/modules/402-ingress-nginx/) about the capabilities of the `ingress-nginx` module.
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
🔍 <span class="cards-item__title-text">Monitoring your application</span>
</h3>
<div class="cards-item__text" markdown="1">
Add `prometheus.deckhouse.io/custom-target: "my-app"` and `prometheus.deckhouse.io/port: "80"` annotations to the Service created.

For more information, see the `monitoring-custom` module's [documentation](/documentation/v1/modules/340-monitoring-custom/).
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
Run the following command to list nodegroups created in the cluster during the deployment process: `kubectl get nodegroups`. For more information, see the node-manager's [documentation](/documentation/v1/modules/040-node-manager/).

You only need to make changes to `minPerZone` and `maxPerZone` parameters to scale the existing groups. If these two parameters are not equal, Deckhouse will automatically launch an autoscaler.

You need to create a new
[InstanceClass](/documentation/v1/modules/030-cloud-provider-{{ page.platform_code | regex_replace: "^(openstack)_.+$", "\1" | downcase }}/cr.html) and a
[NodeGroup](/documentation/v1/modules/040-node-manager/cr.html#nodegroup) referring to it to create new groups.
</div>
</div>

</div>
</div>
</section>
{% endif %}

<div markdown="1">
## What's next?

Detailed information about the system and the Deckhouse Kubernetes Platform components is available in the [documentation](/documentation/v1/).

Please, reach us via our [online community](/community/about.html#online-community) if you have any questions.
</div>
