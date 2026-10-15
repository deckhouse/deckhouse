<script type="text/javascript" src='{% javascript_asset_tag getting-started %}[_assets/js/getting-started.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag getting-started-access %}[_assets/js/getting-started-access.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag bcrypt %}[_assets/js/bcrypt.js]{% endjavascript_asset_tag %}'></script>

At this point, you have created a cluster consisting of a **single node** — the master node. By default, only a limited set of system components runs on the master node. To ensure the full functionality of the cluster, you need to either add at least one worker node to the cluster or allow the remaining Deckhouse components to run on the master node.

Select one of the two options below to continue installing the cluster:

<div class="tabs-block">
  <ul class="tabs__container tabs__container--title">
    <li id='tab_layout_worker' href="javascript:void(0)" class="tabs__item tabs__item--title active"
    onclick="openTabAndSaveStatus(event,'tabs__item--title','tabs__container--descr','block_layout_worker');">
    A cluster of several nodes
    </li>
    <li id='tab_layout_master' href="javascript:void(0)" class="tabs__item tabs__item--title"
    onclick="openTabAndSaveStatus(event,'tabs__item--title','tabs__container--descr','block_layout_master');">
    A cluster of a single node
    </li>
  </ul>

<div id="block_layout_master" class="tabs__container tabs__container--descr">
<p>A single-node cluster may be sufficient, for example, for familiarization purposes.</p>
<ul>
  <li>
<p>Run the following command on the <strong>master node</strong>, to remove the taint from the master node and permit the other Deckhouse components to run on it:</p>

<div markdown="1">
```bash
sudo -i d8 k patch nodegroup master --type json -p '[{"op": "remove", "path": "/spec/nodeTemplate/taints"}]'
```
</div>
  </li>
  <li>
<p>Configure the StorageClass for the <a href="/modules/local-path-provisioner/cr.html#localpathprovisioner">local storage</a> by running the following command on the <strong>master node</strong>:</p>
<div markdown="1">
```shell
sudo -i d8 k create -f - << EOF
apiVersion: deckhouse.io/v1alpha1
kind: LocalPathProvisioner
metadata:
  name: localpath
spec:
  path: "/opt/local-path-provisioner"
  reclaimPolicy: Delete
EOF
```
</div>
  </li>
  <li>
<p>Make the created StorageClass as the default one in the cluster:</p>
<div markdown="1">
```shell
sudo -i d8 k patch mc global --type merge -p "{\"spec\": {\"settings\":{\"defaultClusterStorageClass\":\"localpath\"}}}"
```
</div>
  </li>
</ul>
</div>

<div id="block_layout_worker" class="tabs__container tabs__container--descr active">
<p>Add a new node to the cluster (for more information, see <a href="/modules/node-manager/examples.html#adding-a-static-node-to-a-cluster">adding a static node to a cluster</a>):</p>

<ul>
  <li>
    Start a <strong>new virtual machine</strong> that will become the cluster node.
  </li>
  <li>
  Configure the StorageClass for the <a href="/modules/local-path-provisioner/cr.html#localpathprovisioner">local storage</a> by running the following command on the <strong>master node</strong>:
<div markdown="1">
```shell
sudo -i d8 k create -f - << EOF
apiVersion: deckhouse.io/v1alpha1
kind: LocalPathProvisioner
metadata:
  name: localpath
spec:
  path: "/opt/local-path-provisioner"
  reclaimPolicy: Delete
EOF
```
</div>
  </li>
  <li>
  <p>Make the created StorageClass as the default one in the cluster:</p>
<div markdown="1">
```shell
sudo -i d8 k patch mc global --type merge -p "{\"spec\": {\"settings\":{\"defaultClusterStorageClass\":\"localpath\"}}}"
```
</div>
  </li>
  <li>
    <p>Create a <a href="/modules/node-manager/cr.html#nodegroup">NodeGroup</a> <code>worker</code>. To do so, run the following command on the <strong>master node</strong>:</p>
