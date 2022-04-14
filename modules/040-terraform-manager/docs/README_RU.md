---
title: "Модуль terraform-manager"
---

Модуль предоставляет инструменты для работы с состоянием Terraform в кластере Kubernetes.

* Модуль состоит из 2-х частей:
  * `terraform-auto-converger` — проверяет состояние Terraform'а и применяет недеструктивные изменения;
  * `terraform-state-exporter` — проверяет состояние Terraform'а и экспортирует метрики кластера.

* Модуль включен по умолчанию, если в кластере есть Secret'ы:
    * `kube-system/d8-provider-cluster-configuration`;
    * `d8-system/d8-cluster-terraform-state`.
