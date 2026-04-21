---
title: "Управление control plane: примеры"
---

## Подключение внешнего плагина планировщика

Пример подключения внешнего плагина планировщика через вебхук.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: KubeSchedulerWebhookConfiguration
metadata:
  name: sds-replicated-volume
webhooks:
- weight: 5
  failurePolicy: Ignore
  clientConfig:
    service:
      name: scheduler
      namespace: d8-sds-replicated-volume
      port: 8080
      path: /scheduler
    caBundle: ABCD=
  timeoutSeconds: 5
```

## CRD с чувствительными полями

В примере показано, как защитить чувствительные поля Custom Resource с помощью feature gate
`CRDSensitiveData` и маркера схемы `x-kubernetes-sensitive-data`.

### 1. Включите шифрование

Включение `apiserver.encryptionEnabled` автоматически активирует feature gate `CRDSensitiveData` для `kube-apiserver` — отдельного переключателя нет:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: control-plane-manager
spec:
  version: 2
  enabled: true
  settings:
    apiserver:
      encryptionEnabled: true
```

### 2. Определите CRD с чувствительными полями

Поля, помеченные `x-kubernetes-sensitive-data: true`, будут шифроваться в etcd и удаляться
из ответов API для вызывающих сторон без доступа к сабресурсу `<resource>/sensitive`.

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: dbconfigs.example.com
spec:
  group: example.com
  scope: Namespaced
  names:
    plural: dbconfigs
    singular: dbconfig
    kind: DbConfig
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              properties:
                host:
                  type: string
                username:
                  type: string
                password:
                  type: string
                  x-kubernetes-sensitive-data: true
```

### 3. Создайте Custom Resource

Создайте экземпляр CRD, указав значение в чувствительном поле. Для клиента создание объекта выглядит как обычно — защита применяется `kube-apiserver` прозрачно:

```yaml
apiVersion: example.com/v1
kind: DbConfig
metadata:
  name: primary
  namespace: default
spec:
  host: db.example.com
  username: admin
  password: s3cr3t
```

```shell
kubectl apply -f dbconfig.yaml
```

После сохранения объект целиком шифруется в etcd, значение `password` маскируется в журнале аудита и удаляется из ответов API, если у вызывающей стороны нет доступа к сабресурсу `dbconfigs/sensitive` (см. следующий шаг).

### 4. Настройте RBAC

Предоставьте доступ к чувствительным полям через сабресурс `<resource>/sensitive`:

```yaml
# Обычная роль: может читать ресурс, но чувствительные поля будут удалены из ответа.
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: dbconfig-reader
rules:
- apiGroups: ["example.com"]
  resources: ["dbconfigs"]
  verbs: ["get", "list", "watch"]
---
# Привилегированная роль: может читать полные данные, включая чувствительные поля.
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: dbconfig-sensitive-reader
rules:
- apiGroups: ["example.com"]
  resources: ["dbconfigs"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["example.com"]
  resources: ["dbconfigs/sensitive"]
  verbs: ["get", "list", "watch"]
```

### 5. Результат

Пользователь с ролью `dbconfig-reader`, выполнивший `kubectl get dbconfig primary -o json`, увидит ресурс с удалёнными чувствительными полями:

```json
{
  "spec": {
    "host": "db.example.com",
    "username": "admin"
  }
}
```

Пользователь с ролью `dbconfig-sensitive-reader` увидит полные данные:

```json
{
  "spec": {
    "host": "db.example.com",
    "username": "admin",
    "password": "s3cr3t"
  }
}
```

В журнале аудита значения чувствительных полей всегда маскируются, независимо от прав вызывающей стороны:

```json
{
  "spec": {
    "host": "db.example.com",
    "username": "admin",
    "password": "******"
  }
}
```
