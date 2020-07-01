---
title: "Модуль user-authz" 
sidebar: modules-user-authz
permalink: modules/140-user-authz/
hide_sidebar: false
---

Данный модуль отвечает за генерацию RBAC для пользователей и реализует простейший multi-tenancy с разграничением доступа по namespace.

**Важно!** Мы категорически не рекомендуем создавать Pod'ы и ReplicaSet'ы – эти объекты являются второстепенными и должны создаваться из других контроллеров. Доступ к созданию и изменению Pod'ов и ReplicaSet'ов полностью отсутствует.  

Конфигурация
------------

**Важно!** Режим multi-tenancy (авторизация по namespace) в данный момент реализован по временной схеме и **не гарантирует безопасность**! Если webhook, который реализовывает систему авторизации по какой-то причине упадёт, авторизация по namespace (опции `allowAccessToSystemNamespaces` и `limitNamespaces` в CRD) перестанет работать и пользователи получат доступы во все namespace. После восстановления доступности webhook'а все вернется на свои места.

### Параметры

* `enableMultiTenancy` — включить авторизацию по namespace.
  * Так как данная опция реализована через [плагин авторизации Webhook](https://kubernetes.io/docs/reference/access-authn-authz/webhook/), то потребуется дополнительная [настройка kube-apiserver](#настройка-kube-apiserver). Для автоматизации этого процесса используйте модуль [control-plane-configurator](/modules/160-control-plane-configurator).
  * Значение по-умолчанию – `false` (то есть multi-tenancy отключен).
* `controlPlaneConfigurator` — настройки параметров для модуля автоматической настройки kube-apiserver [control-plane-configurator](/modules/160-control-plane-configurator).
  * `enabled` — передавать ли в control-plane-configurator параметры для настройки authz-webhook (см. [параметры control-plane-configurator-а](/modules/160-control-plane-configurator#параметры)).
    * При выключении этого параметра, модуль control-plane-configurator будет считать, что по-умолчанию Webhook-авторизация выключена и, соответственно, если не будет дополнительных настроек, то control-plane-configurator будет стремиться вычеркнуть упоминания Webhook-плагина из манифеста. Даже если вы настроите манифест вручную.
    * По-умолчанию `true`.

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
  - kind: ServiceAccount
    name: gitlab-runner-deploy
    namespace: d8-service-accounts
  - kind: Group
    name: some-group-name
  accessLevel: PrivilegedUser
  portForwarding: true
  allowAccessToSystemNamespaces: false     # Опция доступна только при enableMultiTenancy
  limitNamespaces:                         # Опция доступна только при enableMultiTenancy
  - review-.*
  - stage
```

В `spec` возможны такие параметры:
* `subjects` — Пользователи и/или группы, которым вы хотите предоставить права. [Спецификация](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#subject-v1-rbac-authorization-k8s-io).
* `accessLevel` — `User`, `PrivilegedUser`, `Editor`, `Admin`, `ClusterAdmin`, `SuperAdmin`. Не обязательный параметр.
    * `User` — позволяет получать информацию обо всех объектах (включая доступ к журналам подов), но не позволяет заходить в контейнеры, читать секреты и выполнять port-forward;
    * `PrivilegedUser` — то же самое, что и User, но позволяет заходить в контейнеры, читать секреты, а также позволяет удалять поды (что обеспечивает возможность перезагрузки);
    * `Editor` — то же самое, что и PrivilegedUser, но предоставляет возможность создавать и изменять namespace и все объекты, которые обычно нужны для прикладных задач;
      * **Важно!** т.к. Editor уполномочен редактировать RoleBindings, он может сам себе расширить полномочия в рамках namespace.
    * `Admin` — то же самое, что и Editor, но позволяет удалять служебные объекты (производные ресурсы, например, ReplicaSet'ы, certmanager.k8s.io/challenges и certmanager.k8s.io/orders);
    * `ClusterEditor` — то же самое, что и Editor, но позволяет управлять ограниченным набором cluster-wide объектов, которые могут понадобиться для прикладных задач (ClusterXXXMetric, ClusterRoleBindings, KeepalivedInstance, DaemonSet...). Роль для работы оператора кластера.
      * **Важно!** т.к. ClusterEditor уполномочен редактировать ClusterRoleBindings, он может сам себе расширить полномочия.
    * `ClusterAdmin` — то же самое, что и ClusterEditor + Admin, но позволяет управлять служебными cluster-wide объектами (производные ресурсы, например, MachineSets, Machines, OpenstackInstanceClasses...). Роль для работы администратора кластера.
      * **Важно!** т.к. ClusterAdmin уполномочен редактировать ClusterRoleBindings, он может сам себе расширить полномочия.
    * `SuperAdmin` — разрешены любые действия с любыми объектами, при этом ограничения `limitNamespaces` (см. ниже) продолжат работать.
* `portForwarding` — возможные значения `true`, `false` разрешить выполнять `port-forward`;
    * По-умолчанию `false`.
* `allowScale` — возможные значения `true`, `false` разрешить масштабировать (выполнять scale) Deployment'ы и StatefulSet'ы;
    * По-умолчанию `false`.
* `limitNamespaces` — белый список разрешённых namespace в формате регулярных выражений.
    * Политика:
        * Если список указан, то разрешаем доступ только по нему.
        * Если список не указан, то считаем, что разрешено всё, кроме системных namespace (см. `spec.allowAccessToSystemNamespaces` ниже).
    * Опция доступна только с включённым параметром `enableMultiTenancy`.
* `allowAccessToSystemNamespaces` — разрешить пользователю доступ в служебные namespace (`["antiopa", "kube-.*", "d8-.*", "loghouse", "default"]`).
    * По-умолчанию доступа в служебные namespace у пользователей нет.
    * Опция доступна только с включённым параметром `enableMultiTenancy`.
* `additionalRoles` — какие дополнительные роли необходимо выдать для заданных `subjects`.
    * Параметр сделан на крайний случай, вместо него категорически рекомендуется использовать `accessLevel`.
    * Фомат:
    ```yaml
    additionalRoles:
    - apiGroup: rbac.authorization.k8s.io
      kind: ClusterRole
      name: cluster-write-all
    - apiGroup: rbac.authorization.k8s.io
      kind: ClusterRole
      name: cluster-read-all
    ```

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
  accessLevel: PrivilegedUser
  portForwarding: true
```

## Настройка kube-apiserver

Для корректной работы параметра `enableMultiTenancy` необходимо настроить kube-apiserver. Для этого предусмотрен специальный модуль [control-plane-configurator](/modules/160-control-plane-configurator).

<details>
  <summary>Изменения манифеста, которые произойдут
  </summary>

* Будет поправлен аргумент `--authorization-mode`, добавится перед методом RBAC метод Webhook (например, --authorization-mode=Node,Webhook,RBAC).
* Добавится `--authorization-webhook-config-file=/etc/kubernetes/authorization-webhook-config.yaml`.
* Добавится `volumeMounts`:

  ```yaml
  - name: authorization-webhook-config
    mountPath: /etc/kubernetes/authorization-webhook-config.yaml
    readOnly: true
  ```
* Добавится `volumes`:

  ```yaml
  - name:authorization-webhook-config
    hostPath:
      path: /etc/kubernetes/authorization-webhook-config.yaml
      type: FileOrCreate
  ```
</details>

## Кастомизация прав для предустановленных accessLevel

См. [документацию по разработке]({{ site.baseurl }}/modules/140-user-authz/development.html#кастомизация-прав-для-предустановленных-accesslevel).

## Как проверить, что у пользователя есть доступ?
Необходимо выполнить следующую команду, в которой будут указаны:
* `resourceAttributes` (как в RBAC) - к чему мы проверяем доступ
* `user` - имя пользователя
* `groups` - группы пользователя
 
P.S. при совместном использовании с модулем `user-authn` группы и имя пользователя можно посмотреть в логах dex'а `kubectl -n d8-user-authn logs -l app=dex` (видны только при авторизации)
```bash
cat  <<EOF | 2>&1 kubectl create -v=8 -f - | tail -2 \
  | grep "Response Body" | awk -F"Response Body:" '{print $2}' \
  | jq -rc .status
apiVersion: authorization.k8s.io/v1
kind: SubjectAccessReview
spec:
  resourceAttributes:
    namespace: d8-monitoring
    verb: get
    group: ""
    resource: "pods"
  user: "user@gmail.com"
  groups:
  - Everyone
  - Admins
EOF
```
В результате увидим, есть ли доступ и на основании какой роли:
```bash
{
  "allowed": true,
  "reason": "RBAC: allowed by ClusterRoleBinding \"user-authz:myuser:super-admin\" of ClusterRole \"user-authz:super-admin\" to User \"user@gmail.com\""
}
```
