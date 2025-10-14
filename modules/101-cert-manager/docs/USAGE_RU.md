---
title: "Модуль cert-manager: примеры конфигурации"
---


## Пример заказа сертификата

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: example-com                          # Имя сертификата, через него потом можно смотреть статус.
  namespace: default
spec:
  secretName: example-com-tls                # Название Secret'а, в который положить приватный ключ и сертификат.
  issuerRef:
    kind: ClusterIssuer                      # Ссылка на "выдаватель" сертификатов, см. подробнее ниже.
    name: letsencrypt
  commonName: example.com                    # Основной домен сертификата.
  dnsNames:                                  # Дополнительные домены сертификата (как минимум одно DNS-имя или IP-адрес должны быть указаны).
  - www.example.com
  - admin.example.com
```

При этом:
* создается отдельный Ingress-ресурс на время прохождения challenge'а (соответственно, аутентификация и whitelist основного Ingress не будут мешать);
* можно заказать один сертификат на несколько Ingress-ресурсов (и он не отвалится при удалении того, в котором была аннотация `tls-acme`);
* можно заказать сертификат с дополнительными именами (как в примере);
* можно валидировать разные домены, входящие в один сертификат, через разные Ingress-контроллеры.


## Заказ self-signed-сертификата

Все еще проще, чем с LetsEncrypt. Просто меняем `letsencrypt` на `selfsigned`:

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: example-com                          # Имя сертификата, через него потом можно смотреть статус.
  namespace: default
spec:
  secretName: example-com-tls                # Название Secret'а, в который положить приватный ключ и сертификат.
  issuerRef:
    kind: ClusterIssuer                      # Ссылка на ClusterIssuer.
    name: selfsigned
  commonName: example.com                    # Основной домен сертификата.
  dnsNames:                                  # Дополнительные домены сертификата. Требуется, как минимум, дублирование записи из commonName.
  - example.com
  - www.example.com
  - admin.example.com
```
