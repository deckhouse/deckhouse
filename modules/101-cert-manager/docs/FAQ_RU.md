---
title: "Модуль cert-manager: FAQ"
---

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

На данный момент модуль устанавливает два ClusterIssuer'а:
* letsencrypt
* letsencrypt-staging

## Работает ли старая аннотация tls-acme?

Да, работает! Специальный компонент (`cert-manager-ingress-shim`) видит эти аннотации и на их основании автоматически создает ресурсы `Certificate` (в тех же namespace, что и Ingress-ресурсы с аннотациями).

**Важно!** При использовании аннотации, Certificate создается "прилинкованным" к существующему Ingress-ресурсу, и для прохождения chalenge НЕ создается отдельный Ingress, а вносятся дополнительные записи в существующий. Это означает, что если на основном Ingress'е настроена аутентификация или whitelist — ничего не выйдет. Лучше не использовать аннотацию и переходить на Certificate.

**Важно!** Если перешли с аннотации на Certificate, то нужно удалить Certificate который был создан по аннотации, иначе, по обоим Certificate будет обновляться один Secret (это может привести к попаданию на лимиты Let’s Encrypt).

```yaml
apiVersion: networking.k8s.io/v1beta1
kind: Ingress
metadata:
  annotations:
    kubernetes.io/ingress.class: nginx
    kubernetes.io/tls-acme: "true"           # вот она, аннотация!
  name: example-com
  namespace: default
spec:
  rules:
  - host: example.com
    http:
      paths:
      - backend:
          serviceName: site
          servicePort: 80
        path: /
  - host: www.example.com                    # дополнительный домен
    http:
      paths:
      - backend:
          serviceName: site
          servicePort: 80
        path: /
  - host: admin.example.com                  # еще один дополнительный домен
    http:
      paths:
      - backend:
          serviceName: site-admin
          servicePort: 80
        path: /
  tls:
  - hosts:
    - example.com
    - www.example.com                        # дополнительный домен
    - admin.example.com                      # еще один дополнительный домен
    secretName: example-com-tls              # так будут называться и certificate и secret
```

## Ошибка: CAA record does not match issuer

Если `cert-manager` не может заказать сертификаты с ошибкой:

```
CAA record does not match issuer
```

То необходимо проверить `CAA (Certificate Authority Authorization)` DNS запись у домена, для которого заказывается сертификат.
Если вы хотите использовать Let’s Encrypt сертификаты, то у домена должна быть CAA запись: `issue "letsencrypt.org"`.
Подробнее про CAA можно почитать [тут](https://www.xolphin.com/support/Terminology/CAA_DNS_Records) и [тут](https://letsencrypt.org/docs/caa/).
