---
title: "Мониторинг control plane"
description: "Мониторинг компонентов control plane кластера Deckhouse Platform Certified Security Edition."
---

Мониторинг control plane осуществляется с помощью модуля `monitoring-kubernetes-control-plane`, который организует безопасный сбор метрик и предоставляет базовый набор правил мониторинга следующих компонентов кластера:
* kube-apiserver;
* kube-controller-manager;
* kube-scheduler;
* kube-etcd.
