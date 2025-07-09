---
title: Использование TLS-сертификатов
permalink: ru/user/tls.html
lang: ru
---

Deckhouse Kubernetes Platform (DKP) предоставляет встроенные средства управления TLS-сертификатами,
упрощающие настройку и управление шифрованием трафика в приложениях, работающих в кластере.

На этой странице описаны следующие аспекты использования сертификатов в DKP:

- как вручную заказывать TLS-сертификаты с помощью ресурсов Certificate и ClusterIssuer;
- как безопасно хранить и использовать учётные данные для доступа к удостоверяющим центрам (CA);
- как автоматически получать сертификаты с помощью аннотации `tls-acme` в ресурсах Ingress.

{% alert level="info" %}
Общее описание порядка управления сертификатами в DKP, список поддерживаемых издателей,
а также рекомендации по их настройке приведены [на странице «Управление сертификатами»](../admin/configuration/security/certificates.html).
{% endalert %}

## Работа с сертификатами

### Получение информации о сертификатах

- Чтобы вывести список всех сертификатов в кластере, используйте следующую команду:

  ```shell
  kubectl get certificate --all-namespaces
  ```

- Чтобы проверить статус конкретного сертификата, воспользуйтесь следующей командой:

  ```shell
  kubectl -n <NAMESPACE> describe certificate <CERTIFICATE-NAME>
  ```

### Заказ сертификата

Чтобы заказать выпуск сертификата `letsencrypt`, выполните следующие шаги:

