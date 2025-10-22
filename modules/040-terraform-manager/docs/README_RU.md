---
title: "Модуль terraform-manager"
description: Описание модуля terraform-manager Deckhouse. Модуль следит за приведением объектов в кластере к состоянию, описанному в Terraform state.   
---

Модуль предоставляет инструменты для работы с состоянием Terraform'а в кластере Kubernetes.

* Модуль состоит из двух частей:
  * `terraform-auto-converger` — проверяет состояние Terraform'а и применяет недеструктивные изменения;
  * `terraform-state-exporter` — проверяет состояние Terraform'а и экспортирует метрики кластера.

* Модуль включен по умолчанию, если в кластере есть Secret'ы:
  * `kube-system/d8-provider-cluster-configuration`;
  * `d8-system/d8-cluster-terraform-state`.
