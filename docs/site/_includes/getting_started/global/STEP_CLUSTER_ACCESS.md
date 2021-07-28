<script type="text/javascript" src='{{ assets["getting-started.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["getting-started-access.js"].digest_path }}'></script>

{% if page.platform_type == 'cloud' %}
Get the IP address of the IP load balancer. Run the following command from the root user:
{% if page.platform_code == 'aws' %}
{% snippetcut %}
{% raw %}
```bash
BALANCER_HOSTNAME=$(kubectl -n d8-ingress-nginx get svc -l app=controller,name=nginx \
-o=go-template='{{ (index (index .items 0).status.loadBalancer.ingress 0).hostname }}') ;\
echo "$BALANCER_HOSTNAME"
```
{% endraw %}
{% endsnippetcut %}
{% else %}
{% snippetcut %}
{% raw %}
```bash
BALANCER_IP=$(kubectl -n d8-ingress-nginx get svc -l app=controller,name=nginx \
-o=go-template='{{ (index (index .items 0).status.loadBalancer.ingress 0).ip }}') ;\
echo "$BALANCER_IP"
```
{% endraw %}
{% endsnippetcut %}
{% endif %}
{% endif %}

Point a DNS domain for Deckhouse services you specified in the "[Cluster Installation](./step3.html)" step in one of the following ways:
<div markdown="1">
<ul><li><p>If you have the DNS server and you can add a DNS records, then we recommend using the following option — add
{%- if page.platform_code == 'aws' %} CNAME wildcard record for the <code>*.example.com</code> name and the hostname of the load balancer (<code>BALANCER_HOSTNAME</code>)
{%- else %} wildcard record for the <code>*.example.com</code> and the IP of the load balancer (<code>BALANCER_IP</code>)
{%- endif -%}
, you've got higher.</p></li>
  <li><p>If you don't have a DNS server and want to test Deckhouse services, add static records of matching the names of specific services to the IP address of the load balancer in the file <code>/etc/hosts</code> for Linux (<code>%SystemRoot%\system32\drivers\etc\hosts</code> for Windows).</p>
{% if page.platform_code == 'aws' %}
    <p>You can determine the IP address of the load balancer using the following command (in the cluster):</p>

<div markdown="1">
{% snippetcut %}
```bash
BALANCER_IP=$(dig "$BALANCER_HOSTNAME" +short | head -1); echo "$BALANCER_IP"
```
{% endsnippetcut %}
</div>
{% endif %}

    <p>To add records to the <code>/etc/hosts</code> file locally, for example, follow these steps:</p>

  <ul><li><p>Export the <code>BALANCER_IP</code> variable by specifying the IP address you got:</p>
{% snippetcut %}
```bash
export BALANCER_IP="<PUT_BALANCER_IP_HERE>"
```
{% endsnippetcut %}
    </li>
  <li><p>Add DNS records for the Deckhouse services:</p>
{% snippetcut selector="example-hosts" %}
```bash
sudo -E bash -c "cat <<EOF >> /etc/hosts
$BALANCER_IP dashboard.example.com
$BALANCER_IP deckhouse.example.com
$BALANCER_IP kubeconfig.example.com
$BALANCER_IP grafana.example.com
$BALANCER_IP dex.example.com
EOF
"
```
{% endsnippetcut %}
</li>
</ul></li>
</ul>
</div>
