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

## Защита ресурсов с чувствительными полями

Далее показан пример конфигурации с защитой чувствительных полей ресурсов с помощью feature gate `CRDSensitiveData` и маркера схемы `x-kubernetes-sensitive-data`.

Инструкция по включению защиты доступна [в разделе «FAQ»](faq.html#как-защитить-чувствительные-поля-кастомных-ресурсов).

1. Включение шифрования etcd с помощью параметра `apiserver.encryptionEnabled`. Это автоматически активирует feature gate `CRDSensitiveData` для `kube-apiserver`.

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

1. Определение чувствительных полей в конфигурации.

   Поля, помеченные маркером `x-kubernetes-sensitive-data: true`, будут шифроваться в etcd и удаляться из ответов API для вызывающих сторон без доступа к субресурсу `<resource>/sensitive`.

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

1. Создание кастомного ресурса с заполненными значениями в чувствительных полях.

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

   После сохранения объект целиком шифруется в etcd, значение `password` маскируется в журнале аудита и удаляется из ответов API, если у вызывающей стороны нет доступа к субресурсу `dbconfigs/sensitive`.

1. Настройка доступа к чувствительным полям с помощью RBAC через субресурс `<resource>/sensitive`.

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

1. Результат защиты чувствительных полей.

   - Пользователь с ролью `dbconfig-reader`, выполнивший команду `d8 k get dbconfig primary -o json`, увидит ресурс с удалёнными чувствительными полями:

     ```json
     {
       "spec": {
         "host": "db.example.com",
         "username": "admin"
       }
     }
     ```

   - Пользователь с ролью `dbconfig-sensitive-reader` увидит полные данные:

     ```json
     {
       "spec": {
         "host": "db.example.com",
         "username": "admin",
         "password": "s3cr3t"
       }
     }
     ```

   - В журнале аудита значения чувствительных полей всегда маскируются, независимо от прав вызывающей стороны:

     ```json
     {
       "spec": {
         "host": "db.example.com",
         "username": "admin",
         "password": "******"
       }
     }
     ```
