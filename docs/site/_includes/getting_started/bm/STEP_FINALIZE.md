<script type="text/javascript" src='{% javascript_asset_tag getting-started %}[_assets/js/getting-started.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag getting-started-access %}[_assets/js/getting-started-access.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag bcrypt %}[_assets/js/bcrypt.js]{% endjavascript_asset_tag %}'></script>

At this point, you have created a cluster consisting of a **single node** — the master node. By default, only a limited set of system components runs on the master node. To ensure the full functionality of the cluster, you need to either add at least one worker node to the cluster or allow the remaining Deckhouse components to run on the master node.

Select one of the two options below to continue installing the cluster:

<div class="tabs">
        <a id='tab_layout_worker' href="javascript:void(0)" class="tabs__btn tabs__btn_revision active"
        onclick="openTabAndSaveStatus(event, 'tabs__btn_revision', 'tabs__content_worker', 'block_layout_master');
                 openTabAndSaveStatus(event, 'tabs__btn_revision', 'tabs__content_master', 'block_layout_worker');">
        A cluster of several nodes
        </a>
        <a id='tab_layout_master' href="javascript:void(0)" class="tabs__btn tabs__btn_revision"
        onclick="openTabAndSaveStatus(event, 'tabs__btn_revision', 'tabs__content_master', 'block_layout_worker');
                 openTabAndSaveStatus(event, 'tabs__btn_revision', 'tabs__content_worker', 'block_layout_master');">
        A cluster of a single node
        </a>
</div>

<div id="block_layout_master" class="tabs__content_master" style="display: none;">
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
sudo -i d8 k patch mc global --type merge \
  -p "{\"spec\": {\"settings\":{\"defaultClusterStorageClass\":\"localpath\"}}}"
```
</div>
  </li>
</ul>
</div>

<div id="block_layout_worker" class="tabs__content_worker">
<p>Add a new node to the cluster (for more information about adding a static node to a cluster, read <a href="/modules/node-manager/examples.html#adding-a-static-node-to-a-cluster">the documentation</a>):</p>

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
sudo -i d8 k patch mc global --type merge \
  -p "{\"spec\": {\"settings\":{\"defaultClusterStorageClass\":\"localpath\"}}}"
```
</div>
  </li>
  <li>
    <p>Create a <a href="/modules/node-manager/cr.html#nodegroup">NodeGroup</a> <code>worker</code> and add a node using Cluster API Provider Static (CAPS) or manually using a bootstrap script.</p>
    
<div class="tabs">
        <a id='tab_block_caps' href="javascript:void(0)" class="tabs__btn tabs__btn_caps_bootstrap active"
        onclick="openTabAndSaveStatus(event, 'tabs__btn_caps_bootstrap', 'tabs__caps', 'block_bootstrap');
                 openTabAndSaveStatus(event, 'tabs__btn_caps_bootstrap', 'tabs__bootstrap', 'block_caps');">
        CAPS
        </a>
        <a id='tab_block_bootstrap' href="javascript:void(0)" class="tabs__btn tabs__btn_caps_bootstrap"
        onclick="openTabAndSaveStatus(event, 'tabs__btn_caps_bootstrap', 'tabs__bootstrap', 'block_caps');
                 openTabAndSaveStatus(event, 'tabs__btn_caps_bootstrap', 'tabs__caps', 'block_bootstrap');">
        Bootstrap script
        </a>
</div>

  <div id="block_bootstrap" class="tabs__bootstrap" style="display: none;">
  <ul>
  <li><p>Create a NodeGroup <code>worker</code>, by running the following command on the <strong>master node</strong>:</p>
<div markdown="1">
```bash
sudo -i d8 k create -f - << EOF
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
EOF
```
</div>
  </li>
  <li><p>Get the script code for adding and configuring a node in Base64 encoding.</p>
  <p>To do so, run the following command on the <strong>master node</strong>:</p>
<div markdown="1">
```shell
export NODE_GROUP=worker
sudo -i d8 k -n d8-cloud-instance-manager get secret manual-bootstrap-for-${NODE_GROUP} -o json | jq '.data."bootstrap.sh"' -r
```
</div>
  </li>
  <li><p>On the <strong>prepared virtual machine</strong>, run the following command, inserting the Base64-encoded script code obtained in the previous step:</p>
<div markdown="1">
```shell
echo <Base64-CODE> | base64 -d | bash
```
  </div>
  </li>
  </ul>
  </div>
  <div id="block_caps" class="tabs__caps">
  <ul>
