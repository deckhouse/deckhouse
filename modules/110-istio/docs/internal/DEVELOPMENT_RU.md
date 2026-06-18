---
title: Поддержка модуля istio
searchable: false
---

Оригинальный Istio поставляется в виде helm-чарта, который мы преобразовали в наш формат, выкинув лишнее и переделав их костыли на наши.

Каждый релиз Istio содержит:

* Исполняемый файл `istioctl` с встроенными helm-чартами (для Deckhouse не используется при деплое, полезен для утилит).
* Image с оператором и CR `IstioOperator` / `Istio` — **только для версий ниже 1.27.9** (`supportsOperator: true`).
* Набор образов с компонентами Istio (istiod, proxyv2, cni, …).
* helm-чарты upstream.

## Добавление новой версии Istio

### Общие шаги (любая версия)

1. Images в `images/` — по аналогии с предыдущим minor.
2. Версия в `oss.yaml`.
3. CRD в `_crds/istio/<major.minor>/`:
   * **С оператором:** `crd-all.gen.yaml`, `crd-operator.yaml`, для 1.25+ — Sail CRD.
   * **Без оператора:** только `crd-all.gen.yaml`.
4. **Без оператора:** `_rules_v-<major>-<minor>.tpl` + ветка в `_istiod_clusterroles.tpl`.
5. Grafana — [`istio-grafana-dashboards.sh`](istio-grafana-dashboards.sh).
6. **Без оператора:** каталог `files/<revision>/` — см. ниже.
7. [`template_tests/module_test.go`](../../template_tests/module_test.go).

---

## Версия без оператора: каталог `files/<revision>/`

Читать только если `supportsOperator: false`. Иначе inject через IOP/Istio CR ([`istios.yaml`](../../templates/control-plane/istios.yaml)).

`<revision>` = `versionMap.<ver>.revision` (1.27.x → `v1x27`).

### Что это

Deckhouse создаёт ConfigMap **`istio-sidecar-injector-<revision>`** ([`configmap-inject.yaml`](../../templates/control-plane/configmap-inject.yaml)) из каталога:

```none
files/<revision>/
├── static/   sidecar-injection-template.yaml, gateway-injection-template.yaml
└── templates/   sidecar-injection-values.yaml, sidecar-injection-config.yaml
        │                    │
        └─► data.values (JSON)   data.config (YAML)
```

Mesh (`istio-<revision>`) — в [`configmap-mesh.yaml`](../../templates/control-plane/configmap-mesh.yaml), **не** в `files/`.

Образец для копирования: **`files/v1x27/`**.

### Как добавить новую revision (пример 1.28)

**A. Модуль в целом** — images, oss, CRD, hooks, `_rules_v-1-28.tpl`, тесты (как в общих шагах выше).

**B. Каталог files/<revision>** — обычно 3 команды и всё

```bash
# 1. Клон нужного тега Istio
git clone --depth 1 --branch 1.28.0 <ISTIO_REPO.git> /tmp/istio
UP=/tmp/istio/manifests/charts/istio-control/istio-discovery

# 2. Скопировать предыдущую revision целиком
cp -r files/v1x27 files/v1x28

# 3. Заменить только upstream-тела шаблонов (без правок)
cp "$UP/files/injection-template.yaml"         files/v1x28/static/sidecar-injection-template.yaml
cp "$UP/files/gateway-injection-template.yaml" files/v1x28/static/gateway-injection-template.yaml
```

**На этом для большинства minor bump достаточно.**  
`templates/sidecar-injection-values.yaml` и `sidecar-injection-config.yaml` уже лежат в копии — **не трогаем**, если static не требует новых настроек.

**C. Control plane** (не `files/`) — istiod, webhooks, mesh: шаблоны в `templates/control-plane/`, env в [`deployment.yaml`](../../templates/control-plane/deployment.yaml).

### Когда править `templates/` (редко)

| Файл | Когда менять |
|------|----------------|
| `sidecar-injection-values.yaml` | Новый static-шаблон ссылается на `.Values.…`, которого нет в нашем JSON. Или осознанно меняется D8-логика (образы, CNI, sidecar ranges). |
| `sidecar-injection-config.yaml` | Upstream изменил `defaultTemplates`, селекторы inject, или нужны новые имена шаблонов. Блоки **`d8-*`** — только Deckhouse, обычно копируются как есть. |

**Как проверить, нужен ли values:** после `cp` static откройте новые `static/*.yaml`, поищите `.Values.` — если поле используется без запасного default в шаблоне, добавьте его в `sidecar-injection-values.yaml` (с D8-хелперами, как в `v1x27`).

**Как проверить config:** откройте upstream  
`$UP/templates/istiod-injector-configmap.yaml`, секция `config:` (до `templates:`) — сравните с верхом нашего `sidecar-injection-config.yaml`. Если Istio не менял policy/селекторы — не трогайте.

### Важно: не сравнивать наш JSON с upstream JSON

Upstream в `istiod-injector-configmap.yaml` собирает **широкий** `data.values` (почти весь `global` из chart values + `gateways` + `sidecarInjectorWebhook`).

У нас **узкий** JSON: только то, что нужно inject-шаблонам + настройки Deckhouse (registry, namespace `d8-istio`, CNI, …). Webhook-конфиг — в **`config`**, mesh — в **отдельном CM**.

Поэтому diff «upstream render» vs «наш render» на **одном и том же теге** всегда большой — **это нормально**, не признак ошибки и не чеклист для правок.

При bump смотрите:
1. **Diff двух static-файлов** (старый vs новый upstream) — что изменилось в inject.
2. **Новые `.Values.*` в static** — нужно ли дописать `values`.
3. **Тесты** — `go test` в `template_tests/`.

### Откуда что в upstream

Тег Istio → `manifests/charts/istio-control/istio-discovery/`:

| Нужно нам | Файл upstream |
|-----------|---------------|
| static sidecar | `files/injection-template.yaml` |
| static gateway | `files/gateway-injection-template.yaml` |
| справочно: что Istio кладёт в CM | `templates/istiod-injector-configmap.yaml` |

[`istios.yaml`](../../templates/control-plane/iop/istios.yaml) — как было с оператором; для operator-free **не копировать целиком**, только подсмотреть D8-логику.

### Чеклист `files/`

- [ ] `cp -r files/<prev> files/<revision>`
- [ ] `static/` — 2 файла из upstream as-is
- [ ] при необходимости — правки `templates/` (см. таблицу выше)
- [ ] `template_tests/module_test.go`

---

## Grafana-дашборды

Скрипт [`istio-grafana-dashboards.sh`](istio-grafana-dashboards.sh) → JSON в `monitoring/grafana-dashboards/istio/`:

* Клонирует Istio нужной версии.
* `irate` → `rate`, `Resolution` → `1/1`, убирает Min Step.
* Графики → Staircase (Stack+Percent при необходимости править вручную).
* datasource → null, правит ссылки на дашборды.
