{%- include getting_started/dvp/global/partials/gs_scripts.liquid step='access' -%}

Platform installation is complete. Verify the cluster and configure DNS to access DVP web interfaces.

## Verify the cluster

1. Connect to the **master node**:

   ```shell
   ssh ubuntu@<MASTER_IP>
   ```

1. Verify that all nodes are `Ready`:

   ```shell
   sudo -i d8 k get nodes
   ```

   {% offtopic title="Example output" %}
   <!-- markdownlint-disable MD031 -->
   ```console
   NAME            STATUS   ROLES                  AGE   VERSION
   dvp-master-0    Ready    control-plane,master   30m   v1.29.x
   dvp-worker-1    Ready    worker                 5m    v1.29.x
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->
   {% endofftopic %}

   DVP components may take some time to start after installation.

## Configure DVP web interfaces

Make sure the cluster is healthy and set up DNS so you can open DVP web interfaces from your workstation.

1. On the **master node**, make sure the [`ingress-nginx`](/modules/ingress-nginx/) pods are running:

   ```shell
   sudo -i d8 k -n d8-ingress-nginx get po -l app=kruise
   sudo -i d8 k -n d8-ingress-nginx get po -l app=controller
   ```

   Wait until the Ingress controller pods are `Ready`.

   {% offtopic title="Example output" %}
   <!-- markdownlint-disable MD031 -->
   ```console
   NAME                                         READY   STATUS    RESTARTS   AGE
   kruise-controller-manager-7dfcbdc549-b4wk7   3/3     Running   0          15m

   NAME                   READY   STATUS    RESTARTS   AGE
   controller-nginx-r6hxc   3/3     Running   0          5m
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->
   {% endofftopic %}

1. Configure DNS for DVP web interfaces. The [DNS name template](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate) (`publicDomainTemplate`) defines hostnames for Ingress: for `%s.domain.my`, Grafana is at `grafana.domain.my`, the DVP web interface — at `console.domain.my`.

   {% alert level="warning" %}
   The domain in the template must not match `clusterDomain` (for example `cluster.local`) or internal service zones from your cluster configuration.
   {% endalert %}

   On the **master node**, check the template and the Ingress node IP:

   ```shell
   sudo -i d8 k get mc global -ojsonpath='{.spec.settings.modules.publicDomainTemplate}{"\n"}'
   sudo -i d8 k get pods -n d8-ingress-nginx -o=jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.hostIP}{"\n"}{end}' | grep '^controller' | awk '{print $2}'
   ```

   Add DNS records:

   - **Wildcard template** (for example `%s.domain.my`) — one wildcard A record pointing to the Ingress node IP.
   - **Non-wildcard template** (for example `%s-kube.company.my`) — A or CNAME records for each hostname:

     ```bash
     api.domain.my
     code.domain.my
     commander.domain.my
     console.domain.my
     dex.domain.my
     documentation.domain.my
     grafana.domain.my
     hubble.domain.my
     istio.domain.my
     istio-api-proxy.domain.my
     kubeconfig.domain.my
     openvpn-admin.domain.my
     registry.domain.my
     prometheus.domain.my
     status.domain.my
     tools.domain.my
     ```

   If you do not manage DNS, add static mappings on your **workstation** (on Windows — `%SystemRoot%\system32\drivers\etc\hosts`):

   ```bash
   export PUBLIC_IP="<MASTER_IP>"
   sudo -E bash -c "cat <<EOF >> /etc/hosts
   $PUBLIC_IP api.domain.my
   $PUBLIC_IP code.domain.my
   $PUBLIC_IP commander.domain.my
   $PUBLIC_IP console.domain.my
   $PUBLIC_IP dex.domain.my
   $PUBLIC_IP documentation.domain.my
   $PUBLIC_IP grafana.domain.my
   $PUBLIC_IP hubble.domain.my
   $PUBLIC_IP istio.domain.my
   $PUBLIC_IP istio-api-proxy.domain.my
   $PUBLIC_IP kubeconfig.domain.my
   $PUBLIC_IP openvpn-admin.domain.my
   $PUBLIC_IP registry.domain.my
   $PUBLIC_IP prometheus.domain.my
   $PUBLIC_IP status.domain.my
   $PUBLIC_IP tools.domain.my
   EOF
   "
   ```

The DVP cluster is deployed and ready to use.
