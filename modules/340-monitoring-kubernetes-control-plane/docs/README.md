---
title: "Monitoring the control plane"
---

The `monitoring-kubernetes-control-plane` module is responsible for monitoring the Kubernetes control plane. It safely scrapes metrics and provides a basic set of rules for monitoring the following cluster components:
* kube-apiserver
* kube-controller-manager
* kube-scheduler
* kube-etcd
