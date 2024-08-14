---
title: "The service-with-healthchecks module"
---

In kubernetes, traffic is directed only to "ready" Pods (using Service and EndpointSlices), while "readiness" is determined by only one for the whole under test - readiness-probe of the pod.

There are options when the pods contain several processes (like VMs) and each of them can handle traffic independently. In this case, one readiness-probe is not enough.

This module implements the ability to independently check the availability of each process and, based on the results of a new set of probes, direct traffic to them.

You can set up a new balancing method using the ServiceWithHealthchecks resource:
- its specification is identical to Service with the addition of the healthchecks section, which contains a new set of probes.
- when creating this resource, child Services (without selector) and corresponding EndpointSlices are created.
- agents deployed on each node directly check the availability of target processes.
