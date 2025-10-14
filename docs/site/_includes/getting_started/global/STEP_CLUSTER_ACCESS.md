<script type="text/javascript" src='{% javascript_asset_tag getting-started %}[_assets/js/getting-started.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag getting-started-access %}[_assets/js/getting-started-access.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag getting-started-finish %}[_assets/js/getting-started-finish.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag bcrypt %}[_assets/js/bcrypt.js]{% endjavascript_asset_tag %}'></script>

## Accessing to the master node
Deckhouse have finished installation process. It remains to make some settings, for which you need to connect to the **master node**.

Connect to the master node via SSH (the IP address of the master node was printed by the installer upon completion of the installation, but you can also find it using the cloud provider web interface/CLI tool):

```shell
ssh {% if page.platform_code == "azure" %}azureuser{% elsif page.platform_code == "gcp" or page.platform_code == "dynamix" %}user{% else %}ubuntu{% endif %}@<MASTER_IP>
```

Check the kubectl is working by displaying a list of cluster nodes:

```shell
sudo -i d8 k get nodes
```

{% offtopic title="Example of the output..." %}
```
$ sudo -i d8 k get nodes
NAME                                     STATUS   ROLES                  AGE   VERSION
cloud-demo-master-0                      Ready    control-plane,master   12h   v1.23.9
cloud-demo-worker-01a5df48-84549-jwxwm   Ready    worker                 12h   v1.23.9
```
{%- endofftopic %}

It may take some time to start the Ingress controller after installing Deckhouse. Make sure that the Ingress controller has started before continuing:

```shell
sudo -i d8 k -n d8-ingress-nginx get po
```

Wait for the Pods to switch to `Ready` state.

{% offtopic title="Example of the output..." %}
```
$ sudo -i d8 k -n d8-ingress-nginx get po
NAME                                       READY   STATUS    RESTARTS   AGE
controller-nginx-r6hxc                     3/3     Running   0          16h
kruise-controller-manager-78786f57-82wph   3/3     Running   0          16h
```
{%- endofftopic %}

{% if page.platform_type == 'cloud' and page.platform_code != 'vsphere' %}
Also wait for the load balancer to be ready:

```shell
sudo -i d8 k -n d8-ingress-nginx get svc nginx-load-balancer
```

The `EXTERNAL-IP` value must be filled with a public IP address or DNS name.

{% offtopic title="Example of the output..." %}
```
$ sudo -i d8 k -n d8-ingress-nginx get svc nginx-load-balancer
NAME                  TYPE           CLUSTER-IP      EXTERNAL-IP     PORT(S)                      AGE
nginx-load-balancer   LoadBalancer   10.222.91.204   1.2.3.4         80:30493/TCP,443:30618/TCP   1m
```
{%- endofftopic %}
{% endif %}

## DNS

To access the web interfaces of Deckhouse services, you need to:
- configure DNS
- specify [template for DNS names](../../documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate)

The *DNS names template* is used to configure Ingress resources of system applications. For example, the name `grafana` is assigned to the Grafana interface. Then, for the template `%s.kube.company.my` Grafana will be available at `grafana.kube.company.my`, etc.

{% if page.platform_type == 'cloud' and page.platform_code != 'vsphere' %}
The guide will use [sslip.io](https://sslip.io/) to simplify configuration.

Run the following command on **the master node** to get the load balancer IP and to configure [template for DNS names](../../documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate) to use the *sslip.io*:
{% if page.platform_code == 'aws' %}
{% raw %}
```shell
BALANCER_IP=$(dig $(sudo -i d8 k -n d8-ingress-nginx get svc nginx-load-balancer -o json | jq -r '.status.loadBalancer.ingress[0].hostname') +short | head -1) && \
echo "Balancer IP is '${BALANCER_IP}'." && sudo -i d8 k patch mc global --type merge \
  -p "{\"spec\": {\"settings\":{\"modules\":{\"publicDomainTemplate\":\"%s.${BALANCER_IP}.sslip.io\"}}}}" && echo && \
echo "Domain template is '$(sudo -i d8 k get mc global -o=jsonpath='{.spec.settings.modules.publicDomainTemplate}')'."
```
{% endraw %}
{% else %}
{% raw %}
```shell
BALANCER_IP=$(sudo -i d8 k -n d8-ingress-nginx get svc nginx-load-balancer -o json | jq -r '.status.loadBalancer.ingress[0].ip') && \
echo "Balancer IP is '${BALANCER_IP}'." && sudo -i d8 k patch mc global --type merge \
  -p "{\"spec\": {\"settings\":{\"modules\":{\"publicDomainTemplate\":\"%s.${BALANCER_IP}.sslip.io\"}}}}" && echo && \
echo "Domain template is '$(sudo -i d8 k get mc global -o=jsonpath='{.spec.settings.modules.publicDomainTemplate}')'."
```
{% endraw %}
{% endif %}

The command will also print the DNS name template set in the cluster. Example output:
```text
Balancer IP is '1.2.3.4'.
moduleconfig.deckhouse.io/global patched

Domain template is '%s.1.2.3.4.sslip.io'.
```

{% alert %}
Regenerating certificates after changing the DNS name template can take up to 5 minutes.
{% endalert %}

{% offtopic title="Other options..." %}
Instead of using *sslip.io*, you can use other options.
{% include getting_started/global/partials/DNS_OPTIONS.liquid %}

Then, run the following command on the **master node** (specify the template for DNS names to use in the <code>DOMAIN_TEMPLATE</code> variable):
<div markdown="1">
```shell
DOMAIN_TEMPLATE='<DOMAIN_TEMPLATE>'
sudo -i d8 k patch mc global --type merge -p "{\"spec\": {\"settings\":{\"modules\":{\"publicDomainTemplate\":\"${DOMAIN_TEMPLATE}\"}}}}"
```
</div>
{% endofftopic %}
{% endif %}

{% if page.platform_type == 'cloud' and page.platform_code == 'vsphere' %} 
Configure DNS for Deckhouse services using one of the following methods:

{% include getting_started/global/partials/DNS_OPTIONS.liquid %}

Then, run the following command on the **master node** (specify the template for DNS names to use in the <code>DOMAIN_TEMPLATE</code> variable):
{% raw %}
```shell
DOMAIN_TEMPLATE='<DOMAIN_TEMPLATE>'
sudo -i d8 k patch mc global --type merge -p "{\"spec\": {\"settings\":{\"modules\":{\"publicDomainTemplate\":\"${DOMAIN_TEMPLATE}\"}}}}"
```
{% endraw %}
{% endif %}

## Configure remote access to the cluster 

On **a personal computer** follow these steps to configure the connection of `kubectl` to the cluster:
- Open *Kubeconfig Generator* web interface. The name `kubeconfig` is reserved for it, and the address for access is formed according to the DNS names template (which you set up erlier). For example, for the DNS name template `%s.1.2.3.4.sslip.io`, the *Kubeconfig Generator* web interface will be available at `https://kubeconfig.1.2.3.4.sslip.io`.
- Log in as a user `admin@deckhouse.io`. The user password generated in the previous step is `<GENERATED_PASSWORD>` (you can also find it in the `User` CustomResource in the `config.yml` file).
- Select the tab with the OS of the personal computer.
- Sequentially copy and execute the commands given on the page.
- Check that `kubectl` connects to the cluster (for example, execute the command `kubectl get no`).
