<section class="cards-blocks">
<div class="cards-blocks__content">
<h2 class="cards-blocks__title text_h2">
Getting started with the cluster
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
{%- endif %}
</div>
</div>
</section>

<section class="cards-blocks">
<div class="cards-blocks__content">
<h2 class="cards-blocks__title text_h2">
Deploying your first application
</h2>
<p class="cards-blocks__lead">Your cluster is up and still empty. Below are the ways to get an application into it — from a single command to a full GitOps pipeline. If you just want to see something running, start with the first one.</p>
<div class="cards-blocks__cards">

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
⌨ <span class="cards-item__title-text">Start here: plain manifests</span>
</h3>
<div class="cards-item__text" markdown="1">
No extra tooling to install — everything you need is already in `d8`. The `d8 k` command is a built-in `kubectl`: apply your application manifests, and a minute later it is running in the cluster.

<!-- TODO: link -->
[Deploy your first application](#TODO)
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
🧩 <span class="cards-item__title-text">An application from the Marketplace</span>
</h3>
<div class="cards-item__text" markdown="1">
Pick a ready-made application from the catalog and install it into your namespace — no hunting for charts and figuring out their values. Your cluster administrator fills the catalog; the Marketplace is available from DKP 1.76.

<!-- TODO: link -->
[More about the Marketplace](/products/kubernetes-platform/documentation/latest/admin/configuration/marketplace/)
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
🗄 <span class="cards-item__title-text">Managed services</span>
</h3>
<div class="cards-item__text" markdown="1">
Bring up PostgreSQL, Kafka, ClickHouse, RabbitMQ, OpenSearch, or another service with Deckhouse modules — scaling, backups, and upgrades are the platform's job.

<!-- TODO: link -->
[More about managed services](/products/kubernetes-platform/features/managed-services/)
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
🚢 <span class="cards-item__title-text">Building and shipping your own application</span>
</h3>
<div class="cards-item__text" markdown="1">
Build your images, push them to the registry, and deploy the application to the cluster — with one tool, `d8 delivery-kit`, instead of three.

<!-- TODO: link -->
[More about Delivery Kit](/products/delivery-kit/)
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
🔁 <span class="cards-item__title-text">GitOps: deploy with Argo CD</span>
</h3>
<div class="cards-item__text" markdown="1">
Keep your application manifests in Git — Argo CD brings the cluster in line with what they describe. Argo CD itself is deployed and maintained by the platform, so you never install it by hand.

<!-- TODO: link -->
[More about the operator-argo module](/products/kubernetes-platform/documentation/latest/admin/configuration/delivery/argocd/)
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
⎈ <span class="cards-item__title-text">Helm charts without <code>helm install</code></span>
</h3>
<div class="cards-item__text" markdown="1">
Add a Helm or OCI repository, pick a chart and a version — the platform installs the release and maintains it.

<!-- TODO: link -->
[More about the operator-helm module](/modules/operator-helm/stable/)
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
⟳ <span class="cards-item__title-text">Integration with your existing CI/CD system</span>
</h3>
<div class="cards-item__text" markdown="1">
Create a ServiceAccount with deploy permissions for the cluster — you get a `kubeconfig` that suits any delivery system for Kubernetes.

[More about service access to the cluster](/modules/user-authz/usage.html#creating-a-serviceaccount-for-a-machine-and-granting-it-access)
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
🖥 <span class="cards-item__title-text">Legacy applications in a virtual machine</span>
</h3>
<div class="cards-item__text" markdown="1">
Can't containerize an application? Run it as a virtual machine in this same cluster: the same API, the same permissions, the same practices as for containers.

<!-- TODO: link -->
[More about Deckhouse Virtualization Platform](/products/virtualization-platform/documentation/)
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
Run the following command to list nodegroups created in the cluster during the deployment process: `d8 k get nodegroups`. For more information, see the node-manager's [documentation](/modules/node-manager/).

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
## Next steps

Detailed information about the system and the Deckhouse Kubernetes Platform components is available in the [documentation](/products/kubernetes-platform/documentation/v1/).

Contact our [online community](/community/about.html#online-community) if you have any questions.
</div>
