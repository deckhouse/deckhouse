---
title: Управление сертификатами
permalink: ru/admin/security/certificates.html
lang: ru
---

Deckhouse Kubernetes Platform (DKP) предоставляет доступ к [`cert-manager` версии v1.17.1](https://github.com/jetstack/cert-manager),
инструменту автоматизации при работе с TLS-сертификатами в кластере.

При установке `cert-manager` в кластер учитываются особенности инфраструктуры:

- вебхук-компонент, к которому обращается `kube-apiserver`, размещается на master-узлах;
- при недоступности вебхука временно удаляется `apiservice`, чтобы не блокировать работу кластера;
- обновление инструмента `cert-manager` и миграция его ресурсов происходят автоматически.

## Возможности DKP по управлению сертификатами

DKP поддерживает все возможности оригинального `cert-manager`, включая:

- заказ сертификатов во всех поддерживаемых источниках, таких как [Let’s Encrypt](https://letsencrypt.org/), [HashiCorp Vault](https://developer.hashicorp.com/vault), [Venafi](https://docs.venafi.com/);
- выпуск самоподписанных сертификатов;
- автоматический перевыпуск и контроль срока действия сертификатов;
- установку `cm-acme-http-solver` на master-узлы и выделенные узлы.

## Мониторинг

DKP экспортирует метрики в Prometheus, что позволяет отслеживать:

- срок действия сертификатов;
- статус перевыпуска сертификатов.

## Роли доступа

В DKP предопределены несколько ролей для доступа к ресурсам:

| Роль       | Права доступа |
| ---------- | ------------- |
| `User`     | Просмотр ресурсов Certificate и Issuer в доступных пространствах имён, а также глобальных ресурсов ClusterIssue. |
| `Editor`   | Управление ресурсами Certificate и Issuer в доступных пространствах имён. |
| `ClusterEditor` | Управление ресурсами Certificate и Issuer во всех пространствах имён. |
| `SuperAdmin` | Управление внутренними служебными объектами. |

## Работа с издателями сертификатов

В DKP по умолчанию поддерживаются следующие издатели сертификатов (ClusterIssuer):

- `letsencrypt`;
- `letsencrypt-staging`;
- `selfsigned`;
- `selfsigned-no-trust`.

В некоторых случаях вам могут понадобиться дополнительные виды ClusterIssuer:

- если вы хотите использовать сертификат от Let’s Encrypt, но с DNS-валидацией через стороннего DNS-провайдера;
- когда необходимо использовать удостоверяющий центр (CA), отличный от Let's Encrypt.
  Все виды поддерживаемых удостоверяющих центров перечислены [в документации `cert-manager`](https://cert-manager.io/docs/configuration/acme/dns01/).

### Добавление ClusterIssuer с валидацией `DNS-01` через вебхук

Для подтверждения владения доменом через Let’s Encrypt с помощью метода `DNS-01` необходимо,
чтобы `cert-manager` мог создавать TXT-записи в зоне DNS, связанной с доменом.
У `cert-manager` есть встроенная поддержка популярных DNS-провайдеров,
таких как AWS Route53, Google Cloud DNS, Cloudflare и других.
Полный перечень доступен [в официальной документации cert-manager](https://cert-manager.io/docs/configuration/acme/dns01/).

Если провайдер не поддерживается напрямую,
можно настроить вебхук и разместить в кластере собственный обработчик ACME-запросов,
который будет выполнять нужные операции для обновления DNS-записей.

Данный пример основан на использовании сервиса Yandex Cloud DNS:

1. Для обработки вебхука разместите в кластере сервис `Yandex Cloud DNS ACME webhook`
   согласно [официальной документации](https://github.com/yandex-cloud/cert-manager-webhook-yandex).
1. Cоздайте ресурс ClusterIssuer, следуя примеру:

   ```yaml
   apiVersion: cert-manager.io/v1
   kind: ClusterIssuer
   metadata:
     name: yc-clusterissuer
     namespace: default
   spec:
     acme:
       # Заменить этот адрес электронной почты на свой собственный.
       # Let's Encrypt будет использовать его, чтобы связаться с вами по поводу истекающих
       # сертификатов и вопросов, связанных с вашей учетной записью.
       email: your@email.com
       server: https://acme-staging-v02.api.letsencrypt.org/directory
       privateKeySecretRef:
         # Ресурс секретов, который будет использоваться для хранения закрытого ключа аккаунта.
         name: secret-ref
       solvers:
         - dns01:
             webhook:
               config:
                 # Идентификатор папки, в которой расположена DNS-зона.
                 folder: <your-folder-ID>
                 # Секрет, используемый для доступа к учетной записи сервиса.
                 serviceAccountSecretRef:
                   name: cert-manager-secret
                   key: iamkey.json
               groupName: acme.cloud.yandex.com
               solverName: yandex-cloud-dns
   ```

### Добавление ClusterIssuer, использующего собственный удостоверяющий центр (CA)

1. Сгенерируйте сертификат:

   ```shell
   openssl genrsa -out rootCAKey.pem 2048
   openssl req -x509 -sha256 -new -nodes -key rootCAKey.pem -days 3650 -out rootCACert.pem
   ```

1. В пространстве имён `d8-cert-manager` создайте секрет с произвольным именем, содержащий данные файлов сертификатов.

   - Пример создания секрета с помощью команды `kubectl`:

     ```shell
     kubectl create secret tls internal-ca-key-pair -n d8-cert-manager --key="rootCAKey.pem" --cert="rootCACert.pem"
     ```

   - Пример создания секрета из YAML-файла (содержимое файлов сертификатов должно быть закодировано в Base64):

     ```yaml
     apiVersion: v1
     data:
       tls.crt: <результат команды `cat rootCACert.pem | base64 -w0`>
       tls.key: <результат команды `cat rootCAKey.pem | base64 -w0`>
     kind: Secret
     metadata:
       name: internal-ca-key-pair
       namespace: d8-cert-manager
     type: Opaque
     ```

1. Создайте ClusterIssuer с произвольным именем из созданного секрета:

   ```yaml
   apiVersion: cert-manager.io/v1
   kind: ClusterIssuer
   metadata:
     name: inter-ca
   spec:
     ca:
       secretName: internal-ca-key-pair    # Имя созданного секрета.
   ```

Теперь можно использовать созданный ClusterIssuer для получения сертификатов
для всех компонентов DKP или конкретного компонента.

Например, чтобы использовать ClusterIssuer для получения сертификатов для всех компонентов DKP,
укажите его имя в глобальном параметре `clusterIssuerName`(#TODO):

```yaml
  spec:
    settings:
      modules:
        https:
          certManager:
            clusterIssuerName: inter-ca
          mode: CertManager
        publicDomainTemplate: '%s.<public_domain_template>'
    version: 1
```

### Добавление Issuer и ClusterIssuer, использующих HashiСorp Vault для заказа сертификатов

Для настройки заказа сертификатов с помощью Vault используйте [документацию HashiСorp](https://developer.hashicorp.com/vault/tutorials/archive/kubernetes-cert-manager?in=vault%2Fkubernetes).

После настройки PKI и включения авторизации в Kubernetes(#TODO), выполните следующее:

1. Создайте ServiceAccount и скопируйте ссылку на его Secret:

   ```shell
   kubectl create serviceaccount issuer
     
   ISSUER_SECRET_REF=$(kubectl get serviceaccount issuer -o json | jq -r ".secrets[].name")
   ```

1. Создайте ресурс Issuer:

   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: cert-manager.io/v1
   kind: Issuer
   metadata:
     name: vault-issuer
     namespace: default
   spec:
     vault:
       server: http://vault.default.svc.cluster.local:8200
       # Указывается на этапе конфигурации PKI.
       path: pki/sign/example-dot-com 
       auth:
         kubernetes:
           mountPath: /v1/auth/kubernetes
           role: issuer
           secretRef:
             name: $ISSUER_SECRET_REF
             key: token
   EOF
   ```

1. Создайте ресурс Certificate для получения TLS-сертификата, подписанного CA Vault:

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
       name: vault-issuer
     # Домены указываются на этапе конфигурации PKI в Vault.
     commonName: www.example.com 
     dnsNames:
     - www.example.com
   EOF
   ```

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

1. `Cert-manager` автоматически запустит проверку владения доменом (challenge) с использованием метода,
   указанного в ресурсе ClusterIssuer — например, `HTTP-01` или `DNS-01`.
1. `Cert-manager` автоматически создаст временный ресурс Ingress для проверки владения доменом.
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

1. Отредактируйте настройки `cert-manager`(#TODO), добавив следующую секцию:

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

1. Отредактируйте настройки `cert-manager`(#TODO), добавив следующую секцию:

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

1. Сохраните полученную Base64-строку в параметре `cloudDNSServiceAccount`(#TODO).

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
