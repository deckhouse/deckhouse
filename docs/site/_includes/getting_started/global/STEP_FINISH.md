<script type="text/javascript" src='{% javascript_asset_tag getting-started %}[_assets/js/getting-started.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag getting-started-finish %}[_assets/js/getting-started-finish.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag bcrypt %}[_assets/js/bcrypt.js]{% endjavascript_asset_tag %}'></script>

{::options parse_block_html="false" /}

<div markdown="1">
## Everything is installed, configured, and working!

Now that you have installed and properly configured Deckhouse Kubernetes Platform, let's look at what you can do with it.

By default, the [Dex](https://dexidp.io/) is used for accessing all the components.

{% unless page.gs_installer %}
Here are credentials **generated** in the previous steps:

- Username — `admin@deckhouse.io`
- Password — `<GENERATED_PASSWORD>` (you can also find it in the `User` CustomResource in the `config.yml` file)

Use them to access the web interface of the Deckhouse Kubernetes Platform components.
{% endunless %}
</div>

{% if page.gs_installer %}
<div markdown="1">

Open the cluster web interface by clicking the "Connect and open" button in the row with the created cluster on the main screen.

<img src="/images/gs/installer/open-console.png" alt='What the "Connect and open" button looks like...' style="width: 100%;">

The web interface for managing the installed DKP cluster will open in the same window.

<img src="/images/gs/installer/console.png" alt="What the web interface looks like..." style="width: 100%;">

{% if page.platform_type == 'baremetal' %}
If you **did not** configure the DNS name template and **did not** create an Ingress controller during installation, follow these steps:
{% else %}
Follow these steps:
{% endif %}

1. Install an Ingress controller.  
   Go to "Network" → "Load balancing" → "Ingress controllers" and create a new Ingress controller by clicking "Add" and selecting "Host port".

   <img src="/images/gs/installer/ingress-create.png" alt="Creating an Ingress controller" style="width: 100%;">

   Enter a name and click "Create".  
   If you need HTTPS access to cluster components, enable it in the "Default certificate" section.

   <img src="/images/gs/installer/ingress-settings.png" alt="New Ingress controller settings" style="width: 100%;">

2. Configure the DNS name template to be used for cluster components.
   {% alert level="info" %}
   The DNS name template is used to configure Ingress resources for system applications. For example, the Grafana UI is bound to the name `grafana`. Then, for the template `%s.kube.company.my`, Grafana will be available at `grafana.kube.company.my`, and so on.
   {% endalert %}
   Go to "Deckhouse" → "Global settings" and enter the template in the "DNS name template" field.

   <img src="/images/gs/installer/dns-settings.png" alt="DNS template settings" style="width: 100%;">

</div>
{% endif %}

{% include getting_started/global/partials/FINISH_CARDS.md %}
