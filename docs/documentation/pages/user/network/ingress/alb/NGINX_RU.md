---
title: "Публикация приложений средствами Ingress NGINX Controller"
description: "Публикация приложений с Ingress NGINX Controller в Deckhouse Kubernetes Platform. Примеры ресурса Ingress, HTTPS, gRPC и проверка."
permalink: ru/user/network/ingress/alb/nginx.html
lang: ru
extractedLinksMax: 4
relatedLinks:
  - title: "Использование Application Load Balancer (ALB)"
    url: /products/kubernetes-platform/documentation/v1/user/network/ingress/alb.html
  - title: "ALB средствами Ingress NGINX Controller"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/nginx.html
  - title: "Документация модуля ingress-nginx"
    url: /modules/ingress-nginx/
  - title: "Custom Resources модуля ingress-nginx"
    url: /modules/ingress-nginx/cr.html
  - title: "Примеры модуля ingress-nginx"
    url: /modules/ingress-nginx/examples.html
---

## Публикация приложений средствами Ingress NGINX Controller

Для публикации приложений администратор кластера должен создать Ingress-контроллер. Порядок настройки описан в разделе [«Примеры настройки балансировки»](/products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/nginx.html#load-balancing-configuration-examples). Запросите у администратора имя `ingressClass` контроллера и укажите его в манифесте ресурса Ingress, который маршрутизирует входящий трафик к приложению.

{% tabs Примеры Ingress %}
{% tab "HTTP" %}

Пример базового Ingress для публикации приложения по HTTP:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-ingress
  namespace: prod
spec:
  ingressClassName: nginx # Имя IngressClass, предоставленное администратором кластера.
  rules:
  - host: application.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: productpage
            port:
              number: 80
```

{% endtab %}
{% tab "HTTPS" %}

Для HTTPS укажите Secret с сертификатом в секции `tls`. Secret должен существовать в том же неймспейсе, что и Ingress. Сертификат может выпустить cert-manager или его создаёт администратор.

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-ingress
  namespace: prod
spec:
  ingressClassName: nginx
  tls:
    - hosts:
        - application.example.com
      secretName: application-tls # Secret с tls.crt и tls.key в неймспейсе prod.
  rules:
  - host: application.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: productpage
            port:
              number: 80
```

{% endtab %}
{% endtabs %}

Дополнительные параметры маршрутизации задаются аннотациями Ingress. Список поддерживаемых аннотаций — в [документации модуля `ingress-nginx`](/modules/ingress-nginx/). Частые примеры:

- `nginx.ingress.kubernetes.io/rewrite-target` — перезапись пути;
- `nginx.ingress.kubernetes.io/whitelist-source-range` — список разрешённых CIDR;
- `nginx.ingress.kubernetes.io/backend-protocol: "GRPC"` — протокол к бэкенду для gRPC, если имя порта Service не указывает на gRPC.

### Проверка

После применения Ingress проверьте объект и адрес контроллера:

```shell
d8 k -n <NAMESPACE> get ingress
d8 k -n <NAMESPACE> describe ingress <INGRESS_NAME>
```

Убедитесь, что в статусе Ingress есть адрес и что DNS для `host` указывает на точку входа контроллера, которую сообщил администратор.

Проверка с рабочей станции (подставьте адрес точки входа):

```shell
curl -vk \
  --resolve app.example.com:443:<ENTRY_POINT_ADDRESS> \
  https://app.example.com/
```

## Балансировка gRPC

Раздел относится к публикации gRPC через Ingress NGINX Controller (`ingress-nginx`). Для Gateway API используйте [GRPCRoute](gateway-api.html#grpcroute-tlsroute-tcproute-and-udproute-objects).

Чтобы балансировка gRPC-сервисов за Ingress NGINX заработала автоматически, присвойте порту соответствующего объекта Service имя с префиксом или значением `grpc`. Если имя порта другое, добавьте аннотацию `nginx.ingress.kubernetes.io/backend-protocol: "GRPC"` к Ingress.
