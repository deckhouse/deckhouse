---
title: "The metallb module"
---

This module implements the `LoadBalancer` mechanism for services in bare metal clusters.

It is based on the [MetalLB](https://metallb.universe.tf/) load balancer implementation.

## Layer 2 mode

In layer 2 mode, one of the nodes is responsible of advertising the service to the local network. From a network perspective, it looks as if the machine has multiple IP addresses assigned to its network interface.
Under the hood, MetalLB responds to ARP requests for IPv4 services and NDP requests for IPv6.
The major advantage of the layer 2 mode is its versatility: it works on any Ethernet network, and no special hardware is required — not even fancy routers.

### Load-Balancing Behavior

In layer 2 mode, all traffic for a service IP goes to a single node. Then kube-proxy distributes it to all service Pods.
As such, layer 2 does not implement load balancing. Rather, it implements a failover mechanism so that a different node can take over should the current leader node fail for some reason.
When a leader node fails for some reason, failover occurs automatically: the failed node gets detected using memberlist, and then the new nodes take over ownership of the IP addresses from the failed one.

### Limitations

Layer 2 mode has two main limitations you should be aware of: 
- **Single-node bottlenecking.**

  In layer 2 mode, a single leader-elected node receives all traffic for the service IP. This means that your service’s Ingress bandwidth is limited to the bandwidth of a single node. This is a fundamental limitation of using ARP and NDP to direct traffic.
- **Potentially slow failover.** 

  In the current implementation, failover between nodes depends on client cooperation. When failover occurs, MetalLB sends some gratuitous layer 2 packets (a bit of a misnomer — it should really be called “unsolicited layer 2 packets”) to notify clients that the MAC address associated with the service IP has changed. Most operating systems correctly handle “gratuitous” packets and promptly update their neighbor caches. In that case, failover occurs within seconds. However, some systems either don’t implement gratuitous handling or have buggy implementations that delay cache updates.

  All modern versions of major OSes (Windows, Mac, Linux) implement layer 2 failover correctly, so problems may only arise in older or less common operating systems. To minimize the impact of a planned failover on buggy clients, you should keep the old leader node running for a couple of minutes after the leader change so that it can continue forwarding traffic to the old clients until their caches are updated. During an unplanned failover, the service IPs will be unreachable until the buggy clients update their cache entries.

### Comparison To Keepalived

MetalLB’s layer 2 mode has a lot in common with Keepalived, so if you’re familiar with Keepalived, all this should sound pretty familiar. However, there are a few differences worth mentioning.

Keepalived relies on the Virtual Router Redundancy Protocol (VRRP). Keepalived instances continuously exchange VRRP messages with each other, both to select a leader and to detect when that leader fails.
On the other hand, MetalLB relies on memberlist as a way to learn that a node in the cluster is no longer reachable and the service IPs from that node should be moved elsewhere.

Still, Keepalived and MetalLB “look” the same from the client’s perspective: the service IP address sort of migrates from one machine to another when a failover occurs, and the rest of the time, it just looks as if machines have multiple IP addresses.
Since it doesn’t use VRRP, MetalLB isn’t subject to some limitations of that protocol. For example, the VRRP limit of 255 load balancers per network doesn’t exist in MetalLB. You can have as many load-balanced IPs as you want as long as there are free IPs in your network. MetalLB is also easier to configure than VRRP — for example, there are no Virtual Router IDs.

The flip side is that MetalLB cannot interoperate with third-party VRRP-aware routers and infrastructure since it relies on memberlist for cluster membership information. That's the idea: MetalLB is purpose-built to provide load balancing and failover within a Kubernetes cluster, and there is no interoperability with third-party LB software in such a scenario.

## BGP mode

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
