---
title: Статический под
description: Под, который kubelet запускает напрямую из манифеста в директории `/etc/kubernetes/manifests` узла, без участия API Kubernetes. Так в Deckhouse Platform работают компоненты control plane и kubernetes-api-proxy.
lang: ru
---

В Deckhouse Platform kubelet обрабатывает в этой директории только файлы с расширением `.yaml` или `.yml`. Резервные копии, swap-файлы редакторов и другие временные файлы игнорируются. Статическими подами являются компоненты [control plane](glossary.html#control-plane) kube-apiserver, etcd, kube-controller-manager и kube-scheduler, а также [kubernetes-api-proxy](glossary.html#kubernetes-api-proxy).

Подробнее в разделе [«Kubelet»](architecture/kubernetes-and-scheduling/kubelet.html).
