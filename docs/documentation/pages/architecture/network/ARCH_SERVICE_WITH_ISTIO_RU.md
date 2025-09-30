---
title: "Архитектура прикладного сервиса с включенным Istio"
permalink: ru/architecture/network/service-with-istio.html
lang: ru
---

<!-- перенесено из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/istio/#%D0%B0%D1%80%D1%85%D0%B8%D1%82%D0%B5%D0%BA%D1%82%D1%83%D1%80%D0%B0-%D0%BF%D1%80%D0%B8%D0%BA%D0%BB%D0%B0%D0%B4%D0%BD%D0%BE%D0%B3%D0%BE-%D1%81%D0%B5%D1%80%D0%B2%D0%B8%D1%81%D0%B0-%D1%81-%D0%B2%D0%BA%D0%BB%D1%8E%D1%87%D0%B5%D0%BD%D0%BD%D1%8B%D0%BC-istio -->

## Особенности архитектуры

* **Sidecar-proxy**:
  Каждый под сервиса получает дополнительный контейнер — sidecar-proxy, содержащий два приложения:
  * **Envoy** — проксирует прикладной трафик и реализует весь функционал, который предоставляет Istio, включая маршрутизацию, аутентификацию, авторизацию и пр.
  * **Pilot-agent** — часть Istio, отвечает за поддержание конфигурации Envoy в актуальном состоянии, а также содержит в себе кэширующий DNS-сервер.
* **Настройки DNAT**:
  * В каждом поде настраивается DNAT входящих и исходящих прикладных запросов в sidecar-proxy. Делается это с помощью дополнительного init-контейнера. Таким образом, трафик будет перехватываться прозрачно для приложений.
  * Поскольку входящий трафик перенаправляется в sidecar-proxy, это касается и readiness/liveness-проб. Так как подсистема Kubernetes не поддерживает пробы в формате Mutual TLS, все существующие пробы перенастраиваются на порт в sidecar-proxy, который передает их приложению без изменений.
* **Ingress-контроллер**:
  * Каждый под Ingress-контроллера также включает sidecar-proxy, который обрабатывает трафик между контроллером и сервисами.
  * Входящий трафик от пользователей обрабатывается непосредственно контроллером.
* **Ресурсы типа Ingress**:
  Эти ресурсы требуют минимальной доработки в виде добавления аннотаций:
  * `nginx.ingress.kubernetes.io/service-upstream: "true"` — Ingress-контроллер в качестве upstream будет использовать ClusterIP сервиса вместо адресов подов. Балансировкой трафика между подами теперь занимается sidecar-proxy. Используйте эту опцию, только если у вашего сервиса есть ClusterIP.
  * `nginx.ingress.kubernetes.io/upstream-vhost: "myservice.myns.svc"` — sidecar-proxy Ingress-контроллера принимает решения о маршрутизации на основе заголовка Host. Без данной аннотации контроллер оставит заголовок с адресом сайта, например `Host: example.com`.
* **Сервисы**:
  * Ресурсы типа Service не требуют изменений и продолжают работать без адаптации. Приложениям все так же доступны адреса сервисов вида `servicename`, `servicename.myns.svc` и пр.
* **DNS-запросы**:
  * Внутренние DNS-запросы подов прозрачно перенаправляются на обработку в sidecar-proxy для разрешения DNS-имен сервисов из соседних кластеров.

### Жизненный цикл пользовательского запроса

#### Приложение с выключенным Istio

<div data-presentation="../../presentations/istio/request_lifecycle_istio_disabled_ru.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1_lw3EyDNTFTYNirqEfrRANnEAVjGhrOCdFJc-zCOuvs/ --->

#### Приложение с включенным Istio

<div data-presentation="../../presentations/istio/request_lifecycle_istio_enabled_ru.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1gQfX9ge2vhp74yF5LOfpdK2nY47l_4DIvk6px_tAMPU/ --->
