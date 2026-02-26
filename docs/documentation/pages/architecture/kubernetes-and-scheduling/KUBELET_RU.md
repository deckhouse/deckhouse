---
title: Kubelet
permalink: ru/architecture/kubernetes-and-scheduling/kubelet/
lang: ru
search: kubelet, агент kubelet, архитектура kubelet, взаимодействия kubelet
description: Архитектура и роль kubelet в Deckhouse Kubernetes Platform.
---

Kubelet не является компонентом control plane, но играет ключевую роль в работе Kubernetes-кластера.

Kubelet — это агент, который работает на каждом узле Kubernetes-кластера. Он обеспечивает запуск контейнеров в подах и их работу в соответствии со спецификациями. Kubelet непрерывно взаимодействует с kube-apiserver, проверяя и поддерживая состояние узлов и контейнеров. Kubelet также отвечает за запуск компонентов control plane.

## Взаимодействия kubelet

Взаимодействия kubelet изображены на [схеме архитектуры модуля `control-plane-manager`](../control-plane-management/).

Kubelet взаимодействует со следующими компонентами:

1. **kubernetes-api-proxy** — проксирует запросы к **kube-apiserver**, отправляемые на адрес `localhost`. Входит в состав модуля [`control-plane-manager`](/modules/control-plane-manager/).
2. **kube-apiserver-healthcheck** — проверяет состояние **kube-apiserver**.

C kubelet взаимодействуют следующие компоненты:

1. **kube-apiserver**:

   * получение логов с подов (обработка команды `kubectl logs`);
   * подключение к запущенным подам (обработка команды `kubectl exec`);
   * переадресация портов (обработка команды `kubectl port-forward`).

2. **prometheus-main** — собирает метрики kubelet.
