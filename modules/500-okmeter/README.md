Модуль okmeter
==============

Данный модуль устанавливает агент [okmeter](http://okmeter.io), как `daemonset` в namespace `d8-okmeter`, и удаляет ранее установленный вручную `okmeter`.

Конфигурация
------------

### Включение модуля

Модуль по-умолчанию **выключен**. Для включения добавьте в CM `deckhouse`:

```yaml
data:
  okmeterEnabled: "true"
```

### Что нужно настраивать?

В конфигурацию Deckhouse необходимо добавить `apiKey` для модуля `okmeter`:

* `apiKey` - этот ключ можно взять на странице документации по установке `okmeter` для нужного проекта (`OKMETER_API_TOKEN`).

Пример:

```yaml
okmeterEnabled: "true"
okmeter: |
  apiKey: 5ff9z2a3-9127-1sh4-2192-06a3fc6e13e3
```

