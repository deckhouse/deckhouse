---
title: Автоматическое назначение лейблов и аннотаций пространствам имён
permalink: ru/admin/configuration/label-assignment.html
lang: ru
---

Вы можете автоматизировать назначение лейблов и аннотаций пространствам имён в кластере Deckhouse
на основе заданных шаблонов.
Это полезно, например, когда нужно автоматически включать новые пространства имён в мониторинг
без ручного редактирования каждого из них.

## Как это работает

- Все пространства имён, чьи названия совпадают с шаблонами в `includeNames` и не совпадают с шаблонами в `excludeNames`,
  получают указанные лейблы и аннотации.
- При изменении конфигурации лейблы и аннотации на существующих пространствах имён обновляются автоматически.
- Новые пространства имён, подходящие под условия шаблонов, также получают нужные лейблы и аннотации автоматически.

## Настройка автоматического назначения лейблов и аннотаций

Включите [модуль `namespace-configurator`](/modules/namespace-configurator/):

```shell  
d8 platform module enable namespace-configurator
```

Настройте автоматическое назначение лейблов и аннотаций в ModuleConfig [`namespace-configurator`](/modules/namespace-configurator/configuration.html):

1. Перечислите аннотации и лейблы, которые должны применяться к пространствам имён, в полях `settings.configurations.annotations` и `settings.configurations.labels` соответственно;
1. Укажите шаблоны названий пространств имён, к которым должны применяться настройки:
   - в поле `includeNames` перечислите регулярные выражения, соответствующие нужным названиям;
   - в поле `excludeNames` перечислите те, которые нужно исключить.

## Пример конфигурации

В следующем примере конфигурации настраивается автоматическое добавление лейбла `extended-monitoring.deckhouse.io/enabled=true` и аннотации `foo=bar` ко всем пространствам имён, названия которых начинаются с `prod-` или `infra-`, за исключением `infra-test`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: namespace-configurator
spec:
  version: 1
  enabled: true
  settings:
    configurations:
    - annotations:
        foo: bar
      labels:
        extended-monitoring.deckhouse.io/enabled: "true"
      includeNames:
      - "^prod"
      - "^infra"
      excludeNames:
      - "infra-test"
```
