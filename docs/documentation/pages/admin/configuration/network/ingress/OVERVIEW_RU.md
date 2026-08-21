---
title: "Балансировка входящего трафика"
permalink: ru/admin/configuration/network/ingress/
description: "Настройка балансировки входящего трафика в Deckhouse Kubernetes Platform с NLB и ALB. Маршрутизация трафика, SSL-терминация и настройка балансировки на уровне приложений."
lang: ru
extractedLinksMax: 2
relatedLinks:
  - title: "ALB средствами Ingress NGINX Controller"
    url: alb/nginx.html
  - title: "ALB средствами Kubernetes Gateway API"
    url: alb/alb-gateway-api.html
  - title: "ALB средствами Istio"
    url: alb/istio.html
  - title: "Документация модуля ingress-nginx"
    url: /modules/ingress-nginx/
  - title: "Документация модуля alb"
    url: /modules/alb/
  - title: "Документация модуля istio"
    url: /modules/istio/
---

В этом разделе описываются подходы к балансировке входящего трафика в Deckhouse Kubernetes Platform (DKP):

- NLB (Network Load Balancer) — работает на сетевом уровне, маршрутизирует трафик по IP-адресам и портам без анализа содержимого запросов.
- ALB (Application Load Balancer) — действует на прикладном уровне, анализирует HTTP(S)-заголовки, пути и домены. Поддерживает SSL-терминацию и маршрутизацию в зависимости от содержимого запроса.

## Балансировка на сетевом уровне (NLB)

Балансировка NLB может быть организована двумя способами:

- с помощью внешнего балансировщика от облачного провайдера;
- средствами внутреннего балансировщика MetalLB, работающего как в облачных, так и в bare-metal-кластерах.

## Балансировка на прикладном уровне (ALB)

Для балансировки трафика на уровне приложений в DKP доступны следующие решения:

- [Ingress NGINX Controller](https://github.com/kubernetes/ingress-nginx) (модуль [`ingress-nginx`](/modules/ingress-nginx/));
- [Kubernetes Gateway API](https://kubernetes.io/docs/concepts/services-networking/gateway/) (модуль [`alb`](/modules/alb/));
- [Istio](https://istio.io/) (модуль [`istio`](/modules/istio/)).

### Отличие Kubernetes Gateway API от API Gateway {#gateway-api-vs-api-gateway}

Kubernetes Gateway API и API Gateway выполняют разные функции:

- Kubernetes Gateway API — спецификация Kubernetes, определяющая набор ресурсов для настройки маршрутизации входящего трафика к приложениям.
- API Gateway — архитектурный компонент или программный продукт, предоставляющий единую точку входа к API приложений и централизованно реализующий такие функции, как аутентификация, авторизация и ограничение частоты запросов.

Иными словами, Kubernetes Gateway API описывает, как настроить маршрутизацию трафика, а API Gateway — это разновидность инфраструктуры, которая этот трафик обрабатывает. Некоторые API Gateway можно настраивать через Kubernetes Gateway API. Модуль `alb` является реализацией Kubernetes Gateway API.

### Разделение ролей в модели Gateway API {#role-separation}

При использовании модуля `alb` ответственность за настройку и публикацию приложений разделяется между следующими ролями:

- администратор кластера — разворачивает кластерную инфраструктуру шлюза через ClusterALBInstance;
- администратор неймспейса — разворачивает локальную инфраструктуру шлюза через ALBInstance и настраивает приём трафика через ListenerSet;
- команда приложения — настраивает маршрутизацию к приложениям с помощью HTTPRoute и других ресурсов маршрутизации.

Такое разделение позволяет независимо управлять инфраструктурой шлюза и маршрутизацией приложений.

### Сравнение возможностей модулей ingress-nginx и alb {#сравнение-возможностей-модулей-ingress-nginx-и-alb}

Оба модуля решают одну задачу — приём и маршрутизацию внешнего трафика к приложениям, но опираются на разные стандарты: `ingress-nginx` использует Ingress API с аннотациями, а `alb` — Kubernetes Gateway API. Модули можно использовать в кластере одновременно. В таблице ниже сравниваются их возможности в текущих версиях.

| Параметр | `ingress-nginx` | `alb` |
| :--- | :--- | :--- |
| Стандарт маршрутизации | Ingress API с аннотациями | Kubernetes Gateway API |
| Реализация прокси | nginx | Envoy Proxy |
| Стадия жизненного цикла | General Availability | Preview |
| Развитие | Режим сопровождения: upstream-проект Ingress NGINX не развивает новые возможности, обновления безопасности поставляет DKP | Активно развивается |
| Минимальная версия DKP | Доступен во всех поддерживаемых версиях | 1.76 |
| Редакции DKP | Все редакции | Все редакции |
| Модель разграничения ролей | администратор кластера, администратор неймспейса | администратор кластера, администратор неймспейса, команда приложения |
| Несколько независимых точек входа | Несколько Ingress-контроллеров, выбор через `ingressClass` | Несколько объектов Gateway, выбор через `gatewayName`; общекластерные шлюзы и шлюзы в неймспейсе |
| HTTP/HTTPS (HTTP/1.1, HTTP/2, HTTP/3) | Есть | Есть |
| WebSocket | Есть | Есть |
| gRPC | Есть | Есть |
| FastCGI | Есть | Нет |
| TCP | Нет | Есть (TCPRoute) |
| UDP | Нет | Есть (UDPRoute) |
| TLS passthrough | Есть | Есть (TLSRoute) |
| Proxy Protocol | Есть | Есть |
| Способы приёма трафика | Инлеты [`LoadBalancer`](/modules/ingress-nginx/cr.html#ingressnginxcontroller-v2-spec-loadbalancer), [`HostNetwork`](/modules/ingress-nginx/cr.html#ingressnginxcontroller-v2-spec-inlet) и [`HostPort`](/modules/ingress-nginx/cr.html#ingressnginxcontroller-v2-spec-hostport) | Инлеты [`LoadBalancer`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-loadbalancer) и [`HostPort`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-inlet-hostport) |
| Автоматический выпуск TLS-сертификатов (`cert-manager`) | Есть | Есть |
| Настройка политик HTTPS (версии TLS, шифры, HSTS) | Есть | По умолчанию TLSv1.2/1.3; HSTS — через аннотацию заголовков ответа |
| WAF | ModSecurity на уровне контроллера или Ingress-ресурса | ModSecurity/Coraza на уровне маршрута, набор правил OWASP CRS |
| Внешняя аутентификация | Есть | Есть |
| Ограничение доступа по IP (whitelist) | Есть | Есть |
| Базовая аутентификация | Есть | Есть |
| Ограничение частоты запросов | Есть | Есть |
| Закрепление сессии (session affinity) | Есть | Есть |
| GeoIP | Гео-статистика запросов в метриках | Добавление полей GeoIP в заголовки HTTP-запросов на основе баз MaxMind |
| Метрики Prometheus и дашборды Grafana | Есть, с детализацией по неймспейсам, вхостам, Ingress-ресурсам и локациям | Есть: метрики Envoy Proxy и дашборды по запросам, маршрутам и upstream |
| Трассировка OpenTelemetry | Есть | Есть |
