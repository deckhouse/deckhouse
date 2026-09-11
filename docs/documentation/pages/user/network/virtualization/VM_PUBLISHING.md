---
title: "Accessing applications on a virtual machine"
permalink: en/user/network/virtualization/vm-publishing.html
description: "Accessing applications on a virtual machine: Kubernetes services of every type and publishing through Ingress."
search: VM publishing, Service, Ingress, NodePort, LoadBalancer, application access
---

You can reach a virtual machine directly by its IP address, but this approach has limitations. You have to know the address in advance, it can change when the machine is recreated, and you can't reach a group of machines at once. Kubernetes services solve all of these tasks.

A service gives a machine or a group of machines a permanent name that hides their addresses, and distributes requests evenly among them. The name is formed as `<SERVICE_NAME>.<NAMESPACE>.svc.<CLUSTER_NAME>`, and within the same namespace the short form `<SERVICE_NAME>` is enough.

Which service type to choose depends on the task:

- `Headless`: Direct access to specific machines inside the cluster without a single entry point.
- `ClusterIP`: A single internal address with balancing between machines.
- `NodePort`: External access through a port on the cluster nodes.
- `LoadBalancer`: External access through an external load balancer.

{% alert level="info" %}
If a connection to the VM from a cluster node doesn't go through, check the `NetworkPolicy` in the project. The policy may deny traffic to the machine.
{% endalert %}

A machine gets into a service by labels. Assign the machine the label that the service looks for:

{% tabs vm-labels %}

{% tab "Using the CLI" %}

Assign the label with the `d8 k label` command:

```bash
d8 k label vm linux-vm app=nginx
```

Example output:

```console
virtualmachine.virtualization.deckhouse.io/linux-vm labeled
```

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **Projects** tab and select the project you need.
1. Go to **Virtualization** → **Virtual machines**.
1. Select the VM you need from the list and click its name.
1. Go to the **Meta** tab.
1. Click **Add** in the **Labels** or **Annotations** section.
1. In the window that opens, set the key and the value, then press **Enter**.
1. Click the **Save** button that appears.

{% endtab %}

{% endtabs %}

## Headless service

A headless service doesn't allocate an IP address of its own, but returns the addresses of the machines themselves. This way you reach a specific machine by its DNS name without setting up a separate entry point for it. Even for a single machine, this is more convenient than a fixed address, because the name doesn't change when the machine is recreated.

{% tabs svc-headless %}

{% tab "Using the CLI" %}

Create a service with `clusterIP: None`:

```bash
d8 k apply -f - <<EOF
apiVersion: v1
kind: Service
metadata:
  name: http
  namespace: default
spec:
  clusterIP: None
  selector:
    # The label the service uses to select virtual machines.
    app: nginx
EOF
```

After creation, you can reach the machine by the `http.default.svc` name.

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **Projects** tab and select the project you need.
1. Go to **Network** → **Services**.
1. Click **Create**.
1. In the form that opens, enter the service name in the **Name** field.
1. In the **Type** field, select `Headless`.
1. In the **Workload selector** block, mark the virtual machines you need, and their labels go into the service selector.
1. In the **Ports** block, set the **Port** and **Target port** values.
1. Click **Create**.

{% endtab %}

{% endtabs %}

## Service of the ClusterIP type

A service of this type gives the machine application a stable address inside the cluster.

{% tabs svc-clusterip %}

{% tab "Using the CLI" %}

`ClusterIP` is the standard service type that provides an internal IP address for accessing the service inside the cluster. This IP address is used to route traffic between different system components. `ClusterIP` lets virtual machines interact with each other through a predictable and stable IP address, which simplifies internal communication in the cluster.

Here is an example of a `ClusterIP` configuration:

```yaml
d8 k apply -f - <<EOF
apiVersion: v1
kind: Service
metadata:
  name: http
spec:
  selector:
    # The label the service uses to decide which virtual machine to route traffic to.
    app: nginx
EOF
```

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **Projects** tab and select the project you need.
1. Go to **Network** → **Services**.
1. In the window that opens, configure the service.
1. Click **Create**.

{% endtab %}

{% endtabs %}

## Service of the NodePort type

A service of this type opens the machine application on a port of every cluster node.

{% tabs svc-nodeport %}

{% tab "Using the CLI" %}

`NodePort` is an extension of the `ClusterIP` service that provides access to the service through a specified port on all cluster nodes. This makes the service reachable from outside the cluster through the combination of a node IP address and a port.

