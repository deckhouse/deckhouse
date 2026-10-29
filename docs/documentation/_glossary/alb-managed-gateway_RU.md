---
title: Управляемый Gateway
description: Объект Gateway, который модуль [`alb`](/modules/alb/) создаёт из [ClusterALBInstance](/modules/alb/cr.html#clusteralbinstance) или [ALBInstance](/modules/alb/cr.html#albinstance). Через него публикуют приложения с помощью ListenerSet и маршрутов Kubernetes Gateway API.
lang: ru
---

Общекластерный шлюз создаёт администратор кластера из ClusterALBInstance, обычно в [неймспейсе](glossary.html#namespace) `d8-alb`. Отдельный шлюз в неймспейсе приложения создаётся из ALBInstance. Имя управляемого Gateway публикуется в поле `status.gateway` инстанса. Администратор неймспейса привязывает к нему ListenerSet с именами хостов, TLS и портами, разработчики — HTTPRoute и другие маршруты. Слушатели `d8-http` и `d8-https` предназначены для служебных задач, например проверки доступности, и не используются для маршрутов приложений.

Подробнее в разделе [«Публикация приложений средствами Kubernetes Gateway API»](user/network/ingress/alb/gateway-api.html).
