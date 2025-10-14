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
* `DNS-01` —  `cert-manager` делает TXT-запись в DNS для подтверждения владения доменом. У `cert-manager` есть встроенная поддержка популярных провайдеров DNS.

{% alert level="danger" %}
Метод `HTTP-01` не поддерживает выпуск wildcard-сертификатов.
{% endalert %}

Поставляемые `ClusterIssuers`, издающие сертификаты через Let's Encrypt, делятся на два типа:
1. `ClusterIssuer,` специфичные для используемого cloud-провайдера.  
1. `ClusterIssuer` использующие метод `HTTP-01`.  
   Добавляются автоматически, если их создание не отключено в [настройках модуля](./configuration.html#parameters-disableletsencrypt).
   * `letsencrypt`
   * `letsencrypt-staging`

Таким образом, дополнительный `ClusterIssuer` может потребоваться в случаях издания сертификатов:
1. В удостоверяющем центре (УЦ), отличном от Let's Encrypt (в т.ч. в приватном).
2. Через Let's Encrypt с помощью метода `DNS-01` через сторонний провайдер.

### Как добавить дополнительный `Issuer` и `ClusterIssuer`, использующий HashiCorp Vault для выпуска сертификатов?

После конфигурации PKI и [включения авторизации](/modules/user-authz/), нужно:
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

Например, чтобы использовать `ClusterIssuer` для получения сертификатов для всех компонентов Deckhouse, укажите его имя в глобальном параметре [clusterIssuerName](/reference/api/global.html#parameters-modules-https-certmanager-clusterissuername) (`d8 k edit mc global`):

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

- Создайте Secret с учетными данными:

  ```shell
  d8 k apply -f - <<EOF
  apiVersion: v1
  kind: Secret
  type: Opaque
  metadata:
    name: XXX
    namespace: default
  data:
    secret-access-key: {{ "MY-ACCESS-KEY-TOKEN" | b64enc | quote }}
  EOF
  ```

- Создайте простой `ClusterIssuer` со ссылкой на этот Secret:

  ```shell
  d8 k apply -f - <<EOF
  apiVersion: cert-manager.io/v1
  kind: ClusterIssuer
  metadata:
    name: XXX
    namespace: default
  spec:
    acme:
      server: https://acme-v02.api.letsencrypt.org/directory
      privateKeySecretRef:
        name: tls-key
      solvers:
      - dns01:
          <solver>:
            region: us-east-1
            accessKeyID: {{ "MY-ACCESS-KEY-ID" }}
            secretAccessKeySecretRef:
              name: XXX
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
      name: XXX
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
