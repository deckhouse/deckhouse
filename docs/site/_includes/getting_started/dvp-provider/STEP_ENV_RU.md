{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

Для взаимодействия с ресурсами в DVP компоненты Deckhouse используют API DVP. Для настройки этого подключения требуется создать пользователя и назначить ему соответствующие права доступа, а также сгенерировать kubeconfig

## Создание пользователя

Пользователя необходимо создать в кластере DVP

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

Добавить роль созданному пользователю в кластере DVP

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

Как сгенерировать kubeconfig можно прочитать по ссылке начиная с 3го пункта
<https://deckhouse.ru/products/kubernetes-platform/documentation/v1/modules/user-authz/usage.html#%D1%81%D0%BE%D0%B7%D0%B4%D0%B0%D0%BD%D0%B8%D0%B5-serviceaccount-%D0%B4%D0%BB%D1%8F-%D1%81%D0%B5%D1%80%D0%B2%D0%B5%D1%80%D0%B0-%D0%B8-%D0%BF%D1%80%D0%B5%D0%B4%D0%BE%D1%81%D1%82%D0%B0%D0%B2%D0%BB%D0%B5%D0%BD%D0%B8%D0%B5-%D0%B5%D0%BC%D1%83-%D0%B4%D0%BE%D1%81%D1%82%D1%83%D0%BF%D0%B0>
