# Гранты на кластерные объекты — демо

Демонстрирует фичу грантов модуля multitenancy-manager: доступность по проектам (allow-лист),
квоты на объекты, подстановку/коэрс дефолта и read-only каталог.

> Требуется образ multitenancy-manager с `coerceToDefault` (PR #20520, текущий `pr20520`).

## Запуск

На master-узле:

```bash
alias k='sudo /opt/deckhouse/bin/kubectl --kubeconfig=/etc/kubernetes/admin.conf'
cd grants-demo
```

`00-setup.yaml` (проект `demo` + грант + квота) обычно уже применён. Чтобы (пере)создать:

```bash
k apply -f 00-setup.yaml
```

Каждый файл-сценарий содержит и разрешённые, и запрещённые объекты; `kubectl apply` создаёт
разрешённые и печатает ошибку `Forbidden` для запрещённых — это и есть демонстрация.

## Что настроено для проекта `demo`

| Ресурс                | Разрешено          | Дефолт      | Квота                          | coerceToDefault |
|-----------------------|--------------------|-------------|--------------------------------|-----------------|
| storageclasses        | `local`            | `local`     | 5Gi суммарно, 3 PVC            | да              |
| clusterissuers        | `selfsigned`       | `selfsigned`| —                              | нет             |
| loadbalancerclasses   | `internal`         | —           | 1 LoadBalancer-сервис          | нет             |
| clusterroles          | d8:use:role:* + user-authz access (user,priv-user,editor,admin) | —   | —                              | нет             |

Ключевое отличие: у стораджа `coerceToDefault: true` (встроенный DefaultStorageClass admission
заранее проставляет `storageClassName`), поэтому недоступный/опущенный класс молча переписывается в
`local`. У issuers / LB / cluster roles такого дефолтера нет — недоступное явное значение **отклоняется**.

## Discovery и защита (запускать, не применять)

```bash
# Каталог «что мне доступно» для тенанта. Колонка AVAILABLE — счётчик, чтобы таблица оставалась
# читаемой даже для cluster roles (десятки записей):
k -n demo get availableresources

# Полный список доступных имён для ресурса:
k -n demo get availableresource clusterroles -o jsonpath='{.status.available[*].name}'
# или структурно (с флагом дефолта):
k -n demo get availableresource storageclasses -o yaml

# Потребление квоты vs лимит:
k -n demo get grantquota objects -o yaml

# Каталог read-only (protect-вебхук) — это будет запрещено:
k -n demo delete availableresource storageclasses
```

## Очистка

```bash
k -n demo delete pvc,svc,certificate,ingress,rolebinding --all
k delete project demo
```

## Файлы

- `00-setup.yaml`         — проект + грант + квота на объекты
- `10-storage.yaml`       — диски разрешены / коэрснуты (omit и недоступный класс → `local`)
- `11-storage-quota.yaml` — квота на размер и количество дисков (отказ)
- `20-issuer-cert.yaml`   — ClusterIssuer в Certificate (разрешено / отказ)
- `21-issuer-ingress.yaml`— ClusterIssuer в аннотации Ingress (разрешено / отказ)
- `30-lb-class.yaml`      — loadBalancerClass (разрешено / отказ)
- `31-lb-quota.yaml`      — квота на количество балансировщиков (2-й — отказ)
- `40-clusterrole.yaml`   — RoleBinding на ClusterRole (разрешено / отказ)
