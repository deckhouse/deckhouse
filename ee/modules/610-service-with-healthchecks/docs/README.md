---
title: "The service-with-healthchecks module"
description: "Readiness Probes with the service-with-healthchecks Module in Deckhouse Kubernetes Platform"
---

In Kubernetes, traffic is directed only to available pods, achieved through resources like `Service` and `EndpointSlices`. This ensures high availability and reliability of the application, as only pods ready to handle requests receive traffic.

The availability of a pod in Kubernetes is determined by a check known as a readiness probe. This probe serves as a mechanism that allows Kubernetes to ascertain whether a pod is ready to accept traffic.

### What is a readiness probe?

- **Purpose**: The primary goal of a readiness probe is to ensure that the application inside the pod is ready to serve requests. If the probe fails, Kubernetes will not route traffic to that pod, helping to avoid errors and ensuring more stable application performance.

- **Configuration**: A readiness probe is configured through the pod specification. It can be implemented as an HTTP request, TCP connection, or command-line check (exec). For example, an HTTP probe sends a request to a specific URL, and if the response is successful (usually a 200 code), the pod is considered "ready."

- **Behavior**: If the readiness probe fails, the pod will be marked as "not ready," and all requests to the service will be redirected to other "live" pods. Once the pod successfully passes the check, it will be available for traffic again.

- **Importance**: The use of readiness probes is critical for maintaining high availability and optimal load distribution among pods. This is especially important in distributed systems where the state of applications can change dynamically.

Thus, readiness probes play a key role in traffic management in Kubernetes — requests are processed only by available pods.

There are scenarios where pods contain multiple processes, each capable of handling traffic independently, similar to virtual machines. In such cases, a single readiness probe is insufficient to determine the availability of all processes.

The service-with-healthchecks module provides the ability to independently check the availability of each process and, based on the results of the new set of probes, direct traffic to the appropriate instances. This is useful, for example, for complex multi-container applications in Kubernetes where a "common" health check is defined for multiple containers in a pod.

A new load-balancing method can be configured using the `ServiceWithHealthchecks` resource:
- Its specification is identical to `Service`, with the addition of a `healthchecks` section that contains a set of new checks.
- When creating this resource, child `Service` objects (without a selector) and corresponding `EndpointSlices` are created.
- Availability checks for the target processes are performed by agents deployed on each node.
