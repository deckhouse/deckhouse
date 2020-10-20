---
title: FAQ
permalink: /features/core-faq.html
---

## Как найти документацию в кластере?

Документация запущенной в кластере версии Deckhouse доступна по адресу `deckhouse.<cluster_domain>`, где `<cluster_domain>` - DNS имя в соответствии с шаблоном из параметра `global.modules.publicDomainTemplate` конфигурации.

## Как установить желаемый канал обновлений?
Чтобы перейти на другой канал обновлений автоматически, минимизировав переключение версий в кластере, нужно у модуля изменить (установить) параметр `releaseChannel`.

Пример конфигурации модуля:
```yaml
deckhouse: |
  releaseChannel: RockSolid
```

## Как узнать все параметры Deckhouse?

Все ключевые настройки Deckhouse, включая параметры модулей, хранятся в ConfigMap `deckhouse` namespace `d8-system`. Посмотреть содержимое:
```
kubectl -n d8-system get cm deckhouse -o yaml
```