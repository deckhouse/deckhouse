---
title: "The priority-class module"
---

This module creates a set of [priority classes](https://kubernetes.io/docs/concepts/configuration/pod-priority-preemption/#priorityclass) and assigns them to components installed by Deckhouse and applications in the cluster.

[Priority Class](https://kubernetes.io/docs/concepts/configuration/pod-priority-preemption) relates to the scheduler and allows it to schedule a Pod based on its priority (which is defined by the class the Pod belongs to).

Suppose we need to schedule a Pod belonging to the `priorityClassName: production-low` priority class. If the cluster does not have enough resources for this Pod, Kubernetes will start evicting Pods with the lowest priority to deploy our `production-low` Pod.
That is, Kubernetes will first evict all the `priorityClassName: develop` Pods, then proceed to `cluster-low` Pods, and so on.

When setting the priority class, it is crucial to understand what kind of application we have and what environment this application works in. Any `priorityClassName` set to a Pod cannot lower its priority because the scheduler considers Pods without `priority-class` as having the lowest (`develop`) priority.
