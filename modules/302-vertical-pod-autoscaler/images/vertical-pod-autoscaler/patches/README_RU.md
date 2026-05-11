# Patches

## 002-openkruise-daemonset-apiversion.patch

Этот патч для корректной работы VPA с OpenKruise DaemonSet (apiVersion == apps.kruise.io/v1alpha1) в Дэкхаусе

## 003-recommender.patch

Этот патч не работает для хранилища Prometheus. Только для контрольных точек VPA.
Не имею представления, для чего он нужен.
Поскольку мы используем хранилище Prometheus, не буду перемещать этот патч.

## 004-daemonset-scope-node-label.patch

Добавляет scoped-рекомендации для DaemonSet с группировкой по ключу лейбла ноды из `spec.scope`.

- Работает только для `targetRef.kind=DaemonSet` при непустом `spec.scope`.
- Использует значение лейбла ноды как ключ группы рекомендаций для потока с Prometheus.
- Использует специальное значение `scopeValue="__absent__"`, если на ноде нет ключа лейбла из `spec.scope`.
- Использует `status.groups` как источник истины для scoped-рекомендаций.
- Хранит grouped-рекомендации в compact-виде (`containerName` + `target`).
- Не дублирует `scope` внутри grouped `containerRecommendations`; consumer-ы вычисляют его из `spec.scope` + `group.scopeValue`.
- Делает поля взаимоисключающими:
  - `status.recommendation` используется только для обычных VPA (включая DaemonSet без `spec.scope`);
  - `status.groups` используется только для scoped DaemonSet, а `status.recommendation` не заполняется.
- Admission-controller/updater читают только актуальное поле статуса в зависимости от режима VPA.
