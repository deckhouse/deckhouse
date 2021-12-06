<script type="text/javascript" src='{{ assets["getting-started.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["getting-started-access.js"].digest_path }}'></script>

At this point, you have created a basic **single-master** cluster.

For real-world conditions (production and test environments), you need to add additional nodes to the cluster according to <a href="/en/documentation/v1/modules/040-node-manager/faq.html#how-do-i-automatically-add-a-static-node-to-a-cluster">the documentation</a>.

<blockquote>
<p>If you install Deckhouse for <strong>evaluation purposes</strong> and one node in  the cluster is enough for you, allow Deckhouse components to work on the master node. To do this, remove the taint from the master node by running the following command:</p>
{% snippetcut %}
```bash
kubectl patch nodegroup master --type json -p '[{"op": "remove", "path": "/spec/nodeTemplate/taints"}]'
```
{% endsnippetcut %}
</blockquote>

After that, there will be next actions.
<ul>
<li><strong>Create DNS records</strong> to organize access to the cluster web-interfaces:
  <ul><li>Discover public IP address of the node where the Ingress controller is running.</li>
  <li>If you have the DNS server and you can add a DNS records:
  <ul>
    <li>If your cluster DNS name template is a <a href="https://en.wikipedia.org/wiki/Wildcard_DNS_record">wildcard DNS</a> (e.g. - <code>%s.kube.my</code>), then add a corresponding wildcard A record containing the public IP, you've discovered previously.
    </li>
    <li>If your cluster DNS name template is <strong>NOT</strong> a <a
            href="https://en.wikipedia.org/wiki/Wildcard_DNS_record">wildcard DNS</a> (e.g. - <code>%s-kube.company.my</code>), then add А or CNAME records containing the public IP, you've discovered previously, for the following Deckhouse service DNS names:
          <div class="highlight">
<pre class="highlight">
<code example-hosts>dashboard.example.com
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
  <li><p>If you <strong>don't have a DNS server</strong>: on your PC add static entries (specify your public IP address in the <code>PUBLIC_IP</code>variable) that match the names of specific services to the public IP to the <code>/etc/hosts</code> file for Linux (<code>%SystemRoot%\system32\drivers\etc\hosts</code> for Windows):</p>
{% snippetcut selector="example-hosts" %}
```bash
export PUBLIC_IP="<PUT_PUBLIC_IP_HERE>"
sudo -E bash -c "cat <<EOF >> /etc/hosts
$PUBLIC_IP dashboard.example.com
$PUBLIC_IP deckhouse.example.com
$PUBLIC_IP dex.example.com
$PUBLIC_IP grafana.example.com
$PUBLIC_IP kubeconfig.example.com
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
    generate_password();
    update_parameter('dhctl-user-password-hash', 'password', '<GENERATED_PASSWORD_HASH>', null, null);
    update_parameter('dhctl-user-password-hash', null, '<GENERATED_PASSWORD_HASH>', null, '[user-yml]');
    update_parameter('dhctl-user-password', null, '<GENERATED_PASSWORD>', null, '[user-yml]');
    update_parameter('dhctl-user-password', null, '<GENERATED_PASSWORD>', null, 'code span.c1');
    update_domain_parameters();
});

</script>
