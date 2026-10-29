---
title: Установка в существующий кластер
description: Режим установки, в котором Deckhouse Platform работает поверх готового кластера Kubernetes, устанавливает и обновляет платформенные модули, но не управляет операционной системой, control plane и жизненным циклом узлов.
lang: ru
---

В этом режиме существующая инфраструктура продолжает управлять ОС, kubelet, containerd, etcd, control plane и обновлением Kubernetes, существующий CNI отвечает за сеть подов, а машины создаёт и удаляет внешний провайдер или пользователь. [Набор модулей](glossary.html#module-bundle) `Managed` выбирается при установке и впоследствии не меняется. [ClusterConfiguration](reference/api/cr.html#clusterconfiguration) в [файле конфигурации установки](glossary.html#installation-config) не требуется.

Подробнее в разделе [«Установка Deckhouse Platform в существующий кластер Talos»](guides/talos-existing-cluster.html).
