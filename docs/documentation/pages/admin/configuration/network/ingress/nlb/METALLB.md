---
title: "Balancing with MetalLB"
permalink: en/admin/configuration/network/ingress/nlb/metallb.html
description: "Configure MetalLB load balancing in Deckhouse Kubernetes Platform for bare-metal and cloud environments. LoadBalancer service support and IP address management."
---

The [`metallb`](/modules/metallb/) module implements support for
LoadBalancer-type services in Deckhouse Kubernetes Platform (DKP) clusters.
It is suitable for both bare-metal clusters and cloud environments
where built-in load balancers by providers are unavailable.

Two operating modes are supported:

- **Layer 2**: An enhanced version of the standard L2 mode in MetalLB, allowing multiple public addresses to be used for services.
- **BGP**: Fully based on the [MetalLB](https://metallb.io/) solution and available only in DKP Enterprise Edition.

## Layer 2 mode

### How it works

In Layer 2 mode, one or more cluster nodes receive traffic for a service from the public network.
From the network layer's perspective, each of these nodes has multiple IP addresses assigned to its network interface.
This is implemented by the module responding to ARP requests (for IPv4) and NDP requests (for IPv6).

The main advantage of this mode is universality. It works in any Ethernet network without requiring specialized hardware.

### Advantages over classic MetalLB

In classic MetalLB (L2 mode), when creating a LoadBalancer service,
load balancing is achieved by having a single node in the cluster reply to ARP requests for the public IP.
This means:

- Only one node handles all incoming traffic for the given IP at a time.
- The leader node becomes a bottleneck with no horizontal scaling.
- If the leader node fails, all active connections are dropped during the switchover to the new node.

<div data-presentation="../../../../../presentations/metallb/basics_metallb_en.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/18vcVJ1cY2yn19vBM_dTNW3hF0w9SE4S81VZc2P6fVFM/ --->

The `metallb` module removes these limitations. It provides the [MetalLoadBalancerClass](/modules/metallb/cr.html#metalloadbalancerclass) resource, which lets you:

- Bind a group of nodes to an IP address pool using `nodeSelector`.
- Create a standard LoadBalancer Service and specify the name of the target MetalLoadBalancerClass.
- Define the number of IP addresses for L2 announcement via an annotation.

<div data-presentation="../../../../../presentations/metallb/basics_metallb_l2balancer_en.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1FYbc7jUhvJFy8x592ihm644i0qpeQSJFUc4Ly2coWFQ/ --->

This approach means:

- The application receives multiple public IPs, which must be added as A records in DNS.
- To scale, add more load balancer nodes. Associated Services will be created automatically —
  only adding the IPs to the application domain's A records is required.
- If one balancer fails, only part of the traffic is rerouted, without a complete connection drop.

#### Behavior comparison

| Feature                         | Classic MetalLB (L2) | New module with MetalLoadBalancerClass |
|----------------------------------------|----------------------------|----------------------------------------|
| Traffic handling                      | Single node (leader) | Multiple nodes                        |
| Scalability                       | No                        | Yes                                     |
| Fault tolerance                   | All connections dropped      | Part of the traffic is rerouted smoothly     |
| Number of public IPs                | One                       | Multiple (configurable)              |
| DNS configuration                          | One A record              | Multiple A records                    |

### Example of using MetalLB in L2 LoadBalancer mode

1. Enable the [`metallb`](/modules/metallb/) module:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: metallb
   spec:
     enabled: true
     version: 2
   ```

1. Deploy the application to expose:

   ```shell
   d8 k create deploy nginx --image=nginx
   ```

1. Create a [MetalLoadBalancerClass](/modules/metallb/cr.html#metalloadbalancerclass) resource:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: MetalLoadBalancerClass
   metadata:
     name: ingress
   spec:
     addressPool:
       - 192.168.2.100-192.168.2.150
     isDefault: false
     nodeSelector:
       node-role.kubernetes.io/loadbalancer: "" # Load balancer node selector.
     type: L2
   ```

1. Create a Service resource with an annotation and the MetalLoadBalancerClass name:

   ```yaml
   apiVersion: v1
   kind: Service
   metadata:
     name: nginx-deployment
     annotations:
       network.deckhouse.io/l2-load-balancer-external-ips-count: "3"
   spec:
     type: LoadBalancer
     loadBalancerClass: ingress # MetalLoadBalancerClass name.
     ports:
     - port: 8000
       protocol: TCP
       targetPort: 80
     selector:
       app: nginx
   ```

As a result, the LoadBalancer Service will be assigned the configured number of addresses:

```shell
d8 k get svc
```

Example output:

```console
NAME                   TYPE           CLUSTER-IP      EXTERNAL-IP                                 PORT(S)        AGE
nginx-deployment       LoadBalancer   10.222.130.11   192.168.2.100,192.168.2.101,192.168.2.102   80:30544/TCP   11s
```

The obtained `EXTERNAL-IP` can be set as A records for the application domain:

```shell
curl -s -o /dev/null -w "%{http_code}" 192.168.2.100:8000
curl -s -o /dev/null -w "%{http_code}" 192.168.2.101:8000
curl -s -o /dev/null -w "%{http_code}" 192.168.2.102:8000
```

Example output:

```console
200
```

## BGP mode

{% alert level="info" %}
Available in DKP Enterprise Edition only.
{% endalert %}

In BGP mode, [`metallb`](/modules/metallb/) provides LoadBalancer-type services in Kubernetes clusters deployed on physical infrastructure.
Service IP addresses are announced directly to routers (or top-of-rack switches) via the BGP protocol.

### How it works

#### Configuration

- Define an IP address pool available for Service allocation.
- Specify BGP session parameters: the Kubernetes cluster Autonomous System (AS) number, router (peer) IP addresses,
  peer AS numbers, and passwords for authentication (if needed).
- For each IP pool, define specific announcement parameters, such as community strings.

#### Establishing BGP sessions

- On each Kubernetes node running `metallb`, the `speaker` component establishes BGP sessions with the configured routers.
- Route information is exchanged between `metallb` and the routers.

#### Assigning IP addresses to Services

- When a LoadBalancer Service is created, `metallb` selects a free IP address from the configured pool and assigns it to the Service.
- The `controller` component tracks Service changes and manages IP address assignments.

#### Announcing IP addresses

- After assigning an IP address, the `speaker` component on one of the nodes (leader for the Service)
  announces it via the established BGP sessions.
- Routers receive the announcement and update their routing tables to direct traffic to the corresponding node.

#### Traffic distribution

- Routers use Equal-Cost Multi-Path (ECMP) protocols or other load balancing algorithms
  to distribute traffic between nodes announcing the same Service IP.
- Once delivered to the node, incoming traffic is forwarded to the Service pods
  using the CNI mechanisms (iptables/IPVS, eBPF, etc.).

### Benefits of using BGP

- BGP is supported by most network equipment.
- The network can include multiple routers and many nodes.
- Traffic balancing via ECMP.
- If a node stops announcing an IP, routers automatically redirect traffic to other nodes with the same IP.

### Drawbacks of using BGP

- More complex to configure than ARP/GARP announcements (Layer 2 mode).
- Routers must support BGP and ECMP.

### Example of using MetalLB in BGP LoadBalancer mode

1. Enable the [`metallb`](/modules/metallb/) module and configure the required parameters:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: metallb
   spec:
     enabled: true
     settings:
       addressPools:
       - addresses:
         - 192.168.219.100-192.168.219.200
         name: mypool
         protocol: bgp
       bgpPeers:
       - hold-time: 3s
         my-asn: 64600
         peer-address: 172.18.18.10
         peer-asn: 64601
       speaker:
         nodeSelector:
           node-role.deckhouse.io/metallb: ""
     version: 2
   ```

1. Configure BGP peering on the network equipment.
