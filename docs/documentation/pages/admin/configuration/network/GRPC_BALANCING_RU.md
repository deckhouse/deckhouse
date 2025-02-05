---
title: "Балансировка gRPC"
permalink: ru/admin/network/grpc-balancing.html
lang: ru
---

В Deckhouse Kubernetes Platform Балансировка gRPC реализуется средствами Istio (модуль [`istio`](../#)).

<!-- перенесено из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/istio/examples.html#%D0%B1%D0%B0%D0%BB%D0%B0%D0%BD%D1%81%D0%B8%D1%80%D0%BE%D0%B2%D0%BA%D0%B0-grpc -->

**Важно!** Чтобы балансировка gRPC-сервисов заработала автоматически, присвойте name с префиксом или значением `grpc` для порта в соответствующем Service.
