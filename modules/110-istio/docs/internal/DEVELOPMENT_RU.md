---
title: Поддержка модуля istio
searchable: false
---

Оригинальный Istio поставляется в виде helm-чарта, который мы преобразовали в наш формат выкинув много лишнего и переделав их костыли на наши.

Каждый релиз Istio содержит:
* Исполняемый файл `istioctl` с встроенными helm-чартами и инструментами для деплоя. Он нужен для мануального деплоя (в случае Deckhouse это ни к чему) и для эксплуатации полезных утилит.
* Image с оператором, который также содержит исполняемый файл с встроенными helm-чартами, но он ещё и следит за custom resource `IstioOperator`. Можно считать, что это — аналог addon-operator-а. (Для версий ниже 1.27)
* Набор образов с компонентами Istio (istiod, proxyv2, ...).
* helm-чарты с компонентами Istio. Полезны для разбирательств, как работает оператор.

Как обновлять Istio
-------------------

Для добавления новой версии:
* Добавить images по аналогии с предыдущими версиями.
* Добавить новую версию в openapi/values.yaml (`istio.supportedVersions`) и поправить значение default у `globalVersion`.
* Актуализировать crd-all.gen.yaml и crd-operator.yaml в папке crds.
* Добавить injection data templates  в `files/{{revision}}/templates/sidecar-injection-config.yaml|sidecar-injection-values.yaml`, используя предыдущую версию конфигов, исправив согласно изменениям новой версии.
  * sidecar-injection-config.yaml должен содержать `data.config` для ConfigMap `sidecar-injector-{{ $revision }}`, соответствующей версии.
  * sidecar-injection-values.yaml должен содержать `data.values` для ConfigMap `sidecar-injector-{{ $revision }}`, соответствующей версии.
* Добавить injection шаблоны из upstream в `files/{{revision}}/static/gateway-injection-template.yaml|sidecar-injection-template.yaml`.
  * Скопировать файл из upsteam `charts/istiod/files/injection-template.yaml` в `files/{{revision}}/static/sidecar-injection-template.yaml`.
  * Скопировать файл из upsteam `charts/istiod/files/gateway-injection-template.yaml` в `files/{{revision}}/static/gateway-injection-template.yaml`
* Чтобы обновить istio-дашборды необходимо выполнить скрипт `istio-grafana-dashboard.sh` и сложить в полученные json-ки в `monitoring/grafana-dashboards/istio/`. Что делает скрипт:
  * Клонирует репозиторий istio с необходимой версией.
  * Заменяет `irate` на `rate`.
  * Заменяет `Resolution` на `1/1`.
  * Убирает `Min Step`.
  * Заменяет все графики на `Staircase` (может поломать графики `Stack` + `Percent`, которые придется поправить руками на `Bars`).
  * Заменяет все datasource на null.
  * Исправляет ссылки на дашборды.
