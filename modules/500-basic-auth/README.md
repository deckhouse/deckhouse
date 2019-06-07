Модуль basic-auth
=======

Модуль устанавливает сервис для базовой авторизации.

Конфигурация
------------

### Что нужно настраивать?
Обязательных настроек нет.
По умолчанию создается location `/` с пользователем `admin`.

### Параметры

* `nodeSelector` — как в Kubernetes в `spec.nodeSelector` у pod'ов.
    * Если ничего не указано — будет использоваться значение `{"node-role.flant.com/basic-auth":""}` или `{"node-role.flant.com/system":""}` (если в кластере есть такие узлы) или ничего не будет указано.
    * Можно указать `false`, чтобы не добавлять никакой nodeSelector.
* `tolerations` — как в Kubernetes в `spec.tolerations` у pod'ов.
    * Если ничего не указано — будет использовано значение Если ничего не указано — будет использовано значение `[{"key":"dedicated.flant.com","operator":"Equal","value":"basic-auth"},{"key":"dedicated.flant.com","operator":"Equal","value":"system"}]`. (если в кластере есть такие узлы) или ничего не будет указано.
    * Можно указать `false`, чтобы не добавлять никакие toleration'ы.
* `locations` — если нам необходимо создать несколько location'ов для разных приложений с разной авторизацией, то добавляем данный параметр.
    * `location` — это location, для которого будут определяться `whitelist` и `users`, в конфиге nginx `root` заменяется на `/`.
    * `whitelist` — список IP адресов и подсетей для которых разрешена авторизация без логина/пароля.
    * `users` — список пользователей в формате `username: "password"`.

### Пример конфигурации:

```yaml
basicAuth: |
  locations:
  - location: "/"
    whitelist:
      - 1.1.1.1
    users:
      username: "password"
  nodeSelector:
    node-role/example: ""
  tolerations:
  - key: dedicated
    operator: Equal
    value: example
```

### Использование
Просто добавляем подобную аннотацию к ингрессу:

`ingress.kubernetes.io/auth-url: "http://basic-auth.kube-basic-auth.svc.cluster.local/"`
