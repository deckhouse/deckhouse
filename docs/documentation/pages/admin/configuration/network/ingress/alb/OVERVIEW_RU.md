---
title: "ALB в Deckhouse Kubernetes Platform"
permalink: ru/admin/configuration/network/ingress/alb/
lang: ru
---

В Deckhouse Kubernetes Platform (далее — DKP) поддерживается балансировка входящего трафика на уровне приложений (ALB — Application Load Balancer) средствами [NGINX Ingress controller](https://github.com/kubernetes/ingress-nginx) (модуль `ingress-nginx`) и Istio (модуль `istio`).

Возможности функции ALB в Deckhouse Kubernetes Platform:

- Автоматическое создание балансировщиков нагрузки. DKP автоматически создает и настраивает ALB на основе Ingress-ресурсов.
- Поддержка HTTP/HTTPS. Поддерживается терминация SSL/TLS и перенаправление HTTP на HTTPS.
- Маршрутизация на основе правил. Маршрутизировать трафик можно на основе пути, хоста или других параметров запроса.
- Интеграция с сертификатами. Поддержка автоматического получения и обновления сертификатов.
