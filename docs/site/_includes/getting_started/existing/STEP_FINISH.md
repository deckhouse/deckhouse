<script type="text/javascript" src='{{ assets["getting-started.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["getting-started-finish.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["bcrypt.js"].digest_path }}'></script>

{::options parse_block_html="false" /}

<div markdown="1">
## Congratulations, your Deckhouse platform is up and running!

Now that you have installed and properly configured Deckhouse, let's look at what you can do with it.

The in-cluster documentation is available at [deckhouse.example.com](https://deckhouse.example.com)

Access to the documentation is restricted via the basic authentication mechanism (additional authentication options are provided in the [user-auth](/documentation/v1/modules/150-user-authn/) module:
- Login — `admin`
- Password — generated automatically. It can be found in the `deckhouse` ConfigMap in the configuration section of the `deckhouse-web` module. Use the following command to get it:

  ```bash
  kubectl -n d8-system get cm deckhouse -o jsonpath="{.data.deckhouseWeb}" | grep password
  ```

  Sample output:

  ```
  $ kubectl -n d8-system get cm deckhouse -o jsonpath="{.data.deckhouseWeb}" | grep password 
  password: UJvSB4UYTa3fnDOco6LF
  ```

> The following problems may cause [deckhouse.example.com](https://deckhouse.example.com) to be unreachable:
- Ingress controller-level issues;
- DNS-related issues;
- network problems (filtering, routing...).
</div>

<section class="cards-blocks">
<div class="cards-blocks__content container">
<h2 class="cards-blocks__title text_h2">
Essentials
</h2>
<div class="cards-blocks__cards">

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
Monitoring
</h3>
<div class="cards-item__text" markdown="1">
Explore Grafana dashboards bundled with Deckhouse at [grafana.example.com](https://grafana.example.com).

Go to [grafana.example.com/prometheus/](https://grafana.example.com/prometheus/) to access Prometheus directly.

[Learn more](/documentation/v1/modules/300-prometheus/) about the `monitoring` module.
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
Dashboard
</h3>
<div class="cards-item__text" markdown="1">
Enable the [dashboard](/documentation/v1/modules/500-dashboard/) module and get access to the Kubernetes Dashboard at: [dashboard.example.com](https://dashboard.example.com)
</div>
</div>

<div class="cards-item cards-item_inverse">
<h3 class="cards-item__title text_h3">
Status page
</h3>
<div class="cards-item__text" markdown="1">
Enable the [upmeter](/documentation/v1/modules/500-upmeter/) module and get information about the overall status of Deckhouse and its components at [status.example.com](https://status.example.com).

At the [upmeter.example.com](https://upmeter.example.com) page you can get detailed SLA statistics for each component and time frame.
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
Enable the [user-authz](/documentation/v1/modules/140-user-authz/) module and [create](/documentation/v1/modules/140-user-authz/usage.html#creating-a-serviceaccount-for-a-machine-and-granting-it-access) a ServiceAccount to use for deploying to the cluster and grant it all the necessary privileges.

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
Enable the [monitoring-custom](/documentation/v1/modules/340-monitoring-custom/) module and add `prometheus.deckhouse.io/custom-target: "my-app"` and `prometheus.deckhouse.io/port: "80"` annotations to the Service created.

For more information, see the `monitoring-custom` module's [documentation](/documentation/v1/modules/340-monitoring-custom/).
</div>
</div>

</div>
</div>
</section>

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
You have to [configure](/documentation/v1/modules/150-user-authn/usage.html) a `DexProvider` object to enable, e.g., GitHub-based authentication. After creating the `DexProvider` object, all access attempts to Deckhouse components such as Grafana, Dashboard, etc., will be authenticated using GitHub.
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
Configure [`publishAPI`](/documentation/v1/modules/150-user-authn/faq.html#how-can-i-generate-a-kubeconfig-and-access-kubernetes-api), download kubectl
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
