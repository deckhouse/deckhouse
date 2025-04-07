---
title: "Внутрикластерное взаимодействие"
permalink: ru/user/intra-cluster-interaction-using.html
lang: ru
---

Для организации внутрикластерного взаимодействия в Deckhouse Kubernetes Platform рекомендуется использовать сервисы (ресурсы c `kind: Service`) вместо обращения напрямую к подам. Они обеспечивают балансировку нагрузки между подами, стабильность сетевого взаимодействия, поддержку различных сценариев доступа, интеграции с DNS для удобного обнаружения сервисов, изоляции и безопасность сетевого взаимодействия.

## Пример сервиса

```yaml
apiVersion: v1
kind: Service
metadata:
  name: productpage
  namespace: bookinfo
spec:
  ports:
  - name: http
    port: 9080
  selector:
    app: productpage
  type: ClusterIP
```

<!-- перенесено из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/service-with-healthchecks/ -->

## ServiceWithHealthCheck для  обеспечения отказоустойчивости и контроля состояния сервисов

Deckhouse Kubernetes Platform ServiceWithHealthCheck для обеспечения отказоустойчивости сервисов и контроля их состояния можно использовать ServiceWithHealthCheck.

### Ограничения стандартного балансировщика Service

В Kubernetes за внутреннюю и внешнюю балансировку запросов отвечает ресурс типа `Service`. Он распределяет запросы между рабочими подами приложения и исключает из балансировки повреждённые экземпляры. Для проверки способности пода обрабатывать входящие запросы применяются readiness-пробы, которые указываются в спецификации контейнеров, входящих в этот под.

Стандартный инструмент балансировки Service подходит для большинства задач облачных приложений, но имеет два ограничения:

* Если хотя бы один контейнер в поде не проходит проверку готовности (readiness-пробу), весь под отмечается как `NotReady` и исключается из балансировки всех сервисов, с которыми он связан.
* Для каждого контейнера можно настроить только одну пробу, поэтому невозможно создать отдельные пробы для проверки, например, доступности чтения и записи.

Примеры сценариев, где стандартного балансировщика недостаточно:

* База данных:
  * Работает в трёх подах — `db-0`, `db-1` и `db-2`, каждый из которых содержит один контейнер с запущенным процессом базы данных.
  * Необходимо создать два сервиса (Service) — `db-write` для записи и `db-read` для чтения.
  * Запросы на чтение должны балансироваться между всеми подами.
  * Запросы на запись балансируются только на тот под, который назначен мастером средствами самой базы данных.
* Виртуальная машина:
  * Под содержит единственный контейнер, в котором запущен процесс `qemu`, выполняющий роль гипервизора для гостевой виртуальной машины.
  * В гостевой виртуальной машине запущены независимые процессы, например, веб-сервер и SMTP-сервер.
  * Требуется создать два Service — `web` и `smtp`, каждый из которых которых будет иметь свои readiness-пробы.

### Возможности балансировщика ServiceWithHealthcheck

В отличие от стандартного балансировщика, где readiness-пробы привязаны к состоянию контейнеров, `ServiceWithHealthcheck` позволяет настраивать активные пробы на отдельные TCP-порты. Таким образом, каждый балансировщик, обслуживающий один и тот же под, может работать независимо от других.

