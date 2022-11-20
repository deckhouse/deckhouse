<script type="text/javascript" src='{{ assets["getting-started.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["getting-started-finish.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["bcrypt.js"].digest_path }}'></script>

{::options parse_block_html="false" /}

<div markdown="1">
## Congratulations, your Deckhouse platform is up and running!

Let's see what is available in the [monitoring](/documentation/v1/modules/300-prometheus/) module.

- **Grafana** is available at [grafana-127-0-0-1.nip.io](http://grafana-127-0-0-1.nip.io).

  Access to Grafana is restricted via the basic authentication mechanism (additional authentication options are provided in the [user-auth](/documentation/v1/modules/150-user-authn/) module:
  - Login — `admin`;
  - Password — generated automatically. It can be found in the `deckhouse` ConfigMap in the configuration section of the `prometheus` module. Use the following command to get it:

    ```bash
    kubectl -n d8-system get cm deckhouse -o jsonpath="{.data.prometheus}" | grep password
    ```

    Sample output:

    ```
    $ kubectl -n d8-system get cm deckhouse -o jsonpath="{.data.prometheus}" | grep password 
    password: UJvSB4UYTa3fnDOco6LF
    ```
  
  Explore Grafana dashboards bundled with Deckhouse.

- **Prometheus** is available at [grafana-127-0-0-1.nip.io/prometheus/](http://grafana-127-0-0-1.nip.io/prometheus/).

</div>

Deckhouse, deployed in the kind cluster, is suitable for getting acquainted with other features that might need for production environments.
Read further about such Deckhouse features.

<section class="cards-blocks">

<div class="cards-blocks__content container">
<h2 class="cards-blocks__title text_h2">
Essentials
</h2>
<div class="cards-blocks__cards">

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
Dashboard
</h3>
<div class="cards-item__text" markdown="1">
Enable the [dashboard](/documentation/v1/modules/500-dashboard/) module and get access to the Kubernetes Dashboard at: [dashboard-127-0-0-1.nip.io](http://dashboard-127-0-0-1.nip.io/).

You need to enable the [user-authz](/documentation/v1/modules/140-user-authz/) module for Dashboard to work.
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
Status page
</h3>
<div class="cards-item__text" markdown="1">
Enable the [upmeter](/documentation/v1/modules/500-upmeter/) module and get information about the overall status of Deckhouse and its components at [status-127-0-0-1.nip.io](http://status-127-0-0-1.nip.io).

At the [upmeter-127-0-0-1.nip.io](http://upmeter-127-0-0-1.nip.io) page you can get detailed SLA statistics for each component and time frame.
</div>
</div>

<div style="width: 30%">&nbsp;</div>

</div>
</div>
</section>

<section class="cards-blocks">
<div class="cards-blocks__content container">
<h2 class="cards-blocks__title text_h2">
Deploying your first application
</h2>
<div markdown="1">
Deckhouse makes it easier to set up CI/CD system access to a cluster to deploy your application and allows your application to be monitored by custom metrics.
</div>

<div class="cards-blocks__cards">

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
Setting up a CI/CD system
</h3>
<div class="cards-item__text" markdown="1">
[Create](/documentation/v1/modules/140-user-authz/usage.html#creating-a-serviceaccount-for-a-machine-and-granting-it-access) a ServiceAccount to use for deploying to the cluster and grant it all the necessary privileges.

You can use the generated `kubeconfig` file in Kubernetes with any deployment system.
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
Routing traffic
</h3>
<div class="cards-item__text" markdown="1">
Create a `Service` and `Ingress` for your application.

[Learn more](/documentation/v1/modules/402-ingress-nginx/) about the capabilities of the `ingress-nginx` module.
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
Monitoring your application
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
Managing nodes
</h3>
<div class="cards-item__text" markdown="1">
Run the following command to list nodegroups created in the cluster during the deployment process: `kubectl get nodegroups`. For more information, see the node-manager's [documentation](/documentation/v1/modules/040-node-manager/).

You only need to make changes to `minPerZone` and `maxPerZone` parameters to scale the existing groups. If these two parameters are not equal, Deckhouse will automatically launch an autoscaler.

You need to create a new
[InstanceClass](/documentation/v1/modules/030-cloud-provider-{{ page.platform_code | downcase }}/cr.html) and a
[NodeGroup](/documentation/v1/modules/040-node-manager/cr.html#nodegroup) referring to it to create new groups.
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
Deckhouse supports [various](/documentation/v1/modules/150-user-authn/usage.html)
external authentication mechanisms.
</div>
<div class="cards-blocks__cards">

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
Configuring DexProvider
</h3>
<div class="cards-item__text" markdown="1">
You have to [configure](/documentation/v1/modules/150-user-authn/usage.html) a
`DexProvider` object to enable, e.g., GitHub-based authentication. After creating the `DexProvider` object, all access attempts to Deckhouse components such as Grafana, Dashboard, etc.,
will be authenticated using GitHub.
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
External authentication for any Ingress
</h3>
<div class="cards-item__text" markdown="1">
You have to create a [DexAuthenticator](/documentation/v1/modules/150-user-authn/cr.html#dexauthenticator) object to enable external authentication for any Ingress resource.
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
External authentication for the Kubernetes API
</h3>
<div class="cards-item__text" markdown="1">
Configure
[`publishAPI`](/documentation/v1/modules/150-user-authn/faq.html#how-can-i-generate-a-kubeconfig-and-access-kubernetes-api), download kubectl
and create a `kubeconfig` file for external access to the API using the web interface available at `kubeconfig.example.com`.
</div>
</div>

</div>
</div>
</section>

<div markdown="1">
## What's next?

Detailed information about the system and the Deckhouse Platform components is available in the [documentation](/documentation/v1/).

Please, reach us via our [online community](/en/community/about.html#online-community) if you have any questions.
</div>
