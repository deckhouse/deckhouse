# Гранты на кластерные ресурсы — демо

Демонстрирует фичу грантов модуля multitenancy-manager: доступность по проектам (allow-лист),
подстановку/коэрс дефолта и read-only каталог. Квота на объекты делегирована Kubernetes
`ResourceQuota` и в эту фичу не входит.

> Требуется образ multitenancy-manager из редизайна на references (PR #20611, `pr20611`).

## Запуск

На master-узле:

```bash
alias k='sudo /opt/deckhouse/bin/kubectl --kubeconfig=/etc/kubernetes/admin.conf'
cd grants-demo
```

`00-setup.yaml` (проект `demo` + политика доступности) обычно уже применён. Чтобы (пере)создать:

```bash
k apply -f 00-setup.yaml
```

Каждый файл сценария смешивает ALLOW и DENY объекты; `kubectl apply` создаёт разрешённые и печатает
ошибку `Forbidden` для запрещённых — это и есть демо.

## Что настроено для проекта `demo`

| Ресурс                | Разрешено                                                      | Дефолт       | Дефолтинг |
|-----------------------|----------------------------------------------------------------|--------------|-----------|
| storageclasses        | `local`                                                       | `local`      | Coerce    |
| clusterissuers        | `selfsigned`                                                  | `selfsigned` | FillEmpty (Certificate) / None (аннотация Ingress) |
| loadbalancerclasses   | `internal`                                                    | —            | FillEmpty |
| clusterroles          | d8:use:role:* + user-authz access (user,priv-user,editor,admin) | —            | None      |

Ключевое отличие: у PVC-пути storageclasses `defaulting: Coerce` (встроенный DefaultStorageClass
предзаполняет `storageClassName`), поэтому недопустимый/опущенный класс переписывается в `local`.
У issuers / LB / cluster roles предзаполнителя нет — недопустимое явное значение **отклоняется**.

## Discovery, статусы и защита (выполнять, не применять)

```bash
# Вид тенанта «что мне доступно» (каталог под контроллером). AVAILABLE — счётчик, таблица читаема
# даже для cluster roles (десятки записей):
k -n demo get availableclusterresources

# Полный список доступных имён ресурса (с пометкой дефолта):
k -n demo get availableclusterresource storageclasses -o yaml

# Статус definition — какие пути на него ссылаются:
k get grantableclusterresourcedefinition storageclasses -o jsonpath='{.status.references}'

# Статус reference — привязан к definition или промахнулись именем:
k get grantableclusterresourcereference

# Каталог read-only (protect-вебхук) — это будет отклонено:
k -n demo delete availableclusterresource storageclasses
```

## Очистка

```bash
k -n demo delete pvc,svc,certificate,ingress,rolebinding --all
k delete project demo
```

## Файлы

- `00-setup.yaml`         — проект + политика доступности
- `10-storage.yaml`       — диск allowed / coerced (пропуск и недопустимый класс → `local`)
- `20-issuer-cert.yaml`   — ClusterIssuer в Certificate (разрешено / отказ)
- `21-issuer-ingress.yaml`— ClusterIssuer в аннотации Ingress (разрешено / отказ)
- `30-lb-class.yaml`      — loadBalancerClass (разрешено / отказ)
- `40-clusterrole.yaml`   — RoleBinding на ClusterRole (разрешено / отказ)
