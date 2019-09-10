Модуль user-authz
=================

Данный модуль отвечает за генерацию RBAC для пользователей и реализует простейший multi-tenancy с разграничением доступа по namespace.

**Важно!** Модуль управляет правами пользователей, а не правами администраторов кластера. Даже уровень доступа `Admin`, представленный в данном модуле, значительно ограничен по правам. Наиболее значительные ограничения:
1. Полностью отсутствует доступ на запись к любым глобальным объектам (кроме namespace), и только некоторые доступны для чтения.
2. Нет доступа на создание DaemonSet'ов (это служебный контроллер, мы категорически не рекомендуем его использовать в конечных приложениях пользователей).


**Важно!** Мы категорически не рекомендуем создавать Pod'ы и ReplicaSet'ы – эти объекты являются второстепенными и должны создаваться из других контроллеров. Доступ к созданию и изменению Pod'ов и ReplicaSet'ов полностью отсутствует.  

Конфигурация
------------

**Важно!** Режим multi-tenancy (авторизация по namespace) в данный момент реализован по временной схеме и **не гарантирует безопасность**! Если webhook, который реализовывает систему авторизации по какой-то причине упадёт, авторизация по namespace (опции `allowAccessToSystemNamespaces` и `limitNamespaces` в CRD) перестанет работать и пользователи получат доступы во все namespace. После восстановления доступности webhook'а все вернется на свои места.

### Параметры

