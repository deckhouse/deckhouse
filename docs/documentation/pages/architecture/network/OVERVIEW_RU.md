---
title: Подсистема Network
permalink: ru/architecture/network/
lang: ru
search: network, сетевая подсистема, сеть
description: Архитектура подсистемы Network в Deckhouse Platform.
extractedLinksOnlyMax: 0
extractedLinksMax: 0
---

В данном подразделе описана архитектура подсистемы Network (сетевой подсистемы) Deckhouse Platform (DP).

В подсистему Network входят следующие модули:

* [`kube-dns`](/modules/kube-dns/) — устанавливает компоненты CoreDNS для управления DNS в кластере Kubernetes;
* [`node-local-dns`](/modules/node-local-dns/) — разворачивает кеширующий DNS-сервер на каждом узле кластера и экспортирует данные в Prometheus для анализа работы DNS в кластере [на дашборде Grafana](/modules/node-local-dns/#grafana-dashboard);
* [`kube-proxy`](/modules/kube-proxy/) — управляет компонентами kube-proxy для сетевого взаимодействия и балансировки нагрузки в кластере;
* [`cni-cilium`](/modules/cni-cilium/) — обеспечивает работу сети в кластере Kubernetes с помощью CNI Cilium;
* [`cilium-hubble`](/modules/cilium-hubble/) — обеспечивает визуализацию сетевого стека кластера, если включен Cilium CNI;
* [`ingress-nginx`](/modules/ingress-nginx/) — устанавливает и управляет [Ingress NGINX Controller](https://kubernetes.github.io/ingress-nginx/) с помощью кастомных ресурсов;
* [`metallb`](/modules/metallb/) — реализует механизм LoadBalancer для сервисов в bare-metal-кластерах;
* [`istio`](/modules/istio/) — реализует Service Mesh на основе Istio для централизованного управления сетевым трафиком в кластере;
* [`alb`](/modules/alb/) — реализует прикладной балансировщик нагрузки (ALB) на основе [Kubernetes Gateway API](https://gateway-api.sigs.k8s.io/);
* [`sdn`](/modules/sdn/) — предоставляет функции программно-определяемых сетей (SDN) в кластере: настройку сетевых интерфейсов на узлах, дополнительные сети для подов и ВМ, underlay-сети и системные сети.

Также в подразделе описаны:

* [архитектура кластера с включенным Istio](cluster-with-istio.html);
* [архитектура прикладного сервиса с включенным Istio](service-with-istio.html).
