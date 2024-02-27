---
title: "Prometheus-operator: примеры конфигурации"
type:
  - instruction
---

## Установка дополнительного prometheus-operator в кластер

Пользователю может понадобиться установка дополнительного prometheus-operator в кластер (для добавления Prometheus или alertmanager).

1. Чтобы не пересекаться с prometheus-operator из Deckhouse, укажите флаг `--deny-namespaces=d8-monitoring` для пользовательской инсталляции prometheus-operator.

2. Prometheus-operator из Deckhouse должен следить за ресурсами правил и мониторов только в пространствах имен с меткой `heritage: deckhouse`. Не ставьте эту метку на пользовательские пространства имен.