<div markdown="1">
```bash
sudo -i d8 k create -f - << EOF
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
  staticInstances:
    count: 1
    labelSelector:
      matchLabels:
        role: worker
EOF
```
</div>
  </li>
  <li>
    <p>Generate a new SSH key with an empty passphrase. To do so, run the following command on the <strong>master node</strong>:</p>
<div markdown="1">
```bash
ssh-keygen -t ed25519 -f /dev/shm/caps-id -C "" -N ""
```
</div>
  </li>
  <li>
    <p>Create an <a href="/modules/node-manager/cr.html#sshcredentials">SSHCredentials</a>. To do so, run the following command on the <strong>master node</strong>:</p>
<div markdown="1">
```bash
sudo -i d8 k create -f - <<EOF
apiVersion: deckhouse.io/v1alpha2
kind: SSHCredentials
metadata:
  name: caps
spec:
  user: caps
  privateSSHKey: "`cat /dev/shm/caps-id | base64 -w0`"
EOF
```
</div>
  </li>
  <li>
    <p>Print the public part of the previously generated SSH key (you will need it in the next step). To do so, run the following command on the <strong>master node</strong>:</p>
<div markdown="1">
```bash
cat /dev/shm/caps-id.pub
```
</div>
  </li>
  <li>
    <p>Create the <code>caps</code> user on the <strong>virtual machine you have started</strong>. To do so, run the following command, specifying the public part of the SSH key obtained in the previous step:</p>
<div markdown="1">
```bash
# Specify the public part of the user SSH key.
export KEY='<SSH_PUBLIC_KEY>'
useradd -m -s /bin/bash caps
usermod -aG sudo caps
echo 'caps ALL=(ALL) NOPASSWD: ALL' | sudo EDITOR='tee -a' visudo
mkdir /home/caps/.ssh
echo $KEY >> /home/caps/.ssh/authorized_keys
chown -R caps:caps /home/caps
chmod 700 /home/caps/.ssh
chmod 600 /home/caps/.ssh/authorized_keys
```
</div>
  </li>
  <li>
    <p>Create a <a href="/modules/node-manager/cr.html#staticinstance">StaticInstance</a> for the node to be added. To do so, run the following command on the <strong>master node</strong> (specify IP address of the node):</p>
<div markdown="1">
```bash
# Specify the IP address of the node you want to connect to the cluster.
export NODE=<NODE_IP_ADDRESS>
sudo -i d8 k create -f - <<EOF
apiVersion: deckhouse.io/v1alpha2
kind: StaticInstance
metadata:
  name: d8cluster-worker
  labels:
    role: worker
spec:
  address: "$NODE"
  credentialsRef:
    kind: SSHCredentials
    name: caps
EOF
```
</div>
  </li>
  <li><p>If you have added additional nodes to the cluster, ensure they are <code>Ready</code>.</p>
<p>On the <strong>master node</strong>, run the following command to get nodes list:</p>
<div markdown="1">
```shell
sudo -i d8 k get no
```
</div>

{% offtopic title="Example of the output..." %}
```console
sudo -i d8 k get no
NAME               STATUS   ROLES                  AGE    VERSION
d8cluster          Ready    control-plane,master   30m   v1.33.13
d8cluster-worker   Ready    worker                 10m   v1.33.13
```
{%- endofftopic %}
</li>
</ul>
</div>
</div>

<p>Note that it may take some time to get all Deckhouse components up and running after the installation is complete.</p>

<ul>
<li><p>Make sure the Kruise controller manager is <code>Ready</code> before continuing.</p>
<p>On the <strong>master node</strong>, run the following command:</p>

<div markdown="1">
```shell
sudo -i d8 k -n d8-ingress-nginx get po -l app=kruise
```
</div>

{% offtopic title="Example of the output..." %}
```console
$ sudo -i d8 k -n d8-ingress-nginx get po -l app=kruise
NAME                                         READY   STATUS    RESTARTS    AGE
kruise-controller-manager-7dfcbdc549-b4wk7   3/3     Running   0           15m
```
{%- endofftopic %}
</li></ul>

