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

1. Wait until the platform finishes converging — the task queue must be empty:

   ```shell
   sudo -i d8 system queue list
   ```

   {% offtopic title="Example output" %}
   <!-- markdownlint-disable MD031 -->
   ```console
   Summary:
   - 'main' queue: empty.
   - 132 other queues (0 active, 132 empty): 0 tasks.
   - no tasks to handle.
   ```
   <!-- markdownlint-enable MD031 -->
   {% endofftopic %}

   While the queue has tasks, platform components are still being deployed. This is normal right after installation and may take up to 15 minutes.

1. Make sure every enabled module is running:

   ```shell
   sudo -i d8 k get module | grep -E "True[[:space:]]+False"
   ```

   The command lists modules that are enabled but not ready yet. Empty output means all of them are ready.

1. Make sure the platform is ready to run VMs:

   ```shell
   sudo -i d8 k get vmclass generic
   ```

   Wait until the status is `Ready`.

   {% alert level="info" %}
   On RHEL-based operating systems (AlmaLinux, Rocky Linux, CentOS), the initial node configuration downloads additional packages — while this is in progress, the `generic` class stays in the `Pending` phase.
   {% endalert %}

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

   Configure DNS in one of the following ways:

   {% tabs dns_options %}
   {% tab "Wildcard DNS record" %}
   Use this option if you have a DNS server and your cluster DNS name template is a [wildcard template](https://en.wikipedia.org/wiki/Wildcard_DNS_record) (for example, `%s.domain.my`).

   On the DNS server, add a wildcard A record pointing to the IP address of the Ingress controller node.
   {% endtab %}
   {% tab "Individual DNS records" %}
   Use this option if you have a DNS server and your cluster DNS name template is **not** a [wildcard template](https://en.wikipedia.org/wiki/Wildcard_DNS_record) (for example, `%s-kube.company.my`).

   On the DNS server, add A or CNAME records pointing to the IP address of the Ingress controller node for the following DNS names:

   ```text
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
   prometheus.domain.my
   registry.domain.my
   status.domain.my
   tools.domain.my
   upmeter.domain.my
   virtualization.domain.my
   ```
   {% endtab %}
   {% tab "Hosts file" %}
   Use this option if you don't have a DNS server. Add static records to the hosts file on your **workstation** (`/etc/hosts` for Linux, `%SystemRoot%\system32\drivers\etc\hosts` for Windows).

   To get the command for adding the records, use the [hosts file data generator](/products/kubernetes-platform/documentation/latest/reference/public-domains.html#hosts-file-data-generator): enter the DNS name template and the IP address of the Ingress controller node. The commands above print both values.
   {% endtab %}
   {% endtabs %}

The DVP cluster is deployed and ready to use.
