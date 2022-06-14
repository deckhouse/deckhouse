---
title: "Monitoring the control plane"
---

The `monitoring-kubernetes-control-plane` module is responsible for monitoring the Kubernetes control plane. It safely scrapes metrics and provides a basic set of rules for monitoring the following cluster components:
* kube-apiserver
* kube-controller-manager
* kube-scheduler
* kube-etcd

There are no standard rules for organizing a cluster. Thus, various components can be configured differently. The control plane monitoring looks for common patterns in the components' configuration and uses them to collect metrics. In the case of some "exotic" pattern, you can [define the configuration manually](configuration.html#parameters).