Настроить данный способ балансировки можно при помощи ресурса [ServiceWithHealthchecks](cr.html#servicewithhealthchecks):

* Его спецификация идентична стандартному `Service` с добавлением раздела `healthcheck`, который содержит набор проверок.
* На данный момент поддерживается три вида проб:
  * `TCP` — обычная проверка с помощью установки TCP-соединения.
  * `HTTP` — возможность отправить HTTP-запрос и ожидать определённый код ответа.
  * `PostgreSQL` — возможность отправить SQL-запрос и ожидать его успешного завершения.

Ознакомиться с примерами можно в [документации](examples.html).

### Внутреннее устройство балансировщика ServiceWithHealthcheck

Балансировщик состоит из двух компонентов:

* контроллер — работает на мастер-узлах кластера и управляет ресурсами `ServiceWithHealthcheck`,
* агенты — работают на каждом узле кластера и выполняют пробы для подов, запущенных на этом узле.

Балансировщик ServiceWithHealthcheck спроектирован так, чтобы не зависеть от реализации CNI, используя при этом стандартные ресурсы `Service` и `EndpointSlice`:

* Контроллер при создании ресурса `ServiceWithHealthcheck` автоматически создает одноименный ресурс Service в том же пространстве имен с пустым полем `selector`. Это позволяет избежать создания стандартным контроллером `EndpointSlice`, которые используются для настройки балансировки.
* Каждый агент при появлении на своём узле подов, которые попадают под управление `ServiceWithHealthcheck`, осуществляет настроенные пробы и создаёт для них `EndpointSlice` со списком проверенных IP-адресов и портов. Данный `EndpointSlice` привязан к дочернему ресурсу `Service`, созданному выше.
* CNI сопоставит все `EndpointSlice` со стандартными сервисами, созданными выше и осуществит балансировку по проверенным IP-адресам и портам на всех узлах кластера.

Миграция с Service на ресурс ServiceWithHealthchecks, например в рамках CI/CD, не должна вызвать затруднений. Спецификация ServiceWithHealthchecks в основе своей повторяет спецификацию Service, но содержит дополнительный раздел healthchecks. Во время жизненного цикла ресурса ServiceWithHealthchecks создается одноименный сервис в том же namespace, чтобы привычным способом (kube-proxy или cni) направить трафик на рабочие нагрузки в кластере.

<!-- перенесено из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/service-with-healthchecks/examples.html -->

### Размещение двух независимых балансировщиков на одной виртуальной машине

На виртуальной машине с операционной системой Linux работают два приложения — HTTP-сервер (TCP 8080) и SMTP-сервер (TCP 2525). Необходимо настроить два отдельных балансировщика для этих сервисов — веб-балансировщик и SMTP-балансировщик.

#### Создание виртуальной машины

Создайте виртуальную машину `my-vm` основываясь на примерах из [документации DVP](https://deckhouse.ru/products/kubernetes-platform/modules/virtualization/stable/user_guide.html).

В примере манифеста ниже добавлен лейбл `vm: my-vm` для дальнейшей идентификации в балансировщиках.

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachine
metadata:
  name: my-vm
  namespace: my-ns
  labels:
    vm: my-vm
spec:
  virtualMachineClassName: host
  cpu:
    cores: 1
  memory:
    size: 1Gi
  provisioning:
    type: UserData
    userData: |
      #cloud-config
      ssh_pwauth: True
      users:
      - name: cloud
        passwd: '$6$rounds=4096$saltsalt$fPmUsbjAuA7mnQNTajQM6ClhesyG0.yyQhvahas02ejfMAq1ykBo1RquzS0R6GgdIDlvS.kbUwDablGZKZcTP/'
        shell: /bin/bash
        sudo: ALL=(ALL) NOPASSWD:ALL
        lock_passwd: False      
  blockDeviceRefs:
    - kind: VirtualDisk
      name: linux-disk
```

#### Манифесты балансировщиков для веб-сервиса и SMTP

Пример манифеста веб-балансировщика:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: ServiceWithHealthchecks
metadata:
  name: web
  namespace: my-ns
spec:
  ports:
  - port: 80
    protocol: TCP
    targetPort: 8080
  selector:
    vm: my-vm
  healthcheck:
    probes:
    - mode: HTTP
      http:
        targetPort: 8080
        method: GET
        path: /healthz
```

Пример манифеста SMTP-балансировщика:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: ServiceWithHealthchecks
metadata:
  name: smtp
  namespace: my-ns
spec:
  ports:
  - port: 25
    protocol: TCP
    targetPort: 2525
  selector:
    vm: my-vm
  healthcheck:
    probes:
    - mode: TCP
      tcp:
        targetPort: 2525
```

### Балансировщики для работы с PostgreSQL-кластером

#### Создание StatefulSet для PostgreSQL

Для корректной работы `StatefulSet` потребуется создать стандартный сервис (Service) для формирования DNS-имени отдельных подов. Этот сервис не будет использоваться для прямого доступа к базе данных.

```yaml
apiVersion: v1
kind: Service
metadata:
  name: postgres
spec:
  selector:
    app: postgres
  ports:
    - protocol: TCP
      port: 5432
      targetPort: 5432
```

Пример манифеста `StatefulSet`:

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: postgres
  name: my-ns
spec:
  serviceName: postgres
  replicas: 3
  selector:
    matchLabels:
      app: postgres
  template:
    metadata:
      labels:
        app: postgres
    spec:
      containers:
        - name: postgres
          image: postgres:13
          ports:
            - containerPort: 5432
          env:
            - name: POSTGRES_USER
              value: postgres
            - name: POSTGRES_PASSWORD
              value: example
```

#### Конфигурация балансировщиков ServiceWithHealthchecks

Создайте Secret для хранения учетных данных для доступа проб к базе данных:

```shell
kubectl -n my-ns create secret generic cred-secret --from-literal=user=postgres --from-literal=password=example cred-secret
```

Пример манифеста балансировщика для чтения:

```yaml
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
```

Пример манифеста балансировщика для записи:

```yaml
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
```
