---
title: "Модуль terraform-manager"
---
## Описание

Модуль предоставляет инструменты для работы с состоянием Terraform в кластере Kubernetes.

* Модуль состоит из 2-х частей:
  * `terraform-auto-converger` - проверяет стейт терраформа и применяет недеструктивные изменения
  * `terraform-state-exporter` - проверяет стейт терраформа и экспортирует метрики кластера

* У модуля есть следующие настройки:
  
  * `autoConvergerEnabled: true/false` - отключает авто применение состояния терраформа.
    * По умолчанию: `true` - включен
  * `autoConvergerPeriod: interval (например: 5s, 10m5s 1h30m30s)` - через какой промежуток времени проверять стейт терраформа и применять его. 
    * По умолчанию: `1h` - 1 час
  
* Модуль включен по умолчанию, если в кластере есть секреты:
    * kube-system/d8-provider-cluster-configuration
    * d8-system/d8-cluster-terraform-state

  Для отключения модуля добавьте в конфигурацию Deckhouse:
  ```yaml
  terraformManagerEnabled: "false"
  ```
