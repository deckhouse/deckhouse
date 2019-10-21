Модуль cert-manager
===================

Модуль устанавливает [cert-manager v0.10.1](https://github.com/jetstack/cert-manager).

Конфигурация
------------

### Что нужно настраивать?

Обязательных настроек нет.

### Параметры

* `nodeSelector` — как в Kubernetes в `spec.nodeSelector` у pod'ов.
    * Если ничего не указано — будет использоваться значение `{"node-role.flant.com/cert-manager":""}` или `{"node-role.flant.com/system":""}` (если в кластере есть такие узлы) или ничего не будет указано.
    * Можно указать `false`, чтобы не добавлять никакой nodeSelector.
* `tolerations` — как в Kubernetes в `spec.tolerations` у pod'ов.
    * Если ничего не указано — будет использовано значение `[{"key":"dedicated.flant.com","operator":"Equal","value":"cert-manager"},{"key":"dedicated.flant.com","operator":"Equal","value":"system"}]`.
    * Можно указать `false`, чтобы не добавлять никакие toleration'ы.
*  `cloudflareGlobalAPIKey` — Cloudflare Global API key для управления DNS записями (Способ проверки того, что домены указанные в ресурсе Certificate, для которых заказывается сертификат, находятся под управлением `cert-manager` у DNS провайдера Cloudflare. Проверка происходит добавлением специальных TXT записей для домена [ACME DNS01 Challenge Provider](https://github.com/jetstack/cert-manager/blob/master/docs/reference/issuers/acme/dns01.rst))
*  `cloudflareEmail` — Почтовый ящик проекта, на который выдавались доступы для управления Cloudflare
*  `route53AccessKeyID` — Access Key ID пользователя с необходимыми правами [Amazon Route53 IAM Policy](https://cert-manager.readthedocs.io/en/latest/reference/issuers/acme/dns01.html#amazon-route53) для управления доменными записями домена
*  `route53SecretAccessKey` — Secret Access Key пользователя с необходимыми правами для управления доменными записями домена
*  `digitalOceanCredentials` — Access Token от Digital Ocean API, который можно создать в разделе `API`

### Пример конфига

```yaml
certManager: |
  nodeSelector:
    node-role/example: ""
  tolerations:
  - key: dedicated
    operator: Equal
    value: example
```

Как пользоваться модулем?
-------------------------

### Как заказать сертификат?

Все настолько понятно и очевидно, на сколько это вообще может быть! Бери и используй:
```yaml
apiVersion: certmanager.k8s.io/v1alpha1
kind: Certificate
metadata:
  name: example-com                          # имя сертификата, через него потом можно смотреть статус
  namespace: default
spec:
  secretName: example-com-tls                # название секрета, в который положить приватный ключ и сертификат
  issuerRef:
    kind: ClusterIssuer                      # ссылка на "выдаватель" сертификатов, см. подробнее ниже
    name: letsencrypt
  commonName: example.com                    # основной домен сертификата
  dnsNames:                                  # дополнительыне домены сертификата, указывать не обязательно
  - www.example.com
  - admin.example.com
  acme:
    config:
    - http01:
        ingressClass: nginx                  # через какой ingress controller проходить chalenge
      domains:
      - example.com                          # список доменов, для которых проходить chalenge через этот
      - www.example.com                      # ingress controller
    - http01:
        ingressClass: nginx-aws-http
      domains:
      - admin.example.com                    # проходит chalenge через дополнительный ingress controller
```

При этом:
* создается отдельный Ingress-ресурс на время прохождения chalenge'а (соответственно аутентификация и whitelist основного Ingress не будут мешать),
* можно заказать один сертификат на несколько Ingress-ресурсов (и он не отвалится при удалении того, в котором была аннотация `tls-acme`),
* можно заказать сертификат с дополнительными именами (как в примере),
* можно валидировать разные домены, входящие в один сертификат, через разные Ingress-контроллеры.

Подробнее можно прочитать [здесь](https://github.com/jetstack/cert-manager/blob/master/docs/user-guides/acme-http-validation.md).

### Как заказать wildcard сертификат с DNS в Cloudflare?

1. Получим `Global API Key` и `Email Address`:
   * Заходим на страницу: https://dash.cloudflare.com/profile
   * В самом верху страницы написана ваша почта под `Email Address`
   * В самом низу страницы жмем на кнопку "View" напротив `Global API Key`

   В результате чего мы получаем ключ для взаимодействия с API Cloudflare и почту на которую зарегистрирован аккаунт.

2. Редактируем конфигурационный ConfigMap deckhouse, добавляя такую секцию:
   ```
   kubectl -n d8-system edit cm deckhouse
   ```

   ```yaml
   certManager: |
     cloudflareGlobalAPIKey: APIkey
     cloudflareEmail: some@mail.somedomain
   ```

   После чего, Deckhouse автоматически создаст ClusterIssuer и Secret для Cloudflare в namespace `d8-cert-manager`.

3. Создаем Certificate с проверкой с помощью провайдера Cloudflare. Данная возможность появится только при указании настройки `cloudflareGlobalAPIKey` и `cloudflareEmail` в Deckhouse:

   ```yaml
   apiVersion: certmanager.k8s.io/v1alpha1
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
     acme:
       config:
       - dns01:
           provider: cloudflare
         domains:
         - "*.domain.com"
   ```

4. Создаем Ingress:

   ```yaml
   apiVersion: extensions/v1beta1
   kind: Ingress
   metadata:
     annotations:
       kubernetes.io/ingress.class: nginx
     name: domain-wildcard
     namespace: app-namespace
   spec:
     rules:
     - host: "*.domain.com"
       http:
         paths:
         - backend:
             serviceName: svc-web
             servicePort: 80
           path: /
     tls:
     - hosts:
       - "*.domain.com"
       secretName: tls-wildcard
   ```

### Как заказать wildcard сертификат с DNS в Route53?

1. Создаем пользователя с необходимыми правами.

   * Заходим на страницу управления политиками: https://console.aws.amazon.com/iam/home?region=us-east-2#/policies . Создаем политику с такими правами:

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

   * Заходим на страницу управления пользоватяли: https://console.aws.amazon.com/iam/home?region=us-east-2#/users . Создаем пользоватяли с созданной ранее политикой.

2. Редактируем ConfigMap Deckhouse, добавляя такую секцию:

   ```
   kubectl -n d8-system edit cm deckhouse
   ```

   ```yaml
   certManager: |
     route53AccessKeyID: AKIABROTAITAJMPASA4A
     route53SecretAccessKey: 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
   ```

   После чего, Deckhouse автоматически создаст ClusterIssuer и Secret для route53 в namespace `d8-cert-manager`.

3. Создаем Certificate с проверкой с помощью провайдера route53. Данная возможность появится только при указании настроек `route53AccessKeyID` и `route53SecretAccessKey` в Deckhouse:

   ```yaml
   apiVersion: certmanager.k8s.io/v1alpha1
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
     acme:
       config:
       - dns01:
           provider: route53
         domains:
         - "*.domain.com"
   ```

### Как заказать selfsigned сертификат?

Все еще проще, чем с LE. Просто меняем `letsencrypt` на `selfsigned`:

```yaml
apiVersion: certmanager.k8s.io/v1alpha1
kind: Certificate
metadata:
  name: example-com                          # имя сертификата, через него потом можно смотреть статус
  namespace: default
spec:
  secretName: example-com-tls                # название секрета, в который положить приватный ключ и сертификат
  issuerRef:
    kind: ClusterIssuer                      # ссылка на "выдаватель" сертификатов, см. подробнее ниже
    name: selfsigned
  commonName: example.com                    # основной домен сертификата
  dnsNames:                                  # дополнительыне домены сертификата, указывать не обязательно
  - www.example.com
  - admin.example.com
  acme:
    config:
    - http01:
        ingressClass: nginx                  # через какой ingress controller проходить chalenge
      domains:
      - example.com                          # список доменов, для которых проходить chalenge через этот
      - www.example.com                      # ingress controller
    - http01:
        ingressClass: nginx-aws-http
      domains:
      - admin.example.com                    # проходит chalenge через дополнительный ingress controller
```

### Как посмотреть состояние сертификата?

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

### Как получить список сертификатов?

```console
# kubectl get certificate --all-namespaces
NAMESPACE          NAME                            AGE
default            example-com                     13m
```

### Какие виды сертификатов поддерживаются?

На данный момент модуль устанавливает два ClusterIssuer'а:
* letsencrypt
* letsencrypt-staging

### Работает ли старая аннотация tls-acme?

Да, работает! Специальный компонент (`cert-manager-ingress-shim`) видит эти аннотации и на их основании автоматически создает ресурсы `Certificate` (в тех же namespace, что и Ingress-ресурсы с аннотациями).

**Важно!** При использовании аннотации, Certificate создается "прилинкованным" к существующему Ingress-ресурсу, и для прохождения chalenge НЕ создается отдельный Ingress, а вносятся дополнительные записи в существующий. Это означает, что если на основном Ingress'е настроена аутентификация или whitelist — ничего не выйдет. Лучше не использовать аннотацию и переходить на Certificate.

**Важно!** Если перешли с аннотации на Certificate, то нужно удалить Certificate который был создан по аннотации, иначе, по обоим Certificate будет обновляться один Secret (это может привести к попаданию на лимиты Let’s Encrypt).

```yaml
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

### Возможные проблемы

#### CAA record does not match issuer

Если `cert-manager` не может заказать сертификаты с ошибкой:

```
CAA record does not match issuer
```

То необходимо проверить `CAA (Certificate Authority Authorization)` DNS запись у домена, для которого заказывается сертификат.
Если вы хотите использовать Let’s Encrypt сертификаты, то у домена должна быть CAA запись: `issue "letsencrypt.org"`.
Подробнее про CAA можно почитать [тут](https://www.xolphin.com/support/Terminology/CAA_DNS_Records) и [тут](https://letsencrypt.org/docs/caa/).
