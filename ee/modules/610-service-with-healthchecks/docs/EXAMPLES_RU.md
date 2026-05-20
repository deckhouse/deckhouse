---
title: "Модуль service-with-healthchecks: примеры"
description: "Примеры настройки балансировщика с модулем service-with-healthchecks в Deckhouse Kubernetes Platform"
---

{% alert level="info" %}

Чтобы создаваемые балансировщики ServiceWithHealthchecks работали, необходимо выполнение следующих условий:

* В сетевой политике пользовательского проекта, в котором будет создаваться ServiceWithHealthchecks, должно присутствовать правило, разрешающее входящий трафик из всех подов неймспейса `d8-service-with-healthchecks`:
  
  ```yaml
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          kubernetes.io/metadata.name: d8-service-with-healthchecks
  ```

  Подробнее о сетевых политиках — в разделе [«Настройка сетевых политик»](/products/kubernetes-platform/documentation/v1/admin/configuration/network/policy/configuration.html).

* Кластерная роль, которая используется в ClusterRoleBinding и RoleBinding при назначении прав пользователям и сервисным аккаунтам, для ресурса ServiceWithHealthchecks должна быть расширена следующими правилами:

  * `get`
  * `list`
  * `watch`
  * `create`
  * `update`
  * `patch`
  * `delete`.

  Подробнее — в разделе [«Выдача прав пользователям и сервисным аккаунтам»](/products/kubernetes-platform/documentation/latest/admin/configuration/access/authorization/granting.html).

{% endalert %}

{% alert level="warning" %}
После включения модуля не происходит автоматическая замена имеющихся ресурсов типа Service на ServiceWithHealthcheck. Для замены имеющихся сервисов на использование ServiceWithHealthcheck выполните следующие действия:

* Создайте ресурсы ServiceWithHealthcheck с такими же именами и параметрами, как существующие ресурсы Service, которые нужно заменить. При создании ServiceWithHealthcheck укажите обязательные параметры [`healthchecks`](cr.html#servicewithhealthchecks-v1alpha1-spec-healthcheck).
* Удалите ресурсы Service, которые требуется заменить ServiceWithHealthcheck.
{% endalert %}

## Размещение двух независимых балансировщиков на одной виртуальной машине

На виртуальной машине с операционной системой Linux работают два приложения — HTTP-сервер (TCP 8080) и SMTP-сервер (TCP 2525). Необходимо настроить два отдельных балансировщика для этих сервисов — веб-балансировщик и SMTP-балансировщик.

### Создание виртуальной машины

Создайте виртуальную машину `my-vm` основываясь на примерах из [документации DVP](https://deckhouse.ru/products/virtualization-platform/documentation/user/resource-management/virtual-machines.html).

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
  virtualMachineClassName: generic
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

### Манифесты балансировщиков для веб-сервиса и SMTP

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

## Балансировщики для работы с PostgreSQL-кластером

### Создание StatefulSet для PostgreSQL

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

### Конфигурация балансировщиков ServiceWithHealthchecks

Создайте Secret для хранения учетных данных для доступа проб к базе данных:

```shell
d8 k -n my-ns create secret generic cred-secret --from-literal=user=postgres --from-literal=password=example cred-secret
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
