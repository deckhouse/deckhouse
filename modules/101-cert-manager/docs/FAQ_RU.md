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

### Как добавить `ClusterIssuer`, использующий свой или промежуточный CA для заказа сертификатов?

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

Например, чтобы использовать `ClusterIssuer` для получения сертификатов для всех компонентов Deckhouse, укажите его имя в глобальном параметре [clusterIssuerName](../../deckhouse-configure-global.html#parameters-modules-https-certmanager-clusterissuername) (`kubectl edit mc global`):

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
kubectl get certificate --all-namespaces

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
Подробнее про CAA можно почитать в документации Let’s Encrypt.
