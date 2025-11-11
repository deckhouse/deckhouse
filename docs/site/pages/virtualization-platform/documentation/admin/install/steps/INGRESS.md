---
title: "Other settings"
permalink: en/virtualization-platform/documentation/admin/install/steps/ingress.html
---

{% alert level=“info” %}
To run the commands below, you need to have the [d8 utility](/products/kubernetes-platform/documentation/v1/cli/d8/) (Deckhouse CLI) installed and a configured kubectl context for accessing the cluster.
Alternatively, you can connect to the master node via SSH and run the command as the `root` user using `sudo -i`.
{% endalert %}

## Ingress Setup

Ensure that the Kruise controller manager for the [ingress-nginx](/modules/ingress-nginx/) module has started and is in the `Running` status.

```shell
d8 k -n d8-ingress-nginx get po -l app=kruise
```

Create an IngressNginxController resource that describes the parameters for the NGINX Ingress controller:

```yaml
d8 k apply -f - <<EOF
# Section describing the NGINX Ingress controller parameters.
# https://deckhouse.io/modules/ingress-nginx/cr.html#ingressnginxcontroller
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: nginx
spec:
  ingressClass: nginx
  # Method of traffic entry from the outside world.
  inlet: HostPort
  hostPort:
    httpPort: 80
    httpsPort: 443
  # Describes which nodes will host the Ingress controller.
  # You might want to change this.
  nodeSelector:
    node-role.kubernetes.io/control-plane: ""
  tolerations:
  - effect: NoSchedule
    key: node-role.kubernetes.io/control-plane
    operator: Exists
EOF
```

The Ingress controller startup may take some time. Make sure the Ingress controller pods have transitioned to the `Running` status by running the following command:

```shell
d8 k -n d8-ingress-nginx get po -l app=controller
```

{% offtopic title="Example output..." %}

```console
NAME                                       READY   STATUS    RESTARTS   AGE
controller-nginx-r6hxc                     3/3     Running   0          5m
```

{% endofftopic %}

## DNS Configuration

To access the platform's web interfaces, DNS records need to be configured for the cluster's domain.

{% alert level="warning" %}
The domain used in the template must not match the domain specified in the `clusterDomain` parameter or the internal network service zone. For example, if `clusterDomain: cluster.local` (default value) is used, and the network service zone is `central1.internal`, the `publicDomainTemplate` cannot be `%s.cluster.local` or `%s.central1.internal`.
{% endalert %}

### Using a Wildcard Domain

Ensure that the subdomains resolve to the IP address of the node where the Ingress controller is running. In this case, it is `master-0`. Also, verify that the name template matches the format `%s.<domain>`:

```shell
d8 k get mc global -ojson | jq -r '.spec.settings.modules.publicDomainTemplate'
```

{% offtopic title="Example outputs..." %}

Example output if a custom Wildcard domain was used:

```console
%s.my-dvp-cluster.example.com
```

Example output if a domain from the sslip.io service was used:

```console
%s.54.43.32.21.sslip.io
```

{% endofftopic %}

### Using Separate Domains Instead of a Wildcard Domain

If the template uses a non-wildcard domain, you need to manually add additional A or CNAME records pointing to the public IP address of the node where the nginx-controller is running. These records are required for all Deckhouse services.

For example, for the domain `my-dvp-cluster.example.com` and a template with subdomains `%s.my-dvp-cluster.example.com`, the records would look like this:

```console
api.my-dvp-cluster.example.com
console.my-dvp-cluster.example.com
dashboard.my-dvp-cluster.example.com
deckhouse-tools.my-dvp-cluster.example.com
dex.my-dvp-cluster.example.com
documentation.my-dvp-cluster.example.com
grafana.my-dvp-cluster.example.com
hubble.my-dvp-cluster.example.com
kubeconfig.my-dvp-cluster.example.com
openvpn-admin.my-dvp-cluster.example.com
prometheus.my-dvp-cluster.example.com
status.my-dvp-cluster.example.com
upmeter.my-dvp-cluster.example.com
```

For the domain `my-dvp-cluster.example.com` and a template with individual domains `%s-my-dvp-cluster.example.com`, the records would look like this:

```console
api-my-dvp-cluster.example.com
console-my-dvp-cluster.example.com
dashboard-my-dvp-cluster.example.com
deckhouse-tools-my-dvp-cluster.example.com
dex-my-dvp-cluster.example.com
documentation-my-dvp-cluster.example.com
grafana-my-dvp-cluster.example.com
hubble-my-dvp-cluster.example.com
kubeconfig-my-dvp-cluster.example.com
openvpn-admin-my-dvp-cluster.example.com
prometheus-my-dvp-cluster.example.com
status-my-dvp-cluster.example.com
upmeter-my-dvp-cluster.example.com
```

For testing, you can add the necessary records to the `/etc/hosts` file on your local machine (for Windows, the file is located at `%SystemRoot%\system32\drivers\etc\hosts`).

For Linux, you can use the following commands to add the records to the `/etc/hosts` file:

```shell
export PUBLIC_IP="<PUBLIC_IP>"
export CLUSTER_DOMAIN="my-dvp-cluster.example.com"
sudo -E bash -c "cat <<EOF >> /etc/hosts
$PUBLIC_IP api.$CLUSTER_DOMAIN
$PUBLIC_IP console.$CLUSTER_DOMAIN
$PUBLIC_IP dashboard.$CLUSTER_DOMAIN
$PUBLIC_IP deckhouse-tools.$CLUSTER_DOMAIN
$PUBLIC_IP dex.$CLUSTER_DOMAIN
$PUBLIC_IP documentation.$CLUSTER_DOMAIN
$PUBLIC_IP grafana.$CLUSTER_DOMAIN
$PUBLIC_IP hubble.$CLUSTER_DOMAIN
$PUBLIC_IP kubeconfig.$CLUSTER_DOMAIN
$PUBLIC_IP openvpn-admin.$CLUSTER_DOMAIN
$PUBLIC_IP prometheus.$CLUSTER_DOMAIN
$PUBLIC_IP status.$CLUSTER_DOMAIN
$PUBLIC_IP upmeter.$CLUSTER_DOMAIN
EOF
"
```

## Creating a User

To access the cluster's web interfaces, you can create a static user:

1. Generate a password:

   ```shell
   echo -n '<USER-PASSWORD>' | htpasswd -BinC 10 "" | cut -d: -f2 | tr -d '\n' | base64 -w0; echo
   ```

   `<USER-PASSWORD>` — the password to be set for the user.

1. Create the user:

   ```yaml
   d8 k create -f - <<EOF
   ---
   apiVersion: deckhouse.io/v1
   kind: ClusterAuthorizationRule
   metadata:
     name: admin
   spec:
     subjects:
     - kind: User
       name: admin@deckhouse.io
     accessLevel: SuperAdmin
     portForwarding: true
   ---
   apiVersion: deckhouse.io/v1
   kind: User
   metadata:
     name: admin
   spec:
     email: admin@my-dvp-cluster.example.com
     password: '<BASE64 PASSWORD>'

   EOF
   ```

Now you can log in to the cluster web interfaces using your email and password. For further configuration, it is recommended to review the section [Access Control / Role Model](../../platform-management/access-control/role-model.html).

## Enable console module

{% alert level="info" %}
The module is available only to EE edition users.
{% endalert %}

The `console` module allows you to manage virtualization components through the Deckhouse web interface.

To enable the module, run the following command:

```shell
d8 system module enable console
```
