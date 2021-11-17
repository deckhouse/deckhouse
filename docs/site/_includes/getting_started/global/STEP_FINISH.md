<script type="text/javascript" src='{{ assets["getting-started.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["getting-started-finish.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["bcrypt.js"].digest_path }}'></script>

{::options parse_block_html="false" /}

<div markdown="1">
## Congratulations, your Deckhouse platform is up and running!

Now that you have installed and properly configured Deckhouse, let's look at what you can do with it.

By default, the [Dex](https://dexidp.io/) is used for accessing all the components.

Here are credentials **generated** in the previous steps:
- Username — `admin@example.com`
- Password — `<GENERATED_PASSWORD>`

Use them to access the web interface of the Deckhouse components.
</div>

<section class="cards-blocks">
<div class="cards-blocks__content container">
<h2 class="cards-blocks__title text_h2">
Essentials
</h2>
<div class="cards-blocks__cards">

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
Up-to-date Deckhouse documentation in your cluster
</h3>
<div class="cards-item__text" markdown="1">
The in-cluster documentation for a specific version of Deckhouse used in your cluster is available at [deckhouse.example.com](https://deckhouse.example.com).
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
Monitoring
</h3>
<div class="cards-item__text" markdown="1">
Explore Grafana dashboards bundled with Deckhouse at [grafana.example.com](https://grafana.example.com).

Go to [grafana.example.com/prometheus/](https://grafana.example.com/prometheus/) to access Prometheus directly.

[Learn more](/en/documentation/v1/modules/300-prometheus/) about the `monitoring` module.
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
Dashboard
</h3>
<div class="cards-item__text" markdown="1">
The Kubernetes Dashboard is available at: [dashboard.example.com](https://dashboard.example.com)
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
Status page
</h3>
<div class="cards-item__text" markdown="1">
Visit [status.example.com](https://status.example.com) to get information about the overall status of Deckhouse and its components.

The [upmeter.example.com](https://upmeter.example.com) page provides detailed SLA statistics for each component and time frame.
</div>
</div>

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
Setting up a CI/CD system
</h3>
<div class="cards-item__text" markdown="1">
[Create](/en/documentation/v1/modules/140-user-authz/usage.html#creating-a-serviceaccount-and-granting-it-access)
a ServiceAccount to use for deploying to the cluster and grant it all the necessary privileges.

You can use the generated `kubeconfig` file in Kubernetes with any deployment system.
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
Routing traffic
</h3>
<div class="cards-item__text" markdown="1">
Create a `Service` and `Ingress` for your application.

[Learn more](/en/documentation/v1/modules/402-ingress-nginx/) about the capabilities of the `ingress-nginx` module.
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
Monitoring your application
</h3>
<div class="cards-item__text" markdown="1">
Add `prometheus.deckhouse.io/custom-target: "my-app"` and `prometheus.deckhouse.io/port: "80"` annotations to the Service created.

For more information, see the `monitoring-custom` module's [documentation](/en/documentation/v1/modules/340-monitoring-custom/).
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
Managing nodes
</h3>
<div class="cards-item__text" markdown="1">
Run the following command to list nodegroups created in the cluster during the deployment process: `kubectl get nodegroups`. For more information, see the node-manager's [documentation](/en/documentation/v1/modules/040-node-manager/).

You only need to make changes to `minReplicas` and `maxReplicas` parameters to scale the existing groups. If these two parameters are not equal, Deckhouse will automatically launch an autoscaler.

You need to create a new
[InstanceClass](/en/documentation/v1/modules/030-cloud-provider-{{ page.platform_code | downcase }}/cr.html) and a
[NodeGroup](/en/documentation/v1/modules/040-node-manager/cr.html#nodegroup) referring to it to create new groups.
</div>
</div>

</div>
</div>
</section>
{% endif %}

<section class="cards-blocks">
<div class="cards-blocks__content container">
<h2 class="cards-blocks__title text_h2">
External authentication
</h2>
<div markdown="1">
Deckhouse supports [various](https://deckhouse.io/en/documentation/v1/modules/150-user-authn/usage.html)
external authentication mechanisms.
</div>
<div class="cards-blocks__cards">

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
Configuring DexProvider
</h3>
<div class="cards-item__text" markdown="1">
You have to [configure](/en/documentation/v1/modules/150-user-authn/usage.html) a
`DexProvider` object to enable, e.g., GitHub-based authentication. After creating the `DexProvider` object, all access attempts to Deckhouse components such as Grafana, Dashboard, etc., 
will be authenticated using GitHub.
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
External authentication for any Ingress
</h3>
<div class="cards-item__text" markdown="1">
You have to create a [DexAuthenticator](https://deckhouse.io/en/documentation/v1/modules/150-user-authn/cr.html#dexauthenticator) object to enable external authentication for any Ingress resource.
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
External authentication for the Kubernetes API
</h3>
<div class="cards-item__text" markdown="1">
Configure
[`publishAPI`](/en/documentation/v1/modules/150-user-authn/faq.html#how-can-i-generate-a-kubeconfig-and-access-kubernetes-api), download kubectl
and create a `kubeconfig` file for external access to the API using the web interface available at `kubeconfig.example.com`.
</div>
</div>

</div>
</div>
</section>

<div markdown="1">
## What's next?

Detailed information about the system and the Deckhouse Platform components is available in the [documentation](/en/documentation/v1/).

Please, reach us via our [online community](/en/community/about.html#online-community) if you have any questions.
</div>
