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

## Заказ wildcard-сертификата с DNS в Route53

1. Создаем пользователя с необходимыми правами.

   * Заходим на [страницу управления политиками](https://console.aws.amazon.com/iam/home?region=us-east-2#/policies). Создаем политику с такими правами:

     ```json
     {
         "Version": "2012-10-17",
         "Statement": [
             {
                 "Effect": "Allow",
                 "Action": "route53:GetChange",
                 "Resource": "arn:aws:route53:::change/*"
             },
             {
                 "Effect": "Allow",
                 "Action": "route53:ChangeResourceRecordSets",
                 "Resource": "arn:aws:route53:::hostedzone/*"
             },
             {
                 "Effect": "Allow",
                 "Action": "route53:ListHostedZonesByName",
                 "Resource": "*"
             }
         ]
     }
     ```

   * Заходим на [страницу управления пользователями](https://console.aws.amazon.com/iam/home?region=us-east-2#/users). Создаем пользователя с созданной ранее политикой.

2. Редактируем [настройки модуля cert-manager](configuration.html) и добавляем такую секцию:

   ```yaml
   settings:
     route53AccessKeyID: AKIABROTAITAJMPASA4A
     route53SecretAccessKey: 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
   ```

   После этого Deckhouse автоматически создаст ClusterIssuer и Secret для route53 в namespace `d8-cert-manager`.

3. Создаем Certificate с проверкой с помощью провайдера route53. Данная возможность появится только при указании настроек `route53AccessKeyID` и `route53SecretAccessKey` в Deckhouse:

   ```yaml
   apiVersion: cert-manager.io/v1
   kind: Certificate
   metadata:
     name: domain-wildcard
     namespace: app-namespace
   spec:
     secretName: tls-wildcard
     issuerRef:
       name: route53
       kind: ClusterIssuer
     commonName: "*.domain.com"
     dnsNames:
     - "*.domain.com"
   ```

## Заказ wildcard-сертификата с DNS в Google

1. Создаем ServiceAccount с необходимой ролью:

   * Заходим на [страницу управления политиками](https://console.cloud.google.com/iam-admin/serviceaccounts).
   * Выбираем нужный проект.
   * Создаем ServiceAccount с желаемым названием, например `dns01-solver`.
   * Заходим в созданный ServiceAccount.
   * Создаем ключ по кнопке «Добавить ключ».
   * Будет скачан `.json`-файл с данными ключа имени.
   * Закодируем полученный файл в строку **base64**:

     ```shell
     base64 project-209317-556c656b81c4.json
     ```

2. Сохраняеем полученную **base64**-строку в параметр модуля `cloudDNSServiceAccount`.

   После этого Deckhouse автоматически создаст ClusterIssuer и Secret для cloudDNS в namespace `d8-cert-manager`.

3. Создаем Certificate с валидацией через cloudDNS:

   ```yaml
   apiVersion: cert-manager.io/v1
   kind: Certificate
   metadata:
     name: domain-wildcard
     namespace: app-namespace
   spec:
     secretName: tls-wildcard
     issuerRef:
       name: clouddns
       kind: ClusterIssuer
     dnsNames:
     - "*.domain.com"
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
    kind: ClusterIssuer                      # Ссылка на "выдаватель" сертификатов, см. подробнее ниже.
    name: selfsigned
  commonName: example.com                    # Основной домен сертификата.
  dnsNames:                                  # Дополнительные домены сертификата, указывать необязательно.
  - www.example.com
  - admin.example.com
```