<li><p>Create a NodeGroup <code>worker</code>, by running the following command on the <strong>master node</strong>:</p>
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
ssh-keygen -t rsa -f /dev/shm/caps-id -C "" -N ""
```
</div>
  </li>
  <li>
    <p>Create an <a href="/modules/node-manager/cr.html#sshcredentials">SSHCredentials</a> resource in the cluster. To do so, run the following command on the <strong>master node</strong>:</p>
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
{% offtopic title="If you are using CentOS or Rocky Linux…" %}
In RHEL-based (Red Hat Enterprise Linux) operating systems, the caps user must be added to the wheel group. To do this, run the following command, specifying the public part of the SSH key obtained in the previous step:
<div markdown="1">
```bash
# Specify the public part of the user SSH key.
export KEY='<SSH-PUBLIC-KEY>'
useradd -m -s /bin/bash caps
usermod -aG wheel caps
echo 'caps ALL=(ALL) NOPASSWD: ALL' | sudo EDITOR='tee -a' visudo
mkdir /home/caps/.ssh
echo $KEY >> /home/caps/.ssh/authorized_keys
chown -R caps:caps /home/caps
chmod 700 /home/caps/.ssh
chmod 600 /home/caps/.ssh/authorized_keys
```
</div>
Next, go to the next step, **you do not need to run the command below**.
{% endofftopic %}
<div markdown="1">
```bash
# Specify the public part of the user SSH key.
export KEY='<SSH-PUBLIC-KEY>'
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
export NODE=<NODE-IP-ADDRESS>
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
  </ul>
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
```
sudo -i d8 k get no
NAME               STATUS   ROLES                  AGE    VERSION
d8cluster          Ready    control-plane,master   30m   v1.23.17
d8cluster-worker   Ready    worker                 10m   v1.23.17
```
{%- endofftopic %}
</li>
</ul>
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
```
$ sudo -i d8 k -n d8-ingress-nginx get po -l app=kruise
NAME                                         READY   STATUS    RESTARTS    AGE
kruise-controller-manager-7dfcbdc549-b4wk7   3/3     Running   0           15m
```
{%- endofftopic %}
</li></ul>

Next, you will need to create an Ingress controller, a user to access the web interfaces, and configure the DNS.
<ul><li><p><strong>Setting up an Ingress controller</strong></p>
<p>On the <strong>master node</strong>, create the <code>ingress-nginx-controller.yml</code> file containing the Ingress controller configuration:</p>

{% capture includePath %}_includes/getting_started/{{ page.platform_code }}/partials/ingress-nginx-controller.yml.inc{% endcapture %}
<div markdown="1">
{% include_file "{{ includePath }}" syntax="yaml" %}
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

Wait for the Ingress controller pods to switch to <code>Ready</code> state.

{% offtopic title="Example of the output..." %}
```
$ sudo -i d8 k -n d8-ingress-nginx get po -l app=controller
NAME                                       READY   STATUS    RESTARTS   AGE
controller-nginx-r6hxc                     3/3     Running   0          5m
```
{%- endofftopic %}
</li>
<li><p><strong>Create a user</strong> to access the cluster web interfaces</p>
<p>Create on the <strong>master node</strong> the <code>user.yml</code> file containing the user account data and access rights:</p>

{% capture includePath %}_includes/getting_started/{{ page.platform_code }}/partials/user.yml.inc{% endcapture %}
<div markdown="1">
{% include_file "{{ includePath }}" syntax="yaml" %}
</div>
<p>Apply it using the following command on the <strong>master node</strong>:</p>
<div markdown="1">
```shell
sudo -i d8 k create -f $PWD/user.yml
```
</div>
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
      <li><strong>Important:</strong> The domain used in the template should not match the domain specified in the clusterDomain parameter and the internal service network zone. For example, if clusterDomain is set to <code>cluster.local</code> (the default value) and the service network zone is <code>ru-central1.internal</code>, then publicDomainTemplate cannot be <code>%s.cluster.local</code> or <code>%s.ru-central1.internal</code>.
      </li>
    </ul>
  </li>
  <li><p>If you <strong>don't have a DNS server</strong>: on your PC add static entries (specify your public IP address in the <code>PUBLIC_IP</code>variable) that match the names of specific services to the public IP to the <code>/etc/hosts</code> file for Linux (<code>%SystemRoot%\system32\drivers\etc\hosts</code> for Windows):</p>
<div markdown="1">
```bash
export PUBLIC_IP="<PUT_PUBLIC_IP_HERE>"
sudo -E bash -c "cat <<EOF >> /etc/hosts
$PUBLIC_IP api.example.com
$PUBLIC_IP argocd.example.com
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
</div>
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
