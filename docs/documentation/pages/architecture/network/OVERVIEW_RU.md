---
title: Подсистема Network
permalink: ru/architecture/network/
lang: ru
search: network, сетевая подсистема, сеть
description: Архитектура подсистемы Network в Deckhouse Kubernetes Platform.
extractedLinksOnlyMax: 0
extractedLinksMax: 0
---

В данном подразделе описана архитектура подсистемы Network (сетевой подсистемы) Deckhouse Kubernetes Platform (DKP).

В подсистему Network входят следующие модули:

* [`kube-dns`](/modules/kube-dns/) — устанавливает компоненты CoreDNS для управления DNS в кластере Kubernetes;
* [`node-local-dns`](/modules/node-local-dns/) — разворачивает кеширующий DNS-сервер на каждом узле кластера и экспортирует данные в Prometheus для анализа работы DNS в кластере на [дашборде Grafana](/modules/node-local-dns/#grafana-dashboard). Архитектура кеширующего DNS-сервера описана на [соответствующей странице](dns-caching.html) данного подраздела;
* [`kube-proxy`](/modules/kube-proxy/) — управляет компонентами kube-proxy для сетевого взаимодействия и балансировки нагрузки в кластере;
* [`cni-cilium`](/modules/cni-cilium/) — обеспечивает работу сети в кластере Kubernetes с помощью CNI Cilium;
* [`ingress-nginx`](/modules/ingress-nginx/) — устанавливает и управляет [Ingress NGINX Controller](https://kubernetes.github.io/ingress-nginx/) с помощью кастомных ресурсов. Архитектура модуля описана на [соответствующей странице](ingress-nginx.html) данного подраздела.
* [`metallb`](/modules/metallb/) — реализует механизм LoadBalancer для сервисов в bare-metal-кластерах.

Также в подразделе описаны:

* [архитектура кластера с включенным Istio](cluster-with-istio.html);
* [архитектура прикладного сервиса с включенным Istio](service-with-istio.html).
