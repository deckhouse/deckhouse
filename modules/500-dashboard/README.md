Модуль dashboard
=======

Модуль устанавливает [dashboard](https://github.com/kubernetes/dashboard).

Конфигурация
------------

### Что нужно настраивать?
Обязательных настроек нет.

### Настройка Gitlab если используем
Регистрируем в гитлаб новое приложение. Для этого идем Admin area -> Applications -> New application
Redirect URI(Callback url) устанавливаем вида https://dashboard.example.com/oauth2/callback

### Параметры
* `gitlabBaseUrl` — url gitlab для авторизации (https://git.example.com)
* `oauth2ProxyClientId`  — Application Id в Admin Area -> Applications gitlab
* `oauth2ProxyClientSecret` — Secret в Admin Area -> Applications gitlab 
* `oauth2ProxyCookieSecret` —  генерируется автоматически если есть gitlabBaseUrl.
* `password` — пароль для http авторизации, используется если не задан gitlabBaseUrl, генерируется автоматически.
* `nodeSelector` — как в Kubernetes в `spec.nodeSelector` у pod'ов.
    * Если ничего не указано — будет использоваться значение `{"node-role/system":""}` (если в кластере есть такие узлы) или ничего не будет указано.
    * Можно указать `false`, чтобы не добавлять никакой nodeSelector.
* `tolerations` — как в Kubernetes в `spec.tolerations` у pod'ов.
    * Если ничего не указано — будет использовано значение `[{"key":"node-role/system","operator":"Exists"}]` (если в кластере есть такие узлы) или ничего не будет указано.
    * Можно указать `false`, чтобы не добавлять никакие toleration'ы.

### Пример конфига
```yaml
dashboard:
  nodeSelector:
    node-role/other: ""
  tolerations:
  - key: node-role/other
    operator: Exists
```

Как пользоваться модулем?
-------------------------
Все настолько понятно и очевидно, на сколько это вообще может быть! Бери и используй.

