---
title: "The metallb module"
---

The module implements the `LoadBalancer` mechanism for services in bare metal clusters.

It is based on [MetalLB](https://metallb.universe.tf/) load balancer implementation.

## Layer 2 mode

In layer 2 mode, one node assumes the responsibility of advertising a service to the local network. From the network’s perspective, it simply looks like that machine has multiple IP addresses assigned to its network interface.
Under the hood, MetalLB responds to ARP requests for IPv4 services, and NDP requests for IPv6.
The major advantage of the layer 2 mode is its universality: it will work on any Ethernet network, with no special hardware required, not even fancy routers.

### Load-Balancing Behavior

In layer 2 mode, all traffic for a service IP goes to one node. From there, kube-proxy spreads the traffic to all the service’s pods.
In that sense, layer 2 does not implement a load balancer. Rather, it implements a failover mechanism so that a different node can take over should the current leader node fail for some reason.
If the leader node fails for some reason, failover is automatic: the failed node is detected using memberlist, at which point new nodes take over ownership of the IP addresses from the failed node.

### Limitations

Layer 2 mode has two main limitations you should be aware of: 
- **Single-node bottlenecking.**

  In layer 2 mode a single leader-elected node receives all traffic for a service IP. This means that your service’s Ingress bandwidth is limited to the bandwidth of a single node. This is a fundamental limitation of using ARP and NDP to steer traffic.
- **Potentially slow failover.** 

  In the current implementation, failover between nodes depends on cooperation from the clients. When a failover occurs, MetalLB sends a number of gratuitous layer 2 packets (a bit of a misnomer — it should really be called “unsolicited layer 2 packets”) to notify clients that the MAC address associated with the service IP has changed. Most operating systems handle “gratuitous” packets correctly, and update their neighbor caches promptly. In that case, failover happens within a few seconds. However, some systems either don’t implement gratuitous handling at all, or have buggy implementations that delay the cache update.  All modern versions of major OSes (Windows, Mac, Linux) implement layer 2 failover correctly, so the only situation where issues may happen is with older or less common OSes. To minimize the impact of planned failover on buggy clients, you should keep the old leader node up for a couple of minutes after flipping leadership, so that it can continue forwarding traffic for old clients until their caches refresh. During an unplanned failover, the service IPs will be unreachable until the buggy clients refresh their cache entries.

### Comparison To Keepalived

MetalLB’s layer 2 mode has a lot of similarities to Keepalived, so if you’re familiar with Keepalived, this should all sound fairly familiar. However, there are also a few differences worth mentioning.

Keepalived uses the Virtual Router Redundancy Protocol (VRRP). Instances of Keepalived continuously exchange VRRP messages with each other, both to select a leader and to notice when that leader goes away.
MetalLB on the other hand relies on memberlist to know when a node in the cluster is no longer reachable and the service IPs from that node should be moved elsewhere.

Keepalived and MetalLB “look” the same from the client’s perspective: the service IP address seems to migrate from one machine to another when a failover occurs, and the rest of the time it just looks like machines have more than one IP address.
Because it doesn’t use VRRP, MetalLB isn’t subject to some of the limitations of that protocol. For example, the VRRP limit of 255 load balancers per network doesn’t exist in MetalLB. You can have as many load-balanced IPs as you want, as long as there are free IPs in your network. MetalLB also requires less configuration than VRRP–for example, there are no Virtual Router IDs.

On the flip side, because MetalLB relies on memberlist for cluster membership information, it cannot interoperate with third-party VRRP-aware routers and infrastructure. This is working as intended: MetalLB is specifically designed to provide load balancing and failover within a Kubernetes cluster, and in that scenario interoperability with third-party LB software is out of scope.

## BGP mode

In BGP mode, each node in your cluster establishes a BGP peering session with your network routers, and uses that peering session to advertise the IPs of external cluster services.
Assuming your routers are configured to support multipath, this enables true load balancing: the routes published by MetalLB are equivalent to each other, except for their nexthop. This means that the routers will use all nexthops together, and load balance between them.
After the packets arrive at the node, kube-proxy is responsible for the final hop of traffic routing, to get the packets to one specific pod in the service.

### Load-Balancing Behavior

The exact behavior of the load balancing depends on your specific router model and configuration, but the common behavior is to balance per-connection, based on a packet hash.

Per-connection means that all the packets for a single TCP or UDP session will be directed to a single machine in your cluster. The traffic spreading only happens between different connections, not for packets within one connection. This is a good thing, because spreading packets across multiple cluster nodes would result in poor behavior on several levels:
- Spreading a single connection across multiple paths results in packet reordering on the wire, which drastically impacts performance at the end host.
- On-node traffic routing in Kubernetes is not guaranteed to be consistent across nodes. This means that two different nodes could decide to route packets for the same connection to different Pods, which would result in connection failures.

Packet hashing is how high-performance routers can statelessly spread connections across multiple backends. For each packet, they extract some of the fields, and use those as a “seed” to deterministically pick one of the possible backends. If all the fields are the same, the same backend will be chosen. The exact hashing methods available depend on the router hardware and software. Two typical options are 3-tuple and 5-tuple hashing. 3-tuple uses protocol, source IP and destination IP as the key, meaning that all packets between two unique IPs will go to the same backend. 5-tuple hashing adds the source and destination ports to the mix, which allows different connections from the same clients to be spread around the cluster.

In general, it’s preferable to put as much entropy as possible into the packet hash, meaning that using more fields is generally good. This is because increased entropy brings us closer to the “ideal” load-balancing state, where every node receives exactly the same number of packets. We can never achieve that ideal state because of the problems we listed above, but what we can do is try and spread connections as evenly as possible, to try and prevent hotspots from forming.

### Limitations

Using BGP as a load-balancing mechanism has the advantage that you can use standard router hardware, rather than bespoke load balancers. However, this comes with downsides as well. The biggest downside is that BGP-based load balancing does not react gracefully to changes in the backend set for an address. What this means is that when a cluster node goes down, you should expect all active connections to your service to be broken (users will see “Connection reset by peer”).
BGP-based routers implement stateless load balancing. They assign a given packet to a specific next hop by hashing some fields in the packet header, and using that hash as an index into the array of available backends.
The problem is that the hashes used in routers are usually not stable, so whenever the size of the backend set changes (for example when a node’s BGP session goes down), existing connections will be rehashed effectively randomly, which means that the majority of existing connections will end up suddenly being forwarded to a different backend, one that has no knowledge of the connection in question.
The consequence of this is that any time the IP→Node mapping changes for your service, you should expect to see a one-time hit where most active connections to the service break. There’s no ongoing packet loss or blackholing, just a one-time clean break.

Depending on what your services do, there are a couple of mitigation strategies you can employ:
- Your BGP routers might have an option to use a more stable ECMP hashing algorithm. This is sometimes called “resilient ECMP” or “resilient LAG”. Using such an algorithm hugely reduces the number of affected connections when the backend set changes.
- Pin your service deployments to specific nodes, to minimize the pool of nodes that you have to be “careful” about.
- Schedule changes to your service deployments during “trough”, when most of your users are asleep and your traffic is low.
- Split each logical service into two Kubernetes services with different IPs, and use DNS to gracefully migrate user traffic from one to the other prior to disrupting the “drained” service.
- Add transparent retry logic on the client side, to gracefully recover from sudden disconnections. This works especially well if your clients are things like mobile apps or rich single-page web apps.
- Put your services behind an Ingress controller. The Ingress controller itself can use MetalLB to receive traffic, but having a stateful layer between BGP and your services means you can change your services without concern. You only have to be careful when changing the deployment of the Ingress controller itself (e.g. when adding more NGINX pods to scale up).
- Accept that there will be occasional bursts of reset connections. For low-availability internal services, this may be acceptable as-is.
