---
title: "Модуль terraform-manager"
description: Описание модуля terraform-manager Deckhouse. Модуль следит за приведением объектов в кластере к состоянию, описанному в Terraform state.   
---

Модуль отвечает за отслеживание и синхронизацию состояния базовой инфраструктуры и постоянных узлов в облачной среде.
Реализация основана на Terraform и применяется в Deckhouse Kubernetes Platform для взаимодействия с поддерживаемыми облачными провайдерами.

* Модуль состоит из двух частей:
  * `terraform-auto-converger` — проверяет состояние Terraform и применяет недеструктивные изменения;
  * `terraform-state-exporter` — проверяет состояние Terraform и экспортирует метрики кластера.

* Модуль включен по умолчанию, если в кластере есть секреты:
  * `kube-system/d8-provider-cluster-configuration`;
  * `d8-system/d8-cluster-terraform-state`.
