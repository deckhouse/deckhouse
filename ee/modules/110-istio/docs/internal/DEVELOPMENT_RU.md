---
title: Поддержка модуля istio
searchable: false
---

Оригинальный Istio поставляется в виде helm-чарта, который мы преобразовали в наш формат выкинув много лишнего и переделав их костыли на наши.

Каждый релиз Istio содержит:
* Бинарь istioctl с вкомпиленными helm-чартами и инструментами для деплоя. Он нужен для мануального деплоя (в случае Deckhouse это ни к чему) и для эксплуатации полезных утилит.
* Image с оператором, который также содержит бинарь с вкомпиленными helm-чартами, но он ещё и следит за CR `IstioOperator`. Можно считать, что это — аналог addon-operator-а.
* Набор image-й с компонентами Istio (istiod, proxyv2, ...).
* helm-чарты с компонентами Istio. Полезны для разбирательств, как работает оператор.

Как обновлять Istio
-------------------

Для добавления новой версии:
* Добавить images по аналогии с предыдущими версиями.
* Добавить новую версию в values.yaml (`istio.internal.supportedVersions`).
* Актуализировать crd-all.gen.yaml и crd-operator.yaml в папке crds.
* Освежить дашборды графаны:
  * Извлечь json-описания дашборд из манифеста samples/addons/grafana.yaml, отформатировать их с помощью jq и сложить в соответствующие json-ки в /monitoring/grafana-dashboards/istio/XXX.json.
  * Найти все range'и и заменить на `$__interval_sx4`:

    ```bash
    for dashboard in *.json; do
      for range in $(grep '\[[0-9]\+[a-z]\]' $dashboard | sed 's/.*\(\[[0-9][a-z]\]\).*/\1/g' | sort | uniq); do
        sed -e 's/\['$range'\]/[$__interval_sx4]/g' -i $dashboard
      done
    done
    ```

  * Заменить `irate` на `rate`:

    ```bash
    sed 's/irate(/rate(/g' -i *.json
    ```
  * Заменить `Resolution` на `1/1`:

    ```bash
    sed 's/"intervalFactor":\s[0-9]/"intervalFactor": 1/' -i *.json
    ```
  * Убрать `Min Step`:

    ```bash
    sed '/"interval":/d' -i *.json
    ```
  * Заменить все графики на `Staircase` (поломает графики `Stack` + `Percent`, которые придется поправить руками на `Bars`):

    ```bash
    sed 's/"steppedLine": false/"steppedLine": true/' -i *.json
    ```

  * Заменить все datasource на null:

    ```bash
    sed 's/"datasource": "Prometheus"/"datasource": null/' -i *.json
    ```
