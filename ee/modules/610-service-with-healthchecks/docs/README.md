---
title: "The service-with-healthchecks module"
description: "Readiness Probes with the service-with-healthchecks Module in Deckhouse Kubernetes Platform"
---

## Limitations of the built-in Service load balancer

In Kubernetes, the `Service` resource handles the internal and external load balancing of requests. It routes requests between the application's worker pods and excludes failed instances from load balancing. The readiness probes defined in the specs of the containers running in this pod make sure that the pod can handle incoming traffic.

The built-in Service load balancer is suitable for most cloud application tasks, but it has two limitations:

* If at least one container in a pod fails the readiness probe, the entire pod is marked as `NotReady` and is excluded from load balancing.
* You can only define one probe for each container, so it is not possible to create independent probes to check, for example, whether reads and writes are available.

The following are examples of scenarios where the capabilities of the regular load balancer are inadequate:

* Database:
  * Runs like a service consisting of three pods, `db-0`, `db-1`, and `db-2`. Each pod contains one container with a running database process.
  * You would like to create two Services, `db-write` for writing and `db-read` for reading.
  * Read queries must be load balanced across all pods.
  * Write queries are only routed to the database's master pod.
* Virtual machine:
  * The pod contains a single container running the `qemu` process, which acts as a hypervisor for the guest virtual machine.
  * The guest virtual machine is running some independent processes, such as a web server and an SMTP server.
  * You would like to create two Services, `web` and `smtp`, and define a separate readiness probe for each service.

## ServiceWithHealthcheck load balancer capabilities

Unlike the regular load balancer, in which readiness probes are tied to the container state, `ServiceWithHealthcheck` allows you to set up active probes on individual TCP ports. In this way, each of the load balancers dealing with the same pod can operate independently of the others.

You can configure this balancing method using the [ServiceWithHealthchecks](cr.html#servicewithhealthchecks) resource:

* Its specification is the same as the regular `Service` except for the `healthcheck` section, which contains a set of probes.
* Currently, three types of probes are supported:
  * `TCP` — a regular probe that establishes a TCP connection.
  * `HTTP` — a probe that sends an HTTP request and waits for a specific response code.
  * `PostgreSQL` — a probe that sends an SQL query and waits for it to complete successfully.

Examples can be found in the [documentation](examples.html).

## How the ServiceWithHealthcheck load balancer works

The load balancer is made up of two components:

* The **controller** runs on the cluster master nodes and manages `ServiceWithHealthcheck` resources,
* The agents operate on each cluster node and carry out probing for pods that run on that node.

The ServiceWithHealthcheck load balancer is designed to be CNI implementation agnostic. It uses the built-in K8s `Service` and `EndpointSlice` resources:

* When creating a `ServiceWithHealthcheck` resource, the controller automatically creates an eponymous Service resource in the same namespace with an empty `selector` field. This prevents the default controller from creating `EndpointSlice`, which are used to configure load balancing.
* When pods subject to `ServiceWithHealthcheck` are scheduled to a particular node, the agent running on that node runs the pre-configured probes and creates an `EndpointSlice` for them with a list of IP addresses and ports to be checked. This `EndpointSlice` is bound to the `Service` child resource created above.
* CNI maps all `EndpointSlice` to the regular services created above and performs load balancing across probed IP addresses and ports on all nodes in the cluster.

Migrating from a Service to a ServiceWithHealthchecks resource, for example within the framework of CI/CD, should not cause difficulties. The ServiceWithHealthchecks specification basically repeats the Service specification, but contains an additional healthchecks section. During the lifecycle of the ServiceWithHealthchecks resource, a service of the same name is created in the same namespace in order to direct traffic to workloads in the cluster in the usual way (kube-proxy or cni).
