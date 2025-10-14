---
title: "Публичный доступ к виртуальной машине"
permalink: ru/virtualization-platform/documentation/user/network/vm-publishing.html
lang: ru
---

## Предоставление публичного доступа с использованием сервисов

Достаточно часто возникает необходимость сделать так, чтобы доступ к виртуальным машинам был возможен извне, например, для удалённого администрирования или подключения к каким-либо сервисам виртуальной машины. Для этих целей предусмотрены специальные сервисы, которые обеспечивают маршрутизацию трафика из внешней сети к внутренним ресурсам кластера. Рассмотрим несколько вариантов.

Предварительно проставьте метки на ранее созданной ВМ:

```shell
d8 k label vm linux-vm app=nginx
```

Пример вывода:

```console
virtualmachine.virtualization.deckhouse.io/linux-vm labeled
```

### Использование сервиса NodePort

Сервис `NodePort` открывает определённый порт на всех узлах кластера, перенаправляя трафик на заданный внутренний порт сервиса.

В этом примере будет создан сервис с типом `NodePort`,  который откроет на всех узлах кластера внешний порт 31880 и направит входящий трафик на внутренний порт 80 виртуальной машины с приложением Nginx.

```yaml
d8 k apply -f - <<EOF
apiVersion: v1
kind: Service
metadata:
  name: linux-vm-nginx-nodeport
spec:
  type: NodePort
  selector:
    # Лейбл, по которому сервис определяет на какую виртуальную машину направлять трафик.
    app: nginx
  ports:
    - protocol: TCP
      port: 80
      targetPort: 80
      nodePort: 31880
EOF
```

![NodePort](/../../../../images/virtualization-platform/lb-nodeport.ru.png)

### Использование сервиса LoadBalancer

При использовании типа сервиса `LoadBalancer` кластер создаёт внешний балансировщик нагрузки, который распределит входящий трафик по всем экземплярам вашей виртуальной машины.

```yaml
d8 k apply -f - <<EOF
apiVersion: v1
kind: Service
metadata:
  name: linux-vm-nginx-lb
spec:
  type: LoadBalancer
  selector:
    # Лейбл, по которому сервис определяет на какую виртуальную машину направлять трафик.
    app: nginx
  ports:
    - protocol: TCP
      port: 80
      targetPort: 80
EOF
```

![LoadBalancer](/../../../../images/virtualization-platform/lb-loadbalancer.ru.png)

### Использование сервисов с активными проверками

> **Внимание.** Находится на стадии тестирования. Будет доступно в ближайших версиях.

Ресурс `ServiceWithHealthchecks` позволяет настраивать для сервиса активные проверки на заданные TCP-порты. Если проверки для виртуальных машин не будут успешными, эти машины не будут включены в балансировку трафика.

Поддерживаются следующие виды проверок:

- `TCP` — обычная проверка с помощью установки TCP-соединения.
- `HTTP` — возможность отправить HTTP-запрос и ожидать определённый код ответа.
- `PostgreSQL` — возможность отправить SQL-запрос и ожидать его успешного завершения.

Пример сервиса с проверкой HTTP:

```yaml
d8 k apply -f - <<EOF
apiVersion: network.deckhouse.io/v1alpha1
kind: ServiceWithHealthchecks
metadata:
  name: linux-vm-active-http-check
spec:
  ports:
  - port: 80
    protocol: TCP
    targetPort: 8080
  selector:
    # Метка, по которой сервис определяет на какую виртуальную машину направлять трафик.
    app: nginx
  healthcheck:
    probes:
    - mode: HTTP
      http:
        targetPort: 8080
        method: GET
        path: /healthz
EOF
```

Пример сервиса с проверкой TCP:

```yaml
d8 k apply -f - <<EOF
apiVersion: network.deckhouse.io/v1alpha1
kind: ServiceWithHealthchecks
metadata:
  name:  linux-vm-active-tcp-check
spec:
  ports:
  - port: 25
    protocol: TCP
    targetPort: 2525
  selector:
    # Метка, по которой сервис определяет на какую виртуальную машину направлять трафик.
    app: nginx
  healthcheck:
    probes:
    - mode: TCP
      http:
        targetPort: 2525
EOF
```

Пример сервиса с проверкой PostgreSQL для чтения:

```yaml
d8 k apply -f - <<EOF
apiVersion: network.deckhouse.io/v1alpha1
kind: ServiceWithHealthchecks
metadata:
  name: postgres-read
spec:
  ports:
  - port: 5432
    protocol: TCP
    targetPort: 5432
  selector:
    app: postgres
  healthcheck:
    probes:
    - mode: PostgreSQL
      postgreSQL:
        targetPort: 5432
        dbName: postgres
        authSecretName: cred-secret
        query: "SELECT 1"
EOF
```

Пример сервиса с проверкой PostgreSQL для записи:

```yaml
d8 k apply -f - <<EOF
apiVersion: network.deckhouse.io/v1alpha1
kind: ServiceWithHealthchecks
metadata:
  name: postgres-write
spec:
  ports:
  - port: 5432
    protocol: TCP
    targetPort: 5432
  selector:
    app: postgres
  healthcheck:
    probes:
    - mode: PostgreSQL
      postgreSQL:
        targetPort: 5432
        dbName: postgres
        authSecretName: cred-secret
        query: "SELECT NOT pg_is_in_recovery()"
EOF
```

где `authSecretName` – это название секрета (Secret) с учетными данными для доступа к PostgreSQL.

Пример создания такого секрета:

