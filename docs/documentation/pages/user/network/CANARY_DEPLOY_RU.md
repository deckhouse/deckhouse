---
title: "Canary deployment"
permalink: ru/user/network/canary-deployment.html
lang: ru
---

Canary deployment — стратегия развертывания приложений, которая позволяет постепенно внедрять в production новую версию приложения.
Этот подход даёт возможность тестировать новые версии на небольшой части трафика, минимизируя риски и обеспечивая плавный переход.
С помощью Canary deployment возможно переключение трафика на новую версию по мере уверенности в её стабильности, с возможностью быстрого отката на старую версию при возникновении проблем.
В Deckhouse Kubernetes Platform Canary deployment может быть реализован средствами [`ingress-nginx`](/modules/ingress-nginx/) или [`istio`](/modules/istio/) (рекомендуемый способ).

## Примеры настроек Canary deployment средствами Ingress NGINX

Для реализации Canary deployment средствами Ingress NGINX используются аннотации и правила, которые определяют направление части трафика на новую версию приложения.

### Создание Deployment и Service для стабильной версии

Пример манифеста для стабильной версии:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app-v1
spec:
  replicas: 3
  selector:
    matchLabels:
      app: my-app
      version: v1
  template:
    metadata:
      labels:
        app: my-app
        version: v1
    spec:
      containers:
      - name: app
        image: app:v1
        ports:
        - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: app-service
spec:
  selector:
    app: my-app
  ports:
  - port: 80
    targetPort: 80
```

### Создание Deployment и Service для Canary-версии

Пример манифеста для Canary-версии:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app-v2
spec:
  replicas: 1
  selector:
    matchLabels:
      app: my-app
      version: v2
  template:
    metadata:
      labels:
        app: my-app
        version: v2
    spec:
      containers:
      - name: app
        image: app:v2
        ports:
        - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: app-canary-service
spec:
  selector:
    app: my-app
    version: v2
  ports:
  - port: 80
    targetPort: 80
```

### Настройка Ingress для Canary deployment

Для реализации Canary deployment с использованием Ingress NGINX используются специальные аннотации:

- `nginx.ingress.kubernetes.io/canary` — включает Canary-режим для Ingress.
- `nginx.ingress.kubernetes.io/canary-weight` — указывает процент трафика, который будет направлен на Canary-версию.

Пример манифеста для Ingress (10% трафика будет направлено на Canary-версию (`app-canary-service`), 90% трафика — на стабильную версию (`app-service`)):

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: app-ingress
  annotations:
    nginx.ingress.kubernetes.io/canary: "true"
    nginx.ingress.kubernetes.io/canary-weight: "10" # 10% трафика на Canary.
spec:
  rules:
  - host: example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: app-canary-service
            port:
              number: 80
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: app-ingress-main
spec:
  rules:
  - host: example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: app-service
            port:
              number: 80
```

### Постепенное увеличение трафика на Canary-версию

Вы можете постепенно увеличивать процент трафика на Canary-версию, изменяя значение аннотации `nginx.ingress.ernetes.io/canary-weight`. Например, чтобы направить 50% трафика на Canary-версию, обновите аннотацию следующим образом:

```yaml
nginx.ingress.kubernetes.io/canary-weight: "50"
```

### Откат или завершение Canary deployment

Если Canary-версия работает стабильно, вы можете полностью переключить трафик на новую версию, удалив Canary-аннотации и обновив основной Ingress.
Если возникли проблемы, вы можете уменьшить процент трафика на Canary-версию или полностью отключить ее, установив `nginx.ingress.kubernetes.io/canary-weight: "0"`.

### Дополнительные аннотации для Canary deployment

- `nginx.ingress.kubernetes.io/canary-by-header` — направляет трафик на Canary-версию на основе значения HTTP-заголовка.
- `nginx.ingress.kubernetes.io/canary-by-cookie` — направляет трафик на Canary-версию на основе значения cookie.

Пример использования заголовка:

```yaml
nginx.ingress.kubernetes.io/canary-by-header: "canary"
nginx.ingress.kubernetes.io/canary-by-header-value: "true"
```

В этом случае трафик будет направлен на Canary-версию, если запрос содержит заголовок `canary: true`.

<!-- перенесено из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/istio/examples.html#canary -->

## Примеры настроек Canary deployment средствами Istio

{% alert level="info" %}
Istio отвечает лишь за гибкую маршрутизацию запросов, которая опирается на спецзаголовки запросов (например, cookie) или просто на случайность.
За настройку этой маршрутизации и переключение между канареечными версиями отвечает CI/CD-система.
{% endalert %}

Подразумевается, что в одном пространстве имён размещены два Deployment с разными версиями приложения. У подов разных версий разные лейблы (`version: v1` и `version: v2`).

Требуется настроить два кастомных ресурса:

* [DestinationRule](../network/managing_request_between_service_istio.html#ресурс-destinationrule) с описанием, как идентифицировать разные версии вашего приложения (subset'ы);
* [VirtualService](../network/retry_istio.html#ресурс-virtualservice) с описанием, как распределять трафик между разными версиями приложения.

Пример:

```yaml
apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: productpage-canary
spec:
  host: productpage
  # Subset'ы доступны только при обращении к хосту через VirtualService из пода под управлением Istio.
  # Эти subset'ы должны быть указаны в маршрутах.
  subsets:
  - name: v1
    labels:
      version: v1
  - name: v2
    labels:
      version: v2
```

### Распределение по наличию cookie

```yaml
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: productpage-canary
spec:
  hosts:
  - productpage
  http:
  - match:
    - headers:
       cookie:
         regex: "^(.*;?)?(canary=yes)(;.*)?"
    route:
    - destination:
        host: productpage
        subset: v2 # Ссылка на subset из DestinationRule.
  - route:
    - destination:
        host: productpage
        subset: v1
```

### Распределение по вероятности

```yaml
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: productpage-canary
spec:
  hosts:
  - productpage
  http:
  - route:
    - destination:
        host: productpage
        subset: v1 # Ссылка на subset из DestinationRule.
      weight: 90 # Процент трафика, который получат поды с лейблом `version: v1`.
  - route:
    - destination:
        host: productpage
        subset: v2
      weight: 10
```
