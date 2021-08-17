---
title: "Модуль terraform-manager"
---
## Описание

Модуль предоставляет инструменты для работы с состоянием Terraform в кластере Kubernetes.

* Модуль состоит из 2-х частей:
  * `terraform-auto-converger` - проверяет стейт терраформа и применяет недеструктивные изменения
  * `terraform-state-exporter` - проверяет стейт терраформа и экспортирует метрики кластера

* Модуль включен по умолчанию, если в кластере есть секреты:
    * kube-system/d8-provider-cluster-configuration
    * d8-system/d8-cluster-terraform-state
