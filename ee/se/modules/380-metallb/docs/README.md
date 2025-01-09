---
title: "The metallb module"
---

This module implements the `LoadBalancer` mechanism for services in bare-metal clusters.

Supports the following operating modes:

- **Layer 2 Mode (L2 LoadBalancer)** – introduces an improved load-balancing mechanism for bare-metal clusters (compared to the standard L2 mode in MetalLB), enabling the use of multiple IP addresses for cluster services.
- **BGP Mode (BGP LoadBalancer)**  – fully based on the [MetalLB](https://metallb.universe.tf/) solution.

## Layer 2 mode

In Layer 2 mode, one or more nodes take responsibility for providing the service within the local network. From the network’s perspective, it appears as if each of these nodes has multiple IP addresses assigned to its network interface. Technically, this is achieved by the module responding to ARP requests for IPv4 services and NDP requests for IPv6 services. The primary advantage of Layer 2 mode is its versatility: it works in any Ethernet network without requiring specialized hardware.

## Principle of operation compared to L2 mode in MetalLB module

MetalLB in L2 mode allows ordering _Service_ with `LoadBalancer` type, the operation of which is based on the fact that balancing nodes simulate ARP-responses from the "public" IP in a peering network. This mode has a significant limitation — only one balancing node handles all the incoming traffic of this service at a time. Therefore:

- The node selected as the leader for the "public" IP becomes a "bottleneck", with no possibility of horizontal scaling.
- If the balancer node fails, all current connections will be dropped while switching to a new balancing node that will be selected as the leader.

<div data-presentation="../../presentations/metallb/basics_metallb_en.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/18vcVJ1cY2yn19vBM_dTNW3hF0w9SE4S81VZc2P6fVFM/ --->

This module helps to overcome these limitations. It introduces a new resource, _MetalLoadBalancerClass_, which allows associating a group of nodes with an IP address pool using a `nodeSelector`. Afterward, a standard _Service_ resource of type `LoadBalancer` can be created, specifying the name of the corresponding _MetalLoadBalancerClass_. Additionally, annotations can be used to define the required number of IP addresses for L2 advertisement.

<div data-presentation="../../presentations/metallb/basics_metallb_l2balancer_en.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1FYbc7jUhvJFy8x592ihm644i0qpeQSJFUc4Ly2coWFQ/ --->

Thus:

- The application will receive not a single, but several (according to the number of balancer nodes) "public" IPs. These IPs will need to be configured as A-records for the application's public domain. For further horizontal scaling, additional balancer nodes will need to be added, the corresponding _Service_ will be created automatically, you just need to add them to the list of A-records for the application domain.
- If one of the balancer nodes fails, only part of the connections will fail over to the healthy node.

## BGP mode

> Available in Enterprise Edition only.

In BGP mode, each node in your cluster establishes a BGP peering session with your network routers and uses this peering session for advertising the IPs of the external cluster services.
Assuming your routers are configured to support multipath, this enables true load balancing: the routes published by MetalLB are equivalent to each other, except for their nexthop. This means that routers will use all the nexthops together and load balance between them.
Once packets arrive at a node, kube-proxy does the final hop of traffic routing, delivering packets to a particular Pod in the service.

### Load-Balancing Behavior

The exact load balancing behavior depends on your specific router model and configuration, but the general behavior is to balance per-connection based on the packet hash.

Per-connection means that all packets for a single TCP or UDP session will be routed to a single machine in your cluster. The traffic spreading happens only between different connections and not for packets within a single connection. This is good because spreading packets across multiple cluster nodes would result in poor behavior on several levels:

- Spreading the same connection over multiple paths would result in packet reordering on the wire, drastically impacting the end host's performance.
- On-node traffic routing in Kubernetes is not guaranteed to be consistent across nodes. This means that two different nodes may decide to route packets for the same connection to different Pods, resulting in connection failures.

Packet hashing allows high-performance routers to spread connections across multiple backends statelessly. For each packet, they extract some of the fields and use those as a “seed” to deterministically select one of the possible backends. If all the fields are the same, the same backend will be chosen. The exact hashing methods available depend on the router's hardware and software. Two typical options are 3-tuple and 5-tuple hashing. 3-tuple uses the protocol, the source and destination IPs as the key, meaning that all packets between two unique IPs will be routed to the same backend. 5-tuple hashing adds the source and destination ports to the mix, allowing different connections from the same clients to be distributed across the cluster.

In general, it’s preferable to put as much entropy as possible into the packet hash, i.e., using more fields is usually a good thing. This is because increased entropy brings us closer to the “ideal” load-balancing state, where each node receives exactly the same number of packets. We can never achieve that ideal state because of the problems listed above, but at least we can try to spread the connections as evenly as possible to prevent hotspots from forming.

### Limitations

Using BGP as a load-balancing mechanism allows you to use standard router hardware rather than bespoke load balancers. However, it also has its disadvantages.

The biggest one is that BGP-based load balancing does not react gracefully to changes in the backend set for an address. This means that when a cluster node goes down, all the active connections to your service are expected to fail (users will see the “Connection reset by peer” error message).
BGP-based routers implement stateless load balancing. They assign a given packet to a specific next hop by hashing some fields in the packet header and using that hash as an index into the array of available backends.

The problem is that the hashes used in routers are usually not stable, so whenever the size of the backend set changes (e.g., when a node’s BGP session goes down), existing connections will be rehashed effectively at random. That means that most existing connections will suddenly be redirected to a different backend with no knowledge of the connection in question.
The consequence is that whenever the IP→Node mapping gets changed for your service, you should expect to see a one-time hit with the active connections to the service being dropped. There’s no ongoing packet loss or blackholing, just a one-time clean break.

Depending on what your services do, there are several mitigation strategies you can employ:

- Your BGP routers might provide the option to use a more stable ECMP hashing algorithm. This is sometimes called “resilient ECMP” or “resilient LAG”. This algorithm massively reduces the number of affected connections when the backend set gets changed.
- Pin your service deployments to specific nodes to minimize the pool of nodes you must be “careful” about.
- Schedule changes to your service deployments during “trough” times when most users sleep, and traffic is low.
- Split each logical service into two Kubernetes services with different IPs and use DNS to gracefully migrate user traffic from one to the other prior to disrupting the “drained” service.
- Add transparent retry logic on the client side to gracefully recover from sudden disconnections. This works especially well if your clients are things like mobile apps or rich single-page web apps.
- Place your services behind an Ingress controller. The Ingress controller itself can use MetalLB to receive traffic, but having a stateful layer between BGP and your services means you can change your services without concern. You only have to be careful when modifying the deployment of the Ingress controller itself (e.g., when adding more NGINX Pods to scale up).
- Accept that there will be occasional bursts of reset connections. For low-availability internal services, this may be acceptable as-is.