`NodePort` suits scenarios where you need direct access to the service from outside the cluster without an external load balancer.

Create the following service:

```yaml
d8 k apply -f - <<EOF
apiVersion: v1
kind: Service
metadata:
  name: linux-vm-nginx-nodeport
spec:
  type: NodePort
  selector:
    # The label the service uses to decide which virtual machine to route traffic to.
    app: nginx
  ports:
    - protocol: TCP
      port: 80
      targetPort: 80
      nodePort: 31880
EOF
```

![Diagram of accessing a machine application through a NodePort service](../../../images/virtualization/lb-nodeport.png)

In this example, a service of the `NodePort` type is created, which opens external port 31880 on all nodes of your cluster. This port routes incoming traffic to internal port 80 of the virtual machine where the Nginx application runs.

If you don't specify the `nodePort` value explicitly, an arbitrary port is assigned to the service, and you can see it in the service status right after creation.

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **Projects** tab and select the project you need.
1. Go to **Network** → **Services**.
1. Click **Create**.
1. In the form that opens, enter the service name in the **Name** field.
1. In the **Type** field, select `NodePort`.
1. In the **Workload selector** block, mark the virtual machines you need.
1. In the **Ports** block, set the **Port**, **Target port**, and, if required, **Node port** values.
1. Click **Create**.

{% endtab %}

{% endtabs %}

## Service of the LoadBalancer type

A service of this type gives the application an external address through a load balancer.

{% tabs svc-lb %}

{% tab "Using the CLI" %}

`LoadBalancer` is a service type that automatically creates an external load balancer with a permanent IP address. This balancer distributes incoming traffic among virtual machines, making the service available from the internet.

```yaml
d8 k apply -f - <<EOF
apiVersion: v1
kind: Service
metadata:
  name: linux-vm-nginx-lb
spec:
  type: LoadBalancer
  selector:
    # The label the service uses to decide which virtual machine to route traffic to
    app: nginx
  ports:
    - protocol: TCP
      port: 80
      targetPort: 80
EOF
```

![Diagram of accessing a machine application through a LoadBalancer service](../../../images/virtualization/lb-loadbalancer.png)

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **Projects** tab and select the project you need.
1. Go to **Network** → **Services**.
1. Click **Create**.
1. In the form that opens, enter the service name in the **Name** field.
1. In the **Type** field, select `LoadBalancer`.
1. In the **Workload selector** block, mark the virtual machines you need.
1. In the **Ports** block, set the **Port** and **Target port** values.
1. Click **Create**.
1. The external address of the service is shown in the service list, in the **External IP** column.

{% endtab %}

{% endtabs %}

## Publishing VM services with Ingress

Ingress opens the machine application by a domain name and handles TLS termination.

{% tabs svc-ingress %}

{% tab "Using the CLI" %}

`Ingress` lets you manage incoming HTTP/HTTPS requests and route them to different servers within your cluster. This is the most suitable method if you want to use domain names and SSL termination to access your virtual machines.

To publish a virtual machine service through `Ingress`, create the following resources:

An internal service to bind with `Ingress`. Example:

```yaml
d8 k apply -f - <<EOF
apiVersion: v1
kind: Service
metadata:
  name: linux-vm-nginx
spec:
  selector:
    # the label the service uses to decide which virtual machine to route traffic to
    app: nginx
  ports:
    - protocol: TCP
      port: 80
      targetPort: 80
EOF
```

And an `Ingress` resource for publishing. Example:

```yaml
d8 k apply -f - <<EOF
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: linux-vm
spec:
  rules:
    - host: linux-vm.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: linux-vm-nginx
                port:
                  number: 80
EOF
```

![Diagram of accessing a machine application through an Ingress](../../../images/virtualization/lb-ingress.png)

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **Projects** tab and select the project you need.
1. Go to **Network** → **Ingresses**.
1. Click **Create**.
1. In the **Create Ingress** form that opens, enter the resource name in the **Name** field, and select the controller class (`spec.ingressClassName`) in the **Ingress Class** field.
1. In the **Rules** block, click **Add rule (host)** and describe the host and the routing paths to the service you need.
1. If HTTPS is required, click **Add certificate** in the **TLS certificates** block and specify the secret with the certificate; set the **Default backend** if required.
1. Click **Create**.

{% endtab %}

{% endtabs %}
