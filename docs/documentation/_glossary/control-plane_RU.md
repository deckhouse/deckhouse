---
title: Control plane
description: Компоненты Kubernetes, которые управляют кластером, такие как kube-apiserver и etcd. В Deckhouse Platform они работают на master-узлах, а в режиме HA с двумя master-узлами кворум etcd обеспечивает arbiter-узел.
lang: ru
---

Deckhouse Platform управляет сертификатами control plane, генерирует конфигурацию и манифесты его компонентов, повышает и понижает их версии, управляет кластером etcd и его участниками, выполняет дефрагментацию etcd и поддерживает файлы kubeconfig. Модуль [`control-plane-manager`](/modules/control-plane-manager/) работает на всех [master-узлах](glossary.html#master-node).

Control plane работает в режиме [с одним master-узлом](glossary.html#single-master) или в [мультимастерном](glossary.html#multi-master) режиме. В [режиме HA](glossary.html#ha-mode) с двумя master-узлами третий экземпляр etcd работает на [arbiter-узле](glossary.html#arbiter-node): на нём размещается только etcd, без остальных компонентов control plane, чтобы кластер etcd сохранял кворум.

Подробнее в разделе [«Общее управление и настройка control plane»](admin/configuration/platform-scaling/control-plane/control-plane-management-and-configuration.html).
