---
title: Использование TLS-сертификатов
permalink: ru/user/security/tls.html
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
а также рекомендации по их настройке приведены [на странице «Управление сертификатами»](../../admin/configuration/security/certificates.html).
{% endalert %}

## Работа с сертификатами

### Получение информации о сертификатах

- Чтобы вывести список всех сертификатов в кластере, используйте следующую команду:

  ```shell
  d8 k get certificate --all-namespaces
  ```

- Чтобы проверить статус конкретного сертификата, воспользуйтесь следующей командой:

  ```shell
  d8 k -n <NAMESPACE> describe certificate <CERTIFICATE-NAME>
  ```

### Автоматический заказ сертификата

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

Подробнее про CAA-записи можно почитать [в документации Let's Encrypt](https://letsencrypt.org/docs/caa/).
{% endalert %}

#### Заказ wildcard-сертификата с DNS в Cloudflare

1. Получите `GlobalAPIKey` и `Email`:
   - зайдите на страницу [`dash.cloudflare.com/profile`](https://dash.cloudflare.com/profile);
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
     route53AccessKeyID: <ACCESS_KEY_ID>
     route53SecretAccessKey: <SECRET_ACCESS_KEY>
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

1. Сохраните полученную Base64-строку в [параметре `cloudDNSServiceAccount`](/modules/cert-manager/configuration.html#parameters-clouddnsserviceaccount).

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

#### Заказ самоподписанного сертификата

Чтобы заказать самоподписанный сертификат, укажите `selfsigned` в качестве имени издателя в поле `issuerRef.name`:

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
    name: selfsigned
  commonName: example.com      # Основной домен сертификата.
  dnsNames:                    # Опциональные дополнительные домены сертификата (как минимум одно DNS-имя или IP-адрес).
  - www.example.com
  - admin.example.com
```

### Создание самоподписанного сертификата

При самостоятельной генерации сертификатов важно корректно заполнить все поля запроса на сертификат, чтобы итоговый сертификат был правильно издан и гарантированно проходил валидацию в различных сервисах.  

Важно придерживаться следующих правил:

1. Указывать доменные имена в поле `SAN` (Subject Alternative Name).

   Поле `SAN` является более современным и распространенным методом указания доменных имен, на которые распространяется сертификат.
   Некоторые сервисы на данный момент уже не рассматривают поле `CN` (Common Name) как источник для доменных имен.

2. Корректно заполнять поля `keyUsage`, `basicConstraints`, `extendedKeyUsage`, а именно:
   - `basicConstraints = CA:FALSE`  

     Данное поле определяет, относится ли сертификат к конечному пользователю (end-entity certificate) или к центру сертификации (CA certificate). CA-сертификат не может использоваться в качестве сертификата сервиса.

   - `keyUsage = digitalSignature, keyEncipherment`  

     Поле `keyUsage` ограничивает допустимые сценарии использования данного ключа:

     - `digitalSignature` — позволяет использовать ключ для подписи цифровых сообщений и обеспечения целостности соединения.
     - `keyEncipherment` — позволяет использовать ключ для шифрования других ключей, что необходимо для безопасного обмена данными с помощью TLS (Transport Layer Security).

   - `extendedKeyUsage = serverAuth`  

     Поле `extendedKeyUsage` уточняет дополнительные сценарии использования ключа, которые могут требоваться конкретными протоколами или приложениями:

     - `serverAuth` — указывает, что сертификат предназначен для использования на сервере для аутентификации сервера перед клиентом в процессе установления защищенного соединения.

Также рекомендуется:

1. Издать сертификат на срок не более 1 года (365 дней).

   Срок действия сертификата влияет на его безопасность. Срок в 1 год позволяет обеспечить актуальность криптографических методов и своевременно обновлять сертификаты в случае возникновения угроз.
   Также некоторые современные браузеры на текущий момент отвергают сертификаты со сроком действия более 1 года.

2. Использовать стойкие криптографические алгоритмы, например, алгоритмы на основе эллиптических кривых (в т.ч. `prime256v1`).

   Алгоритмы на основе эллиптических кривых (ECC) предоставляют высокий уровень безопасности при меньшем размере ключа по сравнению с традиционными методами, такими как RSA. Это делает сертификаты более эффективными по производительности и безопасными в долгосрочной перспективе.

3. Не указывать домены в поле `CN` (Common Name).

   Исторически поле `CN` использовалось для указания основного доменного имени, для которого выдается сертификат. Однако современные стандарты, такие как [RFC 2818](https://datatracker.ietf.org/doc/html/rfc2818), рекомендуют использовать поле `SAN` (Subject Alternative Name) для этой цели.
   Если сертификат распространяется на несколько доменных имен, указанных в поле `SAN`, то при дополнительном указании одного из доменов в `CN` в некоторых сервисах может возникнуть ошибка валидации при обращении к домену, не указанному в `CN`.
   Если указывать в `CN` информацию, не относящуюся напрямую к доменным именам (например, идентификатор или имя сервиса), то сертификат также будет распространяться на эти имена, что может быть использовано для вредоносных целей.

#### Пример создания сертификата

Для генерации сертификата воспользуемся утилитой `openssl`.

1. Заполните конфигурационный файл `cert.cnf`:

   ```ini
   [ req ]
   default_bits       = 2048
   default_md         = sha256
   prompt             = no
   distinguished_name = dn
   req_extensions     = req_ext

   [ dn ]
   C = RU
   ST = Moscow
   L = Moscow
   O = Example Company
   OU = IT Department
   # CN = Не указывайте поле CN.

   [ req_ext ]
   subjectAltName = @alt_names

   [ alt_names ]
   # Укажите все доменные имена.
   DNS.1 = example.com
   DNS.2 = www.example.com
   DNS.3 = api.example.com
   # Укажите IP-адреса (если требуется).
   IP.1 = 192.0.2.1
   IP.2 = 192.0.4.1

   [ v3_ca ]
   basicConstraints = CA:FALSE
   keyUsage = digitalSignature, keyEncipherment
   extendedKeyUsage = serverAuth

   [ v3_req ]
   basicConstraints = CA:FALSE
   keyUsage = digitalSignature, keyEncipherment
   extendedKeyUsage = serverAuth
   subjectAltName = @alt_names

   # Параметры эллиптических кривых.
   [ ec_params ]
   name = prime256v1
   ```

2. Сгенерируйте ключ на основе эллиптических кривых:

   ```shell
   openssl ecparam -genkey -name prime256v1 -noout -out ec_private_key.pem
   ```

3. Создайте запрос на сертификат:

   ```shell
   openssl req -new -key ec_private_key.pem -out example.csr -config cert.cnf
   ```

4. Сгенерируйте самоподписанный сертификат:

   ```shell
   openssl x509 -req -in example.csr -signkey ec_private_key.pem -out example.crt -days 365 -extensions v3_req -extfile cert.cnf
   ```

## Защита учётных данных

Если вы не хотите хранить учётные данные в конфигурации DKP,
можно создать отдельный Secret и ссылаться на него в ресурсе ClusterIssuer.

Для этого выполните следующее:

1. Создайте Secret с ключом доступа:

   ```yaml
   d8 k apply -f - <<EOF
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
   d8 k apply -f - <<EOF
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
   d8 k apply -f - <<EOF
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
что может привести превышению лимита запросов Let's Encrypt.
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
