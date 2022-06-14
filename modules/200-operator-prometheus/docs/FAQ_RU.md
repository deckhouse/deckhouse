---
title: "Prometheus-operator: примеры конфигурации"
type:
  - instruction
---

## Установка еще одного prometheus-operator в кластер

Пользователю может понадобится установить в кластер еще один prometheus-operator,
чтобы добавить Prometheus'ы или alertmanager'ы в кластер.

1. Чтобы не пересекаться с prometheus-operator из Deckhouse, необходимо указать флаг
   `--deny-namespaces=d8-monitoring` для пользовательской инсталляции prometheus-operator.

2. Prometheus-operator из Deckhouse следит за ресурсами правил и мониторов только в пространствах имен
   с меткой `heritage: deckhouse`. Не устанавливайте эту метку на пользовательские пространства имен.
