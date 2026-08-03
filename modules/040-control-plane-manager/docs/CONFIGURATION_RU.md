---
title: "Управление control plane: настройки"
---

Некоторые параметры кластера, влияющие на управление control plane, исторически брались из ресурса [ClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration). Предпочтительнее задавать соответствующие настройки в этом ModuleConfig — например, [`kubernetesVersion`](configuration.html#parameters-kubernetesversion). Поля ClusterConfiguration остаются только как устаревший fallback на время миграции.

<!-- SCHEMA -->