```shell
d8 k create secret generic cred-secret --from-literal=user=postgres --from-literal=password=example cred-secret
```

## Предоставление публичного доступа к сервисам виртуальной машины с использованием Ingress

Ingress позволяет управлять входящими HTTP/HTTPS-запросами и маршрутизировать их к различным серверам в рамках вашего кластера. Это наиболее подходящий метод, если вы хотите использовать доменные имена и SSL-терминацию для доступа к вашим виртуальным машинам.

Для публикации сервиса виртуальной машины через Ingress необходимо создать следующие ресурсы:

1. Внутренний сервис для связки с Ingress. Пример:

   ```yaml
   d8 k apply -f - <<EOF
   apiVersion: v1
   kind: Service
   metadata:
     name: linux-vm-nginx
   spec:
     selector:
       # Метка, по которой сервис определяет на какую виртуальную машину направлять трафик.
       app: nginx
     ports:
       - protocol: TCP
         port: 80
         targetPort: 80
   EOF
   ```

1. Ingress-ресурс для публикации. Пример:

   ```yaml
   d8 k apply -f - <<EOF
   apiVersion: networking.k8s.io/v1
   kind: Ingress
   metadata:
     name: linux-vm
   spec:
     rules:
       - host: linux-vm.example.com
         http:
           paths:
             - path: /
               pathType: Prefix
               backend:
                 service:
                   name: linux-vm-nginx
                   port:
                     number: 80
   EOF
   ```

### Как защитить приложение опубликованное через Ingress

Чтобы включить аутентификацию через `Dex` для приложения, выполните следующие шаги:

1. Создайте кастомный ресурс [DexAuthenticator](/modules/user-authn/cr.html#dexauthenticator). Это приведет к созданию экземпляра [oauth2-proxy](https://github.com/oauth2-proxy/oauth2-proxy), подключенного к `Dex`. После появления кастомного ресурса `DexAuthenticator`, в указанном `namespace` появятся необходимые объекты Deployment, Service, Ingress, Secret.

   Пример ресурса `DexAuthenticator`:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: DexAuthenticator
   metadata:
     # Префикс имени подов Dex authenticator.
     # Например, если префикс имени `app-name`, то поды Dex authenticator будут вида `app-name-dex-authenticator-7f698684c8-c5cjg`.
     name: app-name
     # Пространство имён, в котором будет развернут Dex authenticator.
     namespace: app-ns
   spec:
     # Домен вашего приложения. Запросы на него будут перенаправляться для прохождения аутентификацию в Dex.
     applicationDomain: "app-name.kube.my-domain.com"
     # Отправлять ли `Authorization: Bearer` header приложению. Полезно в связке с auth_request в NGINX.
     sendAuthorizationHeader: false
     # Имя Secret'а с SSL-сертификатом.
     applicationIngressCertificateSecretName: "ingress-tls"
     # Название Ingress-класса, которое будет использоваться в создаваемом для Dex authenticator Ingress-ресурсе.
     applicationIngressClassName: "nginx"
     # Время, на протяжении которого пользовательская сессия будет считаться активной.
     keepUsersLoggedInFor: "720h"
     # Список групп, пользователям которых разрешено проходить аутентификацию.
     allowedGroups:
       - everyone
       - admins
     # Список адресов и сетей, с которых разрешено проходить аутентификацию.
     whitelistSourceRanges:
       - 1.1.1.1/32
       - 192.168.0.0/24
   ```

1. Подключите приложение к `Dex`. Для этого добавьте в Ingress-ресурс приложения аннотации:

   - `nginx.ingress.kubernetes.io/auth-signin: https://$host/dex-authenticator/sign_in`
   - `nginx.ingress.kubernetes.io/auth-response-headers: X-Auth-Request-User,X-Auth-Request-Email`
   - `nginx.ingress.kubernetes.io/auth-url: https://<NAME>-dex-authenticator.<NS>.svc.{{ C_DOMAIN }}/dex-authenticator/auth`, где:
     - `NAME` — значение параметра `metadata.name` ресурса DexAuthenticator;
     - `NS` — значение параметра `metadata.namespace` ресурса DexAuthenticator;
     - `C_DOMAIN` — домен кластера (параметр `clusterDomain`) ресурса ClusterConfiguration).

   Пример аннотаций на Ingress-ресурсе приложения, для подключения его к `Dex`:

   ```yaml
   annotations:
     nginx.ingress.kubernetes.io/auth-signin: https://$host/dex-authenticator/sign_in
     nginx.ingress.kubernetes.io/auth-url: https://app-name-dex-authenticator.app-ns.svc.cluster.local/dex-authenticator/auth
     nginx.ingress.kubernetes.io/auth-response-headers: X-Auth-Request-User,X-Auth-Request-Email
   ```

### Настройка ограничений на основе CIDR

В DexAuthenticator нет встроенной системы управления разрешением аутентификации на основе IP-адреса пользователя. Вместо этого вы можете воспользоваться аннотациями для Ingress-ресурсов:

- Если нужно ограничить доступ по IP и оставить прохождение аутентификации в Dex, добавьте аннотацию с указанием разрешенных CIDR через запятую:

  ```yaml
  nginx.ingress.kubernetes.io/whitelist-source-range: 192.168.0.0/32,1.1.1.1
  ```

- Если необходимо, чтобы пользователи из указанных сетей освобождались от прохождения аутентификации в Dex, а пользователи из остальных сетей обязательно аутентифицировались в Dex, добавьте аннотацию:

  ```yaml
  nginx.ingress.kubernetes.io/satisfy: "any"
  ```
