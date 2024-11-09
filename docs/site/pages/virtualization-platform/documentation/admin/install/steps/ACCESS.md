---
title: "Deckhouse Virtualization Platform"
permalink: en/virtualization-platform/documentation/admin/install/steps/access.html
---

## Accessing to the master node

Deckhouse have finished installation process. It remains to make some settings, for which you need to connect to the master node.

Connect to the master node via SSH (the IP address of the master node was printed by the installer upon completion of the installation, but you can also find it using the cloud provider web interface/CLI tool):

```bash
ssh ubuntu@<MASTER_IP>
```

Check the kubectl is working by displaying a list of cluster nodes:

```bash
sudo /opt/deckhouse/bin/kubectl get nodes
```

It may take some time to start the Ingress controller after installing Deckhouse. Make sure that the Ingress controller has started before continuing:

```bash
sudo /opt/deckhouse/bin/kubectl -n d8-ingress-nginx get po
```

Wait for the Pods to switch to `Ready` state.

Also wait for the load balancer to be ready:

```bash
sudo /opt/deckhouse/bin/kubectl -n d8-ingress-nginx get svc nginx-load-balancer
```

The `EXTERNAL-IP` value must be filled with a public IP address or DNS name.

## DNS

To access the web interfaces of Deckhouse services, you need to:

1. Configure DNS.
2. Specify template for DNS names.

The DNS names template is used to configure Ingress resources of system applications. For example, the name `grafana` is assigned to the Grafana interface. Then, for the template `%s.kube.company.my` Grafana will be available at `grafana.kube.company.my`, etc.

The guide will use sslip.io to simplify configuration.

Run the following command on the master node to get the load balancer IP and to configure template for DNS names to use the `sslip.io`:

```bash
BALANCER_IP=$(sudo /opt/deckhouse/bin/kubectl -n d8-ingress-nginx get svc nginx-load-balancer -o json | jq -r '.status.loadBalancer.ingress[0].ip') && \
echo "Balancer IP is '${BALANCER_IP}'." && sudo /opt/deckhouse/bin/kubectl patch mc global --type merge \
  -p "{\"spec\": {\"settings\":{\"modules\":{\"publicDomainTemplate\":\"%s.${BALANCER_IP}.sslip.io\"}}}}" && echo && \
echo "Domain template is '$(sudo /opt/deckhouse/bin/kubectl get mc global -o=jsonpath='{.spec.settings.modules.publicDomainTemplate}')'."
```

The command will also print the DNS name template set in the cluster. Example output:


```bash
Balancer IP is '1.2.3.4'.
moduleconfig.deckhouse.io/global patched

Domain template is '%s.1.2.3.4.sslip.io'.
```

## Configure remote access to the cluster

On a personal computer follow these steps to configure the connection of `kubectl` to the cluster:

- Open Kubeconfig Generator web interface. The name `kubeconfig` is reserved for it, and the address for access is formed according to the DNS names template (which you set up erlier). For example, for the DNS name template `%s.1.2.3.4.sslip.io`, the Kubeconfig Generator web interface will be available at `https://kubeconfig.1.2.3.4.sslip.io`.
- Log in as a user `admin@deckhouse.io`. The user password generated in the previous step is `035hduuvo7` (you can also find it in the `User` CustomResource in the `resource.yml` file).
- Select the tab with the OS of the personal computer.
- Sequentially copy and execute the commands given on the page.
- Check that `kubectl` connects to the cluster (for example, execute the command `kubectl get no`).