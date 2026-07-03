---
title: Kubelet
permalink: ru/architecture/kubernetes-and-scheduling/kubelet.html
lang: ru
search: kubelet, агент kubelet, архитектура kubelet, взаимодействия kubelet
description: Архитектура и роль kubelet в Deckhouse Kubernetes Platform.
---

Kubelet не является компонентом control plane, но играет ключевую роль в работе Kubernetes-кластера.

Kubelet — это агент, который работает на каждом узле Kubernetes-кластера. Он обеспечивает запуск контейнеров в подах и их работу в соответствии со спецификациями. Kubelet непрерывно взаимодействует с kube-apiserver, проверяя и поддерживая состояние узлов и контейнеров. Kubelet также отвечает за запуск компонентов control plane.

## Манифесты статических подов

Kubelet запускает компоненты control plane из манифестов статических подов, расположенных в директории `/etc/kubernetes/manifests`. В Deckhouse Kubernetes Platform kubelet обрабатывает в этой директории только файлы с расширением `.yaml` или `.yml`.

Файлы с другими расширениями, например `kube-apiserver.backup`, `kube-apiserver.yaml.bak`, swap-файлы редакторов или другие временные файлы, игнорируются. Это предотвращает случайную обработку резервных копий и файлов, не являющихся манифестами, как манифестов статических подов.

## Взаимодействия kubelet

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

* На схеме показано, что контейнеры разных подов взаимодействуют друг с другом напрямую. Фактически они взаимодействуют через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса указано над стрелкой.
* Поды могут быть запущены в нескольких репликах, однако на схеме все поды изображены в одной реплике.
* На схеме показан под `application`, который представляет собой любой под в кластере: системный и пользовательский.
{% endalert %}

Взаимодействия kubelet изображены на следующей диаграмме:

<!--- Source: structurizr code --->
![Взаимодействия kubelet](../../images/architecture/kubernetes-and-scheduling/c4-l2-kubelet.ru.png)

Kubelet контролирует состояние контейнеров всех подов, запущенных на узле, относящихся как к пользовательским приложениям, так и к компонентам DKP, выполняя пробы Startup, Liveness и Readiness в соответствии со спецификацией пода. Подробнее о пробах можно узнать в [документации Kubernetes](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes).

Kubelet также взаимодействует со следующими компонентами:

1. **Containerd** — отправляет команды для управления жизненным циклом контейнеров на узле, используя для этого [Container Runtime Interface (CRI)](https://kubernetes.io/docs/concepts/containers/cri/);
1. **Kubernetes-api-proxy** — проксирует запросы к kube-apiserver, отправляемые на адрес `localhost`. Входит в состав модуля [`control-plane-manager`](/modules/control-plane-manager/);
1. **Kube-apiserver-healthcheck** — проверяет состояние kube-apiserver.

С kubelet взаимодействуют следующие компоненты:

1. **Kube-apiserver**:

   * получение логов с подов (обработка команды `kubectl logs`);
   * подключение к запущенным подам (обработка команды `kubectl exec`);
   * переадресация портов (обработка команды `kubectl port-forward`).

1. **Prometheus-main** — собирает метрики kubelet.
