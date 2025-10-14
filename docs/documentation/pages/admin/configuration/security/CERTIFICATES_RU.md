---
title: Управление сертификатами
permalink: ru/admin/configuration/security/certificates.html
description: "Управление TLS-сертификатами в Deckhouse Kubernetes Platform с интеграцией Let's Encrypt, HashiCorp Vault и Venafi. Автоматическое обновление, мониторинг и управление жизненным циклом сертификатов."
lang: ru
search: certificate management, TLS certificates, SSL certificates, certificate automation, Let's Encrypt, управление сертификатами, TLS сертификаты, SSL сертификаты, автоматизация сертификатов
---

Deckhouse Kubernetes Platform (DKP) предоставляет встроенные средства управления TLS-сертификатами в кластере
и поддерживает:

- заказ сертификатов во всех поддерживаемых источниках, таких как [Let’s Encrypt](https://letsencrypt.org/), [HashiCorp Vault](https://developer.hashicorp.com/vault), [Venafi](https://docs.venafi.com/);
- выпуск самоподписанных сертификатов;
- автоматический перевыпуск и контроль срока действия сертификатов;
- установку `cm-acme-http-solver` на master-узлы и выделенные узлы.

На этой странице описаны доступные в DKP возможности управления сертификатами,
а также порядок работы с издателями сертификатов.

{% alert level="info" %}
Описание примеров настройки сертификатов, порядок работы с аннотацией `tls-acme` и способы безопасного использования учётных данных приведены [на странице «Использование TLS-сертификатов»](../../../user/security/tls.html).
{% endalert %}

## Мониторинг

DKP экспортирует метрики в Prometheus, что позволяет отслеживать:

- срок действия сертификатов;
- статус перевыпуска сертификатов.

## Роли доступа

В DKP предопределены несколько ролей для доступа к ресурсам:

| Роль       | Права доступа |
| ---------- | ------------- |
| `User`     | Просмотр ресурсов Certificate и Issuer в доступных пространствах имён, а также глобальных ресурсов ClusterIssuer. |
| `Editor`   | Управление ресурсами Certificate и Issuer в доступных пространствах имён. |
| `ClusterEditor` | Управление ресурсами Certificate и Issuer во всех пространствах имён. |
| `SuperAdmin` | Управление внутренними служебными объектами. |

## Работа с издателями сертификатов

В DKP по умолчанию поддерживаются следующие издатели сертификатов (ClusterIssuer):

- `letsencrypt` — выпускает TLS-сертификаты, используя публичный удостоверяющий центр Let’s Encrypt
  и HTTP-валидацию по протоколу ACME.
  Используется для автоматического получения доверенных сертификатов, подходящих для большинства публичных сервисов.
  Подробное описание настроек доступно [в официальной документации `cert-manager`](https://cert-manager.io/docs/configuration/acme/).

- `letsencrypt-staging` — аналогичен `letsencrypt`, но использует тестовый сервер Let’s Encrypt.
  Подходит для отладки конфигурации и проверки процесса выпуска сертификатов.
  Подробнее про тестовую среду Let’s Encrypt можно прочитать [в официальной документации](https://letsencrypt.org/docs/staging-environment/).

- `selfsigned` — выпускает самоподписанные сертификаты.
  Используется в ситуациях, когда не требуется внешнее доверие к сертификату (например, для внутренних сервисов).

- `selfsigned-no-trust` — также выпускает самоподписанные сертификаты,
  но без автоматического добавления корневого сертификата в доверенные.
  Используется для ручного управления доверием.

В некоторых случаях вам могут понадобиться дополнительные виды ClusterIssuer:

- если вы хотите использовать сертификат от Let’s Encrypt, но с DNS-валидацией через стороннего DNS-провайдера;
- когда необходимо использовать удостоверяющий центр (CA), отличный от Let's Encrypt.
  Все виды поддерживаемых удостоверяющих центров перечислены [в документации `cert-manager`](https://cert-manager.io/docs/configuration/issuers/).

### Добавление ClusterIssuer с валидацией DNS-01 через вебхук

Для подтверждения владения доменом через Let’s Encrypt с помощью метода `DNS-01` необходимо,
чтобы [модуль `cert-manager`](/modules/cert-manager/) мог создавать TXT-записи в зоне DNS, связанной с доменом.

У модуля `cert-manager` есть встроенная поддержка популярных DNS-провайдеров,
таких как AWS Route53, Google Cloud DNS, Cloudflare и других.
Полный перечень доступен [в официальной документации `cert-manager`](https://cert-manager.io/docs/configuration/acme/dns01/).

Если провайдер не поддерживается напрямую,
можно настроить вебхук и разместить в кластере собственный обработчик ACME-запросов,
который будет выполнять нужные операции для обновления DNS-записей.

Данный пример основан на использовании сервиса Yandex Cloud DNS:

1. Для обработки вебхука разместите в кластере сервис `Yandex Cloud DNS ACME webhook`
   согласно [официальной документации](https://github.com/yandex-cloud/cert-manager-webhook-yandex).
1. Создайте ресурс ClusterIssuer, следуя примеру:

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

   - Пример создания секрета с помощью команды `d8 k`:

     ```shell
     d8 k create secret tls internal-ca-key-pair -n d8-cert-manager --key="rootCAKey.pem" --cert="rootCACert.pem"
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
укажите его имя [в глобальном параметре `clusterIssuerName`](../../../reference/api/global.html#parameters-modules-https-certmanager-clusterissuername):

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

### Добавление Issuer и ClusterIssuer, использующих HashiCorp Vault для заказа сертификатов

Для настройки заказа сертификатов с помощью Vault используйте [документацию HashiCorp](https://developer.hashicorp.com/vault/tutorials/archive/kubernetes-cert-manager?in=vault%2Fkubernetes).

После настройки PKI и [включения авторизации в Kubernetes](../access/authorization/), выполните следующее:

1. Создайте ServiceAccount и скопируйте ссылку на его Secret:

   ```shell
   d8 k create serviceaccount issuer
     
   ISSUER_SECRET_REF=$(d8 k get serviceaccount issuer -o json | jq -r ".secrets[].name")
   ```

1. Создайте ресурс Issuer:

   ```shell
   d8 k apply -f - <<EOF
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