Next, you will need to create an Ingress controller, a user to access the web interfaces, and configure the DNS.
<ul><li><p><strong>Setting up an Ingress controller</strong></p>
<p>On the <strong>master node</strong>, create the <code>ingress-nginx-controller.yml</code> file containing the Ingress controller configuration:</p>
<div markdown="1">
{% include_file "_includes/getting_started/{{ page.platform_code }}/partials/ingress-nginx-controller.yml.inc" syntax="yaml" %}
</div>
<p>Apply it using the following command on the <strong>master node</strong>:</p>
<div markdown="1">
```shell
sudo -i d8 k create -f $PWD/ingress-nginx-controller.yml
```
</div>

<p>It may take some time to start the Ingress controller after installing Deckhouse. Make sure the Ingress controller has started before continuing (run on the <code>master</code> node):</p>

<div markdown="1">
```shell
sudo -i d8 k -n d8-ingress-nginx get po -l app=controller
```
</div>

<p>Wait for the Ingress controller pods to switch to <code>Ready</code> state.</p>

{% offtopic title="Example of the output..." %}
```console
$ sudo -i d8 k -n d8-ingress-nginx get po -l app=controller
NAME                                       READY   STATUS    RESTARTS   AGE
controller-nginx-r6hxc                     3/3     Running   0          5m
```
{%- endofftopic %}
</li>
<li><p><strong>Create a user</strong> to access the cluster web interfaces</p>
<p>Create on the <strong>master node</strong> the <code>user.yml</code> file containing the user account data and access rights:</p>
<div markdown="1">
{% include_file "_includes/getting_started/stronghold/{{ page.platform_code }}/partials/user.yml.inc" syntax="yaml" %}
</div>
<p>Apply it using the following command on the <strong>master node</strong>:</p>
<div markdown="1">
```shell
sudo -i d8 k create -f $PWD/user.yml
```
</div>
</li>
<li><strong>Create DNS records</strong> to organize access to the cluster web interfaces:
{% alert level="warning" %}The domain in the template must not match the domain from the `clusterDomain` parameter or the internal network service zone. Example: `clusterDomain: cluster.local` (default value), the network service zone is `central1.internal`. In this case, `publicDomainTemplate` cannot be `%s.cluster.local` or `%s.central1.internal`.{% endalert %}

<p>Find out the public IP address of the node where the Ingress controller is running. Configure DNS in one of the following ways:</p>
{% tabs dns_options %}
{% tab "Wildcard DNS record" %}
Use this option if you have a DNS server and your cluster DNS name template is a [wildcard template](https://en.wikipedia.org/wiki/Wildcard_DNS_record) (e.g., `%s.kube.company.my`).

On the DNS server, add a wildcard A record containing this public IP address.
{% endtab %}
{% tab "Individual DNS records" %}
Use this option if you have a DNS server and your cluster DNS name template is **not** a [wildcard template](https://en.wikipedia.org/wiki/Wildcard_DNS_record) (e.g., `%s-kube.company.my`).

On the DNS server, add A or CNAME records containing this public IP address for the DNS names from the list. The names in the list are built from the DNS name template you specified when configuring the installation:

<div class="highlight">
<pre class="highlight">
<code example-hosts>{%- assign domains = site.data.reference.public-domains %}
{%- for domain in domains %}
  {%- if domain.gs %}{{ domain.name }}.example.com
{% endif -%}
{%- endfor %}</code>
</pre>
</div>

{% alert type="info" %}
The list above contains the DNS names that a cluster deployed using this guide requires. The complete list is given in the [“List of Reserved Deckhouse Platform DNS Names”](/products/kubernetes-platform/documentation/latest/reference/public-domains.html) section of the documentation.
{% endalert %}
{% endtab %}
{% tab "Hosts file" %}
Use this option if you don't have a DNS server. On the computer from which you need access to Deckhouse services, add static records to the hosts file (`/etc/hosts` for Linux, `%SystemRoot%\system32\drivers\etc\hosts` for Windows). Each record maps a service name to the public IP address of the node where the Ingress controller is running.

To get the command for adding the records, use the [hosts file data generator](/products/kubernetes-platform/documentation/latest/reference/public-domains.html#hosts-file-data-generator): enter the DNS name template and the public IP address of the node where the Ingress controller is running.
{% endtab %}
{% endtabs %}

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
