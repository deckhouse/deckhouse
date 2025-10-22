---
title: "The metallb module"
description: "Load balancing for services in bare metal clusters."
---

This module implements the `LoadBalancer` mechanism for services in bare metal clusters.

Supports the following operating modes:

- **Layer 2 Mode** – introduces an improved load-balancing mechanism for bare metal clusters (compared to the standard L2 mode in MetalLB), enabling the use of multiple "public" IP addresses for cluster services.
- **BGP Mode**  – fully based on the [MetalLB](https://metallb.universe.tf/) solution.

## Layer 2 mode

In Layer 2 mode, one or more nodes take responsibility for providing the service within the "public" network. From the network’s perspective, it appears as if each of these nodes has multiple IP addresses assigned to its network interface. Technically, this is achieved by the module responding to ARP requests for IPv4 services and NDP requests for IPv6 services. The primary advantage of Layer 2 mode is its versatility: it works in any Ethernet network without requiring specialized hardware.

## Advantages of the module over the classic MetalLB

MetalLB in L2 mode allows ordering Service with `LoadBalancer` type, the operation of which is based on the fact that balancing nodes simulate ARP-responses from the "public" IP in a peering network. This mode has a significant limitation — only one balancing node handles all the incoming traffic of this service at a time. Therefore:

- The node selected as the leader for the "public" IP becomes a "bottleneck", with no possibility of horizontal scaling.
- If the balancer node fails, all current connections will be dropped while switching to a new balancing node that will be selected as the leader.

<div data-presentation="presentations/basics_metallb_en.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/18vcVJ1cY2yn19vBM_dTNW3hF0w9SE4S81VZc2P6fVFM/ --->

This module helps to overcome these limitations. It introduces a new resource, MetalLoadBalancerClass, which allows associating a group of nodes with an IP address pool using a `nodeSelector`. Afterward, a standard Service resource of type `LoadBalancer` can be created, specifying the name of the corresponding MetalLoadBalancerClass. Additionally, annotations can be used to define the required number of IP addresses for L2 advertisement.

<div data-presentation="presentations/basics_metallb_l2balancer_en.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1FYbc7jUhvJFy8x592ihm644i0qpeQSJFUc4Ly2coWFQ/ --->

Thus:

- The application will receive not a single, but several (according to the number of balancer nodes) "public" IPs. These IPs will need to be configured as A-records for the application's public domain. For further horizontal scaling, additional balancer nodes will need to be added, the corresponding Service will be created automatically, you just need to add them to the list of A-records for the application domain.
- If one of the balancer nodes fails, only part of the connections will fail over to the healthy node.

## BGP Mode

{% alert level="warning" %}Available only in Enterprise Edition.{% endalert %}

Metallb in BGP mode provides an efficient and scalable way to expose `LoadBalancer` type Services in Kubernetes clusters running on bare metal. By utilizing the standardized BGP protocol, metallb seamlessly integrates into existing network infrastructure and ensures high availability of Services.

### How metallb Works in BGP Mode

In BGP mode, metallb establishes BGP sessions with routers (or Top-of-Rack switches) and announces the IP addresses of `LoadBalancer` type services to them. This is accomplished as follows:

Configuration.

- Metallb is configured with a pool of IP addresses that it can assign to Services.
- BGP session parameters are defined: the Autonomous System (AS) number of the Kubernetes cluster, the IP addresses of the routers (peers), the AS number of the peers, and optionally, authentication passwords.
- For each IP address pool, specific announcement parameters can be set, such as community strings.

Establishing BGP Sessions.

- On each node of the Kubernetes cluster where metallb is running, the speaker component establishes BGP sessions with the specified routers.
- Routing information is exchanged between metallb and the routers.

Assigning IP Addresses to Services.

- When a Service of type `LoadBalancer` is created, metallb selects a free IP address from the configured pool and assigns it to the Service.
- The controller component tracks changes to Services and manages IP address assignments.

Announcing IP Addresses.

- After an IP address is assigned, the speaker on the node elected as the leader for that Service begins announcing the IP address over the established BGP sessions.
- The routers receive this announcement and update their routing tables, directing traffic for that IP address to the node that announced it.

Traffic Distribution.

- Routers use Equal-Cost Multi-Path (ECMP) or other load balancing algorithms to distribute traffic among nodes announcing the same Service IP address.
- Inside the Kubernetes cluster, traffic arriving at a node is forwarded to the Service's pods using the mechanisms of the employed CNI (iptables/IPVS, eBPF programs, etc.).

### Advantages of Using BGP

- **Standardized Protocol:** BGP is a widely used and well-established routing protocol.
- **Flexibility and Scalability:** BGP allows you to integrate your Kubernetes cluster into your existing network infrastructure and scale your services.
- **Load Distribution:** Using ECMP allows for efficient load distribution across cluster nodes.
- **Fault Tolerance:** If a node announcing an IP address fails, routers automatically redirect traffic to other nodes announcing the same IP.

### Disadvantages of Using BGP

- **Configuration Complexity:** Configuring BGP can be more complex than configuring ARP/NDP announcements (in Layer 2 mode).
- **Network Equipment Requirements:** Routers must support BGP and ECMP.
