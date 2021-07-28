<script type="text/javascript" src='{{ assets["getting-started.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["getting-started-access.js"].digest_path }}'></script>

At this point, you have created a basic single-master cluster, and that's enough for evaluation purposes. But, the Deckhouse Platform is designed for real-world conditions, where in addition to the master there are always additional working nodes.

To continue discovering the Deckhouse Platform, you will need to choose one of the approaches:
<ul><li>If a single master is enough for you, execute the command:
{% snippetcut %}
```bash
kubectl patch nodegroup master --type json -p '[{"op": "remove", "path": "/spec/nodeTemplate/taints"}]'
```
{% endsnippetcut %}
</li>
<li>If you need additional nodes, add them to the cluster according to <a href="/en/documentation/v1/modules/040-node-manager/faq.html#how-do-i-automatically-add-a-static-node-to-a-cluster">the documentation</a>) of the node-manager module.
</li></ul>

After that, there will be three more actions.
<ul><li><p>Setup Ingress controller:</p>
  {% snippetcut name="ingress-nginx-controller.yml" selector="ingress-nginx-controller-yml" %}
  {% include_file "_includes/getting_started/{{ page.platform_code }}/partials/ingress-nginx-controller.yml.inc" syntax="yaml" %}
  {% endsnippetcut %}
</li>
<li><p>Create a user to access the cluster web interfaces:</p>
  {% snippetcut name="user.yml" selector="user-yml" %}
  {% include_file "_includes/getting_started/{{ page.platform_code }}/partials/user.yml.inc" syntax="yaml" %}
  {% endsnippetcut %}
</li>
<li>Create DNS records to organize access to the cluster web-interfaces:
  <ul><li>Discover public IP address of the node where the Ingress controller is running.</li>
  <li>If you can add a DNS record using the DNS server, we recommend adding a wildcard record for <code>*.example.com</code> and the public IP.</li>
  <li>If you want to test the cluster, but do not have a DNS server under control, add static entries that match the names of specific services to the public IP to the <code>/etc/hosts</code> file for Linux (<code>%SystemRoot%\system32\drivers\etc\hosts</code> for Windows):
{% snippetcut selector="example-hosts" %}
```bash
export PUBLIC_IP="<PUT_PUBLIC_IP_HERE>"
sudo -E bash -c "cat <<EOF >> /etc/hosts
$PUBLIC_IP dashboard.example.com
$PUBLIC_IP deckhouse.example.com
$PUBLIC_IP kubeconfig.example.com
$PUBLIC_IP grafana.example.com
$PUBLIC_IP dex.example.com
EOF
"
```
{% endsnippetcut %}
</li></ul>
</li>
</ul>

<script type="text/javascript">
$( document ).ready(function() {
   generate_password();
   update_parameter('dhctl-user-password-hash', 'password', '<GENERATED_PASSWORD_HASH>',  null ,null);
   update_parameter('dhctl-user-password-hash', null, '<GENERATED_PASSWORD_HASH>',  null ,'[user-yml]');
   update_parameter('dhctl-user-password', null, '<GENERATED_PASSWORD>',  null ,'[user-yml]');
   update_parameter('dhctl-user-password', null, '<GENERATED_PASSWORD>',  null ,'code span.c1');
   update_parameter((sessionStorage.getItem('dhctl-domain')||'example.com').replace('%s.',''), null, 'example.com',  null ,'[user-yml]');
});

</script>
