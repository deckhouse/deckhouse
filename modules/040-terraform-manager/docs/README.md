---
title: "Модуль terraform-manager"
---
## Описание

Модуль предоставляет инструменты для работы с состоянием Terraform в кластере Kubernetes.

* У модуля нет никаких параметров для настройки.
* Модуль включен по умолчанию, если в кластере есть секреты:
    * kube-system/d8-provider-cluster-configuration
    * d8-system/d8-cluster-terraform-state

  Для отключения модуля добавьте в конфигурацию Deckhouse:
  ```yaml
  terraformManager: "false"
  ```
