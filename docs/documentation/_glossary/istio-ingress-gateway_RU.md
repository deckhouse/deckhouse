---
title: Istio Ingress Gateway
description: Точка входа балансировщика уровня приложений на базе Istio. Администратор разворачивает её с помощью [IngressIstioController](/modules/istio/cr.html#ingressistiocontroller), а разработчики публикуют приложения с помощью объектов Gateway и VirtualService.
lang: ru
---

Публикация средствами Istio настраивается в два слоя: администратор кластера разворачивает IngressIstioController и связанную инфраструктуру, разработчики создают объекты Gateway и VirtualService в своём [неймспейсе](glossary.html#namespace). Это одна из реализаций [ALB](glossary.html#alb) в Deckhouse Platform наряду с Ingress NGINX и Gateway API с его [управляемым Gateway](glossary.html#alb-managed-gateway).

Подробнее в разделе [«Публикация приложений средствами Istio»](user/network/ingress/alb/istio.html).
