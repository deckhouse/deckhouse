---
title: kubernetes-api-proxy
description: Прокси-сервер на базе NGINX на каждом узле кластера, который принимает запросы к API Kubernetes по адресу localhost и перенаправляет их на master-узлы, обеспечивая высокую доступность и балансировку нагрузки API.
lang: ru
---

[bashible](glossary.html#bashible) устанавливает и настраивает прокси и обновляет его список upstream-серверов — [master-узлов](glossary.html#master-node). На master-узлах он работает как [статический под](glossary.html#static-pod): по умолчанию проксирует запросы к локальному kube-apiserver, а при его недоступности последовательно опрашивает остальные экземпляры. Через него проходят запросы kubelet к `localhost`.

Подробнее в разделе [«Управление control plane»](architecture/kubernetes-and-scheduling/control-plane-management.html).
