<script type="text/javascript" src='{{ assets["getting-started.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["getting-started-access.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["bcrypt.js"].digest_path }}'></script>

At this point, you have created a basic **single-master** cluster.

For real-world conditions (production and test environments), <a href="/documentation/v1/modules/040-node-manager/faq.html#how-do-i-add-a-static-node-to-a-cluster">add nodes</a> to the cluster and familiarize yourself with how to <a href="/documentation/v1/modules/040-node-manager/">manage nodes</a>.

<blockquote>
<p>If you install Deckhouse for <strong>evaluation purposes</strong> and one node in  the cluster is enough for you, allow Deckhouse components to work on the master node. To do this, remove the taint from the master node by running the following command:</p>
{% snippetcut %}
```bash
sudo /opt/deckhouse/bin/kubectl patch nodegroup master --type json -p '[{"op": "remove", "path": "/spec/nodeTemplate/taints"}]'
```
{% endsnippetcut %}
</blockquote>

It may take some time to start Deckhouse components.

Before you go further:
<ul><li><p>If you have added additional nodes to the cluster, ensure they are <code>Ready</code>.</p>
<p>On the <strong>master node</strong>, run the following command to get nodes list:</p>

{% snippetcut %}
```shell
sudo /opt/deckhouse/bin/kubectl get no
```
{% endsnippetcut %}

{% offtopic title="Example of the output..." %}
```
$ sudo /opt/deckhouse/bin/kubectl get no
NAME               STATUS   ROLES                  AGE    VERSION
d8cluster          Ready    control-plane,master   30m   v1.23.17
d8cluster-worker   Ready    worker                 10m   v1.23.17
```
{%- endofftopic %}
</li>
<li><p>Make sure the Kruise controller manager is <code>Ready</code> before continuing.</p>
<p>On the <strong>master node</strong>, run the following command:</p>

{% snippetcut %}
```shell
sudo /opt/deckhouse/bin/kubectl -n d8-ingress-nginx get po -l app=kruise
```
{% endsnippetcut %}

{% offtopic title="Example of the output..." %}
```
$ sudo /opt/deckhouse/bin/kubectl -n d8-ingress-nginx get po -l app=kruise
NAME                                         READY   STATUS    RESTARTS    AGE
kruise-controller-manager-7dfcbdc549-b4wk7   3/3     Running   0           15m
```
{%- endofftopic %}
</li></ul>

After that, creating an Ingress controller, creating a user to access web interfaces, and configuring DNS remains.
<ul><li><p><strong>Setup Ingress controller</strong></p>
<p>On the <strong>master node</strong>, create the <code>ingress-nginx-controller.yml</code> file containing the Ingress controller configuration:</p>
  {% snippetcut name="ingress-nginx-controller.yml" selector="ingress-nginx-controller-yml" %}
  {% include_file "_includes/getting_started/{{ page.platform_code }}/partials/ingress-nginx-controller.yml.inc" syntax="yaml" %}
  {% endsnippetcut %}
  <p>Apply it using the following command on the <strong>master node</strong>>:</p>
{% snippetcut %}
```shell
sudo /opt/deckhouse/bin/kubectl create -f ingress-nginx-controller.yml
```
{% endsnippetcut %}

It may take some time to start the Ingress controller after installing Deckhouse. Make sure the Ingress controller has started before continuing (run on the <code>master</code> node):

{% snippetcut %}
```shell
sudo /opt/deckhouse/bin/kubectl -n d8-ingress-nginx get po -l app=controller
```
{% endsnippetcut %}

Wait for the Ingress controller pods to switch to <code>Ready</code> state.

