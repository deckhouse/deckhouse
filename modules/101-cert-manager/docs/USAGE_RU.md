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

Подробнее можно прочитать [в документации cert-manager](https://cert-manager.io/docs/tutorials/acme/http-validation/).

## Заказ wildcard-сертификата с DNS в Cloudflare

1. Получим `Global API Key` и `Email Address`:
   * Заходим на страницу: <https://dash.cloudflare.com/profile>.
   * В самом верху страницы написана ваша почта под `Email Address`.
   * В самом низу страницы жмем на кнопку `View` напротив `Global API Key`.

   В результате этого мы получаем ключ для взаимодействия с API Cloudflare и почту, на которую зарегистрирован аккаунт.

2. Редактируем [настройки модуля cert-manager](configuration.html) и добавляем такую секцию:

   ```yaml
   settings:
     cloudflareGlobalAPIKey: APIkey
     cloudflareEmail: some@mail.somedomain
   ```

   или

   ```yaml
   settings:
     cloudflareAPIToken: some-token
     cloudflareEmail: some@mail.somedomain
   ```

   После этого Deckhouse автоматически создаст ClusterIssuer и Secret для Cloudflare в namespace `d8-cert-manager`.

   * Конфигурация с помощью [APIToken](https://cert-manager.io/docs/configuration/acme/dns01/cloudflare/#api-tokens) является рекомендуемой и более безопасной.

3. Создаем Certificate с проверкой с помощью провайдера Cloudflare. Данная возможность появится только при указании настройки `cloudflareGlobalAPIKey` и `cloudflareEmail` в Deckhouse:

   ```yaml
   apiVersion: cert-manager.io/v1
   kind: Certificate
   metadata:
     name: domain-wildcard
     namespace: app-namespace
   spec:
     secretName: tls-wildcard
     issuerRef:
       name: cloudflare
       kind: ClusterIssuer
     commonName: "*.domain.com"
     dnsNames:
     - "*.domain.com"
   ```

4. Создаем Ingress:

   ```yaml
   apiVersion: networking.k8s.io/v1
   kind: Ingress
   metadata:
     name: domain-wildcard
     namespace: app-namespace
   spec:
     ingressClassName: nginx
     rules:
     - host: "*.domain.com"
       http:
         paths:
         - backend:
             service:
               name: svc-web
               port:
                 number: 80
           path: /
     tls:
     - hosts:
       - "*.domain.com"
       secretName: tls-wildcard
   ```

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

{% alert level="info" %}
Пример создания самоподписанного сертификата вручную, без использования утилиты `cert-manager`, доступен в разделе [FAQ](../../deckhouse-faq.html#как-сгенерировать-самоподписанный-сертификат).
{% endalert %}
