<script type="text/javascript" src='{{ assets["getting-started.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["getting-started-access.js"].digest_path }}'></script>

# Access cluster Kubernetes API
Deckhouse have just finished installation process of your cluster. Now you can connect to master via ssh.
To do, so you need to get master IP either from dhctl logs or from cloud provider web interface/cli tool.
{% snippetcut %}
```shell
ssh {% if page.platform_code == "azure" %}azureuser{% elsif page.platform_code == "gcp" %}user{% else %}ubuntu{% endif %}@<MASTER_IP>
```
{% endsnippetcut %}
You can run kubectl on master node from the `root` user. This is not secure way and we recommend to configure [external access](/documentation/v1/modules/150-user-authn/faq.html#how-can-i-generate-a-kubeconfig-and-access-kubernetes-api) to Kubernetes API later.
{% snippetcut %}
```shell
sudo -i
kubectl get nodes
```
{% endsnippetcut %}

# Access cluster using NGINX Ingress
[IngressNginxController](/documentation/v1/modules/402-ingress-nginx/cr.html#ingressnginxcontroller) was created during the installation process of the cluster.
The only thing left is to configure access to web interfaces of components that are already installed in the cluster (Grafana, Prometheus, Dashboard, etc.).
{% if page.platform_type == 'cloud' and page.platform_code != 'vsphere' %}
LoadBalancer is already created, and you just need to point a DNS domain to it.
First, you need to connect to your master node as described [previously](#access-cluster-kubernetes-api).

Get the IP address of the load balancer. Run the following command from the root user:
{% if page.platform_code == 'aws' %}
{% snippetcut %}
{% raw %}
```shell
BALANCER_HOSTNAME=$(kubectl -n d8-ingress-nginx get svc nginx-load-balancer -o json | jq -r '.status.loadBalancer.ingress[0].hostname')
echo "$BALANCER_HOSTNAME"
```
{% endraw %}
{% endsnippetcut %}
{% else %}
{% snippetcut %}
{% raw %}
```shell
BALANCER_IP=$(kubectl -n d8-ingress-nginx get svc nginx-load-balancer -o json | jq -r '.status.loadBalancer.ingress[0].ip')
echo "$BALANCER_IP"
```
{% endraw %}
{% endsnippetcut %}
{% endif %}
{% endif %}

Point a DNS domain you specified in the "[Cluster Installation](./step3.html)" step to Deckhouse web interfaces in one of the following ways:
<ul>
  <li>If you have the DNS server and you can add a DNS records:
    <ul>
      <li>If your cluster DNS name template is a <a href="https://en.wikipedia.org/wiki/Wildcard_DNS_record">wildcard
        DNS</a> (e.g., <code>%s.kube.my</code>), then add
        {%- if page.platform_code == 'aws' %} a corresponding wildcard CNAME record containing the hostname of load
        balancer (<code>BALANCER_HOSTNAME</code>)
        {%- else %} a corresponding wildcard A record containing the IP of {% if page.platform_code == 'vsphere' %}the master node, you've discovered previously (if dedicated frontend nodes are configured, then use their IP instead of the IP of the master node){% else %}the load balancer (<code>BALANCER_IP</code>), you've discovered previously{% endif %}{%- endif -%}.
      </li>
      <li>If your cluster DNS name template is <strong>NOT</strong> a <a
              href="https://en.wikipedia.org/wiki/Wildcard_DNS_record">wildcard DNS</a> (e.g., <code>%s-kube.company.my</code>),
        then add A or CNAME records containing the IP of {% if page.platform_code == 'vsphere' %}the master node, you've discovered
        previously (if dedicated frontend nodes are configured, then use their IP instead of the IP of the master node){% else %}the load balancer (<code>BALANCER_IP</code>), you've discovered
        previously{% endif %}, for the following Deckhouse service DNS names:
        <div class="highlight">
<pre class="highlight">
<code example-hosts>api.example.com
dashboard.example.com
deckhouse.example.com
dex.example.com
grafana.example.com
kubeconfig.example.com
status.example.com
upmeter.example.com</code>
</pre>
      </div>
    </li>
  </ul>
</li>
  <li><p>If you don't have a DNS server, then on the computer from which you need access to Deckhouse services add static records to the file <code>/etc/hosts</code> (for Linux, or <code>%SystemRoot%\system32\drivers\etc\hosts</code> for Windows).</p>
{% if page.platform_code == 'aws' %}
    <p>You can determine the IP address of the AWS load balancer using the following command (in the cluster):</p>

<div markdown="1">
{% snippetcut %}
```bash
BALANCER_IP=$(dig "$BALANCER_HOSTNAME" +short | head -1); echo "$BALANCER_IP"
```
{% endsnippetcut %}
</div>
{% endif %}

    <p>To add records to the <code>/etc/hosts</code> file locally, follow these steps:</p>

  <ul>
{%- if page.platform_code != 'vsphere' %}
    <li><p>Export the <code>BALANCER_IP</code> variable by specifying the IP address you got:</p>
{% snippetcut %}
```bash
export BALANCER_IP="<BALANCER_IP>"
```
{% endsnippetcut %}
    </li>
{%- else %}
    <li><p>Export the <code>BALANCER_IP</code> variable by specifying the IP address of <strong>the master node</strong> you've got (if dedicated frontend nodes are configured, then use their IP instead of the IP of the master node):</p>
{% snippetcut %}
```bash
export BALANCER_IP="<MASTER_OR_FRONT_IP>"
```
{% endsnippetcut %}
    </li>
{%- endif %}
  <li><p>Add DNS records for the Deckhouse services:</p>
{% snippetcut selector="example-hosts" %}
```bash
sudo -E bash -c "cat <<EOF >> /etc/hosts
$BALANCER_IP api.example.com
$BALANCER_IP dashboard.example.com
$BALANCER_IP deckhouse.example.com
$BALANCER_IP dex.example.com
$BALANCER_IP grafana.example.com
$BALANCER_IP kubeconfig.example.com
$BALANCER_IP status.example.com
$BALANCER_IP upmeter.example.com
EOF
"
```
{% endsnippetcut %}
</li>
</ul></li>
</ul>
