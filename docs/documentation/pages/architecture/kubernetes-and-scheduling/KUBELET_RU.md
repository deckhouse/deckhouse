---
title: Kubelet
permalink: ru/architecture/kubernetes-and-scheduling/kubelet/
lang: ru
search: kubelet
---

Хотя **Kubelet** не является компонентом control plane, необходимо о нем рассказать, поскольку он играет важную (если не ключевую) роль в функционировании кластера.

**Kubelet** — это агент, который работает на каждом узле в кластере Kubernetes. Он отвечает за то, чтобы контейнеры в подах были запущены и функционировали в соответствии с предоставленными спецификациями. **Kubelet** непрерывно общается с **kube-apiserver**, чтобы проверять и поддерживать состояние узлов и контейнеров. **Kubelet** отвечает также за запуск компонентов control plane кластера.

## Взаимодействия kubelet

Взаимодействия **kubelet** изображены [на схеме архитектуры модуля control-plane-manager](control-plane-management/).

**kubelet** взаимодействует с:

1. **kubernetes-api-proxy** - запросы до **kube-apiserver**, отправляемые на адрес localhost, проксируются компонентом **kubernetes-api-proxy** модуля [control-plane-manager](/modules/control-plane-manager/).
2. **kube-apiserver-healthcheck** - проверяет состояние **kube-apiserver**.

C **kubelet** взаимодействуют:

1. **kube-apiserver**:

   * получение логов с подов (обработка команды `kubectl logs`).  
   * подключение к запущенным подам (обработка команды `kubectl exec`).  
   * переадресация портов (обработка команды `kubectl port-forward`).  

2. **prometheus-main** - сбор метрик **kubelet**.
