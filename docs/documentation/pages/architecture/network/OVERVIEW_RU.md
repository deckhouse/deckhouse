---
title: Обзор
permalink: ru/architecture/network/
lang: ru
search: network, сетевая подсистема, сеть
---

Данный подраздел посвящён архитектуре подсистемы **Network** (сетевой подсистемы) DKP.

В подсистему **Network** входят следующие модули:

* [kube-dns](/modules/kube-dns/) - устанавливает компоненты CoreDNS для управления DNS в кластере Kubernetes,
* [node-local-dns](/modules/node-local-dns/) - разворачивает кеширующий DNS-сервер на каждом узле кластера, экспортирует данные в **Prometheus** для удобного анализа работы DNS в кластере [на доске](/modules/node-local-dns/#grafana-dashboard) Grafana. Архитектура кеширующего DNS-сервера описана в данном подразделе на соответствующей [странице](dns-caching.html).
* [kube-proxy](/modules/kube-proxy/) - управляет компонентами **kube-proxy** для сетевого взаимодействия и балансировки нагрузки в кластере.
* [cni-cilium](/modules/cni-cilium/) - обеспечивает работу сети в кластере Kubernetes с помощью CNI Cilium.
* [ingress-nginx](/modules/ingress-nginx/) - устанавливает и управляет [Ingress NGINX Controller](https://kubernetes.github.io/ingress-nginx/) с помощью кастомных ресурсов. Архитектура модуля **ingress-nginx** описана в подразделе на соответствующей [странице](ingress-nginx/).
* [metallb](/modules/metallb/) - реализует механизм LoadBalancer для сервисов в кластерах bare metal.

Также в подразделе описаны:

* [архитектура кластера с включенным Istio](cluster-with-istio.html),
* [архитектура прикладного сервиса с включенным Istio](service-with-istio.html).
