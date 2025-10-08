---
title: Подключение и авторизация
permalink: ru/admin/integrations/virtualization/dvp/authorization.html
lang: ru
---

Для взаимодействия с ресурсами в DVP компоненты Deckhouse Kubernetes Platform используют API DVP. Для настройки подключения создайте пользователя (ServiceAccount), назначьте ему соответствующие права доступа и сгенерируйте kubeconfig.

{% alert level="warning" %}
Провайдер поддерживает работу только с одним диском в шаблоне виртуальной машины. Убедитесь, что шаблон содержит только один диск.
{% endalert %}

## Создание пользователя

Создайте нового пользователя в кластере DVP с помощью следующей команды:

```bash
d8 k create -f -<<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: sa-demo
  namespace: default
---
apiVersion: v1
kind: Secret
metadata:
  name: sa-demo-token
  namespace: default
  annotations:
    kubernetes.io/service-account.name: sa-demo
type: kubernetes.io/service-account-token
EOF
```

## Добавление роли

Добавьте роль созданному пользователю в кластере DVP с помощью следующей команды:

```bash
d8 k create -f -<<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: sa-demo-rb
  namespace: default
subjects:
  - kind: ServiceAccount
    name: sa-demo
    namespace: default
roleRef:
  kind: ClusterRole
  name: d8:use:role:manager
  apiGroup: rbac.authorization.k8s.io
EOF
```

## Генерация kubeconfig

Чтобы сгенерировать kubeconfig, следуйте указаниям из [инструкции](/modules/user-authn/faq.html#как-сгенерировать-kubeconfig-для-доступа-к-kubernetes-api).
