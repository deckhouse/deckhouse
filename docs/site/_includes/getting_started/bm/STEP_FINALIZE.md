<script type="text/javascript" src='{{ assets["getting-started.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["getting-started-access.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["bcrypt.js"].digest_path }}'></script>

At this point, you have created a cluster that consists of a **single** master node. Since only a limited set of system components run on the master node by default, <a href="/documentation/latest/modules/040-node-manager/examples.html#adding-a-static-node-to-a-cluster">you have to add</a> at least one worker node to the cluster for the cluster to work properly.

<div class="tabs">
        <a id='tab_layout_worker' href="javascript:void(0)" class="tabs__btn tabs__btn_revision active"
        onclick="openTabAndSaveStatus(event, 'tabs__btn_revision', 'tabs__content_worker', 'block_layout_master');
                 openTabAndSaveStatus(event, 'tabs__btn_revision', 'tabs__content_master', 'block_layout_worker');">
        Adding a worker node to a cluster
        </a>
        <a id='tab_layout_master' href="javascript:void(0)" class="tabs__btn tabs__btn_revision"
        onclick="openTabAndSaveStatus(event, 'tabs__btn_revision', 'tabs__content_master', 'block_layout_worker');
                 openTabAndSaveStatus(event, 'tabs__btn_revision', 'tabs__content_worker', 'block_layout_master');">
        If one master node is enough
        </a>
</div>

<div id="block_layout_master" class="tabs__content_master" style="display: none;">
<p>A single-node cluster may be sufficient for <strong>familiarization purposes</strong>. In this case, you do not need to add more nodes to the cluster. To permit the other Deckhouse components to run on the master node, remove the taint from the master node by running the following command on it:</p>

<div markdown="0">
{% snippetcut %}
```bash
sudo /opt/deckhouse/bin/kubectl patch nodegroup master --type json -p '[{"op": "remove", "path": "/spec/nodeTemplate/taints"}]'
```
{% endsnippetcut %}
</div>

<p>Configure the StorageClass for the <a href="/documentation/v1/modules/031-local-path-provisioner/cr.html#localpathprovisioner">local storage</a> by running the following command on the <strong>master node</strong>:</p>
{% snippetcut %}
```shell
sudo /opt/deckhouse/bin/kubectl create -f - << EOF
apiVersion: deckhouse.io/v1alpha1
kind: LocalPathProvisioner
metadata:
  name: localpath-deckhouse
spec:
  nodeGroups:
  - master
  path: "/opt/local-path-provisioner"
EOF
```
{% endsnippetcut %}

<p>Make the created StorageClass as the default one by adding the <code>storageclass.kubernetes.io/is-default-class='true'</code> annotation:</p>
{% snippetcut %}
```shell
sudo /opt/deckhouse/bin/kubectl annotate sc localpath-deckhouse storageclass.kubernetes.io/is-default-class='true'
```
{% endsnippetcut %}
</div>

<div id="block_layout_worker" class="tabs__content_worker">
<p>Add a new node to the cluster:</p>

<ul>
  <li>
    Start a <strong>new virtual machine</strong> that will become the cluster node.
  </li>
  <li>
  Configure the StorageClass for the <a href="/documentation/v1/modules/031-local-path-provisioner/cr.html#localpathprovisioner">local storage</a> by running the following command on the <strong>master node</strong>:
{% snippetcut %}
```shell
sudo /opt/deckhouse/bin/kubectl create -f - << EOF
apiVersion: deckhouse.io/v1alpha1
kind: LocalPathProvisioner
metadata:
  name: localpath-deckhouse
spec:
  nodeGroups:
  - worker
  path: "/opt/local-path-provisioner"
EOF
```
{% endsnippetcut %}
  </li>
  <li>
  Make the created StorageClass as the default one by adding the <code>storageclass.kubernetes.io/is-default-class='true'</code> annotation:
{% snippetcut %}
```shell
sudo /opt/deckhouse/bin/kubectl annotate sc localpath-deckhouse storageclass.kubernetes.io/is-default-class='true'
```
{% endsnippetcut %}
  </li>
  <li>
    Create a <a href="/documentation/v1/modules/040-node-manager/cr.html#nodegroup">NodeGroup</a> <code>worker</code>. To do so, run the following command on the <strong>master node</strong>:
{% snippetcut %}
```bash
sudo /opt/deckhouse/bin/kubectl create -f - << EOF
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
EOF
```
{% endsnippetcut %}
  </li>
  <li>
    Deckhouse will generate the script needed to configure the prospective node and include it in the cluster. Print its contents in Base64 format (you will need them at the next step):
{% snippetcut %}
```bash
sudo /opt/deckhouse/bin/kubectl -n d8-cloud-instance-manager get secret manual-bootstrap-for-worker -o json | jq '.data."bootstrap.sh"' -r
```
{% endsnippetcut %}
  </li>
  <li>
    On the <strong>virtual machine you have started</strong>, run the following command by pasting the script code from the previous step:
{% snippetcut %}
```bash
echo <Base64-SCRIPT-CODE> | base64 -d | sudo bash
```
{% endsnippetcut %}
  </li>
</ul>

<p>Note that it may take some time to get all Deckhouse components up and running after the installation is complete.</p>
</div>

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

Next, you will need to create an Ingress controller, a Storage Class for data storage, a user to access the web interfaces, and configure the DNS.
<ul><li><p><strong>Setting up an Ingress controller</strong></p>
<p>On the <strong>master node</strong>, create the <code>ingress-nginx-controller.yml</code> file containing the Ingress controller configuration:</p>
  {% snippetcut name="ingress-nginx-controller.yml" selector="ingress-nginx-controller-yml" %}
  {% include_file "_includes/getting_started/{{ page.platform_code }}/partials/ingress-nginx-controller.yml.inc" syntax="yaml" %}
  {% endsnippetcut %}
  <p>Apply it using the following command on the <strong>master node</strong>:</p>
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