1. Создайте ресурс Certificate, опираясь [на документацию `cert-manager`](https://cert-manager.io/docs/usage/certificate/).
   Сверяйтесь с примером ниже:

   ```yaml
   apiVersion: cert-manager.io/v1
   kind: Certificate
   metadata:
     name: example-com            # Имя сертификата.
     namespace: default
   spec:
     secretName: example-com-tls  # Имя Secret, в котором будет сохранён приватный ключ и сертификат.
     issuerRef:
       kind: ClusterIssuer        # Данные об издателе сертификата.
       name: letsencrypt
     commonName: example.com      # Основной домен сертификата.
     dnsNames:                    # Опциональные дополнительные домены сертификата (как минимум одно DNS-имя или IP-адрес).
     - www.example.com
     - admin.example.com
   ```

1. Модуль `cert-manager` автоматически запустит проверку владения доменом (challenge) с использованием метода,
   указанного в ресурсе ClusterIssuer — например, `HTTP-01` или `DNS-01`.
1. Модуль `cert-manager` автоматически создаст временный ресурс Ingress для проверки владения доменом.
   Временный ресурс не влияет на работу основного Ingress-ресурса.
1. После успешной проверки выпущенный сертификат будет сохранён в Secret, указанный в поле `secretName`.

{% alert level="info" %}
Если в процессе заказа сертификата выводится ошибка `CAA record does not match issuer`,
проверьте DNS-записи домена, для которого заказывается сертификат.
Для использования сертификата `letsencrypt` у домена должна быть следующая CAA-запись: `issue "letsencrypt.org"`.

Подробнее про CAA-записи можно почитать [в документации Let’s Encrypt](https://letsencrypt.org/docs/caa/).
{% endalert %}

#### Заказ wildcard-сертификата с DNS в Cloudflare

1. Получите `GlobalAPIKey` и `Email`:
   - зайдите на страницу <https://dash.cloudflare.com/profile>;
   - ваша почта указана наверху под **Email Address**;
   - для просмотра API-ключа нажмите **View** напротив **Global API Key** внизу страницы.

1. Отредактируйте [настройки модуля `cert-manager`](/modules/cert-manager/configuration.html), добавив следующую секцию:

   ```yaml
   settings:
     cloudflareGlobalAPIKey: APIkey
     cloudflareEmail: some@mail.somedomain
   ```

   или указав [API-токен](https://cert-manager.io/docs/configuration/acme/dns01/cloudflare/#api-tokens) вместо ключа (рекомендуемый вариант):

   ```yaml
   settings:
     cloudflareAPIToken: some-token
     cloudflareEmail: some@mail.somedomain
   ```

   После этого DKP автоматически создаст ClusterIssuer и Secret для Cloudflare в пространстве имён `d8-cert-manager`.

1. Создайте ресурс Certificate с проверкой с помощью провайдера Cloudflare.
   Данная возможность появится только при указании настройки `cloudflareGlobalAPIKey` и `cloudflareEmail` в DKP:

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

1. Создайте ресурс Ingress:

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

#### Заказ wildcard-сертификата с DNS в AWS Route53

1. Создайте пользователя с необходимыми правами:

   - зайдите на [страницу управления политиками](https://console.aws.amazon.com/iam/home?region=us-east-2#/policies) и создайте политику со следующими правами:

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

   - зайдите на [страницу управления пользователями](https://console.aws.amazon.com/iam/home?region=us-east-2#/users) и добавьте пользователя с созданной ранее политикой.

1. Отредактируйте [настройки модуля `cert-manager`](/modules/cert-manager/configuration.html), добавив следующую секцию:

   ```yaml
   settings:
     route53AccessKeyID: AKIABROTAITAJMPASA4A
     route53SecretAccessKey: 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
   ```

   После этого Deckhouse автоматически создаст ClusterIssuer и Secret для Route53 в пространстве имён `d8-cert-manager`.

1. Создайте ресурс Certificate с проверкой с помощью провайдера Route53.
   Данная возможность появится только при указании настроек `route53AccessKeyID` и `route53SecretAccessKey` в DKP:

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

#### Заказ wildcard-сертификата с DNS в Google

1. Создайте ServiceAccount с необходимой ролью:

   - зайдите на [страницу управления политиками](https://console.cloud.google.com/iam-admin/serviceaccounts);
   - выберите нужный проект и создайте ServiceAccount с желаемым названием, например `dns01-solver`;
   - зайдите в созданный ServiceAccount и создайте ключ, нажав на **Добавить ключ**.
     Будет скачан JSON-файл с данными ключа;
   - закодируйте полученный файл в строку формата Base64:

     ```shell
     base64 project-209317-556c656b81c4.json
     ```

1. Сохраните полученную Base64-строку в параметре `cloudDNSServiceAccount`(/modules/cert-manager/configuration.html#parameters-clouddnsserviceaccount).

   После этого Deckhouse автоматически создаст ClusterIssuer и Secret для CloudDNS в пространстве имён `d8-cert-manager`.

1. Создайте ресурс Certificate с валидацией через CloudDNS:

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

## Защита учётных данных

Если вы не хотите хранить учётные данные в конфигурации DKP,
можно создать отдельный Secret и ссылаться на него в ресурсе ClusterIssuer.

Для этого выполните следующее:

1. Создайте Secret с ключом доступа:

   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: v1
   kind: Secret
   type: Opaque
   metadata:
     name: route53
     namespace: default
   data:
     secret-access-key: MY-AWS-ACCESS-KEY-TOKEN
   EOF
   ```

1. Создайте ресурс ClusterIssuer со ссылкой на этот Secret:

   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: cert-manager.io/v1
   kind: ClusterIssuer
   metadata:
     name: route53
     namespace: default
   spec:
     acme:
       server: https://acme-v02.api.letsencrypt.org/directory
       privateKeySecretRef:
         name: route53-tls-key
       solvers:
       - dns01:
           route53:
             region: us-east-1
             accessKeyID: MY-AWS-ACCESS-KEY-ID
             secretAccessKeySecretRef:
               name: route53
               key: secret-access-key
   EOF
   ```

1. Закажите сертификаты как обычно, используя созданный ClusterIssuer:

   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: cert-manager.io/v1
   kind: Certificate
   metadata:
     name: example-com
     namespace: default
   spec:
     secretName: example-com-tls
     issuerRef:
       name: route53
     commonName: www.example.com 
     dnsNames:
     - www.example.com
   EOF
   ```

## Поддержка аннотации tls-acme

DKP поддерживает аннотацию `kubernetes.io/tls-acme: "true"` в ресурсах Ingress.
Компонент `cert-manager-ingress-shim` следит за появлением аннотации
и автоматически создаёт ресурсы Certificate в тех же пространствах имён, что и Ingress-ресурсы.

{% alert level="warning" %}
При использовании аннотации ресурс Certificate создается в связке с существующим Ingress-ресурсом.
Для подтверждения владения доменом (challenge) не создаётся отдельный Ingress, а вносятся дополнительные записи в существующий.
Следовательно, если на основном ресурсе Ingress настроена аутентификация или whitelist,
попытка подтверждения окончится неудачей.
По этой причине вместо аннотации рекомендуется использовать ресурс Certificate напрямую.

При переходе с аннотации на Certificate удалите ресурс Certificate, который был автоматически создан с аннотацией.
Иначе по обоим ресурсам Certificate будет обновляться один Secret,
что может привести превышению лимита запросов Let’s Encrypt.
{% endalert %}

Пример конфигурации ресурса Ingress с аннотацией:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    kubernetes.io/tls-acme: "true"           # Аннотация.
  name: example-com
  namespace: default
spec:
  ingressClassName: nginx
  rules:
  - host: example.com
    http:
      paths:
      - backend:
          service:
            name: site
            port:
              number: 80
        path: /
        pathType: ImplementationSpecific
  - host: www.example.com                    # Дополнительный домен.
    http:
      paths:
      - backend:
          service:
            name: site
            port:
              number: 80
        path: /
        pathType: ImplementationSpecific
  - host: admin.example.com                  # Ещё один дополнительный домен.
    http:
      paths:
      - backend:
          service:
            name: site
            port:
              number: 80
        path: /
        pathType: ImplementationSpecific
  tls:
  - hosts:
    - example.com
    - www.example.com                        # Дополнительный домен.
    - admin.example.com                      # Ещё один дополнительный домен.
    secretName: example-com-tls              # Имя для Certificate и Secret.
```