* `enableMultiTenancy` — запустить систему авторизации по namespace. Приведёт к [автоматической настройке kube-apiserver](#настройка-kube-apiserver). При *выключении* параметра потребуется вручную модифицировать манифест kube-apiserver.
  * Значение по-умолчанию – `false` (то-есть multi-tenancy отключен).
* `disableWebhookConfigurator` — отключить DaemonSet, который отвечает за [автоматическую настройку kube-apiserver](#настройка-kube-apiserver).
  * Значение по-умолчанию – `false` (то-есть конфигуратор включен).

### CRD

Вся настройка прав доступа происходит с помощью CRD.

Формат CRD выглядит так:
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterAuthorizationRule
metadata:
  name: test
spec:
  subjects:
  - kind: User
    name: some@example.com
  - kind: Group
    name: some-group-name
  accessLevel: Master
  portForwarding: true
  allowAccessToSystemNamespaces: false     # Опция доступна только при enableMultiTenancy
  limitNamespaces:                         # Опция доступна только при enableMultiTenancy
  - review-.*
  - stage
  additionalRoles:
  - apiGroup: rbac.authorization.k8s.io
    kind: ClusterRole
    name: cluster-write-all
  - apiGroup: rbac.authorization.k8s.io
    kind: ClusterRole
    name: cluster-read-all
```

В `spec` возможны такие параметры:
* `subjects` - Пользователи и/или группы, которым вы хотите предоставить права. [Спецификация](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#subject-v1-rbac-authorization-k8s-io).
* `accessLevel` - `User`, `Master`, `Deploy` или `Admin`. Не обязательный параметр.
    * `User` - позволяет получать информацию обо всех объектах (включая доступ к журналам подов), но не позволяет заходить в контейнеры, читать секреты и выполнять port-forward;
    * `Master` - то же самое, что и User, но позволяет заходить в контейнеры, читать секреты, а также позволяет удалять поды (что обеспечивает возможность перезагрузки);
    * `Deploy` - то же самое, что и Master, но предоставляет возможность создавать и изменять namespace и большинство объектов (не позволяет создавать Pod'ы);
    * `Admin` - то же самое, что и Deploy, но позволяет удалять служебные объекты (ReplicaSet'ы, certmanager.k8s.io/challenges и certmanager.k8s.io/orders);
* `portForwarding` - возможные значения `true`, `false` разрешить выполнять `port-forward`;
    * По-умолчанию `false`.
* `allowScale` - возможные значения `true`, `false` разрешить масштабировать (выполнять scale) Deployment'ы и StatefulSet'ы;
    * По-умолчанию `false`.
* `limitNamespaces` — список разрешённых namespace в формате регулярных выражений. Политика — "куда не разрешено, туда запрещено".
    * Опция доступна только с включённым параметром `enableMultiTenancy`.
* `allowAccessToSystemNamespaces` — разрешить пользователю доступ в служебные namespace (`["antiopa", "kube-.*", "loghouse", "default"]`).
    * По-умолчанию доступа в служебные namespace у пользователей нет.
    * Опция доступна только с включённым параметром `enableMultiTenancy`.
* `additionalRoles` — какие дополнительные роли необходимо выдать для заданных `subjects`.
    * Параметр сделан на крайний случай, вместо него категорически рекомендуется использовать `accessLevel`.

## Как создать пользователя?

В kubernetes есть две категории пользователей:
* ServiceAccount-ы, учёт которых ведёт сам Kubernetes через API.
* Остальные пользователи, чей учёт ведёт не сам Kubernetes, а некоторый внешний софт, который настраивает администратор кластера – существует множество механизмов аутентификации и, соответственно, множество способов заводить пользователей. Мы поддерживаем только два способа:
    * Модуль [user-authn](/modules/150-user-authn/) (подробнее см. документацию модуля).
    * Выдача сертификатов.

Для выдачи сертификата нужно сделать сертификат указав `CN=<имя>` и несколько `O=<группа>` и подписать его с помощью корневой CA кластера. Именно этим механизмом вы аутентифицируетесь в кластере когда, например, используете kubectl на бастионе.

### Создаём пользователя с помощью клиентского сертификата

* Достаём корневой сертификат кластера (ca.crt и ca.key).
* Генерим ключ пользователя:
```shell
openssl genrsa -out myuser.key 2048
```
* Создаём CSR, где указываем, что нам требуется пользователь `myuser`, который состоит в группах `mygroup1` и `mygroup2`:
```shell
openssl req -new -key myuser.key -out myuser.csr -subj "/CN=myuser/O=mygroup1/O=mygroup2"
```
* Подписываем CSR корневым сертификатом кластера:
```shell
openssl x509 -req -in myuser.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out myuser.crt -days 10000
```
* Теперь полученный сертификат можно указывать в конфиг-файле:
```shell
cat << EOF
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: $(cat ca.crt | base64 -w0)
    server: https://<хост кластера>:6443
  name: kubernetes
contexts:
- context:
    cluster: kubernetes
    user: myuser
  name: myuser@kubernetes
current-context: myuser@kubernetes
kind: Config
preferences: {}
users:
- name: myuser
  user:
    client-certificate-data: $(cat myuser.crt | base64 -w0)
    client-key-data: $(cat myuser.key | base64 -w0)
EOF
```
### Предоставляем доступ созданному пользователю

Создадим `ClusterAuthorizationRule`:
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterAuthorizationRule
metadata:
  name: myuser
spec:
  subjects:
  - kind: User
    name: myuser
  accessLevel: Master
  portForwarding: true
```

## Настройка kube-apiserver

После *включения* параметра `enableMultiTenancy` и успешного выката модуля, произойдёт автоматическая настройка манифеста с kube-apiserver на мастере. Соответственно, произойдёт перезагрузка kube-apiserver. Бекап оригинального манифеста появится в каталоге /etc/kubernetes.

Изменения в манифесте:
* Добавить поправить аргумент `--authorization-mode`, добавив перед методом RBAC метод Webhook (например, --authorization-mode=Node,Webhook,RBAC).
* Добавить аргумент `--authorization-webhook-config-file=/etc/kubernetes/authorization-webhook-config.yaml`.
* Добавить в `volumeMounts`:
```yaml
- name: authorization-webhook-config
  mountPath: /etc/kubernetes/authorization-webhook-config.yaml
  readOnly: true
```
* Добавить в `volumes`:
```yaml
- name:authorization-webhook-config
  hostPath:
    path: /etc/kubernetes/authorization-webhook-config.yaml
    type: FileOrCreate
```

## Настройка дополнительных ClusterRole для разных accessLevel

См. [DEVELOPMENT.md](/modules/140-user-authz/DEVELOPMENT.md#настройка-дополнительных-clusterrole-для-разных-accesslevel).
