---
title: "Разработка Prometheus Rules"
sidebartitle: "Prometheus Rules"
---

{% raw %}

## Схема метрик

Exporter extended-monitoring экспортирует метрики в следующем формате:

```text
extended_monitoring_{0}_threshold{{namespace="{1}", threshold="{2}", {3}="{4}"}} {5}
```

0. Kind Kubernetes объекта в нижнем регистре;
1. Namespace, где находится Kubernetes объект. `None` для non-namespaced объектов;
2. Имя threshold аннотации;
3. Kind Kubernetes объекта в нижнем регистре. Дублируется для удобства работы с PromQL;
4. Имя Kubernetes объекта;
5. Значение, полученное из value аннотации или из стандартных значений, закрепленных в исходном коде экспортера.

## Добавление стандартных аннотаций и их значений

В файле [extended-monitoring.py](https://github.com/deckhouse/deckhouse/blob/main/modules/340-extended-monitoring/images/extended-monitoring/src/extended-monitoring.py) достаточно добавить в атрибут `default_annotations` в Annotated классе, соответствующий типу Kubernetes объекта, необходимые аннотации.
{% endraw %}