{% offtopic title="Example of the output..." %}
```
$ sudo /opt/deckhouse/bin/kubectl -n d8-ingress-nginx get po -l app=controller
NAME                                       READY   STATUS    RESTARTS   AGE
controller-nginx-r6hxc                     3/3     Running   0          5m
```
{%- endofftopic %}
</li>
<li><p><strong>Create a user</strong> to access the cluster web interfaces</p>
<p>Create on the <strong>master node</strong> the <code>user.yml</code> file containing the user account data and access rights:</p>
{% snippetcut name="user.yml" selector="user-yml" %}
{% include_file "_includes/getting_started/{{ page.platform_code }}/partials/user.yml.inc" syntax="yaml" %}
{% endsnippetcut %}
<p>Apply it using the following command on the <strong>master node</strong>:</p>
{% snippetcut %}
```shell
sudo /opt/deckhouse/bin/kubectl create -f user.yml
```
{% endsnippetcut %}
</li>
<li><strong>Create DNS records</strong> to organize access to the cluster web-interfaces:
  <ul><li>Discover public IP address of the node where the Ingress controller is running.</li>
  <li>If you have the DNS server and you can add a DNS records:
  <ul>
    <li>If your cluster DNS name template is a <a href="https://en.wikipedia.org/wiki/Wildcard_DNS_record">wildcard DNS</a> (e.g., <code>%s.kube.my</code>), then add a corresponding wildcard A record containing the public IP, you've discovered previously.
    </li>
    <li>If your cluster DNS name template is <strong>NOT</strong> a <a
            href="https://en.wikipedia.org/wiki/Wildcard_DNS_record">wildcard DNS</a> (e.g., <code>%s-kube.company.my</code>), then add A or CNAME records containing the public IP, you've discovered previously, for the following Deckhouse service DNS names:
          <div class="highlight">
<pre class="highlight">
<code example-hosts>api.example.com
argocd.example.com
cdi-uploadproxy.example.com
dashboard.example.com
documentation.example.com
dex.example.com
grafana.example.com
hubble.example.com
istio.example.com
istio-api-proxy.example.com
kubeconfig.example.com
openvpn-admin.example.com
prometheus.example.com
status.example.com
upmeter.example.com</code>
</pre>
        </div>
      </li>
    </ul>
  </li>
  <li><p>If you <strong>don't have a DNS server</strong>: on your PC add static entries (specify your public IP address in the <code>PUBLIC_IP</code>variable) that match the names of specific services to the public IP to the <code>/etc/hosts</code> file for Linux (<code>%SystemRoot%\system32\drivers\etc\hosts</code> for Windows):</p>
{% snippetcut selector="example-hosts" %}
```bash
export PUBLIC_IP="<PUT_PUBLIC_IP_HERE>"
sudo -E bash -c "cat <<EOF >> /etc/hosts
$PUBLIC_IP api.example.com
$PUBLIC_IP argocd.example.com
$PUBLIC_IP cdi-uploadproxy.example.com
$PUBLIC_IP dashboard.example.com
$PUBLIC_IP documentation.example.com
$PUBLIC_IP dex.example.com
$PUBLIC_IP grafana.example.com
$PUBLIC_IP hubble.example.com
$PUBLIC_IP istio.example.com
$PUBLIC_IP istio-api-proxy.example.com
$PUBLIC_IP kubeconfig.example.com
$PUBLIC_IP openvpn-admin.example.com
$PUBLIC_IP prometheus.example.com
$PUBLIC_IP status.example.com
$PUBLIC_IP upmeter.example.com
EOF
"
```
{% endsnippetcut %}
</li></ul>
</li>
</ul>

<script type="text/javascript">
$(document).ready(function () {
    generate_password(true);
    update_parameter('dhctl-user-password-hash', 'password', '<GENERATED_PASSWORD_HASH>', null, null);
    update_parameter('dhctl-user-password-hash', null, '<GENERATED_PASSWORD_HASH>', null, '[user-yml]');
    update_parameter('dhctl-user-password', null, '<GENERATED_PASSWORD>', null, '[user-yml]');
    update_parameter('dhctl-user-password', null, '<GENERATED_PASSWORD>', null, 'code span.c1');
    update_domain_parameters();
    config_highlight();
});

</script>
