---
title: "Активация Istio для приложений"
permalink: ru/user/network/app_istio_activation.html
lang: ru
---

Активация Istio для приложений возможна, если в кластере включен и настроен модуль [`istio`](/modules/istio/configuration.html). За это отвечает администратор кластера.

<!-- перенесено с небольшими изменениями из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/istio/#%D0%BA%D0%B0%D0%BA-%D0%B0%D0%BA%D1%82%D0%B8%D0%B2%D0%B8%D1%80%D0%BE%D0%B2%D0%B0%D1%82%D1%8C-istio-%D0%B4%D0%BB%D1%8F-%D0%BF%D1%80%D0%B8%D0%BB%D0%BE%D0%B6%D0%B5%D0%BD%D0%B8%D1%8F -->

Суть активации — добавить сайдкар-контейнер к подам приложения, после чего Istio сможет управлять трафиком.

Рекомендованный способ добавления сайдкаров — использовать сайдкар-injector. Istio умеет «подселять» к подам приложения сайдкар-контейнер с помощью механизма [Admission Webhook](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/). Для добавления сайдкаров используются лейблы и аннотации:

- Лейбл к `namespace` — обозначает ваше пространство имён для компонента сайдкар-injector. После применения лейбла к новым подам будут добавлены сайдкар-контейнеры:
  - `istio-injection=enabled` — использует глобальную версию Istio (`spec.settings.globalVersion` в ресурсе ModuleConfig);
  - `istio.io/rev=v1x16` — использует конкретную версию Istio для этого пространства имён.
- Аннотация к поду `sidecar.istio.io/inject` (`"true"` или `"false"`) позволяет локально переопределить политику `sidecarInjectorPolicy`. Эти аннотации работают только в пространствах имён, обозначенных лейблами из списка выше.

Также существует возможность добавить сайдкар к определенному поду в пространстве имён без установленных лейблов `istio-injection=enabled` или `istio.io/rev=vXxYZ` путем установки лейбла `sidecar.istio.io/inject=true`.

Istio-proxy, который работает в качестве сайдкар-контейнера, тоже потребляет ресурсы и добавляет накладные расходы:

- Каждый запрос DNAT'ится в Envoy, который обрабатывает это запрос и создает еще один. На принимающей стороне — аналогично.
- Каждый Envoy хранит информацию обо всех сервисах в кластере, что требует памяти. Больше кластер — больше памяти потребляет Envoy. Решение — кастомный ресурс [Sidecar](/modules/istio/istio-cr.html#sidecar).

Также важно подготовить Ingress-контроллер и Ingress-ресурсы приложения:

- Включите [`enableIstioSidecar`](/modules/ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-enableistiosidecar) у ресурса IngressNginxController.
- Добавьте аннотации на Ingress-ресурсы приложения:
  - `nginx.ingress.kubernetes.io/service-upstream: "true"` — Ingress-контроллер в качестве upstream использует ClusterIP сервиса вместо адресов подов. Балансировкой трафика между подами теперь занимается сайдкар-proxy. Используйте эту опцию, только если у вашего сервиса есть ClusterIP;
  - `nginx.ingress.kubernetes.io/upstream-vhost: "myservice.myns.svc"` — сайдкар-proxy Ingress-контроллера принимает решения о маршрутизации на основе заголовка `Host`. Без этой аннотации контроллер оставит заголовок с адресом сайта, например `Host: example.com`.
