---
title: "Модуль cert-manager: FAQ"
---


## Какие виды сертификатов поддерживаются?

На данный момент модуль устанавливает следующие `ClusterIssuer`:
* `letsencrypt`
* `letsencrypt-staging`
* `selfsigned`
* `selfsigned-no-trust`

Если требуется поддержка других типов сертификатов, вы можете добавить их самостоятельно.

## Как добавить дополнительный `ClusterIssuer`?

### В каких случаях требуется дополнительный `ClusterIssuer`?

В стандартной поставке присутствуют `ClusterIssuer`, издающие либо сертификаты из доверенного публичного удостоверяющего центра Let's Encrypt, либо самоподписанные сертификаты.

Чтобы издать сертификаты на доменное имя через Let's Encrypt, сервис требует осуществить подтверждение владения доменом.
`Cert-manager` поддерживает несколько методов для такого подтверждения при использовании `ACME`(Automated Certificate Management Environment):
* `HTTP-01` — `cert-manager` создаст временный Pod в кластере, который будет слушать на определенном URL для подтверждения владения доменом. Для его работы необходимо иметь возможность направлять внешний трафик на этот Pod, обычно через `Ingress`.
* `DNS-01` —  `cert-manager` делает TXT-запись в DNS для подтверждения владения доменом. У `cert-manager` есть встроенная поддержка популярных провайдеров DNS: AWS Route53, Google Cloud DNS, Cloudflare и т.д. Полный перечень доступен [в документации cert-manager](https://cert-manager.io/docs/configuration/acme/dns01/).

{% alert level="danger" %}
Метод `HTTP-01` не поддерживает выпуск wildcard-сертификатов.
{% endalert %}

Поставляемые `ClusterIssuers`, издающие сертификаты через Let's Encrypt, делятся на два типа:
1. `ClusterIssuer,` специфичные для используемого cloud-провайдера.  
Добавляются автоматически, при заполнении [настроек модуля](./configuration.html) связанных с cloud-провайдером. Поддерживают метод `DNS-01`.
   * `clouddns`
   * `cloudflare`
   * `digitalocean`
   * `route53`
1. `ClusterIssuer` использующие метод `HTTP-01`.  
   Добавляются автоматически, если их создание не отключено в [настройках модуля](./configuration.html#parameters-disableletsencrypt).
   * `letsencrypt`
   * `letsencrypt-staging`

Таким образом, дополнительный `ClusterIssuer` может потребоваться в случаях издания сертификатов:
1. В удостоверяющем центре (УЦ), отличном от Let's Encrypt (в т.ч. в приватном). Поддерживаемые УЦ доступны [в документации `cert-manager`](https://cert-manager.io/docs/configuration/acme/dns01/)
2. Через Let's Encrypt с помощью метода `DNS-01` через сторонний провайдер.

### Как добавить дополнительный `ClusterIssuer`, использующий Let's Encrypt и метод подтверждения `DNS-01`?

Для подтверждения владения доменом через Let's Encrypt с помощью метода `DNS-01` требуется настроить возможность создания TXT-записей в публичном DNS.

У `cert-manager` есть поддержка механизмов для создания TXT-записей в некоторых популярных DNS: `AzureDNS`, `Cloudflare`, `Google Cloud DNS` и т.д.  
Полный перечень доступен [в документации `cert-manager`](https://cert-manager.io/docs/configuration/acme/dns01/).

Модуль автоматически создает `ClusterIssuer` поддерживаемых cloud-провайдеров, при заполнении настроек модуля связанных с используемым облаком.  
При необходимости можно создать такие `ClusterIssuer` самостоятельно.  

Пример использования AWS Route53 доступен в разделе [Как защитить учетные данные `cert-manager`](#как-защитить-учетные-данные-cert-manager).  
Актуальный перечень всех возможных для создания `ClusterIssuer` доступен в [шаблонах модуля](https://github.com/deckhouse/deckhouse/tree/main/modules/101-cert-manager/templates/cert-manager).

Использование сторонних DNS-провайдеров реализуется через метод `webhook`.  
Когда `cert-manager` выполняет вызов `ACME` `DNS-01`, он отправляет запрос на вебхук-сервер, который затем выполняет нужные операции для обновления записи DNS.  
При использовании данного метода требуется разместить сервис, который будет обрабатывать хук и осуществлять создание TXT-записи в DNS-провайдере.

В качестве примера рассмотрим использование сервиса `Yandex Cloud DNS`.

1. Для обработки вебхука предварительно разместите в кластере сервис `Yandex Cloud DNS ACME webhook` согласно [официальной документации](https://github.com/yandex-cloud/cert-manager-webhook-yandex).

1. Затем создайте ресурс `ClusterIssuer`:

   ```yaml
   apiVersion: cert-manager.io/v1
   kind: ClusterIssuer
   metadata:
     name: yc-clusterissuer
     namespace: default
   spec:
     acme:
       # Вы должны заменить этот адрес электронной почты на свой собственный.
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
                 # Идентификатор папки, в которой расположена DNS-зона
                 folder: <your folder ID>
                 # Это секрет, используемый для доступа к учетной записи сервиса
                 serviceAccountSecretRef:
                   name: cert-manager-secret
                   key: iamkey.json
               groupName: acme.cloud.yandex.com
               solverName: yandex-cloud-dns
   ```

### Как добавить дополнительный `Issuer` и `ClusterIssuer`, использующий HashiCorp Vault для выпуска сертификатов?

Для выпуска сертификатов с помощью HashiCorp Vault, можете использовать [инструкцию](https://learn.hashicorp.com/tutorials/vault/kubernetes-cert-manager?in=vault/kubernetes).

После конфигурации PKI и [включения авторизации](../../modules/user-authz/) в Kubernetes, нужно:
- Создать `ServiceAccount` и скопировать ссылку на его `Secret`:

  ```shell
  d8 k create serviceaccount issuer
  
  ISSUER_SECRET_REF=$(d8 k get serviceaccount issuer -o json | jq -r ".secrets[].name")
  ```

- Создать `Issuer`:

  ```shell
  d8 k apply -f - <<EOF
  apiVersion: cert-manager.io/v1
  kind: Issuer
  metadata:
    name: vault-issuer
    namespace: default
  spec:
    vault:
      # Если Vault разворачивался по вышеуказанной инструкции, в этом месте в инструкции опечатка.
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

- Создать ресурс `Certificate` для получения TLS-сертификата, подписанного CA Vault:

  ```shell
  d8 k apply -f - <<EOF
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

### Как добавить `ClusterIssuer`, использующий свой или промежуточный CA для заказа сертификатов?

Для использования собственного или промежуточного CA:

- Сгенерируйте сертификат (при необходимости):

  ```shell
  openssl genrsa -out rootCAKey.pem 2048
  openssl req -x509 -sha256 -new -nodes -key rootCAKey.pem -days 3650 -out rootCACert.pem
  ```

- В пространстве имён `d8-cert-manager` создайте секрет, содержащий данные файлов сертификатов.
  Пример создания секрета с помощью команды d8 k:  

  ```shell
  d8 k create secret tls internal-ca-key-pair -n d8-cert-manager --key="rootCAKey.pem" --cert="rootCACert.pem"
  ```

  Пример создания секрета из YAML-файла (содержимое файлов сертификатов должно быть закодировано в Base64):  

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

  Имя секрета может быть любым.

- Создайте `ClusterIssuer` из созданного секрета:

  ```yaml
  apiVersion: cert-manager.io/v1
  kind: ClusterIssuer
  metadata:
    name: inter-ca
  spec:
    ca:
      secretName: internal-ca-key-pair    # Имя созданного секрета.
  ```

  Имя `ClusterIssuer` также может быть любым.

Теперь можно использовать созданный `ClusterIssuer` для получения сертификатов для всех компонентов Deckhouse или конкретного компонента.

Например, чтобы использовать `ClusterIssuer` для получения сертификатов для всех компонентов Deckhouse, укажите его имя в глобальном параметре [clusterIssuerName](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-https-certmanager-clusterissuername) (`d8 k edit mc global`):

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

## Как защитить учетные данные `cert-manager`?

Если вы не хотите хранить учетные данные конфигурации Deckhouse (например, по соображениям безопасности), можете создать
свой собственный `ClusterIssuer` / `Issuer`.

Пример создания собственного `ClusterIssuer` для сервиса [route53](https://aws.amazon.com/route53/):
- Создайте Secret с учетными данными:

  ```shell
  d8 k apply -f - <<EOF
  apiVersion: v1
  kind: Secret
  type: Opaque
  metadata:
    name: route53
    namespace: default
  data:
    secret-access-key: {{ "MY-AWS-ACCESS-KEY-TOKEN" | b64enc | quote }}
  EOF
  ```

- Создайте простой `ClusterIssuer` со ссылкой на этот Secret:

  ```shell
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
            accessKeyID: {{ "MY-AWS-ACCESS-KEY-ID" }}
            secretAccessKeySecretRef:
              name: route53
              key: secret-access-key
  EOF
  ```

- Закажите сертификаты как обычно, используя созданный `ClusterIssuer`:

  ```shell
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

## Работает ли старая аннотация TLS-acme?

Да, работает. Специальный компонент `cert-manager-ingress-shim` видит эти аннотации и на их основании автоматически создает ресурсы `Certificate` (в тех же namespaces, что и Ingress-ресурсы с аннотациями).

> **Важно!** При использовании аннотации ресурс Certificate создается «прилинкованным» к существующему Ingress-ресурсу, и для прохождения Challenge НЕ создается отдельный Ingress, а вносятся дополнительные записи в существующий. Это означает, что если на основном Ingress'е настроена аутентификация или whitelist — ничего не выйдет. Лучше не использовать аннотацию и переходить на ресурс Certificate.
>
> **Важно!** При переходе с аннотации на ресурс Certificate нужно удалить ресурс Certificate, который был создан по аннотации. Иначе по обоим ресурсам Certificate будет обновляться один Secret, и это может привести к достижению лимита запросов Let’s Encrypt.

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
  - host: admin.example.com                  # Еще один дополнительный домен.
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
    - admin.example.com                      # Еще один дополнительный домен.
    secretName: example-com-tls              # Имя для Certificate и Secret.
```

## Как посмотреть состояние сертификата?

```shell
d8 k -n default describe certificate example-com
...
Status:
  Acme:
    Authorizations:
      Account:  https://acme-v01.api.letsencrypt.org/acme/reg/22442061
      Domain:   example.com
      Uri:      https://acme-v01.api.letsencrypt.org/acme/challenge/qJA9MGCZnUnVjAgxhoxONvDnKAsPatRILJ4n0lJ7MMY/4062050823
      Account:  https://acme-v01.api.letsencrypt.org/acme/reg/22442061
      Domain:   admin.example.com
      Uri:      https://acme-v01.api.letsencrypt.org/acme/challenge/pW2tFKLBDTll2Gx8UBqmEl846x5W-YpBs8a4HqstJK8/4062050808
      Account:  https://acme-v01.api.letsencrypt.org/acme/reg/22442061
      Domain:   www.example.com
      Uri:      https://acme-v01.api.letsencrypt.org/acme/challenge/LaZJMM9_OKcTYbEThjT3oLtwgpkNfbHVdl8Dz-yypx8/4062050792
  Conditions:
    Last Transition Time:  2018-04-02T18:01:04Z
    Message:               Certificate issued successfully
    Reason:                CertIssueSuccess
    Status:                True
    Type:                  Ready
Events:
  Type     Reason                 Age                 From                     Message
  ----     ------                 ----                ----                     -------
  Normal   PrepareCertificate     1m                cert-manager-controller  Preparing certificate with issuer
  Normal   PresentChallenge       1m                cert-manager-controller  Presenting http-01 challenge for domain example.com
  Normal   PresentChallenge       1m                cert-manager-controller  Presenting http-01 challenge for domain www.example.com
  Normal   PresentChallenge       1m                cert-manager-controller  Presenting http-01 challenge for domain admin.example.com
  Normal   SelfCheck              1m                cert-manager-controller  Performing self-check for domain admin.example.com
  Normal   SelfCheck              1m                cert-manager-controller  Performing self-check for domain example.com
  Normal   SelfCheck              1m                cert-manager-controller  Performing self-check for domain www.example.com
  Normal   ObtainAuthorization    55s               cert-manager-controller  Obtained authorization for domain example.com
  Normal   ObtainAuthorization    54s               cert-manager-controller  Obtained authorization for domain admin.example.com
  Normal   ObtainAuthorization    53s               cert-manager-controller  Obtained authorization for domain www.example.com
```

## Как получить список сертификатов?

```shell
d8 k get certificate --all-namespaces

NAMESPACE          NAME                            AGE
default            example-com                     13m
```

## Что делать, если появляется ошибка: CAA record does not match issuer?

Если `cert-manager` не может заказать сертификаты с ошибкой:

```text
CAA record does not match issuer
```

то необходимо проверить `CAA (Certificate Authority Authorization)` DNS-запись у домена, для которого заказывается сертификат.
Если вы хотите использовать Let’s Encrypt-сертификаты, у домена должна быть CAA-запись: `issue "letsencrypt.org"`.
Подробнее про CAA можно почитать [в документации Let’s Encrypt](https://letsencrypt.org/docs/caa/).
