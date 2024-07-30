---
title: "Модуль cert-manager: FAQ"
---

{% raw %}

## Как посмотреть состояние сертификата?

```console
# kubectl -n default describe certificate example-com
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

```console
# kubectl get certificate --all-namespaces
NAMESPACE          NAME                            AGE
default            example-com                     13m
```

## Какие виды сертификатов поддерживаются?

На данный момент модуль устанавливает следующие ClusterIssuer:
* letsencrypt
* letsencrypt-staging
* selfsigned
* selfsigned-no-trust

## Работает ли старая аннотация TLS-acme?

Да, работает! Специальный компонент (`cert-manager-ingress-shim`) видит эти аннотации и на их основании автоматически создает ресурсы `Certificate` (в тех же namespace, что и Ingress-ресурсы с аннотациями).

> **Важно!** При использовании аннотации Certificate создается «прилинкованным» к существующему Ingress-ресурсу, и для прохождения Challenge НЕ создается отдельный Ingress, а вносятся дополнительные записи в существующий. Это означает, что если на основном Ingress'е настроена аутентификация или whitelist — ничего не выйдет. Лучше не использовать аннотацию и переходить на Certificate.
>
> **Важно!** При переходе с аннотации на Certificate нужно удалить Certificate, который был создан по аннотации. Иначе по обоим Certificate будет обновляться один Secret, и это может привести к достижению лимита запросов Let’s Encrypt.

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    kubernetes.io/tls-acme: "true"           # Вот она, аннотация!
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
    secretName: example-com-tls              # Так будут называться и Certificate, и Secret.
```

## Ошибка: CAA record does not match issuer

Если `cert-manager` не может заказать сертификаты с ошибкой:

```text
CAA record does not match issuer
```

то необходимо проверить `CAA (Certificate Authority Authorization)` DNS-запись у домена, для которого заказывается сертификат.
Если вы хотите использовать Let’s Encrypt-сертификаты, у домена должна быть CAA-запись: `issue "letsencrypt.org"`.
Подробнее про CAA можно почитать [тут](https://www.xolphin.com/support/Terminology/CAA_DNS_Records) и [тут](https://letsencrypt.org/docs/caa/).

## Интеграция с Vault

Вы можете использовать [данную инструкцию](https://learn.hashicorp.com/tutorials/vault/kubernetes-cert-manager?in=vault/kubernetes) для выпуска сертификатов с помощью Vault.

После конфигурации PKI и [включения авторизации](../../modules/140-user-authz/) в Kubernetes, вам нужно:
- Создать ServiceAccount и скопировать ссылку на его Secret:

  ```shell
  kubectl create serviceaccount issuer
  ISSUER_SECRET_REF=$(kubectl get serviceaccount issuer -o json | jq -r ".secrets[].name")
  ```

- Создать Issuer:

  ```shell
  kubectl apply -f - <<EOF
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

- Создать ресурс Certificate для получения TLS-сертификата подписанного Vault CA:

  ```shell
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

## Как использовать свой или промежуточный CA для заказа сертификатов?

Для использования собственного или промежуточного CA:

- Сгенерируйте сертификат (при необходимости):

  ```shell
  openssl genrsa -out rootCAKey.pem 2048
  openssl req -x509 -sha256 -new -nodes -key rootCAKey.pem -days 3650 -out rootCACert.pem
  ```

- В пространстве имён `d8-cert-manager` создайте секрет, содержащий данные файлов сертификатов.

  Пример создания секрета с помощью команды kubectl:

  ```shell
  kubectl create secret tls internal-ca-key-pair -n d8-cert-manager --key="rootCAKey.pem" --cert="rootCACert.pem"
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

- Создайте ClusterIssuer из созданного секрета:

  ```yaml
  apiVersion: cert-manager.io/v1
  kind: ClusterIssuer
  metadata:
    name: inter-ca
  spec:
    ca:
      secretName: internal-ca-key-pair    # Имя созданного секрета.
  ```

  Имя ClusterIssuer также может быть любым.

Теперь можно использовать созданный ClusterIssuer для получения сертификатов для всех компонентов Deckhouse или конкретного компонента.

Например, чтобы использовать ClusterIssuer для получения сертификатов для всех компонентов Deckhouse, укажите его имя в глобальном параметре [clusterIssuerName](../../deckhouse-configure-global.html#parameters-modules-https-certmanager-clusterissuername) (`kubectl edit mc global`):

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

## Как защитить учетные данные cert-manager?

Если вы не хотите хранить учетные данные конфигурации Deckhouse (например, по соображениям безопасности), можете создать
свой собственный ClusterIssuer / Issuer.
Например, вы можете создать свой ClusterIssuer для сервиса [route53](https://aws.amazon.com/route53/) следующим образом:
- Создайте Secret с учетными данными:

  ```shell
  kubectl apply -f - <<EOF
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

- Создайте простой ClusterIssuer со ссылкой на этот Secret:

  ```shell
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
            accessKeyID: {{ "MY-AWS-ACCESS-KEY-ID" }}
            secretAccessKeySecretRef:
              name: route53
              key: secret-access-key
  EOF
  ```

- Закажите сертификаты как обычно, используя созданный ClusterIssuer:

  ```shell
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

{% endraw %}
