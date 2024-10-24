---
title: "The service-with-healthchecks module"
description: "Readiness Probes with the service-with-healthchecks Module in Deckhouse Kubernetes Platform"
---

In Kubernetes, traffic is only routed to pods that are up and running using `Service` and `EndpointSlice` resources. This ensures high availability of the application, as requests are only handled by the pods that are ready to receive them.

Kubernetes finds out if a pod is available and ready to receive traffic by using a check known as a readiness probe.

### What is a readiness probe?

- **Purpose**: The primary goal of a readiness probe is to ensure that the application running in the pod is ready to process requests. If the probe fails, Kubernetes will not route traffic to that pod. This helps to avoid errors and increases the application stability.

- **Configuration**: A readiness probe is configured via the pod specification. It can take the form of an HTTP request, an attempt to establish a TCP connection, or a command line (exec) check. For example, an HTTP probe sends a request to a specific URL, and if the response is successful (usually HTTP code 200 OK), the pod is considered "ready".

- **Behavior**: If the readiness probe fails, the pod will be marked as "not ready," and all requests to the service will be rerouted to other running pods. Once the pod successfully passes the check, it will be able to receive traffic again.

- **Importance**: Readiness probes are critical for maintaining high availability and optimal load distribution among pods. This is especially important in distributed systems where application state is prone to rapid changes.

Thus, readiness probes play a pivotal role in traffic management in Kubernetes, as requests can only be processed by pods for which the probes have completed successfully, which means they are running normally.

There are scenarios in which multiple processes are running in the same pod. Each of these processes can handle traffic independently, just like virtual machines. In such cases, a single availability probe is not sufficient to determine the availability of all processes.

The `service-with-healthchecks` module allows you to independently check the availability of multiple processes and, based on the results of a new set of probes, route traffic to the appropriate instances. This is useful, for example, for complex multi-container applications in Kubernetes, where a "common" healthcheck is defined for multiple containers in a pod.

You can configure the new load balancing method using the [ServiceWithHealthchecks](cr.html#servicewithhealthchecks) resource:
- Its specification is similar to that of a `Service`; only the `healthchecks` section has been added.
- When this resource is created, child `Service` objects (without a selector) and the corresponding `EndpointSlices` are created as well.
- When creating this resource, child `Service` objects (without a selector) and corresponding `EndpointSlices` are created.
- Availability checks for the target processes are performed by agents deployed on each node.
