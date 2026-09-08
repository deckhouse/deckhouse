---
title: "Публикация приложений средствами Istio"
description: "Публикация приложений с Istio в Deckhouse Kubernetes Platform. Ingress NGINX с Istio-сайдкаром и Istio Ingress Gateway с ресурсами Gateway и VirtualService."
permalink: ru/user/network/ingress/alb/istio.html
lang: ru
extractedLinksMax: 0
relatedLinks:
  - title: "Использование Application Load Balancer (ALB)"
    url: ./
  - title: "Документация модуля istio"
    url: /modules/istio/
---

## Публикация приложений средствами Istio

Публикация средствами Istio настраивается в два слоя. Администратор кластера разворачивает [IngressIstioController](/modules/istio/cr.html) (и связанную инфраструктуру), как описано в разделе [«Istio Ingress Gateway»](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/istio.html#istio-ingress-gateway). Разработчики приложения создают ресурсы Gateway и VirtualService, как показано ниже.

При публикации приложения средствами Istio выберите один из способов: [«Публикация приложений с использованием Ingress NGINX»](#публикация-приложений-с-использованием-ingress-nginx) или [«Публикация приложений с использованием ресурса Istio Ingress Gateway»](#публикация-приложений-с-использованием-ресурса-istio-ingress-gateway).

### Публикация приложений с использованием Ingress NGINX {#публикация-приложений-с-использованием-ingress-nginx}

Для публикации приложения средствами Ingress NGINX администратор кластера должен настроить Ingress-контроллер, добавив к нему сайдкар от Istio.

Для публикации приложения подготовьте Ingress-ресурс, который ссылается на сервис. Укажите `ingressClassName` контроллера с Istio-сайдкаром (значение сообщит администратор). Обязательные аннотации для Ingress-ресурса:

- `nginx.ingress.kubernetes.io/service-upstream: "true"` — с этой аннотацией Ingress-контроллер будет отправлять запросы на ClusterIP сервиса (из диапазона Service CIDR) вместо того, чтобы отправлять их напрямую в поды приложения. Сайдкар-контейнер `istio-proxy` перехватывает только этот трафик, остальные запросы отправляются напрямую;
- `nginx.ingress.kubernetes.io/upstream-vhost: productpage.bookinfo.svc` — с этой аннотацией сайдкар сможет идентифицировать прикладной сервис, для которого предназначен запрос.

Примеры:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: productpage
  namespace: bookinfo
  annotations:
    # Включает проксирование трафика через nginx на ClusterIP вместо собственных IP подов.
    nginx.ingress.kubernetes.io/service-upstream: "true"
    # В Istio вся маршрутизация осуществляется на основе `Host:` заголовка запросов.
    # Это позволяет не заводить в Istio внешний домен `productpage.example.com` —
    # вместо него используется внутренний домен, уже известный Istio.
    nginx.ingress.kubernetes.io/upstream-vhost: productpage.bookinfo.svc
spec:
  # Имя IngressClass контроллера с Istio-сайдкаром.
  ingressClassName: nginx
  rules:
    - host: productpage.example.com
      http:
        paths:
        - path: /
          pathType: Prefix
          backend:
            service:
              name: productpage
              port:
                number: 9080
```

```yaml
apiVersion: v1
kind: Service
metadata:
  name: productpage
  namespace: bookinfo
spec:
  ports:
  - name: http
    port: 9080
  selector:
    app: productpage
  type: ClusterIP
```

### Публикация приложений с использованием ресурса Istio Ingress Gateway {#публикация-приложений-с-использованием-ресурса-istio-ingress-gateway}

Для публикации приложения средствами Istio Ingress Gateway администратор кластера должен создать ресурс [IngressIstioController](/modules/istio/cr.html).

Для публикации приложения с использованием ресурса Istio Ingress Gateway создайте ресурс Gateway. В поле `spec.selector` укажите лейбл, ссылающийся на `ingressGatewayClass`, и имя Secret, полученные от администратора кластера:

```yaml
apiVersion: networking.istio.io/v1beta1
kind: Gateway
metadata:
  name: gateway-app
  namespace: app-ns
spec:
  selector:
    # Значение ingressGatewayClass, полученное от администратора кластера.
    istio.deckhouse.io/ingress-gateway-class: istio-hp
  servers:
    - port:
        # Стандартный шаблон для использования протокола HTTP.
        number: 80
        name: http
        protocol: HTTP
      hosts:
        - app.example.com
    - port:
        # Стандартный шаблон для использования протокола HTTPS.
        number: 443
        name: https
        protocol: HTTPS
      tls:
        mode: SIMPLE
        # Secret с сертификатом и ключом, созданный администратором в неймспейсе d8-ingress-istio.
        # Поддерживаемые форматы Secret: https://istio.io/latest/docs/tasks/traffic-management/ingress/secure-ingress/#key-formats.
        credentialName: app-tls-secret
      hosts:
        - app.example.com
```

Затем определите правила маршрутизации с помощью VirtualService, который связывает Gateway и обслуживаемый им сервис:

```yaml
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: vs-app
  namespace: app-ns
spec:
  # Имя ресурса Gateway, созданного выше.
  gateways:
    - gateway-app
  hosts:
    - app.example.com
  http:
    - route:
        - destination:
            # Имя сервиса, на который нужно направить трафик.
            host: app-svc
```

### Canary-развёртывание через VirtualService {#canary-развёртывание-через-virtualservice}

Общий обзор canary в DKP — в разделе [«Canary-развёртывание»](/products/kubernetes-platform/documentation/v1/user/network/canary-deployment.html). Ниже — пример на VirtualService и DestinationRule.

Для постепенного переключения трафика между версиями приложения используйте DestinationRule с подмножествами (subsets) и веса в VirtualService. Пример направляет 90% трафика на стабильную версию и 10% на canary:

```yaml
apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: app-svc
  namespace: app-ns
spec:
  host: app-svc
  subsets:
    - name: stable
      labels:
        version: v1
    - name: canary
      labels:
        version: v2
---
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: vs-app
  namespace: app-ns
spec:
  gateways:
    - gateway-app
  hosts:
    - app.example.com
  http:
    - route:
        - destination:
            host: app-svc
            subset: stable
          weight: 90
        - destination:
            host: app-svc
            subset: canary
          weight: 10
```

Поды версий должны иметь лейблы `version: v1` и `version: v2`, совпадающие с `subsets` в DestinationRule. Перед изменением весов проверьте, что обе версии обслуживаются сервисом `app-svc`.

После применения манифестов отправьте несколько запросов на `app.example.com` и проверьте, что трафик распределяется в заданной пропорции — примерно 90/10. Чтобы определить, какая версия ответила, используйте признак, отличающий `stable` от `canary` в самом приложении, например, значение в теле ответа или отдельный HTTP-заголовок:

```shell
for i in $(seq 1 20); do curl -s https://app.example.com/ | grep version; done \
  | sort | uniq -c
```

При необходимости скорректируйте веса и повторите проверку.
