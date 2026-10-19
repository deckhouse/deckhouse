# Patches

## 002-openkruise-daemonset-apiversion.patch

Этот патч для корректной работы VPA с OpenKruise DaemonSet (apiVersion == apps.kruise.io/v1alpha1) в Дэкхаусе

## 003-recommender.patch

Этот патч не работает для хранилища Prometheus. Только для контрольных точек VPA.
Не имею представления, для чего он нужен.
Поскольку мы используем хранилище Prometheus, не буду перемещать этот патч.

## 004-daemonset-scope-node-label.patch

Добавляет scoped-рекомендации для DaemonSet с группировкой по ключу лейбла узла из `spec.scope`.

- Работает только для `targetRef.kind=DaemonSet` при непустом `spec.scope`.
- Использует значение лейбла узла как ключ группы рекомендаций для потока с Prometheus.
- Использует специальное значение `scopeValue="__absent__"`, если на узле нет ключа лейбла из `spec.scope`.
- Использует `status.groups` как источник истины для scoped-рекомендаций.
- Хранит grouped-рекомендации в compact-виде (`containerName` + `target`).
- Не дублирует `scope` внутри grouped `containerRecommendations`; consumer'ы вычисляют его из `spec.scope` + `group.scopeValue`.
- Делает поля взаимоисключающими:
  - `status.recommendation` используется только для обычных VPA (включая DaemonSet без `spec.scope`);
  - `status.groups` используется только для scoped DaemonSet, а `status.recommendation` не заполняется.
- Admission-controller/updater читают только актуальное поле статуса в зависимости от режима VPA.

## 005-admission-controller-disable-http2.patch

Webhook-сервер admission-controller обслуживает запросы только по HTTP/1.1 (`http.Server.Protocols` с одним HTTP/1).

- Порт webhook доступен любому поду через Service `vpa-webhook`, а клиентские сертификаты сервер не требует. С включённым HTTP/2 любой под мог бы использовать DoS-уязвимости уровня кадров HTTP/2, например CVE-2026-78669.
- kube-apiserver согласует HTTP/1.1 через ALPN и вызывает webhook как раньше.

## 006-admission-controller-dedicated-webhook-mux.patch

Webhook-сервер admission-controller обслуживает запросы через собственный `http.ServeMux` вместо `http.DefaultServeMux`.

- `pkg/utils/server` импортирует `net/http/pprof`, а его `init()` регистрирует `GET /debug/pprof/*` в `http.DefaultServeMux`. Webhook-сервер использовал этот mux, поэтому pprof на порту 8000 был доступен любому поду через Service `vpa-webhook` без аутентификации.
- Обработчики pprof разбирают формы запроса, поэтому уязвимости разбора multipart, например CVE-2026-94440, были доступны любому поду.
- Профилирование по-прежнему доступно на порту метрик с флагом `--profiling`.
