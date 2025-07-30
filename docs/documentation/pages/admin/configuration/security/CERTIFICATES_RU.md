---
title: Управление сертификатами
permalink: ru/admin/configuration/security/certificates.html
lang: ru
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
Описание примеров настройки сертификатов, порядок работы с аннотацией `tls-acme` и способы безопасного использования учётных данных приведены [на странице «Использование TLS-сертификатов»](../../../user/tls.html).
{% endalert %}

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

### Добавление ClusterIssuer с валидацией `DNS-01` через вебхук

Для подтверждения владения доменом через Let’s Encrypt с помощью метода `DNS-01` необходимо,
чтобы модуль `cert-manager` мог создавать TXT-записи в зоне DNS, связанной с доменом.
У модуля `cert-manager` есть встроенная поддержка популярных DNS-провайдеров,
таких как AWS Route53, Google Cloud DNS, Cloudflare и других.
Полный перечень доступен [в официальной документации `cert-manager`](https://cert-manager.io/docs/configuration/acme/dns01/).

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
укажите его имя в глобальном параметре `clusterIssuerName`:

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

После настройки PKI и [включения авторизации в Kubernetes](../access/authorization/), выполните следующее:

1. Создайте ServiceAccount и скопируйте ссылку на его Secret:

   ```shell
   d8 k create serviceaccount issuer
     
   ISSUER_SECRET_REF=$(d8 k get serviceaccount issuer -o json | jq -r ".secrets[].name")
   ```

1. Создайте ресурс Issuer:

   ```yaml
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
       name: vault-issuer
     # Домены указываются на этапе конфигурации PKI в Vault.
     commonName: www.example.com 
     dnsNames:
     - www.example.com
   EOF
   ```

## Генерация самоподписанного сертификата

При самостоятельной генерации сертификатов важно корректно заполнять все поля запроса, чтобы итоговый сертификат был корректно выпущен и успешно проходил валидацию в различных сервисах.  

Важно придерживаться следующих правил:

1. Указывайте доменные имена в поле `SAN` (Subject Alternative Name).

   Поле `SAN` — более современный и распространенный метод указания доменных имен, на которые распространяется сертификат.
   Некоторые сервисы на данный момент уже не рассматривают поле `CN` (Common Name) как источник для доменных имен.

1. Корректно заполняйте поля `keyUsage`, `basicConstraints`, `extendedKeyUsage`, а именно:

   - `basicConstraints = CA:FALSE`  

     Это поле определяет, относится ли сертификат к конечному пользователю (end-entity certificate) или к центру сертификации (CA certificate). CA-сертификат не может использоваться в качестве сертификата сервиса.

   - `keyUsage = digitalSignature, keyEncipherment`  

     Поле `keyUsage` ограничивает допустимые сценарии использования ключа:

     - `digitalSignature` — разрешает использовать ключ для подписи цифровых сообщений и обеспечения целостности соединения.
     - `keyEncipherment` — разрешает использовать ключ для шифрования других ключей, что необходимо для безопасного обмена данными с помощью TLS (Transport Layer Security).

   - `extendedKeyUsage = serverAuth`  

     Поле `extendedKeyUsage` уточняет дополнительные сценарии использования ключа, которые могут требоваться конкретными протоколами или приложениями:

     - `serverAuth` — указывает, что сертификат предназначен для использования на сервере для его аутентификации перед клиентом в процессе установления защищенного соединения.

Также рекомендуется:

1. Издать сертификат на срок не более 1 года (365 дней).

   Срок действия сертификата влияет на его безопасность. Срок в 1 год позволяет обеспечить актуальность криптографических методов и своевременно обновлять сертификаты в случае возникновения угроз.
   Также некоторые современные браузеры на текущий момент отвергают сертификаты со сроком действия более 1 года.

1. Использовать стойкие криптографические алгоритмы, например, алгоритмы на основе эллиптических кривых (в т.ч. `prime256v1`).

   Алгоритмы на основе эллиптических кривых (ECC) предоставляют высокий уровень безопасности при меньшем размере ключа по сравнению с традиционными методами, такими как RSA. Это делает сертификаты более эффективными по производительности и безопасными в долгосрочной перспективе.

1. Не указывать домены в поле `CN` (Common Name).

   Ранее поле `CN` использовалось для указания основного доменного имени, для которого выдается сертификат. Однако современные стандарты, такие как [RFC 2818](https://datatracker.ietf.org/doc/html/rfc2818), рекомендуют использовать поле `SAN` (Subject Alternative Name) для этой цели.
   Если сертификат распространяется на несколько доменных имен, указанных в поле `SAN`, то при дополнительном указании одного из доменов в `CN` в некоторых сервисах может возникнуть ошибка валидации при обращении к домену, не указанному в `CN`.
   Если указывать в `CN` информацию, не относящуюся напрямую к доменным именам (например, идентификатор или имя сервиса), то сертификат также будет распространяться на эти имена, что может быть использовано для вредоносных целей.

### Пример создания сертификата

Для генерации сертификата воспользуйтесь утилитой `openssl`.

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

1. Сгенерируйте ключ на основе эллиптических кривых:

   ```shell
   openssl ecparam -genkey -name prime256v1 -noout -out ec_private_key.pem
   ```

1. Создайте запрос на сертификат:

   ```shell
   openssl req -new -key ec_private_key.pem -out example.csr -config cert.cnf
   ```

1. Сгенерируйте самоподписанный сертификат:

   ```shell
   openssl x509 -req -in example.csr -signkey ec_private_key.pem -out example.crt -days 365 -extensions v3_req -extfile cert.cnf
   ```
