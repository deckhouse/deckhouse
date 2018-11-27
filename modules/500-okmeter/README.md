Модуль okmeter
==========================

Данный модуль устанавливает okmeter (okmeter.io), как daemonset, в namespace kube-okmeter и удаляет ранее установленный вручную okmeter.

Конфигурация
------------

### Что нужно настраивать?

В конфигурацию antiopa необходимо добавить apiKey для модуля okmeter:

* `apiKey` - этот ключ можно взять на странице документации по установке okmeter для нужного проекта (`OKMETER_API_TOKEN`).

Пример:

```yaml
okmeter: |
  apiKey: 5ff9z2a3-9127-1sh4-2192-06a3fc6e13e3
```

