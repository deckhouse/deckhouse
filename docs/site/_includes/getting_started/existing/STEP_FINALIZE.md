<script type="text/javascript" src='{{ assets["getting-started.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["getting-started-access.js"].digest_path }}'></script>

To access the web interfaces of Deckhouse services, you need to:
- configure DNS
- specify [template for DNS names](../../documentation/v1/deckhouse-configure-global.html#parameters-modules-publicdomaintemplate)

The *DNS names template* is used to configure Ingress resources of system applications. For example, the name `deckhouse` is assigned to the in-cluster documentation module interface. Then, for the template `%s.kube.company.my` Grafana will be available at `deckhouse.kube.company.my`, etc.

The guide will use [sslip.io](https://sslip.io/) to simplify configuration.

Run the following command to configure [template for DNS names](../../documentation/v1/deckhouse-configure-global.html#parameters-modules-publicdomaintemplate) to use the *sslip.io* (specify the public IP address of the node where the Ingress controller is running):
{% snippetcut %}
{% raw %}
```shell
BALANCER_IP=<INGRESS_CONTROLLER_IP> 
kubectl patch mc global --type merge \
  -p "{\"spec\": {\"settings\":{\"modules\":{\"publicDomainTemplate\":\"%s.${BALANCER_IP}.sslip.io\"}}}}" && echo && \
echo "Domain template is '$(kubectl get mc global -o=jsonpath='{.spec.settings.modules.publicDomainTemplate}')'."
```
{% endraw %}
{% endsnippetcut %}

The command will also print the DNS name template set in the cluster. Example output:
```text
moduleconfig.deckhouse.io/global patched

Domain template is '%s.1.2.3.4.sslip.io'.
```

{% alert %}
Regenerating certificates after changing the DNS name template can take up to 5 minutes.
{% endalert %}

{% offtopic title="Other options..." %}
Instead of using *sslip.io*, you can use other options.
{% include getting_started/global/partials/DNS_OPTIONS.liquid %}

Then, run the following command to change the DNS name template:
<div markdown="0">
{% snippetcut %}
```shell
kubectl patch mc global --type merge -p "{\"spec\": {\"settings\":{\"modules\":{\"publicDomainTemplate\":\"${DOMAIN_TEMPLATE}\"}}}}"
```
{% endsnippetcut %}
</div>
{% endofftopic %}
