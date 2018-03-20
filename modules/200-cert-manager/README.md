Модуль cert-manager
=======

Модуль устанавливает [cert-manager](https://github.com/jetstack/cert-manager).

Что нужно настраивать?
----------------------

Обязательных настроек нет.

Конфигурация
------------

* `nodeSelector` — как в Kubernetes в `spec.nodeSelector` у pod'ов.
    * Если ничего не указано — будет использоваться значение `{"node-role/system":""}` (если в кластере есть такие узлы) или ничего не будет указано.
    * Можно указать `false`, чтобы не добавлять никакой nodeSelector.
* `tolerations` — как в Kubernetes в `spec.tolerations` у pod'ов.
    * Если ничего не указано — будет использовано значение `[{"key":"node-role/system","operator":"Exists"}]` (если в кластере есть такие узлы) или ничего не будет указано.
    * Можно указать `false`, чтобы не добавлять никакие toleration'ы.

### Пример конфига

```yaml
certManager:
  nodeSelector:
    node-role/other: ""
  tolerations:
  - key: node-role/other
    operator: Exists
```


Как пользоваться модулем?
-------------------------

В рядовом случае, если у сайта нет никакой дополнительной аутентификации и вайтлистов, то просто добавляете в ingress аннотацию и название tls-секрета:

```
kind: Ingress
metadata:
  annotations:
    kubernetes.io/ingress.class: nginx
    kubernetes.io/tls-acme: "true"
  name: web-acme
spec:
  rules:
  - host: web.example.com
    http:
      paths:
      - backend:
          serviceName: site
          servicePort: 80
        path: /
  tls:
  - hosts:
    - web.example.com
    secretName: web-example-com-tls-secret
```

Если аутентификация/вайтлисты есть, то действия следующие (на примере ингресса "web" для хоста web.example.com):


**1**. Добавить основному ингрессу spec "tls" и указать secret-name с произвольным именем, например:

```
kind: Ingress
metadata:
  annotations:
    ingress.kubernetes.io/whitelist-source-range: 1.2.3.4
  name: web
spec:
  rules:
  - host: web.example.com
    http:
      paths:
      - backend:
          serviceName: site
          servicePort: 80
        path: /
  tls:
  - hosts:
    - web.example.com
    secretName: web-example-com-tls-secret
```

**2**. Создать ещё один ингресс (в нашем случае "web-acme") — копию основного ингресса (в нашем случае "web"), но без настроек аутентификации/вайтлистов, с аннотацией kubernetes.io/tls-acme: "true", с единственным path "/.well-known/" и таким же "tls":

```
kind: Ingress
metadata:
  annotations:
    kubernetes.io/ingress.class: nginx
    kubernetes.io/tls-acme: "true"
  name: web-acme
spec:
  rules:
  - host: web.example.com
    http:
      paths:
      - backend:
          serviceName: site
          servicePort: 80
        path: /.well-known/
  tls:
  - hosts:
    - web.example.com
    secretName: web-example-com-tls-secret
```

### Почему так сложно?

Потому, что cert-manager создаёт локейшн "/.well-known/..." в ингрессе с аннотацией "tls-acme" и этот локейшн попадает под правила аутентификации/вайтлистов. Let`s Encrypt, в свою очередь, не может пробиться через фильтр и верифицировать домен.

### А можно составить заявку на сертификат вручную?

https://github.com/jetstack/cert-manager/blob/master/docs/user-guides/acme-http-validation.md
