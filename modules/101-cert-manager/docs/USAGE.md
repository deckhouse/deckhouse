---
title: "Модуль cert-manager: примеры использования"
---


## Как заказать сертификат

Все настолько понятно и очевидно, на сколько это вообще может быть! Бери и используй:
```yaml
apiVersion: certmanager.k8s.io/v1alpha1
kind: Certificate
metadata:
  name: example-com                          # имя сертификата, через него потом можно смотреть статус
  namespace: default
spec:
  secretName: example-com-tls                # название секрета, в который положить приватный ключ и сертификат
  issuerRef:
    kind: ClusterIssuer                      # ссылка на "выдаватель" сертификатов, см. подробнее ниже
    name: letsencrypt
  commonName: example.com                    # основной домен сертификата
  dnsNames:                                  # дополнительыне домены сертификата, указывать не обязательно
  - www.example.com
  - admin.example.com
  acme:
    config:
    - http01:
        ingressClass: nginx                  # через какой ingress controller проходить chalenge
      domains:
      - example.com                          # список доменов, для которых проходить chalenge через этот
      - www.example.com                      # ingress controller
    - http01:
        ingressClass: nginx-aws-http
      domains:
      - admin.example.com                    # проходит chalenge через дополнительный ingress controller
```

При этом:
* создается отдельный Ingress-ресурс на время прохождения chalenge'а (соответственно аутентификация и whitelist основного Ingress не будут мешать),
* можно заказать один сертификат на несколько Ingress-ресурсов (и он не отвалится при удалении того, в котором была аннотация `tls-acme`),
* можно заказать сертификат с дополнительными именами (как в примере),
* можно валидировать разные домены, входящие в один сертификат, через разные Ingress-контроллеры.

Подробнее можно прочитать [здесь](https://cert-manager.io/docs/tutorials/acme/http-validation/).

## Как заказать wildcard сертификат с DNS в Cloudflare

1. Получим `Global API Key` и `Email Address`:
   * Заходим на страницу: https://dash.cloudflare.com/profile
   * В самом верху страницы написана ваша почта под `Email Address`
   * В самом низу страницы жмем на кнопку "View" напротив `Global API Key`

   В результате чего мы получаем ключ для взаимодействия с API Cloudflare и почту на которую зарегистрирован аккаунт.

2. Редактируем конфигурационный ConfigMap deckhouse, добавляя такую секцию:
   ```
   kubectl -n d8-system edit cm deckhouse
   ```

   ```yaml
   certManager: |
     cloudflareGlobalAPIKey: APIkey
     cloudflareEmail: some@mail.somedomain
   ```

   После чего, Deckhouse автоматически создаст ClusterIssuer и Secret для Cloudflare в namespace `d8-cert-manager`.

3. Создаем Certificate с проверкой с помощью провайдера Cloudflare. Данная возможность появится только при указании настройки `cloudflareGlobalAPIKey` и `cloudflareEmail` в Deckhouse:

   ```yaml
   apiVersion: certmanager.k8s.io/v1alpha1
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
     acme:
       config:
       - dns01:
           provider: cloudflare
         domains:
         - "*.domain.com"
   ```

4. Создаем Ingress:

   ```yaml
   apiVersion: extensions/v1beta1
   kind: Ingress
   metadata:
     annotations:
       kubernetes.io/ingress.class: nginx
     name: domain-wildcard
     namespace: app-namespace
   spec:
     rules:
     - host: "*.domain.com"
       http:
         paths:
         - backend:
             serviceName: svc-web
             servicePort: 80
           path: /
     tls:
     - hosts:
       - "*.domain.com"
       secretName: tls-wildcard
   ```

## Как заказать wildcard сертификат с DNS в Route53

1. Создаем пользователя с необходимыми правами.

   * Заходим на страницу управления политиками: https://console.aws.amazon.com/iam/home?region=us-east-2#/policies . Создаем политику с такими правами:

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

   * Заходим на страницу управления пользователями: https://console.aws.amazon.com/iam/home?region=us-east-2#/users . Создаем пользователя с созданной ранее политикой.

2. Редактируем ConfigMap Deckhouse, добавляя такую секцию:

   ```
   kubectl -n d8-system edit cm deckhouse
   ```

   ```yaml
   certManager: |
     route53AccessKeyID: AKIABROTAITAJMPASA4A
     route53SecretAccessKey: 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
   ```

   После чего, Deckhouse автоматически создаст ClusterIssuer и Secret для route53 в namespace `d8-cert-manager`.

3. Создаем Certificate с проверкой с помощью провайдера route53. Данная возможность появится только при указании настроек `route53AccessKeyID` и `route53SecretAccessKey` в Deckhouse:

   ```yaml
   apiVersion: certmanager.k8s.io/v1alpha1
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
     acme:
       config:
       - dns01:
           provider: route53
         domains:
         - "*.domain.com"
   ```

## Как заказать wildcard-сертификат с DNS в Google

1. Создаем сервис-аккаунт с необходимой ролью.

   * Заходим на страницу управления политиками: https://console.cloud.google.com/iam-admin/serviceaccounts.
   * Выбираем нужный проект.
   * Создаем сервис-аккаунт с желаемым названием, например `dns01-solver`.
   * Заходим в созданный сервис-аккаунт.
   * Создаём ключ по кнопке "Добавить ключ".
   * Будет скачан `.json`-файл с данными ключа имени.
   * Закодируем полученный файл в строку **base64**:
       ```bash
       base64 project-209317-556c656b81c4.json
       ```

2. Сохраняеем полученную **base64**-строку в параметр модуля `cloudDNSServiceAccount`.

   После чего, Deckhouse автоматически создаст ClusterIssuer и Secret для cloudDNS в namespace `d8-cert-manager`.

3. Создаем Certificate с валидацией через cloudDNS:

   ```yaml
   apiVersion: certmanager.k8s.io/v1alpha1
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
     acme:
       config:
       - dns01:
           provider: clouddns
         domains:
         - "*.domain.com"
   ```

## Как заказать selfsigned-сертификат

Все еще проще, чем с LetsEncypt. Просто меняем `letsencrypt` на `selfsigned`:

```yaml
apiVersion: certmanager.k8s.io/v1alpha1
kind: Certificate
metadata:
  name: example-com                          # имя сертификата, через него потом можно смотреть статус
  namespace: default
spec:
  secretName: example-com-tls                # название секрета, в который положить приватный ключ и сертификат
  issuerRef:
    kind: ClusterIssuer                      # ссылка на "выдаватель" сертификатов, см. подробнее ниже
    name: selfsigned
  commonName: example.com                    # основной домен сертификата
  dnsNames:                                  # дополнительыне домены сертификата, указывать не обязательно
  - www.example.com
  - admin.example.com
  acme:
    config:
    - http01:
        ingressClass: nginx                  # через какой ingress controller проходить chalenge
      domains:
      - example.com                          # список доменов, для которых проходить chalenge через этот
      - www.example.com                      # ingress controller
    - http01:
        ingressClass: nginx-aws-http
      domains:
      - admin.example.com                    # проходит chalenge через дополнительный ingress controller
```