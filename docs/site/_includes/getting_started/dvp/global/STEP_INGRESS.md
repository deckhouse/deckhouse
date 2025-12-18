<script type="text/javascript" src='{% javascript_asset_tag getting-started %}[_assets/js/getting-started.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag getting-started-access %}[_assets/js/getting-started-access.js]{% endjavascript_asset_tag %}'></script>
Make sure the Kruise controller manager is `Running`.
  Run the following command on the **master node**:

```shell
sudo d8 k -n d8-ingress-nginx get po -l app=kruise
```

Set up the Ingress controller and DNS.

<ol>
  <li><p><strong>Setting up an Ingress controller</strong></p>
<div markdown="1">
```shell
sudo d8 k apply -f - <<EOF
# The parameters of the Ingress NGINX Controller.
# https://deckhouse.io/modules/ingress-nginx/cr.html#ingressnginxcontroller
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: nginx
spec:
  ingressClass: nginx
  # The way traffic goes to cluster from the outer network.
  inlet: HostPort
  hostPort:
    httpPort: 80
    httpsPort: 443
  # Describes on which nodes the Ingress Controller will be located.
  # You might consider changing this.
  nodeSelector:
    node-role.kubernetes.io/control-plane: ""
  tolerations:
  - effect: NoSchedule
    key: node-role.kubernetes.io/control-plane
    operator: Exists
EOF
```
</div>
<p>
It may take some time to start the Ingress controller after installing Deckhouse. Make sure the Ingress controller has started before continuing (run on the <code>master</code> node):</p>
<div markdown="1">
```shell
sudo d8 k -n d8-ingress-nginx get po -l app=controller
```
</div>
<p>Wait for the Ingress controller pods to switch to <code>Running</code> state.</p>

{% offtopic title="Example of the output..." %}
```console
$ sudo -i d8 k -n d8-ingress-nginx get po -l app=controller
NAME                                       READY   STATUS    RESTARTS   AGE
controller-nginx-r6hxc                     3/3     Running   0          5m
```
{% endofftopic %}
</li>
<li><strong>Create DNS records</strong> to organize access to the cluster web-interfaces:
  <ul>
  <li>Discover public IP address of the node where the Ingress controller is running.</li>
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
</li>
</ol>
